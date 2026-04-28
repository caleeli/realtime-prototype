export interface ComponentPropMetadata {
  name: string;
  type: string;
  required: boolean;
  defaultValue?: string;
  description?: string;
}

export interface ComponentSlotMetadata {
  name: string;
  description?: string;
  required?: boolean;
}

export interface ComponentEventMetadata {
  name: string;
  payload?: string;
  description?: string;
}

export interface ComponentExample {
  label: string;
  pug: string;
  description?: string;
}

export interface ComponentRestriction {
  type: 'security' | 'runtime' | 'styling' | 'ux';
  message: string;
}

export interface ComponentInventoryItem {
  name: string;
  module: string;
  tag: string;
  pack: string;
  props: ComponentPropMetadata[];
  slots: ComponentSlotMetadata[];
  events: ComponentEventMetadata[];
  examples: ComponentExample[];
  restrictions: ComponentRestriction[];
  enabled: boolean;
  version?: string;
}

export interface ComponentRegistrationPayload {
  name: string;
  module: string;
  tag: string;
  pack: string;
  props?: ComponentPropMetadata[];
  slots?: ComponentSlotMetadata[];
  events?: ComponentEventMetadata[];
  examples?: ComponentExample[];
  restrictions?: ComponentRestriction[];
  enabled?: boolean;
  version?: string;
}

export type CatalogVersion = string;

export interface ComponentInventoryResponse {
  version: CatalogVersion;
  generatedAt: string;
  components: ComponentInventoryItem[];
}
