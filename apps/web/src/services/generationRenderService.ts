import {
  defineComponent,
  h,
  markRaw,
  reactive,
  type App,
  type DefineComponent,
  type VNode,
} from 'vue';
import * as VueRuntime from 'vue';
import * as bootstrapVueNext from 'bootstrap-vue-next';
import { compile } from '@vue/compiler-dom';

import type {
  GenerationPipelineResult,
  PugTemplateNode,
  PugTreeElementNode,
  PugTreeTextNode,
  PugTreeExpressionNode,
} from './generationPipelineService';

export type ComponentLoader = () => Promise<unknown>;

export interface GenerationRenderOptions {
  app?: App;
  componentLoaders?: Record<string, ComponentLoader>;
  runtimeContext?: PipelineScreenData;
  styleId?: string;
  silent?: boolean;
}

export interface GeneratedScreenView {
  component: ReturnType<typeof defineComponent>;
  css: string;
  sourcePug: string;
  usedTags: string[];
  unresolvedTags: string[];
  styleId: string;
  missingComponents: string[];
  installStyles: () => () => void;
}

export interface PipelineScreenData {
  [key: string]: unknown;
}

const DEFAULT_STYLE_ID = 'pipeline-generated-screen-styles';

const BASE_VOID_TAGS = new Set(['area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'param', 'source', 'track', 'wbr']);
const HTML_TAGS = new Set([
  'a',
  'abbr',
  'address',
  'article',
  'aside',
  'audio',
  'b',
  'blockquote',
  'body',
  'button',
  'canvas',
  'caption',
  'cite',
  'code',
  'data',
  'datalist',
  'dd',
  'details',
  'div',
  'dl',
  'dt',
  'em',
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
  'label',
  'li',
  'main',
  'map',
  'mark',
  'menu',
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
  'ul',
  'var',
  'video',
]);

function resolveChartTagType(tag: string): string | null {
  const normalized = normalizeTag(tag);
  if (!normalized.startsWith('b-chart-')) {
    return null;
  }

  const normalizedType = normalized.slice('b-chart-'.length);
  if (normalizedType === 'polar-area' || normalizedType === 'polar_area' || normalizedType === 'polararea') {
    return 'polararea';
  }

  if (
    normalizedType === 'line' ||
    normalizedType === 'bar' ||
    normalizedType === 'pie' ||
    normalizedType === 'doughnut' ||
    normalizedType === 'radar' ||
    normalizedType === 'bubble' ||
    normalizedType === 'scatter'
  ) {
    return normalizedType;
  }

  return null;
}

function normalizeTag(tag: string): string {
  return toKebabTag(tag.trim()).toLowerCase();
}

function toKebabTag(value: string): string {
  return value
    .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
    .replace(/_/g, '-')
    .toLowerCase();
}

const BOOTSTRAP_PREFIXLESS_COMPONENT_TAGS = (() => {
  const entries = Object.keys(bootstrapVueNext)
    .filter((key): key is keyof typeof bootstrapVueNext => /^B[A-Z]/.test(key))
    .map((key) => {
      const pascal = String(key).replace(/^B/, '');
      const kebab = toKebabTag(pascal);
      const prefixed = `b-${kebab}`;
      const prefixless = kebab;
      return { key, prefixed, prefixless };
    })
    .filter((entry) => !HTML_TAGS.has(entry.prefixless));

  const map = new Map<string, string>();
  for (const entry of entries) {
    map.set(entry.prefixless, entry.prefixed);
  }
  return map;
})();

function hasProperty<T extends Record<string, unknown>>(value: T, key: string): key is Extract<keyof T, string> {
  return key in value;
}

function toPascalTag(tag: string): string {
  if (!tag.trim()) {
    return '';
  }

  if (!tag.includes('-')) {
    return tag.charAt(0).toUpperCase() + tag.slice(1);
  }

  return tag
    .split('-')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join('');
}

function normalizeBootstrapVueTag(tag: string): string {
  const normalized = normalizeTag(tag);
  if (!normalized) {
    return normalized;
  }

  if (normalized.startsWith('b-')) {
    return normalized;
  }

  const alias = BOOTSTRAP_PREFIXLESS_COMPONENT_TAGS.get(normalized);
  if (alias) {
    return alias;
  }

  const directPascal = `B${toPascalTag(normalized)}`;
  if (hasProperty(bootstrapVueNext, directPascal)) {
    return `b-${toKebabTag(normalized)}`;
  }

  return normalized;
}

function isBootstrapVueComponentCandidate(tag: string): boolean {
  return /^b-[a-z0-9-]+$/i.test(tag);
}

function resolveBootstrapVueComponent(tag: string): DefineComponent | null {
  const normalizedTag = normalizeBootstrapVueTag(tag);
  if (!isBootstrapVueComponentCandidate(normalizedTag)) {
    return null;
  }

  const pascalTag = toPascalTag(normalizedTag);
  const directExport = hasProperty(bootstrapVueNext, pascalTag) ? bootstrapVueNext[pascalTag] : null;
  if (directExport && typeof directExport === 'object' && directExport !== null) {
    return directExport as DefineComponent;
  }
  return null;
}

function buildBootstrapComponentRegistry(tags: string[]): Record<string, DefineComponent> {
  const components: Record<string, DefineComponent> = {};

  for (const tag of tags) {
    const normalizedTag = normalizeBootstrapVueTag(tag);
    const component = resolveBootstrapVueComponent(normalizedTag);
    if (!component) {
      continue;
    }

    const original = normalizeTag(tag);
    const normalized = normalizedTag;
    components[normalized] = component;
    components[original] = component;
    const pascalTag = toPascalTag(normalized);
    components[pascalTag] = component;
    if (normalized.startsWith('b-')) {
      const prefixless = normalized.slice(2);
      components[prefixless] = component;
    }
    if (pascalTag === 'BFormInput' && !hasProperty(components, 'BInput')) {
      components.BInput = component;
    }

    if (import.meta.env.DEV && normalized !== original && original.length > 0) {
      console.debug('[GeneratedScreen][bootstrap-alias]', `${original} -> ${normalized}`);
    }
  }

  return components;
}

function toStyleId(prefix: string): string {
  if (prefix.trim().length > 0) {
    return prefix.trim();
  }
  return DEFAULT_STYLE_ID;
}

function isNativeTag(tag: string): boolean {
  return HTML_TAGS.has(tag.toLowerCase()) && !tag.includes('.');
}

function isInterpolationNode(node: PugTemplateNode): node is PugTreeExpressionNode {
  return node.type === 'expression';
}

function isTextNode(node: PugTemplateNode): node is PugTreeTextNode {
  return node.type === 'text';
}

function isElementNode(node: PugTemplateNode): node is PugTreeElementNode {
  return node.type === 'element';
}

function safeParseBoolean(value: unknown): boolean {
  if (value === true || value === false || value === 0 || value === 1) {
    return value === true || value === 1;
  }
  if (typeof value === 'number') {
    return value !== 0;
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (normalized === 'true') {
      return true;
    }
    if (normalized === 'false' || normalized.length === 0) {
      return false;
    }
    const parsed = Number(normalized);
    if (!Number.isNaN(parsed)) {
      return parsed !== 0;
    }
  }
  return Boolean(value);
}

function getPathValue(source: PipelineScreenData, path: string): unknown {
  if (!source || path.trim().length === 0) {
    return undefined;
  }

  const segments = path.split('.');
  let current: unknown = source;

  for (const segment of segments) {
    if (current === null || typeof current !== 'object' || !(segment in (current as Record<string, unknown>))) {
      return undefined;
    }
    current = (current as Record<string, unknown>)[segment];
  }

  return current;
}

interface VForDescriptor {
  itemAlias: string;
  indexAlias: string | null;
  sourceExpression: string;
}

const NOOP_FUNCTION = () => undefined;

function parseVForExpression(expression: string): VForDescriptor | null {
  const normalized = expression.trim();
  if (!normalized) {
    return null;
  }

  const match = normalized.match(
    /^\(\s*([a-zA-Z_$][\w$]*)\s*(?:,\s*([a-zA-Z_$][\w$]*))?\s*\)\s+in\s+(.+)$|^([a-zA-Z_$][\w$]*)\s+in\s+(.+)$/,
  );
  if (!match) {
    return null;
  }

  const itemAlias = match[1] || match[4];
  const indexAlias = match[2] || null;
  const sourceExpression = (match[3] || match[5] || '').trim();
  if (!itemAlias || !sourceExpression) {
    return null;
  }

  return {
    itemAlias,
    indexAlias,
    sourceExpression,
  };
}

function toIterable(value: unknown): unknown[] {
  if (Array.isArray(value)) {
    return value;
  }

  if (value === null || value === undefined) {
    return [];
  }

  if (typeof value === 'string') {
    return [];
  }

  if (typeof value === 'object') {
    if (typeof (value as { [Symbol.iterator]?: () => unknown })?.[Symbol.iterator] === 'function') {
      return Array.from(value as Iterable<unknown>);
    }
    return Object.values(value as Record<string, unknown>);
  }

  return [];
}

function parseScalar(value: string): string | number | boolean | null {
  const trimmed = value.trim();

  if (trimmed === 'true') {
    return true;
  }
  if (trimmed === 'false') {
    return false;
  }
  if (trimmed === 'null') {
    return null;
  }
  if (trimmed === 'undefined') {
    return null;
  }
  if (/^-?\d+(\.\d+)?$/.test(trimmed)) {
    return Number(trimmed);
  }

  if ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
    return trimmed.slice(1, -1);
  }

  return trimmed;
}

function tryEvaluateExpression(expression: string, context: PipelineScreenData): unknown {
  const keys = Object.keys(context);
  const values = keys.map((key) => context[key]);
  try {
    const evaluator = new Function(
      ...keys,
      '"use strict"; const window=undefined, document=undefined, globalThis=undefined, Function=undefined, eval=undefined; return (' +
        expression +
        ');',
    );
    return evaluator(...values);
  } catch (_error) {
    return undefined;
  }
}

function resolveExpression(expression: string, context: PipelineScreenData): unknown {
  const value = expression.trim();
  if (value.length === 0) {
    return '';
  }

  const numericOrBoolean = parseScalar(value);
  if (typeof numericOrBoolean !== 'string') {
    return numericOrBoolean;
  }

  const functionMatch = numericOrBoolean.match(/^\s*([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)\s*\(([\s\S]*)\)\s*$/);
  if (functionMatch) {
    const fnName = functionMatch[1];
    if (!fnName) {
      return undefined;
    }
    const rawArgs = functionMatch[2] ?? '';
    const maybeFunction = getPathValue(context, fnName);
    if (typeof maybeFunction === 'function') {
      const argTemplates = parseFunctionCallArgs(rawArgs);
      return (...eventArgs: unknown[]) => {
        const eventContext = { ...context, $event: eventArgs[0] };
        const args = argTemplates.map((arg) => resolveExpressionArgument(arg, eventContext));
        try {
          return maybeFunction(...args);
        } catch (_error) {
          return undefined;
        }
      };
    }
    return undefined;
  }

  const maybePath = getPathValue(context, value);
  if (maybePath !== undefined) {
    return maybePath;
  }

  const maybeEvaluated = tryEvaluateExpression(value, context);
  if (maybeEvaluated !== undefined) {
    return maybeEvaluated;
  }

  return value;
}

function resolveExpressionArgument(argument: string, context: PipelineScreenData): unknown {
  const parsed = parseScalar(argument);
  if (typeof parsed !== 'string') {
    return parsed;
  }
  const fromPath = getPathValue(context, parsed);
  if (fromPath !== undefined) {
    return fromPath;
  }
  return parsed;
}

function parseFunctionCallArgs(rawArgs: string): string[] {
  const args: string[] = [];
  let current = '';
  let depth = 0;
  let inSingle = false;
  let inDouble = false;
  let inTemplate = false;
  let escape = false;

  const pushArg = () => {
    const normalized = current.trim();
    if (normalized) {
      args.push(normalized);
    }
    current = '';
  };

  for (const char of rawArgs) {
    if (escape) {
      current += char;
      escape = false;
      continue;
    }

    if (char === '\\' && (inSingle || inDouble || inTemplate)) {
      current += char;
      escape = true;
      continue;
    }

    if (char === "'" && !inDouble && !inTemplate) {
      inSingle = !inSingle;
      current += char;
      continue;
    }

    if (char === '"' && !inSingle && !inTemplate) {
      inDouble = !inDouble;
      current += char;
      continue;
    }

    if (char === '`' && !inSingle && !inDouble) {
      inTemplate = !inTemplate;
      current += char;
      continue;
    }

    if (!inSingle && !inDouble && !inTemplate) {
      if (char === '(') {
        depth += 1;
      } else if (char === ')' && depth > 0) {
        depth -= 1;
      } else if ((char === '[' || char === '{') && depth >= 0) {
        depth += 1;
      } else if ((char === ']' || char === '}') && depth > 0) {
        depth -= 1;
      } else if (char === ',' && depth === 0) {
        pushArg();
        continue;
      }
    }

    current += char;
  }

  if (current.trim().length > 0) {
    args.push(current.trim());
  }

  return args;
}

function extractCallableExpressionTarget(expression: string): string | null {
  const trimmed = expression.trim();
  if (!trimmed) {
    return null;
  }

  const callMatch = trimmed.match(/^\s*([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)\s*\(/);
  if (callMatch?.[1]) {
    return callMatch[1];
  }

  const referenceMatch = trimmed.match(/^\s*([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)\s*$/);
  if (referenceMatch?.[1]) {
    return referenceMatch[1];
  }

  return null;
}

function collectReferencedCallablePaths(
  nodes: PugTemplateNode[],
  out: Set<string>,
): void {
  for (const node of nodes) {
    if (isInterpolationNode(node)) {
      const target = extractCallableExpressionTarget(node.expression);
      if (target) {
        out.add(target);
      }
      continue;
    }

    if (!isElementNode(node)) {
      continue;
    }

    for (const [key, rawValue] of Object.entries(node.attributes)) {
      if (typeof rawValue !== 'string') {
        continue;
      }

      const isEventBinding = key.startsWith('@') || key.startsWith('v-on:');
      const isDynamicBinding = key.startsWith(':') || key.startsWith('v-bind:');
      if (!isEventBinding && !isDynamicBinding) {
        continue;
      }

      const target = extractCallableExpressionTarget(rawValue);
      if (target) {
        out.add(target);
      }
    }

    if (node.children.length > 0) {
      collectReferencedCallablePaths(node.children, out);
    }
  }
}

function ensureCallablePath(
  context: PipelineScreenData,
  path: string,
): boolean {
  const trimmed = path.trim();
  if (!trimmed) {
    return false;
  }

  const existing = getPathValue(context, trimmed);
  if (typeof existing === 'function') {
    return false;
  }
  if (existing !== undefined) {
    return false;
  }

  const segments = trimmed.split('.').filter(Boolean);
  if (segments.length === 0) {
    return false;
  }

  let cursor: Record<string, unknown> = context;
  for (let index = 0; index < segments.length - 1; index += 1) {
    const segment = segments[index]!;
    const current = cursor[segment];
    if (current === undefined) {
      const next: Record<string, unknown> = {};
      cursor[segment] = next;
      cursor = next;
      continue;
    }
    if (current === null || typeof current !== 'object' || Array.isArray(current)) {
      return false;
    }
    cursor = current as Record<string, unknown>;
  }

  const leaf = segments[segments.length - 1]!;
  if (cursor[leaf] === undefined) {
    cursor[leaf] = NOOP_FUNCTION;
    return true;
  }
  return false;
}

function populateMissingCallableNoops(
  tree: GenerationPipelineResult['template'],
  context: PipelineScreenData,
): string[] {
  const callablePaths = new Set<string>();
  collectReferencedCallablePaths(tree.children, callablePaths);
  const injected: string[] = [];

  for (const path of callablePaths) {
    if (ensureCallablePath(context, path)) {
      injected.push(path);
    }
  }

  return injected;
}

function interpolateText(text: string, context: PipelineScreenData): string {
  return text.replace(/{{\s*([^}]+)\s*}}/g, (_, variable) => {
    const value = resolveExpression(String(variable), context);
    if (value === null || value === undefined) {
      return '';
    }
    return String(value);
  });
}

function toOnEventName(rawEvent: string): string {
  const eventName = (rawEvent.split('.')[0] ?? '').trim();
  if (!eventName) {
    return '';
  }
  return `on${eventName.charAt(0).toUpperCase()}${eventName.slice(1).replace(/-([a-z])/g, (_, char) => char.toUpperCase())}`;
}

function resolveAttributeValue(
  key: string,
  rawValue: unknown,
  context: PipelineScreenData,
): { skip: boolean; props: Record<string, unknown> } {
  const props: Record<string, unknown> = {};

  if (typeof rawValue === 'boolean' || typeof rawValue === 'number') {
    if (key.startsWith('@') || key.startsWith(':') || key.startsWith('v-bind:') || key.startsWith('v-')) {
      return { skip: false, props: { [key]: rawValue } };
    }
    return { skip: false, props: { [key]: rawValue } };
  }

  const value = String(rawValue ?? '').trim();

  if (key === 'v-if') {
    const evaluated = resolveExpression(value, context);
    return { skip: !safeParseBoolean(evaluated), props: {} };
  }

  if (key.startsWith('@')) {
    const on = toOnEventName(key.slice(1));
    if (!on) {
      return { skip: false, props: {} };
    }

    const fn = resolveExpression(value, context);
    props[on] = typeof fn === 'function' ? fn : () => undefined;
    return { skip: false, props };
  }

  if (key.startsWith(':') || key.startsWith('v-bind:')) {
    const normalizedKey = key.startsWith(':') ? key.slice(1) : key.slice(7);
    const valueFromExpression = resolveExpression(value, context);
    props[normalizedKey] = valueFromExpression;
    return { skip: false, props };
  }

  if (key === 'class') {
    props.class = value;
    return { skip: false, props };
  }

  if (key === 'style') {
    props.style = value;
    return { skip: false, props };
  }

  if (key.startsWith('v-')) {
    return { skip: false, props: {} };
  }

  props[key] = parseScalar(value);
  return { skip: false, props };
}

type SlotRenderer = (slotProps?: PipelineScreenData) => VNode | string | Array<VNode | string>;
type SlotRegistry = Record<string, SlotRenderer>;

function parseSlotName(attributeName: string): string | null {
  if (!attributeName.startsWith('v-slot')) {
    if (attributeName.startsWith('#')) {
      return attributeName.slice(1);
    }
    return null;
  }
  if (attributeName === 'v-slot') {
    return 'default';
  }
  if (attributeName.startsWith('v-slot:')) {
    return attributeName.slice('v-slot:'.length);
  }
  return null;
}

function isSlotTemplateNode(node: PugTemplateNode): node is PugTreeElementNode {
  if (!isElementNode(node)) {
    return false;
  }
  const tag = normalizeTag(node.tag);
  return (
    tag === 'template' &&
    Object.keys(node.attributes).some((name) => name.startsWith('v-slot') || name.startsWith('#'))
  );
}

function getSlotName(node: PugTreeElementNode): string | null {
  for (const attributeName of Object.keys(node.attributes)) {
    const slotName = parseSlotName(attributeName);
    if (slotName !== null) {
      return slotName;
    }
  }
  return null;
}

function renderSlotChildren(
  source: PugTemplateNode[],
  context: PipelineScreenData,
  componentRegistry?: Record<string, DefineComponent>,
): { children: Array<VNode | string>; slots: SlotRegistry } {
  const children: Array<VNode | string> = [];
  const slots: SlotRegistry = {};

  for (const child of source) {
    if (isSlotTemplateNode(child)) {
      const slotName = getSlotName(child);
      if (slotName) {
        slots[slotName] = (slotContext: PipelineScreenData = {}) => {
          const renderContext = {
            ...context,
            ...slotContext,
          };
          const rendered = toVNodeList(child.children, renderContext, componentRegistry);
          return rendered;
        };
        continue;
      }
    }

    const childNode = toVNode(child, context, componentRegistry);
    if (childNode === null) {
      continue;
    }
    if (typeof childNode === 'string') {
      children.push(childNode);
      continue;
    }
    if (Array.isArray(childNode)) {
      for (const item of childNode) {
        if (item !== null) {
          children.push(item);
        }
      }
      continue;
    }
    children.push(childNode);
  }

  return { children, slots };
}

function toVNode(
  node: PugTemplateNode,
  context: PipelineScreenData,
  componentRegistry?: Record<string, DefineComponent>,
): VNode | string | Array<VNode | string> | null {
  if (isTextNode(node)) {
    return interpolateText(node.text, context);
  }

  if (isInterpolationNode(node)) {
    const evaluated = resolveExpression(node.expression, context);
    if (evaluated === null || evaluated === undefined) {
      return null;
    }
    if (Array.isArray(evaluated)) {
      return evaluateArrayToVNodes(evaluated, context);
    }
    return String(evaluated);
  }

  if (node.type === 'element') {
    const forAttribute = node.attributes.vFor ?? node.attributes['v-for'];
    if (typeof forAttribute === 'string') {
      const descriptor = parseVForExpression(forAttribute);
      if (!descriptor) {
        return null;
      }

      const iterableValue = resolveExpression(descriptor.sourceExpression, context);
      const iterable = toIterable(iterableValue);
      if (iterable.length === 0) {
        return null;
      }

      const loopAttributes: Record<string, string | number | boolean> = { ...node.attributes };
      delete loopAttributes.vFor;
      delete loopAttributes['v-for'];

      const children: Array<VNode | string> = [];
      for (let index = 0; index < iterable.length; index += 1) {
        const item = iterable[index];
        const loopContext: PipelineScreenData = {
          ...context,
          [descriptor.itemAlias]: item,
        };
        if (descriptor.indexAlias) {
          loopContext[descriptor.indexAlias] = index;
        }

        const loopNode: PugTreeElementNode = {
          ...node,
          attributes: loopAttributes,
        };

        const rendered = toVNode(loopNode, loopContext, componentRegistry);
        if (rendered === null) {
          continue;
        }

        if (Array.isArray(rendered)) {
          for (const child of rendered) {
            if (child === null) {
              continue;
            }
            children.push(child);
          }
          continue;
        }

        children.push(rendered);
      }

      return children;
    }
  }

  const props: Record<string, unknown> = {};
  const children: Array<VNode | string> = [];
  const slots: SlotRegistry = {};

  let skipNode = false;
  for (const [rawKey, rawValue] of Object.entries(node.attributes)) {
    const resolved = resolveAttributeValue(rawKey, rawValue, context);
    if (resolved.skip) {
      skipNode = true;
      break;
    }
    Object.assign(props, resolved.props);
  }

  if (skipNode) {
    return null;
  }

  if (node.children.length > 0) {
    const rendered = renderSlotChildren(node.children, context, componentRegistry);
    children.push(...rendered.children);
    Object.assign(slots, rendered.slots);
  }

  const normalizedNodeTag = normalizeTag(node.tag);
  const bootstrapNodeTag = normalizeBootstrapVueTag(normalizedNodeTag);
  const chartType = resolveChartTagType(normalizedNodeTag);
  if (
    chartType &&
    !Object.prototype.hasOwnProperty.call(props, 'chartType') &&
    !Object.prototype.hasOwnProperty.call(props, 'chart-type') &&
    !Object.prototype.hasOwnProperty.call(props, 'type')
  ) {
    props.chartType = chartType;
  }
  const registered =
    componentRegistry?.[normalizedNodeTag] ??
    componentRegistry?.[bootstrapNodeTag] ??
    componentRegistry?.[toPascalTag(bootstrapNodeTag)];
  if (registered) {
    if (Object.keys(slots).length > 0) {
      if (!slots.default && children.length > 0) {
        slots.default = () => children;
      }
      return h(registered, props, slots);
    }
    return h(registered, props, children);
  }

  if (isNativeTag(normalizedNodeTag)) {
    if (children.length === 0 && BASE_VOID_TAGS.has(normalizedNodeTag)) {
      return h(normalizedNodeTag, props);
    }
    return h(normalizedNodeTag, props, children);
  }

  return h(normalizedNodeTag, props, children);
}

function evaluateArrayToVNodes(values: unknown[], context: PipelineScreenData): VNode | Array<VNode | string> {
  const nodes = values
    .map((value) => {
      if (value === null || value === undefined) {
        return null;
      }
      if (typeof value === 'object') {
        return h('span', String(JSON.stringify(value)));
      }
      return h('span', String(value));
    })
    .filter((entry): entry is VNode => entry !== null);

  if (nodes.length === 1) {
    return nodes[0]!;
  }
  return nodes;
}

function toVNodeList(
  nodes: PugTemplateNode[],
  context: PipelineScreenData,
  componentRegistry?: Record<string, DefineComponent>,
): Array<VNode | string> {
  return nodes
    .map((child) => toVNode(child, context, componentRegistry))
    .filter((node): node is VNode | string | Array<VNode | string> => node !== null)
    .flatMap((node) => {
      return Array.isArray(node) ? node : [node];
    })
    .filter((node): node is VNode | string => node !== null);
}

function collectChildren(
  tree: GenerationPipelineResult['template'],
  context: PipelineScreenData,
  componentRegistry?: Record<string, DefineComponent>,
): VNode | string | Array<VNode | string> {
  const renderedChildren = toVNodeList(tree.children, context, componentRegistry);

  if (renderedChildren.length === 0) {
    return '';
  }
  if (renderedChildren.length === 1) {
    return renderedChildren[0]!;
  }

  return renderedChildren;
}

function sanitizeVueSfcTemplate(raw: string): string {
  if (!raw) {
    return '';
  }

  const templateMatch = raw.match(/<template[^>]*>([\s\S]*?)<\/template>/i);
  const template = templateMatch?.[1] ?? raw;
  return template
    .replace(/<script[\s\S]*?<\/script>/gi, '')
    .replace(/<style[\s\S]*?<\/style>/gi, '')
    .trim();
}

function sanitizeGeneratedCss(raw: string): string {
  if (!raw) {
    return '';
  }

  let css = raw.trim();
  css = css.replace(/```(?:css)?\s*/gi, '');
  css = css.replace(/```/g, '').trim();

  css = css.replace(/([,{;]\s*)\"([^\"]+)\"\s*:/g, (_match, prefix, property) => `${prefix}${property}:`);
  css = css.replace(/,\s*}/g, ';}');

  return css;
}

function escapeHtmlText(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function escapeHtmlAttribute(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;');
}

function serializeTemplateAttributes(attributes: Record<string, string | number | boolean>): string {
  const pairs: string[] = [];
  for (const [key, rawValue] of Object.entries(attributes)) {
    if (rawValue === true) {
      pairs.push(key);
      continue;
    }
    if (rawValue === false || rawValue === null || rawValue === undefined) {
      continue;
    }
    pairs.push(`${key}="${escapeHtmlAttribute(String(rawValue))}"`);
  }
  return pairs.length > 0 ? ` ${pairs.join(' ')}` : '';
}

function toTemplateHtml(
  node: PugTemplateNode,
  componentRegistry?: Record<string, DefineComponent>,
): string {
  if (isTextNode(node)) {
    return escapeHtmlText(node.text);
  }

  if (isInterpolationNode(node)) {
    return `{{ ${node.expression} }}`;
  }

  const normalizedNodeTag = normalizeTag(node.tag);
  const bootstrapNodeTag = normalizeBootstrapVueTag(normalizedNodeTag);
  const registered =
    componentRegistry?.[normalizedNodeTag] ??
    componentRegistry?.[bootstrapNodeTag] ??
    componentRegistry?.[toPascalTag(bootstrapNodeTag)];
  const tagName = registered ? bootstrapNodeTag : normalizedNodeTag;

  const attrs = serializeTemplateAttributes(node.attributes);
  const children = node.children.map((child) => toTemplateHtml(child, componentRegistry)).join('');

  if (BASE_VOID_TAGS.has(tagName) && children.length === 0) {
    return `<${tagName}${attrs} />`;
  }

  return `<${tagName}${attrs}>${children}</${tagName}>`;
}

function toTemplateHtmlFromTree(
  tree: GenerationPipelineResult['template'],
  componentRegistry?: Record<string, DefineComponent>,
): string {
  return tree.children.map((node) => toTemplateHtml(node, componentRegistry)).join('');
}

function buildGeneratedScreenVNodes(
  styleContent: string,
  styleId: string,
  children: VNode | string | Array<VNode | string>,
): Array<VNode | string> {
  const rendered: Array<VNode | string> = [];
  const normalizedCss = styleContent.trim();
  if (normalizedCss.length > 0) {
    rendered.push(
      h(
        'style',
        {
          id: styleId,
          'data-generated': 'pipeline-runtime',
          type: 'text/css',
        },
        normalizedCss,
      ),
    );
  }

  if (typeof children === 'string') {
    rendered.push(children);
    return rendered;
  }

  if (Array.isArray(children)) {
    rendered.push(...children);
    return rendered;
  }

  rendered.push(children);
  return rendered;
}

function isPugTreeEmpty(tree: GenerationPipelineResult['template']): boolean {
  return tree.children.length === 0;
}

function compileTemplateToRender(templateHtml: string) {
  const wrapped = `<div class="generated-screen">${templateHtml}</div>`;
  const compiled = compile(wrapped, {
    mode: 'function',
    hoistStatic: true,
    prefixIdentifiers: false,
  });
  const factory = new Function('Vue', compiled.code) as (vueRuntime: typeof VueRuntime) => (...args: unknown[]) => VNode;
  return factory(VueRuntime);
}

export class GenerationRenderService {
  private readonly app: App | undefined;
  private readonly componentLoaders: Record<string, ComponentLoader>;
  private readonly runtimeContext: PipelineScreenData;
  private readonly styleId: string;

  constructor(options: GenerationRenderOptions = {}) {
    this.app = options.app;
    this.componentLoaders = options.componentLoaders ?? {};
    this.runtimeContext = options.runtimeContext ? { ...(options.runtimeContext as PipelineScreenData) } : {};
    this.styleId = toStyleId(options.styleId ?? DEFAULT_STYLE_ID);
  }

  private async loadComponent(tagOrName: string): Promise<DefineComponent | null> {
    const loader = this.componentLoaders[tagOrName];
    if (!loader) {
      return null;
    }

    const loaded = await loader();
    const component = (loaded as { default: DefineComponent }).default ?? loaded;
    if (!component || typeof component !== 'object') {
      return null;
    }
    return component;
  }

  private async buildComponentRegistry(imports: GenerationPipelineResult['imports']): Promise<{
    localComponents: Record<string, DefineComponent>;
    unresolved: string[];
  }> {
    const unresolved = new Set<string>();
    const localComponents: Record<string, DefineComponent> = {};

    for (const dependency of imports) {
      if (!dependency.isCatalogResolved) {
        unresolved.add(dependency.tag);
      }

      const component = await this.loadComponent(dependency.localName);
      if (!component) {
        unresolved.add(dependency.tag);
        continue;
      }

      localComponents[dependency.tag] = markRaw(component);
      unresolved.delete(dependency.tag);

      if (this.app) {
        this.app.component(dependency.tag, component);
      }
    }

    return { localComponents, unresolved: Array.from(unresolved) };
  }

  async materializeScreen(output: GenerationPipelineResult): Promise<GeneratedScreenView> {
    if (import.meta.env.DEV) {
      console.info('[GeneratedScreen][meta] usedTags=', output.metadata.usedTags);
      console.info('[GeneratedScreen][meta] unresolvedTags=', output.metadata.unresolvedTags);
      console.info('[GeneratedScreen][template] sourcePug=', output.sourcePug);
      console.info('[GeneratedScreen][source] templateTree=', output.template);
    }

    const registry = await this.buildComponentRegistry(output.imports);
    const context = { ...(output.data ?? {}), ...(this.runtimeContext ?? {}) } as PipelineScreenData;
    const injectedNoops = populateMissingCallableNoops(output.template, context);
    if (import.meta.env.DEV && injectedNoops.length > 0) {
      console.warn('[GeneratedScreen][missing-method-fallback]', injectedNoops);
    }
    const bootstrapRegistry = buildBootstrapComponentRegistry(output.metadata.usedTags);
    if (import.meta.env.DEV) {
      console.debug('[GeneratedScreen][bootstrap-registry]', Object.keys(bootstrapRegistry));
    }
    const componentRegistry: Record<string, DefineComponent> = {
      ...registry.localComponents,
      ...bootstrapRegistry,
    };
    const style = sanitizeGeneratedCss(output.css || '');
    const fallbackHtml = sanitizeVueSfcTemplate(output.sourcePug);
    const renderNodesWithStyle = (value: VNode | string | Array<VNode | string>) =>
      buildGeneratedScreenVNodes(style, this.styleId, value);
    const templateHtml = toTemplateHtmlFromTree(output.template, componentRegistry);

    let component: ReturnType<typeof defineComponent>;
    try {
      const compiledRender = compileTemplateToRender(templateHtml);
      component = defineComponent({
        name: 'GeneratedPipelineScreen',
        components: {
          ...registry.localComponents,
          ...bootstrapRegistry,
        },
        setup() {
          const state = reactive(context as Record<string, unknown>);
          return state;
        },
        render() {
          const vm = this as Record<string, unknown>;
          try {
            return h('div', {}, renderNodesWithStyle(compiledRender(vm, [])));
          } catch (_error) {
            const fallbackChildren = collectChildren(output.template, vm as PipelineScreenData, componentRegistry);
            if (typeof fallbackChildren === 'string') {
              return h('div', { class: 'generated-screen' }, renderNodesWithStyle(fallbackChildren));
            }
            return h('div', { class: 'generated-screen' }, renderNodesWithStyle(fallbackChildren));
          }
        },
      });
    } catch (_error) {
      component = defineComponent({
        name: 'GeneratedPipelineScreen',
        components: {
          ...registry.localComponents,
          ...bootstrapRegistry,
        },
        setup() {
          if (isPugTreeEmpty(output.template) && fallbackHtml) {
            return () => {
              const fallback = sanitizeVueSfcTemplate(fallbackHtml);
              return h('div', { class: 'generated-screen' }, renderNodesWithStyle(fallback));
            };
          }

          const children = collectChildren(output.template, context, componentRegistry);
          return () => {
            if (typeof children === 'string') {
              return h('div', { class: 'generated-screen' }, renderNodesWithStyle(children));
            }
            return h('div', { class: 'generated-screen' }, renderNodesWithStyle(children));
          };
        },
      });
    }

    return {
      component,
      css: style,
      sourcePug: output.sourcePug,
      usedTags: output.metadata.usedTags,
      unresolvedTags: Array.from(new Set([...output.metadata.unresolvedTags, ...registry.unresolved])),
      missingComponents: output.imports.filter((entry) => !entry.isCatalogResolved || !registry.localComponents[entry.tag]).map((entry) => entry.tag),
      styleId: this.styleId,
      installStyles: () => () => undefined,
    };
  }
}

export async function buildGeneratedScreen(
  output: GenerationPipelineResult,
  options?: GenerationRenderOptions,
): Promise<GeneratedScreenView> {
  const service = new GenerationRenderService(options);
  return service.materializeScreen(output);
}
