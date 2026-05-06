import type { GenerationMessage } from './generationPipelineService';

type SessionChatMessage = {
  role: 'user' | 'assistant';
  content: string;
};

type ScreenPayload = {
  sourcePug: string;
  css: string;
  data: unknown;
  messages: GenerationMessage[];
  metadata?: Record<string, unknown>;
};

type SaveScreenStateRequest = {
  conversation: SessionChatMessage[];
  recommendations: string[];
  screenPayload: ScreenPayload;
};

export type SessionScreenState = {
  id: number;
  revision: number;
  screenPayload: {
    sourcePug: string;
    css: string;
    data: unknown;
    messages: GenerationMessage[];
    metadata?: Record<string, unknown>;
  };
  conversation: SessionChatMessage[];
  recommendations: string[];
  createdAt: string;
};

export type SessionScreenSummary = {
  id: string;
  name: string;
  position: number;
  updatedAt: string;
  isActive: boolean;
  lastRevision: number;
};

export type SessionScreenHistoryStateSummary = {
  id: number;
  revision: number;
  createdAt: string;
};

export type SessionScreenHistory = {
  items: SessionScreenHistoryStateSummary[];
};

export type SessionSnapshot = {
  projectId: string;
  projectName: string;
  theme: string;
  activeScreenId: string;
  screens: SessionScreenSummary[];
  activeState: SessionScreenState | null;
};

export type CreateScreenResult = {
  id: string;
  name: string;
  position: number;
  updatedAt: string;
  isActive: boolean;
  lastRevision: number;
};

const DEFAULT_BASE_URL = '/api';
const SESSION_ENDPOINT = '/session';
const SESSION_SCREENS_ENDPOINT = `${SESSION_ENDPOINT}/screens`;

function buildHeaders(): Record<string, string> {
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  };
}

export interface ProjectSessionServiceOptions {
  baseUrl?: string;
}

function parseResponse<T>(response: Response): Promise<T> {
  return response.text().then((text) => {
    if (!response.ok) {
      throw new Error(text || `Request failed with ${response.status}`);
    }
    if (!text) {
      return null as T;
    }
    return JSON.parse(text) as T;
  });
}

export class ProjectSessionService {
  constructor(private readonly options: ProjectSessionServiceOptions = {}) {}

  private get baseUrl(): string {
    return this.options.baseUrl?.trim() || DEFAULT_BASE_URL;
  }

  async getSession(): Promise<SessionSnapshot> {
    const response = await fetch(`${this.baseUrl}${SESSION_ENDPOINT}`, {
      headers: buildHeaders(),
      method: 'GET',
    });
    return parseResponse<SessionSnapshot>(response);
  }

  async updateTheme(theme: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}${SESSION_ENDPOINT}/theme`, {
      method: 'PATCH',
      headers: buildHeaders(),
      body: JSON.stringify({ theme }),
    });
    await parseResponse<Record<string, string>>(response);
  }

  async createScreen(name: string): Promise<CreateScreenResult> {
    const response = await fetch(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}`, {
      method: 'POST',
      headers: buildHeaders(),
      body: JSON.stringify({ name }),
    });
    return parseResponse<CreateScreenResult>(response);
  }

  async activateScreen(screenId: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}/activate`, {
      method: 'PATCH',
      headers: buildHeaders(),
      body: '{}',
    });
    await parseResponse<Record<string, string>>(response);
  }

  async deleteScreen(screenId: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}`, {
      method: 'DELETE',
      headers: buildHeaders(),
    });
    await parseResponse<Record<string, string>>(response);
  }

  async loadLatestState(screenId: string): Promise<SessionScreenState | null> {
    const response = await fetch(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}/state/latest`, {
      headers: buildHeaders(),
      method: 'GET',
    });
    if (!response.ok) {
      if (response.status === 404) {
        return null;
      }
      await response.text().then((text) => {
        throw new Error(text || `Request failed with ${response.status}`);
      });
    }
    const text = await response.text();
    if (!text) {
      return null;
    }
    return JSON.parse(text) as SessionScreenState;
  }

  async loadScreenHistory(screenId: string, limit = 20): Promise<SessionScreenHistory> {
    const response = await fetch(
      `${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}/state?limit=${encodeURIComponent(
        String(limit),
      )}`,
      {
        headers: buildHeaders(),
        method: 'GET',
      },
    );
    return parseResponse<SessionScreenHistory>(response);
  }

  async saveScreenState(screenId: string, payload: SaveScreenStateRequest): Promise<SessionScreenState> {
    const response = await fetch(
      `${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}/state`,
      {
        method: 'POST',
        headers: buildHeaders(),
        body: JSON.stringify(payload),
      },
    );
    return parseResponse<SessionScreenState>(response);
  }
}
