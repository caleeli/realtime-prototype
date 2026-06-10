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

export type FlowTaskPosition = {
  x: number;
  y: number;
};

export type FlowTaskNode = {
  id: string;
  name: string;
  screenId: string;
  position: FlowTaskPosition;
  isPopupTask?: boolean;
  isStartTask?: boolean;
};

export type FlowDiagramConnection = {
  id: string;
  source: string;
  target: string;
  sourceHandle?: string | null;
  targetHandle?: string | null;
  isSubmitPrimary?: boolean;
};

export type TaskFlowDiagram = {
  tasks: FlowTaskNode[];
  edges: FlowDiagramConnection[];
};

export type FlowDiagramRecord = {
  projectId: string;
  diagram: TaskFlowDiagram;
  updatedAt: string;
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

export type ProjectSummary = {
  id: string;
  name: string;
  theme: string;
  activeScreenId: string;
  createdAt: string;
  updatedAt: string;
};

export type ProjectSettings = {
  projectId: string;
  designStyle: string;
  colorPalette: string;
  brandGuidelines: string;
  componentExamples: string;
  technicalConstraints: string;
  layoutPreferences: string;
  imageGenerationNotes: string;
  generationContext: string;
  updatedAt: string;
};

export type CreateScreenResult = {
  id: string;
  name: string;
  position: number;
  updatedAt: string;
  isActive: boolean;
  lastRevision: number;
};

export type ProjectImageVersion = {
  id: string;
  prompt: string;
  createdAt: string;
  sourceType: string;
  fileName: string;
  sizeBytes: number;
};

export type ProjectImageAsset = {
  id: string;
  projectId: string;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
  currentVersionId: string;
  currentImageUrl: string;
  versions: ProjectImageVersion[];
  redoAvailable: boolean;
  rollbackAvailable: boolean;
};

const DEFAULT_BASE_URL = '/api';
const SESSION_ENDPOINT = '/session';
const SESSION_SCREENS_ENDPOINT = `${SESSION_ENDPOINT}/screens`;
const SESSION_FLOW_DIAGRAM_ENDPOINT = `${SESSION_ENDPOINT}/flow-diagram`;
const PROJECTS_ENDPOINT = '/projects';
const PROJECT_IMAGES_ENDPOINT = '/project-images';

function buildHeaders(): Record<string, string> {
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  };
}

export interface ProjectSessionServiceOptions {
  baseUrl?: string;
}

export type ProjectExportMapper = {
  version: string;
  outputPath: string;
  operations: Array<{
    op: 'set' | 'copy' | 'mapArray';
    to: string;
    from?: string;
    value?: unknown;
    itemTemplate?: Record<string, unknown>;
  }>;
};

export type ProjectExportDownload = {
  blob: Blob;
  fileName: string;
};

export type ProjectSyncResult = {
  status: string;
  upstreamStatus: number;
};

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

function addProjectQuery(url: string, projectId?: string): string {
  const trimmedProjectId = projectId?.trim();
  if (!trimmedProjectId) {
    return url;
  }
  const separator = url.includes('?') ? '&' : '?';
  return `${url}${separator}projectId=${encodeURIComponent(trimmedProjectId)}`;
}

export class ProjectSessionService {
  constructor(private readonly options: ProjectSessionServiceOptions = {}) {}

  private get baseUrl(): string {
    return this.options.baseUrl?.trim() || DEFAULT_BASE_URL;
  }

  async listProjects(): Promise<ProjectSummary[]> {
    const response = await fetch(`${this.baseUrl}${PROJECTS_ENDPOINT}`, {
      headers: buildHeaders(),
      method: 'GET',
    });
    return parseResponse<ProjectSummary[]>(response);
  }

  async createProject(name: string): Promise<ProjectSummary> {
    const response = await fetch(`${this.baseUrl}${PROJECTS_ENDPOINT}`, {
      method: 'POST',
      headers: buildHeaders(),
      body: JSON.stringify({ name }),
    });
    return parseResponse<ProjectSummary>(response);
  }

  async renameProject(projectId: string, name: string): Promise<ProjectSummary> {
    const url = `${this.baseUrl}${PROJECTS_ENDPOINT}/${encodeURIComponent(projectId)}`;
    const response = await fetch(url, {
      method: 'PATCH',
      headers: buildHeaders(),
      body: JSON.stringify({ name }),
    });
    return parseResponse<ProjectSummary>(response);
  }

  async deleteProject(projectId: string): Promise<void> {
    const url = `${this.baseUrl}${PROJECTS_ENDPOINT}/${encodeURIComponent(projectId)}`;
    const response = await fetch(url, {
      method: 'DELETE',
      headers: buildHeaders(),
    });
    await parseResponse<Record<string, string>>(response);
  }

