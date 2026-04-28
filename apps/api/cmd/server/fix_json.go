package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

func FixComponentJSON(input string) (map[string]any, error) {
	source := stripCodeFence(strings.TrimSpace(input))
	source = normalizeEscapedObjectKeys(source)

	var parsed map[string]any

	if err := json.Unmarshal([]byte(source), &parsed); err == nil {
		if err := validateGenerationShape(parsed); err != nil {
			return nil, err
		}

		return parsed, nil
	}

	repaired, err := repairKnownStringFields(source, []string{"pug", "css"})
	if err != nil {
		return nil, err
	}

	parsed = nil

	if err := json.Unmarshal([]byte(repaired), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse repaired JSON: %w\nrepaired JSON: %s", err, repaired)
	}

	if err := validateGenerationShape(parsed); err != nil {
		return nil, err
	}

	return parsed, nil
}

func normalizeEscapedObjectKeys(input string) string {
	escapedStartPattern := regexp.MustCompile(`(^|[,{]\s*)\\+"(pug|css|data)"\\*:`)
	input = escapedStartPattern.ReplaceAllString(input, `$1"$2":`)

	keyPattern := regexp.MustCompile(`(^|[,{]\s*)"(pug|css|data)\\+:`)
	input = keyPattern.ReplaceAllString(input, `$1"$2":`)

	valueOpenPattern := regexp.MustCompile(`(("(?:pug|css|data)"\s*:\s*))\\+"`)
	input = valueOpenPattern.ReplaceAllString(input, `$1"`)

	fieldClosePattern := regexp.MustCompile(`\\+"\s*(,\s*"(?:pug|css|data)"\s*:)`)
	input = fieldClosePattern.ReplaceAllString(input, `",$1`)

	lastValuePattern := regexp.MustCompile(`\\+"\s*}`)
	input = lastValuePattern.ReplaceAllString(input, `"}`)

	return input
}

type matchedField struct {
	name      string
	prefixEnd int
}

type boundary struct {
	key       string
	valueEnd  int
	nextStart int
}

func repairKnownStringFields(input string, fields []string) (string, error) {
	var result strings.Builder

	i := 0

	for i < len(input) {
		field, ok := matchFieldAt(input, i, fields)

		if !ok {
			result.WriteByte(input[i])
			i++
			continue
		}

		result.WriteString(input[i:field.prefixEnd])

		b, ok := findFieldValueBoundary(input, field.prefixEnd, field.name)
		if !ok {
			return "", fmt.Errorf("could not safely find end of %q", field.name)
		}

		rawValue := input[field.prefixEnd:b.valueEnd]
		result.WriteString(escapeJSONStringContent(rawValue))

		result.WriteString(input[b.valueEnd : b.nextStart])
		i = b.nextStart
	}

	return result.String(), nil
}

func matchFieldAt(input string, index int, fields []string) (matchedField, bool) {
	remaining := input[index:]

	for _, field := range fields {
		pattern := regexp.MustCompile(`^"` + regexp.QuoteMeta(field) + `"\s*:\s*"`)

		match := pattern.FindStringIndex(remaining)

		if match != nil && match[0] == 0 {
			return matchedField{
				name:      field,
				prefixEnd: index + match[1],
			}, true
		}
	}

	return matchedField{}, false
}

func findFieldValueBoundary(input string, valueStart int, currentField string) (boundary, bool) {
	allPossibleNextKeys := []string{"pug", "css", "data"}

	var candidates []boundary

	for _, key := range allPossibleNextKeys {
		if key == currentField {
			continue
		}

		pattern := regexp.MustCompile(`,\s*"` + regexp.QuoteMeta(key) + `"\s*:`)
		matches := pattern.FindAllStringIndex(input[valueStart:], -1)

		for _, match := range matches {
			commaIndex := valueStart + match[0]

			valueEnd := commaIndex - 1

			for valueEnd >= valueStart && unicode.IsSpace(rune(input[valueEnd])) {
				valueEnd--
			}

			if valueEnd < valueStart || input[valueEnd] != '"' {
				continue
			}

			candidates = append(candidates, boundary{
				key:       key,
				valueEnd:  valueEnd,
				nextStart: commaIndex + 1,
			})

			break
		}
	}

	if len(candidates) == 0 {
		return boundary{}, false
	}

	best := candidates[0]

	for _, candidate := range candidates[1:] {
		if candidate.valueEnd < best.valueEnd {
			best = candidate
		}
	}

	return best, true
}

