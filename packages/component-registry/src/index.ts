import { initialComponentInventory, CATALOG_VERSION } from './seed';
import type {
  CatalogVersion,
  ComponentInventoryResponse,
  ComponentInventoryItem,
  ComponentRegistrationPayload,
} from './types';

export {
  CATALOG_VERSION,
  initialComponentInventory,
};
export type {
  CatalogVersion,
  ComponentInventoryResponse,
  ComponentInventoryItem,
  ComponentRegistrationPayload,
};

export function buildInventoryResponse(
  components: ComponentInventoryItem[],
  version: CatalogVersion = CATALOG_VERSION,
): ComponentInventoryResponse {
  return {
    version,
    generatedAt: new Date().toISOString(),
    components,
  };
}

export function upsertComponent(
  list: ComponentInventoryItem[],
  payload: ComponentRegistrationPayload,
): ComponentInventoryItem[] {
  const idx = list.findIndex((item) => item.name === payload.name);

  const base: ComponentInventoryItem = {
    module: payload.module,
    tag: payload.tag,
    pack: payload.pack,
    name: payload.name,
    props: payload.props ?? [],
    slots: payload.slots ?? [],
    events: payload.events ?? [],
    examples: payload.examples ?? [],
    restrictions: payload.restrictions ?? [],
    enabled: payload.enabled ?? false,
    version: payload.version,
  };

  if (idx >= 0) {
    list[idx] = {
      ...list[idx],
      ...base,
    };
    return list;
  }

  return [...list, base];
}
