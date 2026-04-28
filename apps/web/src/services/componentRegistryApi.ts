import type {
  CatalogVersion,
  ComponentInventoryItem,
  ComponentInventoryResponse,
} from '../../../packages/component-registry/src/types';

export interface ComponentCatalogClientOptions {
  baseUrl?: string;
}

export interface EnabledComponentSet {
  version: CatalogVersion;
  components: ComponentInventoryItem[];
}

const DEFAULT_BASE = '/api';

function buildHeaders(): Record<string, string> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  };
  return headers;
}

export class ComponentCatalogClient {
  constructor(private readonly options: ComponentCatalogClientOptions = {}) {}

  private get baseUrl(): string {
    return this.options.baseUrl ?? DEFAULT_BASE;
  }

  async getCatalog(enabledOnly = false): Promise<ComponentInventoryResponse> {
    const query = enabledOnly ? '?enabled=true' : '';
    const response = await fetch(`${this.baseUrl}/component-registry${query}`, {
      headers: buildHeaders(),
      method: 'GET',
    });

    if (!response.ok) {
      throw new Error(`Component inventory failed with ${response.status}`);
    }

    return response.json() as Promise<ComponentInventoryResponse>;
  }

  async setEnabled(name: string, enabled: boolean): Promise<ComponentInventoryItem> {
    const response = await fetch(`${this.baseUrl}/component-registry/${encodeURIComponent(name)}/enabled`, {
      headers: buildHeaders(),
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    });

    if (!response.ok) {
      throw new Error(`Failed to update ${name} to ${enabled} (${response.status})`);
    }

    return response.json() as Promise<ComponentInventoryItem>;
  }

  async register(entry: Partial<ComponentInventoryItem>): Promise<ComponentInventoryItem> {
    const response = await fetch(`${this.baseUrl}/component-registry`, {
      headers: buildHeaders(),
      method: 'POST',
      body: JSON.stringify(entry),
    });

    if (!response.ok) {
      throw new Error(`Failed to register component (${response.status})`);
    }

    return response.json() as Promise<ComponentInventoryItem>;
  }

  async update(name: string, entry: ComponentInventoryItem): Promise<ComponentInventoryItem> {
    const response = await fetch(`${this.baseUrl}/component-registry/${encodeURIComponent(name)}`, {
      headers: buildHeaders(),
      method: 'PUT',
      body: JSON.stringify(entry),
    });

    if (!response.ok) {
      throw new Error(`Failed to update ${name} (${response.status})`);
    }

    return response.json() as Promise<ComponentInventoryItem>;
  }

  async deleteComponent(name: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}/component-registry/${encodeURIComponent(name)}`, {
      headers: buildHeaders(),
      method: 'DELETE',
    });

    if (!response.ok && response.status !== 204) {
      throw new Error(`Failed to delete ${name} (${response.status})`);
    }
  }

  async getEnabledComponents(): Promise<EnabledComponentSet> {
    const payload = await this.getCatalog(true);
    return { version: payload.version, components: payload.components };
  }
}