func escapeJSONStringContent(value string) string {
	var out strings.Builder

	for i := 0; i < len(value); i++ {
		ch := value[i]

		switch ch {
		case '"':
			out.WriteString(`\"`)

		case '\\':
			if i+1 < len(value) {
				next := value[i+1]

				if isValidSimpleJSONEscape(next) {
					out.WriteByte('\\')
					out.WriteByte(next)
					i++
					continue
				}

				if next == 'u' && i+5 < len(value) && isHex4(value[i+2:i+6]) {
					out.WriteString(value[i : i+6])
					i += 5
					continue
				}
			}

			out.WriteString(`\\`)

		case '\n':
			out.WriteString(`\n`)

		case '\r':
			out.WriteString(`\r`)

		case '\t':
			out.WriteString(`\t`)

		case '\b':
			out.WriteString(`\b`)

		case '\f':
			out.WriteString(`\f`)

		default:
			if ch < 0x20 {
				out.WriteString(fmt.Sprintf(`\u%04x`, ch))
			} else {
				out.WriteByte(ch)
			}
		}
	}

	return out.String()
}

func isValidSimpleJSONEscape(ch byte) bool {
	switch ch {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return true
	default:
		return false
	}
}

func isHex4(s string) bool {
	if len(s) != 4 {
		return false
	}

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if !((ch >= '0' && ch <= '9') ||
			(ch >= 'a' && ch <= 'f') ||
			(ch >= 'A' && ch <= 'F')) {
			return false
		}
	}

	return true
}

func validateGenerationShape(obj map[string]any) error {
	pug, ok := obj["pug"]

	if !ok {
		return errors.New(`missing "pug"`)
	}

	if _, ok := pug.(string); !ok {
		return errors.New(`"pug" must be a string`)
	}

	css, ok := obj["css"]

	if !ok {
		return errors.New(`missing "css"`)
	}

	if _, ok := css.(string); !ok {
		return errors.New(`"css" must be a string`)
	}

	data, ok := obj["data"]

	if !ok {
		return errors.New(`missing "data"`)
	}

	if _, ok := data.(map[string]any); !ok {
		return errors.New(`"data" must be an object`)
	}

	return nil
}

func stripCodeFence(text string) string {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")

		if len(lines) >= 2 {
			lines = lines[1:]
		}

		text = strings.Join(lines, "\n")
	}

	text = strings.TrimSpace(text)

	if strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(text, "```")
	}

	return strings.TrimSpace(text)
}

func parseGenerationJSONCandidate(raw string, output *generationResponse) error {
	fixed, err := FixComponentJSON(raw)
	if err != nil {
		return fmt.Errorf("invalid generated JSON format: %w", err)
	}

	pug, ok := fixed["pug"].(string)
	if !ok {
		return errors.New(`"pug" must be a string`)
	}

	css, ok := fixed["css"].(string)
	if !ok {
		return errors.New(`"css" must be a string`)
	}

	data, ok := fixed["data"]
	if !ok {
		return errors.New(`missing "data"`)
	}

	if _, ok := data.(map[string]any); !ok {
		return errors.New(`"data" must be an object`)
	}

	*output = generationResponse{
		Pug:  strings.TrimSpace(pug),
		Css:  strings.TrimSpace(css),
		Data: data,
	}

	return nil
}