  async getSession(projectId = ''): Promise<SessionSnapshot> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_ENDPOINT}`, projectId), {
      headers: buildHeaders(),
      method: 'GET',
    });
    return parseResponse<SessionSnapshot>(response);
  }

  async updateTheme(theme: string, projectId = ''): Promise<void> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_ENDPOINT}/theme`, projectId), {
      method: 'PATCH',
      headers: buildHeaders(),
      body: JSON.stringify({ theme }),
    });
    await parseResponse<Record<string, string>>(response);
  }

  async createScreen(name: string, projectId = ''): Promise<CreateScreenResult> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}`, projectId), {
      method: 'POST',
      headers: buildHeaders(),
      body: JSON.stringify({ name }),
    });
    return parseResponse<CreateScreenResult>(response);
  }

  async activateScreen(screenId: string, projectId = ''): Promise<void> {
    const response = await fetch(addProjectQuery(
      `${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}/activate`,
      projectId,
    ), {
      method: 'PATCH',
      headers: buildHeaders(),
      body: '{}',
    });
    await parseResponse<Record<string, string>>(response);
  }

  async renameScreen(screenId: string, name: string, projectId = ''): Promise<SessionScreenSummary> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}`, projectId), {
      method: 'PATCH',
      headers: buildHeaders(),
      body: JSON.stringify({ name }),
    });
    return parseResponse<SessionScreenSummary>(response);
  }

  async deleteScreen(screenId: string, projectId = ''): Promise<void> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}`, projectId), {
      method: 'DELETE',
      headers: buildHeaders(),
    });
    await parseResponse<Record<string, string>>(response);
  }

  async duplicateScreen(screenId: string, projectId = ''): Promise<CreateScreenResult> {
    const response = await fetch(
      addProjectQuery(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}/duplicate`, projectId),
      {
        method: 'POST',
        headers: buildHeaders(),
        body: '{}',
      },
    );
    return parseResponse<CreateScreenResult>(response);
  }

  async loadLatestState(screenId: string, projectId = ''): Promise<SessionScreenState | null> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}/state/latest`, projectId), {
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

  async loadScreenHistory(screenId: string, limit = 20, projectId = ''): Promise<SessionScreenHistory> {
    const response = await fetch(
      addProjectQuery(
        `${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}/state?limit=${encodeURIComponent(String(limit))}`,
        projectId,
      ),
      {
        headers: buildHeaders(),
        method: 'GET',
      },
    );
    return parseResponse<SessionScreenHistory>(response);
  }

  async saveScreenState(
    screenId: string,
    payload: SaveScreenStateRequest,
    projectId = '',
  ): Promise<SessionScreenState> {
    const response = await fetch(
      addProjectQuery(`${this.baseUrl}${SESSION_SCREENS_ENDPOINT}/${encodeURIComponent(screenId)}/state`, projectId),
      {
        method: 'POST',
        headers: buildHeaders(),
        body: JSON.stringify(payload),
      },
    );
    return parseResponse<SessionScreenState>(response);
  }

  async loadFlowDiagram(projectId = ''): Promise<FlowDiagramRecord> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_FLOW_DIAGRAM_ENDPOINT}`, projectId), {
      headers: buildHeaders(),
      method: 'GET',
    });
    return parseResponse<FlowDiagramRecord>(response);
  }

  async saveFlowDiagram(payload: TaskFlowDiagram, projectId = ''): Promise<FlowDiagramRecord> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_FLOW_DIAGRAM_ENDPOINT}`, projectId), {
      method: 'POST',
      headers: buildHeaders(),
      body: JSON.stringify(payload),
    });
    return parseResponse<FlowDiagramRecord>(response);
  }

  async loadProjectSettings(projectId = ''): Promise<ProjectSettings> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_ENDPOINT}/settings`, projectId), {
      headers: buildHeaders(),
      method: 'GET',
    });
    return parseResponse<ProjectSettings>(response);
  }

  async saveProjectSettings(payload: Omit<ProjectSettings, 'projectId' | 'updatedAt'>, projectId = ''): Promise<ProjectSettings> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_ENDPOINT}/settings`, projectId), {
      method: 'PATCH',
      headers: buildHeaders(),
      body: JSON.stringify(payload),
    });
    return parseResponse<ProjectSettings>(response);
  }

  async listProjectImages(projectId = ''): Promise<ProjectImageAsset[]> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${PROJECT_IMAGES_ENDPOINT}`, projectId), {
      headers: buildHeaders(),
      method: 'GET',
    });
    return parseResponse<ProjectImageAsset[]>(response);
  }

  async generateProjectImage(
    payload: { prompt: string; name?: string; description?: string; imageModel?: string; imageSize?: string; imageQuality?: string; imageStyle?: string },
    projectId = '',
  ): Promise<ProjectImageAsset> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${PROJECT_IMAGES_ENDPOINT}/generate`, projectId), {
      method: 'POST',
      headers: buildHeaders(),
      body: JSON.stringify(payload),
    });
    return parseResponse<ProjectImageAsset>(response);
  }

  async editProjectImage(imageId: string, prompt: string, projectId = ''): Promise<ProjectImageAsset> {
    const response = await fetch(
      addProjectQuery(`${this.baseUrl}${PROJECT_IMAGES_ENDPOINT}/${encodeURIComponent(imageId)}/edit`, projectId),
      {
        method: 'POST',
        headers: buildHeaders(),
        body: JSON.stringify({ prompt }),
      },
    );
    return parseResponse<ProjectImageAsset>(response);
  }

  async rollbackProjectImage(imageId: string, projectId = ''): Promise<ProjectImageAsset> {
    const response = await fetch(
      addProjectQuery(`${this.baseUrl}${PROJECT_IMAGES_ENDPOINT}/${encodeURIComponent(imageId)}/rollback`, projectId),
      {
        method: 'POST',
        headers: buildHeaders(),
        body: '{}',
      },
    );
    return parseResponse<ProjectImageAsset>(response);
  }

  async redoProjectImage(imageId: string, projectId = ''): Promise<ProjectImageAsset> {
    const response = await fetch(
      addProjectQuery(`${this.baseUrl}${PROJECT_IMAGES_ENDPOINT}/${encodeURIComponent(imageId)}/redo`, projectId),
      {
        method: 'POST',
        headers: buildHeaders(),
        body: '{}',
      },
    );
    return parseResponse<ProjectImageAsset>(response);
  }

  async uploadProjectImage(file: File, name = '', projectId = '', description = ''): Promise<ProjectImageAsset> {
    const form = new FormData();
    form.append('file', file);
    if (name.trim()) {
      form.append('name', name.trim());
    }
    if (description.trim()) {
      form.append('description', description.trim());
    }
    const response = await fetch(addProjectQuery(`${this.baseUrl}${PROJECT_IMAGES_ENDPOINT}/upload`, projectId), {
      method: 'POST',
      body: form,
      headers: {
        Accept: 'application/json',
      },
    });
    return parseResponse<ProjectImageAsset>(response);
  }

  async updateProjectImageMetadata(
    imageId: string,
    payload: { name?: string; description?: string },
    projectId = '',
  ): Promise<ProjectImageAsset> {
    const response = await fetch(
      addProjectQuery(`${this.baseUrl}${PROJECT_IMAGES_ENDPOINT}/${encodeURIComponent(imageId)}`, projectId),
      {
        method: 'PATCH',
        headers: buildHeaders(),
        body: JSON.stringify(payload),
      },
    );
    return parseResponse<ProjectImageAsset>(response);
  }

  getProjectImageDownloadUrl(imageId: string, projectId = '', versionId = ''): string {
    const path = `${this.baseUrl}${PROJECT_IMAGES_ENDPOINT}/${encodeURIComponent(imageId)}/download`;
    const withProject = addProjectQuery(path, projectId);
    if (!versionId.trim()) {
      return withProject;
    }
    return `${withProject}${withProject.includes('?') ? '&' : '?'}versionId=${encodeURIComponent(versionId.trim())}`;
  }

  async exportProject(projectId = '', mapper?: ProjectExportMapper): Promise<ProjectExportDownload> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_ENDPOINT}/export`, projectId), {
      method: 'POST',
      headers: buildHeaders(),
      body: JSON.stringify(mapper ? { mapper } : {}),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `Request failed with ${response.status}`);
    }
    const blob = await response.blob();
    const disposition = response.headers.get('Content-Disposition') || '';
    const fileNameMatch = disposition.match(/filename="([^"]+)"/i);
    const fileName = fileNameMatch?.[1]?.trim() || 'project-export.json';
    return { blob, fileName };
  }

  async syncProject(projectId = '', mapper?: ProjectExportMapper): Promise<ProjectSyncResult> {
    const response = await fetch(addProjectQuery(`${this.baseUrl}${SESSION_ENDPOINT}/sync`, projectId), {
      method: 'POST',
      headers: buildHeaders(),
      body: JSON.stringify(mapper ? { mapper } : {}),
    });
    return parseResponse<ProjectSyncResult>(response);
  }
}
