import type { App, DefineComponent } from 'vue';
import type { ComponentInventoryItem } from '../../../packages/component-registry/src/types';

export type ComponentLoader = () => Promise<DefineComponent | { default: DefineComponent }>;

export type ComponentLoaderRegistry = Record<string, ComponentLoader>;

export interface RegistrarOptions {
  loaders?: ComponentLoaderRegistry;
}

export async function registerCatalogComponents(
  app: App,
  components: ComponentInventoryItem[],
  options: RegistrarOptions = {},
) {
  const enabled = components.filter((item) => item.enabled);

  for (const item of enabled) {
    const loader = options.loaders?.[item.name] ?? options.loaders?.[item.tag];

    if (!loader) {
      continue;
    }

    const loaded = await loader();
    const component = (loaded as { default: DefineComponent }).default ?? loaded;
    if (!component || typeof component !== 'object') {
      continue;
    }

    app.component(item.tag, component);
  }
}

export function toAllowedTagList(components: ComponentInventoryItem[]) {
  return components.filter((item) => item.enabled).map((item) => item.tag);
}

export function toPromptCatalogPayload(components: ComponentInventoryItem[]) {
  return components
    .filter((component) => component.enabled)
    .map((component) => ({
      name: component.name,
      tag: component.tag,
      props: component.props.map((prop) => prop.name),
      slots: component.slots.map((slot) => slot.name),
      events: component.events.map((event) => event.name),
    }));
}
