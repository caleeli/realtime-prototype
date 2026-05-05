import type { ComponentInventoryItem } from '../../../packages/component-registry/src/types';

import { ComponentCatalogClient } from './componentRegistryApi';

export interface GenerationContext {
  locale?: string;
  theme?: string;
  enabledPacks?: string[];
  targetDensity?: string;
}

export interface GenerationMessage {
  role: 'system' | 'user' | 'assistant';
  content: string;
}

export interface GenerationRequest {
  prompt: string;
  context?: GenerationContext;
  messages?: GenerationMessage[];
}

export interface InspirationRequest extends GenerationRequest {
  imagePrompt?: string;
  imageModel?: string;
  imageSize?: string;
  imageQuality?: string;
  imageStyle?: string;
  visionModel?: string;
  conversionNotes?: string;
}

interface BackendGenerationPayload {
  prompt?: string;
  pug?: string;
  css?: string;
  data?: unknown;
  messages?: unknown[];
}

export interface PugTreeTextNode {
  readonly type: 'text';
  readonly line: number;
  readonly indent: number;
  readonly text: string;
}

export interface PugTreeExpressionNode {
  readonly type: 'expression';
  readonly line: number;
  readonly indent: number;
  readonly expression: string;
}

export interface PugTreeElementNode {
  readonly type: 'element';
  readonly line: number;
  readonly indent: number;
  readonly tag: string;
  readonly attributes: Record<string, string | number | boolean>;
  readonly children: PugTemplateNode[];
}

export type PugTemplateNode = PugTreeElementNode | PugTreeTextNode | PugTreeExpressionNode;

export interface PugTemplateTree {
  readonly type: 'root';
  readonly children: PugTemplateNode[];
}

export interface PipelineImport {
  readonly tag: string;
  readonly localName: string;
  readonly source: string | null;
  readonly pack?: string;
  readonly isCatalogResolved: boolean;
}

export interface GenerationPipelineResult {
  readonly template: PugTemplateTree;
  readonly imports: PipelineImport[];
  readonly css: string;
  readonly data: unknown;
  readonly sourcePug: string;
  readonly messages: GenerationMessage[];
  readonly metadata: {
    readonly usedTags: string[];
    readonly nonBootstrapTags: string[];
    readonly unresolvedTags: string[];
  };
}

export type UXEvaluatorResultLine = string;

export interface UXEvaluatorRequest {
  readonly pug: string;
  readonly css: string;
  readonly data?: unknown;
}

export class GenerationServiceError extends Error {
  constructor(
    message: string,
    public readonly status?: number,
    public readonly body?: string,
  ) {
    super(message);
    this.name = 'GenerationServiceError';
  }
}

const BASE_ENDPOINT = '/api/generation';
const BASE_INSPIRATION_ENDPOINT = '/inspiration';
const UX_EVALUATOR_ENDPOINT = '/ux-evaluator';
const DEFAULT_GENERATION_TIMEOUT_MS = readFrontendTimeoutEnv('VITE_GENERATION_TIMEOUT_MS', 30000);
const DEFAULT_INSPIRATION_TIMEOUT_MS = readFrontendTimeoutEnv('VITE_INSPIRATION_TIMEOUT_MS', 120000);
const DEFAULT_EVALUATOR_TIMEOUT_MS = readFrontendTimeoutEnv('VITE_EVALUATOR_TIMEOUT_MS', 30000);

