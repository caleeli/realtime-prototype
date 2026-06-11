export type CollaborationDraft = {
  sourcePug: string;
  css: string;
  data: unknown;
  baseRevision: number;
  docVersion: number;
  updatedAt: string;
};

export type CollaborationPresence = {
  id: string;
  name: string;
};

export type CollaborationField = 'sourcePug' | 'css' | 'data';

export type CollaborationEvent =
  | { type: 'snapshot'; draft: CollaborationDraft }
  | { type: 'presence'; presence: CollaborationPresence[] }
  | {
      type: 'field_updated';
      field: CollaborationField;
      value: unknown;
      clientId: string;
      clientName: string;
      docVersion: number;
    }
  | { type: 'error'; message: string };

export type ScreenCollaborationOptions = {
  baseUrl?: string;
  clientId?: string;
  name?: string;
};

export class ScreenCollaborationService {
  private socket: WebSocket | null = null;
  private readonly clientId: string;

  constructor(private readonly options: ScreenCollaborationOptions = {}) {
    this.clientId = options.clientId || getOrCreateCollaborationClientId();
  }

  get id(): string {
    return this.clientId;
  }

  connect(params: {
    projectId: string;
    screenId: string;
    onEvent: (event: CollaborationEvent) => void;
    onStatus?: (status: 'connecting' | 'connected' | 'disconnected' | 'error') => void;
  }): void {
    this.disconnect();
    const url = this.buildWebsocketUrl(params.projectId, params.screenId);
    params.onStatus?.('connecting');
    const socket = new WebSocket(url);
    this.socket = socket;

    socket.addEventListener('open', () => params.onStatus?.('connected'));
    socket.addEventListener('close', () => {
      if (this.socket === socket) {
        params.onStatus?.('disconnected');
      }
    });
    socket.addEventListener('error', () => params.onStatus?.('error'));
    socket.addEventListener('message', (event) => {
      try {
        params.onEvent(JSON.parse(String(event.data)) as CollaborationEvent);
      } catch (_error) {
        params.onEvent({ type: 'error', message: 'Invalid collaboration message.' });
      }
    });
  }

  disconnect(): void {
    if (!this.socket) {
      return;
    }
    this.socket.close();
    this.socket = null;
  }

  sendFieldUpdate(field: CollaborationField, value: unknown): void {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return;
    }
    this.socket.send(JSON.stringify({
      type: 'field_update',
      field,
      value,
    }));
  }

  private buildWebsocketUrl(projectId: string, screenId: string): string {
    const baseUrl = (this.options.baseUrl?.trim() || '/api').replace(/\/$/, '');
    const apiUrl = new URL(baseUrl, window.location.origin);
    apiUrl.protocol = apiUrl.protocol === 'https:' ? 'wss:' : 'ws:';
    apiUrl.pathname = `${apiUrl.pathname.replace(/\/$/, '')}/collaboration/screens/${encodeURIComponent(screenId)}`;
    apiUrl.searchParams.set('projectId', projectId);
    apiUrl.searchParams.set('clientId', this.clientId);
    apiUrl.searchParams.set('name', this.options.name || 'Editor');
    return apiUrl.toString();
  }
}

function getOrCreateCollaborationClientId(): string {
  const key = 'realtime-prototype-collaboration-client-id';
  const existing = window.sessionStorage.getItem(key);
  if (existing) {
    return existing;
  }
  const next = crypto.randomUUID();
  window.sessionStorage.setItem(key, next);
  return next;
}
