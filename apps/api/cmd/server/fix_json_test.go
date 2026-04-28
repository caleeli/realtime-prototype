package main

import (
	"strings"
	"testing"
)

func TestFixComponentJSONRepairsEscapedFieldKeysAndStringContent(t *testing.T) {
	raw := `{"pug":".login-screen\n  b-form(@submit.prevent=\"onSubmit\")\n    b-form-group(label=\"Usuario\")\n      b-form-input(v-model="form.username" placeholder="Usuario")\n    b-form-group(label=\"Contraseña\")\n      b-form-input(type=\"password\" v-model=\"form.password\" placeholder=\"Contraseña\")\n    b-button(type=\"submit\" variant=\"primary\") Iniciar sesión","css":".login-screen { "max-width": 320px; margin: 0 auto; padding: 1rem; }","data":{}}`

	parsed, err := FixComponentJSON(raw)
	if err != nil {
		t.Fatalf("expected repair to succeed, got error: %v", err)
	}

	expectedPug := ".login-screen\n  b-form(@submit.prevent=\"onSubmit\")\n    b-form-group(label=\"Usuario\")\n      b-form-input(v-model=\"form.username\" placeholder=\"Usuario\")\n    b-form-group(label=\"Contraseña\")\n      b-form-input(type=\"password\" v-model=\"form.password\" placeholder=\"Contraseña\")\n    b-button(type=\"submit\" variant=\"primary\") Iniciar sesión"
	expectedCss := ".login-screen { \"max-width\": 320px; margin: 0 auto; padding: 1rem; }"

	pug, ok := parsed["pug"].(string)
	if !ok {
		t.Fatalf(`expected "pug" as string, got %T`, parsed["pug"])
	}
	css, ok := parsed["css"].(string)
	if !ok {
		t.Fatalf(`expected "css" as string, got %T`, parsed["css"])
	}

	if strings.TrimSpace(pug) != expectedPug {
		t.Fatalf("unexpected pug output:\n got: %q\n want: %q", strings.TrimSpace(pug), expectedPug)
	}
	if strings.TrimSpace(css) != expectedCss {
		t.Fatalf("unexpected css output:\n got: %q\n want: %q", strings.TrimSpace(css), expectedCss)
	}

	if !strings.Contains(pug, "b-form") {
		t.Fatalf("unexpected pug output: %q", pug)
	}

	if !strings.Contains(css, ".login-screen") {
		t.Fatalf("unexpected css output: %q", css)
	}

	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatalf(`expected "data" as map, got %T`, parsed["data"])
	}
	if len(data) != 0 {
		t.Fatalf(`expected empty data object, got: %#v`, data)
	}
}

func TestParseGenerationJSONCandidateFromRepairedOutput(t *testing.T) {
	raw := `{"pug":"div.login-screen\n  b-form-input(v-model=\"user\")","css":".login-screen { \"width\": 100%; }","data":{"form":{"value":"x"}}}`

	var out generationResponse
	err := parseGenerationJSONCandidate(raw, &out)
	if err != nil {
		t.Fatalf("expected repaired payload to parse, got error: %v", err)
	}

	expectedPug := "div.login-screen\n  b-form-input(v-model=\"user\")"
	expectedCss := ".login-screen { \"width\": 100%; }"

	if got := strings.TrimSpace(out.Pug); got != expectedPug {
		t.Fatalf("unexpected pug output:\n got: %q\n want: %q", got, expectedPug)
	}
	if got := strings.TrimSpace(out.Css); got != expectedCss {
		t.Fatalf("unexpected css output:\n got: %q\n want: %q", got, expectedCss)
	}
	if got := strings.TrimSpace(out.Css); !strings.Contains(got, "\"width\"") {
		t.Fatalf("expected escaped property name inside css, got %q", got)
	}
	if out.Data == nil {
		t.Fatal("expected data in generated response")
	}
	if _, ok := out.Data.(map[string]interface{}); !ok {
		t.Fatalf("expected data as map in generated response, got %T", out.Data)
	}
}