const DEFAULT_BOOTSTRAP_VUE_TAGS = new Set<string>([
  'b-alert',
  'b-avatar',
  'b-badge',
  'b-breadcrumb',
  'b-breadcrumb-item',
  'b-button',
  'b-button-group',
  'b-card',
  'b-card-body',
  'b-card-footer',
  'b-card-group',
  'b-card-sub-title',
  'b-card-text',
  'b-card-title',
  'b-col',
  'b-collapse',
  'b-container',
  'b-dropdown',
  'b-dropdown-divider',
  'b-dropdown-item',
  'b-dropdown-item-button',
  'b-dropdown-text',
  'b-form',
  'b-form-checkbox',
  'b-form-input',
  'b-form-select',
  'b-form-textarea',
  'b-form-group',
  'b-form-text',
  'b-input-group',
  'b-list-group',
  'b-list-group-item',
  'b-modal',
  'b-overlay',
  'b-pagination',
  'b-row',
  'b-spinner',
  'b-table',
  'b-tabs',
  'b-tab',
  'b-toast',
  'b-tooltip',
  'b-nav',
  'b-nav-item',
  'b-navbar',
  'b-navbar-brand',
  'b-sidebar',
  'b-offcanvas',
  'b-badge-pill',
  'b-container',
  'b-form-radio',
  'b-form-switch',
  'b-form-rating',
  'b-breadcrumb',
  'b-img-lazy',
  'b-button-toolbar',
  'b-placeholder',
]);

const DEFAULT_BOOTSTRAP_VUE_TAGS_PASCAL = new Set<string>(
  Array.from(DEFAULT_BOOTSTRAP_VUE_TAGS).map((tag) => {
    const parts = tag.split('-').filter(Boolean);
    return parts.map((value) => value.charAt(0).toUpperCase() + value.slice(1)).join('');
  }),
);

const KNOWN_HTML_TAGS = new Set<string>([
  'a',
  'abbr',
  'address',
  'area',
  'article',
  'aside',
  'audio',
  'b',
  'base',
  'bdi',
  'bdo',
  'blockquote',
  'body',
  'br',
  'button',
  'canvas',
  'caption',
  'cite',
  'code',
  'col',
  'colgroup',
  'data',
  'datalist',
  'dd',
  'del',
  'details',
  'dfn',
  'dialog',
  'div',
  'dl',
  'dt',
  'em',
  'embed',
  'fieldset',
  'figcaption',
  'figure',
  'footer',
  'form',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'header',
  'hgroup',
  'hr',
  'html',
  'i',
  'iframe',
  'img',
  'input',
  'ins',
  'label',
  'li',
  'main',
  'map',
  'mark',
  'menu',
  'meta',
  'meter',
  'nav',
  'ol',
  'option',
  'p',
  'path',
  'pre',
  'progress',
  'q',
  'section',
  'select',
  'small',
  'source',
  'span',
  'strong',
  'sub',
  'summary',
  'table',
  'tbody',
  'td',
  'textarea',
  'tfoot',
  'th',
  'thead',
  'time',
  'title',
  'tr',
  'track',
  'ul',
  'var',
  'video',
]);

interface ParsedLine {
  readonly line: number;
  readonly indent: number;
  readonly content: string;
}

interface ParseResult {
  readonly type: 'element' | 'text' | 'expression';
  readonly indent: number;
  readonly line: number;
  readonly node: PugTemplateNode;
}

interface ParsedAttributeResult {
  readonly attrs: Record<string, string | number | boolean>;
  readonly looseText: string[];
}

export interface GenerationPipelineServiceOptions {
  baseUrl?: string;
  componentCatalogClient?: ComponentCatalogClient;
  bootstrapVueTags?: Iterable<string>;
  bootstrapVueTagsPascal?: Iterable<string>;
  generationTimeoutMs?: number;
  inspirationTimeoutMs?: number;
  evaluatorTimeoutMs?: number;
}

function readFrontendTimeoutEnv(envKey: string, fallback: number): number {
  const envValue = (import.meta.env as Record<string, string | undefined>)[envKey]?.trim();
  if (!envValue) {
    return fallback;
  }
  const parsed = Number.parseInt(envValue, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
}

function buildHeaders(): Record<string, string> {
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  };
}

function normalizeTag(tag: string): string {
  return tag.trim().toLowerCase();
}

function toPascalTag(tag: string): string {
  if (!tag.trim()) {
    return '';
  }

  if (tag.includes('-')) {
    return tag
      .split('-')
      .filter(Boolean)
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
      .join('');
  }

  return tag.charAt(0).toUpperCase() + tag.slice(1);
}

function tokenizePugLines(pug: string): ParsedLine[] {
  const content = (pug ?? '').replace(/\r\n?/g, '\n');
  const lines = content.split('\n');

  const parsed: ParsedLine[] = [];
  let lineNumber = 1;

  for (const rawLine of lines) {
    const lineWithoutTrailing = rawLine.replace(/[\u200B-\u200D\uFEFF]/g, '');
    const rawLength = lineWithoutTrailing.length;
    let i = 0;
    while (i < rawLength && (lineWithoutTrailing[i] === ' ' || lineWithoutTrailing[i] === '\t')) {
      i += 1;
    }

    const line = lineWithoutTrailing.trimEnd();
    if (line.trim().length > 0) {
      parsed.push({
        line: lineNumber,
        indent: i,
        content: line.trimEnd().slice(i),
      });
    }
    lineNumber += 1;
  }

  return parsed;
}

function splitTopLevel(input: string, separator: string): string[] {
  const tokens: string[] = [];
  let token = '';
  let braces = 0;
  let brackets = 0;
  let parens = 0;
  let inSingleQuote = false;
  let inDoubleQuote = false;
  let inTemplate = false;
  let escaped = false;

  const flush = () => {
    const trimmed = token.trim();
    if (trimmed.length > 0) {
      tokens.push(trimmed);
    }
    token = '';
  };

  for (const char of input) {
    if (escaped) {
      token += char;
      escaped = false;
      continue;
    }

    if (char === '\\') {
      token += char;
      escaped = true;
      continue;
    }

    if (char === "'" && !inDoubleQuote && !inTemplate) {
      inSingleQuote = !inSingleQuote;
      token += char;
      continue;
    }

    if (char === '"' && !inSingleQuote && !inTemplate) {
      inDoubleQuote = !inDoubleQuote;
      token += char;
      continue;
    }

    if (char === '`' && !inSingleQuote && !inDoubleQuote) {
      inTemplate = !inTemplate;
      token += char;
      continue;
    }

    if (!inSingleQuote && !inDoubleQuote && !inTemplate) {
      if (char === '{') {
        braces += 1;
      } else if (char === '}') {
        braces = Math.max(0, braces - 1);
      } else if (char === '[') {
        brackets += 1;
      } else if (char === ']') {
        brackets = Math.max(0, brackets - 1);
      } else if (char === '(') {
        parens += 1;
      } else if (char === ')') {
        parens = Math.max(0, parens - 1);
      }

      if (char === separator && braces === 0 && brackets === 0 && parens === 0) {
        flush();
        continue;
      }
    }

    token += char;
  }

  flush();
  return tokens;
}

function parseAttributeValue(value: string): string | number | boolean {
  const trimmed = value.trim();
  if (trimmed.length === 0) {
    return '';
  }
  if (trimmed === 'true') {
    return true;
  }
  if (trimmed === 'false') {
    return false;
  }
  if (trimmed === 'null') {
    return 'null';
  }
  if (/^-?\d+(\.\d+)?$/.test(trimmed)) {
    return Number(trimmed);
  }
  if ((trimmed.startsWith("'") && trimmed.endsWith("'")) || (trimmed.startsWith('"') && trimmed.endsWith('"'))) {
    return trimmed.slice(1, -1);
  }
  if ((trimmed.startsWith('`') && trimmed.endsWith('`')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    return trimmed;
  }
  return trimmed;
}

function parseAttributes(raw: string): ParsedAttributeResult {
  const attrs: Record<string, string | number | boolean> = {};
  const looseText: string[] = [];

  const pairs = splitTopLevel(raw, ',');
  for (const pairRaw of pairs) {
    const pairValue = pairRaw.trim();
    if (!pairValue) {
      continue;
    }

    const tokens = splitTopLevel(pairValue, ' ');
    for (const tokenRaw of tokens) {
      const token = tokenRaw.trim();
      if (!token) {
        continue;
      }

      const idx = token.indexOf('=');
      if (idx === -1) {
      if (/^[#]?[a-zA-Z_:][\w:.-]*$/.test(token) || token.startsWith('#')) {
          attrs[token] = true;
        } else {
          looseText.push(token);
        }
        continue;
      }

      const key = token.slice(0, idx).trim();
      const value = token.slice(idx + 1).trim();
      if (key.length === 0) {
        continue;
      }

      attrs[key] = parseAttributeValue(value);
    }
  }

  return { attrs, looseText };
}

function addLooseAttributeAsChild(line: ParsedLine, token: string, children: PugTemplateNode[]): void {
  const trimmed = token.trim();
  if (!trimmed) {
    return;
  }

  const interpolation = trimmed.match(/^#\{(.+)\}$/);
  if (interpolation) {
    children.push({
      type: 'expression',
      line: line.line,
      indent: line.indent + 2,
      expression: interpolation[1].trim(),
    });
    return;
  }

  const parsed = parseAttributeValue(trimmed);
  const text = typeof parsed === 'boolean' ? (parsed ? 'true' : 'false') : parsed === null ? '' : String(parsed);
  children.push({
    type: 'text',
    line: line.line,
    indent: line.indent + 2,
    text,
  });
}

function mergeClass(values: string, attrs: Record<string, string | number | boolean>) {
  if (!('class' in attrs)) {
    attrs.class = values;
    return;
  }

  const existing = String(attrs.class).trim();
  attrs.class = existing.length === 0 ? values : `${existing} ${values}`;
}

function parseTagLine(
  line: ParsedLine,
): { tag: string; attrs: Record<string, string | number | boolean>; text: string; looseText: string[] } {
  const lineContent = line.content.trim();
  let cursor = 0;

  const consume = (pattern: RegExp) => {
    const sliced = lineContent.slice(cursor);
    const match = sliced.match(pattern);
    if (!match) {
      return null;
    }
    cursor += match[0].length;
    return match[1] ?? match[0];
  };

  let tag = '';

  if (lineContent[cursor] === '.' || lineContent[cursor] === '#') {
    tag = 'div';
  } else {
    const parsedTag = consume(/^[A-Za-z][\w-]*/);
    tag = parsedTag ?? '';
  }

  const attrs: Record<string, string | number | boolean> = {};
  let looseText: string[] = [];
  let classText = '';
  let idText = '';

  while (cursor < lineContent.length) {
    const char = lineContent[cursor];

    if (char === '.') {
      const next = consume(/^\.[\w-]+/);
      if (next) {
        const className = next.slice(1);
        classText = classText.length === 0 ? className : `${classText} ${className}`;
      }
      continue;
    }

    if (char === '#') {
      const next = consume(/^#[\w-]+/);
      if (next) {
        idText = next.slice(1);
        attrs.id = idText;
      }
      continue;
    }

    if (char === '(') {
      let level = 0;
      let i = cursor;
      let inSingleQuote = false;
      let inDoubleQuote = false;
      let inTemplate = false;
      let escape = false;
      let closed = false;

      for (; i < lineContent.length; i += 1) {
        const current = lineContent[i];
        if (escape) {
          escape = false;
          continue;
        }
        if (current === '\\') {
          escape = true;
          continue;
        }
        if (current === "'" && !inDoubleQuote && !inTemplate) {
          inSingleQuote = !inSingleQuote;
        } else if (current === '"' && !inSingleQuote && !inTemplate) {
          inDoubleQuote = !inDoubleQuote;
        } else if (current === '`' && !inSingleQuote && !inDoubleQuote) {
          inTemplate = !inTemplate;
        }

        if (!inSingleQuote && !inDoubleQuote && !inTemplate) {
          if (current === '(') {
            level += 1;
          } else if (current === ')') {
            level -= 1;
            if (level === 0) {
              closed = true;
              break;
            }
          }
        }
      }

      const inside = lineContent.slice(cursor + 1, i);
      const parsed = parseAttributes(inside);
      Object.assign(attrs, parsed.attrs);
      looseText.push(...parsed.looseText);
      if (closed) {
        cursor = i + 1;
      } else {
        cursor = i + 1;
      }
      continue;
    }

    break;
  }

  if (classText.length > 0) {
    mergeClass(classText, attrs);
  }

  const remaining = lineContent.slice(cursor).trim();
  return { tag: tag || 'div', attrs, text: remaining, looseText };
}

function parsePugLine(line: ParsedLine): ParseResult | null {
  const lineContent = line.content.trim();
  if (!lineContent.length) {
    return null;
  }

  if (lineContent.startsWith('//')) {
    return null;
  }

  if (lineContent.startsWith('|')) {
    return {
      type: 'text',
      line: line.line,
      indent: line.indent,
      node: {
        type: 'text',
        line: line.line,
        indent: line.indent,
        text: lineContent.slice(1).trim(),
      },
    };
  }

  if (lineContent.startsWith('=')) {
    return {
      type: 'expression',
      line: line.line,
      indent: line.indent,
      node: {
        type: 'expression',
        line: line.line,
        indent: line.indent,
        expression: lineContent.slice(1).trim(),
      },
    };
  }

  if (lineContent.startsWith('-')) {
    return null;
  }

  if (/^[\.#][\w-]/.test(lineContent) || /^[A-Za-z]/.test(lineContent)) {
    const parsedLine = parseTagLine(line);
    const nodeChildren: PugTemplateNode[] = [];

    const lineNode: PugTreeElementNode = {
      type: 'element',
      line: line.line,
      indent: line.indent,
      tag: parsedLine.tag,
      attributes: parsedLine.attrs,
      children: nodeChildren,
    };

    if (parsedLine.text.length > 0 && !parsedLine.text.startsWith('=') && parsedLine.text !== '|') {
      nodeChildren.push({
        type: 'text',
        line: line.line,
        indent: line.indent + 2,
        text: parsedLine.text.replace(/^\s+/, ''),
      });
    }

    for (const token of parsedLine.looseText) {
      addLooseAttributeAsChild(line, token, nodeChildren);
    }

    return {
      type: 'element',
      line: line.line,
      indent: line.indent,
      node: lineNode,
    };
  }

  return null;
}

function parsePugToHierarchy(pug: string): PugTemplateTree {
  const root: PugTemplateTree = { type: 'root', children: [] };
  const stack: Array<{ indent: number; children: PugTemplateNode[] }> = [{ indent: -1, children: root.children }];

  const lines = tokenizePugLines(pug);

  for (const line of lines) {
    const parsed = parsePugLine(line);
    if (!parsed) {
      continue;
    }

    while (stack.length > 1 && parsed.indent <= stack[stack.length - 1].indent) {
      stack.pop();
    }

    stack[stack.length - 1].children.push(parsed.node);
    if (parsed.type === 'element') {
      stack.push({
        indent: parsed.indent,
        children: (parsed.node as PugTreeElementNode).children,
      });
    }
  }

  return root;
}

function traverseTags(node: PugTemplateNode, visit: (tag: string) => void) {
  if (node.type === 'element') {
    visit(node.tag);
    for (const child of node.children) {
      traverseTags(child, visit);
    }
    return;
  }
}

function extractUsedTags(template: PugTemplateTree): string[] {
  const tags = new Set<string>();
  for (const child of template.children) {
    traverseTags(child, (tag) => {
      tags.add(tag);
    });
  }

  return Array.from(tags);
}

function isBootstrapVueTag(tag: string, allowedBootstrapSet: Set<string>, allowedBootstrapPascal: Set<string>): boolean {
  const normalized = normalizeTag(tag);
  if (KNOWN_HTML_TAGS.has(normalized)) {
    return true;
  }
  if (allowedBootstrapSet.has(normalized)) {
    return true;
  }
  if (normalized.startsWith('b-')) {
    return true;
  }
  if (normalized === '') {
    return true;
  }
  if (allowedBootstrapPascal.has(tag) || allowedBootstrapPascal.has(toPascalTag(tag))) {
    return true;
  }
  return false;
}

function resolveImports(
  template: PugTemplateTree,
  componentInventory: ComponentInventoryItem[],
  allowedBootstrapSet: Set<string>,
  allowedBootstrapPascal: Set<string>,
): { imports: PipelineImport[]; nonBootstrapTags: string[]; unresolvedTags: string[] } {
  const usedTags = extractUsedTags(template);
  const importMap = new Map<string, PipelineImport>();
  const unresolved = new Set<string>();
  const nonBootstrapTags = new Set<string>();

  for (const tag of usedTags) {
    if (isBootstrapVueTag(tag, allowedBootstrapSet, allowedBootstrapPascal)) {
      continue;
    }

    nonBootstrapTags.add(tag);

    const resolved = componentInventory.find(
      (item) =>
        item.enabled &&
        (item.tag === tag || item.tag === toPascalTag(tag) || item.name === tag || item.name === toPascalTag(tag)),
    );
    if (resolved) {
      importMap.set(resolved.tag, {
        tag: resolved.tag,
        localName: resolved.name,
        source: resolved.module,
        pack: resolved.pack,
        isCatalogResolved: true,
      });
      continue;
    }

    unresolved.add(tag);
    importMap.set(tag, {
      tag,
      localName: tag,
      source: null,
      isCatalogResolved: false,
    });
  }

  return { imports: Array.from(importMap.values()), nonBootstrapTags: Array.from(nonBootstrapTags), unresolvedTags: Array.from(unresolved) };
}

function safeParseJSON(text: string): unknown {
  const cleaned = text.trim();
  if (!cleaned) {
    throw new Error('Backend response is empty');
  }

  const withoutFence = cleaned.replace(/```json|```/g, '').trim();

  try {
    return JSON.parse(withoutFence);
  } catch (error) {
    const firstBrace = withoutFence.indexOf('{');
    const lastBrace = withoutFence.lastIndexOf('}');
    if (firstBrace === -1 || lastBrace === -1 || lastBrace <= firstBrace) {
      throw error instanceof Error ? error : new Error('Invalid JSON response');
    }

    const candidate = withoutFence.slice(firstBrace, lastBrace + 1);
    return JSON.parse(candidate);
  }
}

function normalizeUxEvaluationText(raw: string): UXEvaluatorResultLine[] {
  const normalized = raw.replace(/\r\n/g, '\n').trim();
  if (!normalized) {
    return [];
  }

  const lines = normalized
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0)
    .map((line) => line.replace(/^\s*(?:\d+[.)]|\*|[-•])\s*/u, '').trim())
    .filter((line) => line.length > 0);

  if (lines.length === 1 && /^No issues identified\.?$/i.test(lines[0])) {
    return [];
  }

  return lines;
}

function normalizeBackendMessages(rawMessages: unknown): GenerationMessage[] {
  if (!Array.isArray(rawMessages)) {
    return [];
  }

  const normalized: GenerationMessage[] = [];
  for (const item of rawMessages) {
    if (!item || typeof item !== 'object') {
      continue;
    }
    const raw = item as { role?: string; content?: string };
    const role = String(raw.role ?? '').toLowerCase();
    if (role !== 'user' && role !== 'assistant') {
      continue;
    }
    const normalizedContent = String(raw.content ?? '').trim();
    if (!normalizedContent) {
      continue;
    }
    normalized.push({
      role,
      content: normalizedContent,
    });
  }
  return normalized;
}

function normalizeBackendResponse(raw: unknown): {
  pug: string;
  css: string;
  data: unknown;
  messages: GenerationMessage[];
} {
  if (typeof raw !== 'object' || raw === null) {
    throw new Error('Backend response payload is malformed');
  }

  const payload = raw as BackendGenerationPayload;

  if (typeof payload.pug !== 'string') {
    throw new Error('Backend response missing `pug`');
  }

  const normalizedData = payload.data === undefined ? {} : payload.data;
  const normalizedCss = payload.css === undefined ? '' : String(payload.css ?? '');
  const normalizedMessages = normalizeBackendMessages(payload.messages);

  return {
    pug: payload.pug,
    css: normalizedCss,
    data: normalizedData,
    messages: normalizedMessages,
  };
}

function isPugLike(text: string): boolean {
  const trimmed = text.trim();
  if (!trimmed) {
    return false;
  }

  if (trimmed.startsWith('<')) {
    return false;
  }

  if (trimmed.includes('<template')) {
    return false;
  }

  return true;
}

export class GenerationPipelineService {
  private readonly componentCatalogClient: ComponentCatalogClient;
  private readonly endpoint: string;
  private readonly inspirationEndpoint: string;
  private readonly evaluatorEndpoint: string;
  private readonly generationTimeoutMs: number;
  private readonly inspirationTimeoutMs: number;
  private readonly evaluatorTimeoutMs: number;
  private readonly bootstrapVueTags: Set<string>;
  private readonly bootstrapVueTagsPascal: Set<string>;
  private catalogCache: ComponentInventoryItem[] | null = null;

  constructor(private readonly options: GenerationPipelineServiceOptions = {}) {
    const baseUrl = options.baseUrl?.trim() || '/api';
    this.endpoint = `${baseUrl}/generation`;
    this.inspirationEndpoint = `${baseUrl}${BASE_INSPIRATION_ENDPOINT}`;
    this.evaluatorEndpoint = `${baseUrl}${UX_EVALUATOR_ENDPOINT}`;
    this.generationTimeoutMs = options.generationTimeoutMs ?? DEFAULT_GENERATION_TIMEOUT_MS;
    this.inspirationTimeoutMs = options.inspirationTimeoutMs ?? DEFAULT_INSPIRATION_TIMEOUT_MS;
    this.evaluatorTimeoutMs = options.evaluatorTimeoutMs ?? DEFAULT_EVALUATOR_TIMEOUT_MS;
    this.componentCatalogClient = options.componentCatalogClient ?? new ComponentCatalogClient({ baseUrl });
    this.bootstrapVueTags = new Set(Array.from(options.bootstrapVueTags ?? DEFAULT_BOOTSTRAP_VUE_TAGS));
    this.bootstrapVueTagsPascal = new Set(
      Array.from(options.bootstrapVueTagsPascal ?? DEFAULT_BOOTSTRAP_VUE_TAGS_PASCAL),
    );
  }

  private async fetchWithTimeout(
    url: string,
    init: RequestInit,
    timeoutMs: number,
    timeoutLabel: string,
  ): Promise<Response> {
    const controller = new AbortController();
    const timeoutId = window.setTimeout(() => {
      controller.abort();
    }, timeoutMs);

    try {
      const response = await fetch(url, {
        ...init,
        signal: controller.signal,
      });
      return response;
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        throw new GenerationServiceError(`${timeoutLabel} request timed out after ${timeoutMs}ms`);
      }
      throw error;
    } finally {
      window.clearTimeout(timeoutId);
    }
  }

  private async fetchEnabledCatalog(): Promise<ComponentInventoryItem[]> {
    if (this.catalogCache) {
      return this.catalogCache;
    }

    const payload = await this.componentCatalogClient.getEnabledComponents();
    this.catalogCache = payload.components;
    return payload.components;
  }

  private async fetchGeneration(
    input: GenerationRequest,
  ): Promise<{ pug: string; css: string; data: unknown; messages: GenerationMessage[] }> {
    const response = await this.fetchWithTimeout(
      this.endpoint,
      {
        method: 'POST',
        headers: buildHeaders(),
        body: JSON.stringify(input),
      },
      this.generationTimeoutMs,
      'Generation',
    );

    const body = await response.text();
    if (!response.ok) {
      throw new GenerationServiceError(`Generation failed with ${response.status}`, response.status, body);
    }

    const parsed = safeParseJSON(body);
    return normalizeBackendResponse(parsed);
  }

  private async fetchInspiration(
    input: InspirationRequest,
  ): Promise<{ pug: string; css: string; data: unknown; messages: GenerationMessage[] }> {
    const response = await this.fetchWithTimeout(
      this.inspirationEndpoint,
      {
        method: 'POST',
        headers: buildHeaders(),
        body: JSON.stringify(input),
      },
      this.inspirationTimeoutMs,
      'Inspiration',
    );

    const body = await response.text();
    if (!response.ok) {
      throw new GenerationServiceError(`Inspiration failed with ${response.status}`, response.status, body);
    }

    const parsed = safeParseJSON(body);
    return normalizeBackendResponse(parsed);
  }

  private async fetchUXEvaluation(input: UXEvaluatorRequest): Promise<UXEvaluatorResultLine[]> {
    const response = await this.fetchWithTimeout(
      this.evaluatorEndpoint,
      {
        method: 'POST',
        headers: buildHeaders(),
        body: JSON.stringify(input),
      },
      this.evaluatorTimeoutMs,
      'UX evaluation',
    );

    const text = await response.text();
    if (!response.ok) {
      throw new GenerationServiceError(`UX evaluation failed with ${response.status}`, response.status, text);
    }

    return normalizeUxEvaluationText(text);
  }

  async generate(input: GenerationRequest, catalog?: ComponentInventoryItem[]): Promise<GenerationPipelineResult> {
    const output = await this.fetchGeneration(input);
    return this.renderPipelineOutput(output, catalog);
  }

  async generateFromInspiration(
    input: InspirationRequest,
    catalog?: ComponentInventoryItem[],
  ): Promise<GenerationPipelineResult> {
    const output = await this.fetchInspiration(input);
    return this.renderPipelineOutput(output, catalog);
  }

  private async renderPipelineOutput(
    output: { pug: string; css: string; data: unknown; messages: GenerationMessage[] },
    catalog?: ComponentInventoryItem[],
  ): Promise<GenerationPipelineResult> {
    const sourcePug = output.pug;
    const template: PugTemplateTree = isPugLike(sourcePug)
      ? parsePugToHierarchy(sourcePug)
      : { type: 'root', children: [] };

    const inventory = catalog ?? (await this.fetchEnabledCatalog());
    const resolved = resolveImports(template, inventory, this.bootstrapVueTags, this.bootstrapVueTagsPascal);

    return {
      template,
      imports: resolved.imports,
      css: output.css,
      data: output.data,
      sourcePug,
      messages: output.messages,
      metadata: {
        usedTags: extractUsedTags(template),
        nonBootstrapTags: resolved.nonBootstrapTags,
        unresolvedTags: resolved.unresolvedTags,
      },
    };
  }

  async evaluateUX(input: UXEvaluatorRequest): Promise<UXEvaluatorResultLine[]> {
    return this.fetchUXEvaluation(input);
  }
}

export function parsePugStructure(pug: string): PugTemplateTree {
  return parsePugToHierarchy(pug);
}

export function resolveNonBootstrapComponents(
  pug: string,
  componentInventory: ComponentInventoryItem[],
  bootstrapVueTags: Iterable<string> = DEFAULT_BOOTSTRAP_VUE_TAGS,
  bootstrapVueTagsPascal: Iterable<string> = DEFAULT_BOOTSTRAP_VUE_TAGS_PASCAL,
): PipelineImport[] {
  const tree = parsePugToHierarchy(pug);
  const resolved = resolveImports(tree, componentInventory, new Set(bootstrapVueTags), new Set(bootstrapVueTagsPascal));
  return resolved.imports;
}
