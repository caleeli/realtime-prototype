<script setup lang="ts">
import {
  computed,
  defineComponent,
  h,
  markRaw,
  type Component,
  onBeforeUnmount,
  onMounted,
  nextTick,
  ref,
  type Ref,
  watch,
} from 'vue';
import { ConnectionMode, VueFlow, Handle, Position, type EdgeMouseEvent } from '@vue-flow/core';
import '@vue-flow/core/dist/style.css';

import {
  GenerationPipelineService,
  type UXEvaluatorResultLine,
  type GenerationMessage,
  type InspirationRequest,
  type GenerationRequest,
  type GenerationPipelineResult,
  type DataGenerationRequest,
  type PugGenerationRequest,
} from './services/generationPipelineService';
import { BButton } from 'bootstrap-vue-next';
import {
  buildGeneratedScreen,
  type GeneratedScreenView,
  type GenerationRenderOptions,
} from './services/generationRenderService';
import {
  ProjectSessionService,
  type SessionScreenSummary,
  type SessionScreenState,
  type TaskFlowDiagram,
} from './services/projectSessionService';

const pipelineService = new GenerationPipelineService({
  baseUrl: import.meta.env.VITE_API_BASE_URL || 'http://localhost:3000/api',
});
const sessionService = new ProjectSessionService({
  baseUrl: import.meta.env.VITE_API_BASE_URL || 'http://localhost:3000/api',
});

function createFallbackComponent(tag: string): GenerationRenderOptions['componentLoaders'][string] {
  return () =>
    Promise.resolve(
      defineComponent({
        name: `Fallback${tag.replace(/[^a-zA-Z0-9]/g, '')}`,
        setup() {
          return () =>
            h(
              'div',
              {
                class: 'pipeline-missing',
              },
              [
                h('p', { class: 'pipeline-missing-title' }, `Componente no resuelto: ${tag}`),
                h('p', { class: 'pipeline-missing-subtitle' }, 'Se renderiza un fallback local.'),
              ],
            );
        },
      }),
    );
}

const componentLoaders: NonNullable<GenerationRenderOptions['componentLoaders']> = {
  DateRangePicker: createFallbackComponent('DateRangePicker'),
  AsyncMultiSelect: createFallbackComponent('AsyncMultiSelect'),
  InputMask: createFallbackComponent('InputMask'),
  pmTable: createFallbackComponent('pm-table'),
  'pm-table': createFallbackComponent('pm-table'),
  DropzoneUploader: createFallbackComponent('DropzoneUploader'),
  BButton: () => Promise.resolve(BButton),
  'b-button': () => Promise.resolve(BButton),
  BBtn: () => Promise.resolve(BButton),
  'b-btn': () => Promise.resolve(BButton),
  VueChart: () => import('./components/charts/VueChart'),
  'vue-chart': () => import('./components/charts/VueChart'),
  Vuechart: () => import('./components/charts/VueChart'),
};

type ChatRole = 'user' | 'assistant';

type ChatMessage = {
  role: ChatRole;
  content: string;
};

type GeneratedViewState = {
  view: GeneratedScreenView;
  component: Component;
};

type UXRecommendationSeverity = 'high' | 'medium' | 'low';

interface UXRecommendationBubble {
  id: string;
  severity: UXRecommendationSeverity;
  text: string;
  requestText: string;
}

interface DataGenerationHistoryEntry {
  instruction: string;
  previousData: unknown;
  previousMessages: GenerationMessage[];
}

interface PugGenerationHistoryEntry {
  instruction: string;
  previousPug: string;
  previousMessages: GenerationMessage[];
}

interface CssGenerationHistoryEntry {
  instruction: string;
  previousCss: string;
  previousMessages: GenerationMessage[];
}

type FlowTask = {
  id: string;
  title: string;
  screenId: string;
  isPopupTask?: boolean;
  isStartTask?: boolean;
};

type FlowConnection = {
  source?: string;
  target?: string;
  sourceHandle?: string | null;
  targetHandle?: string | null;
};

type FlowEdge = {
  id: string;
  source: string;
  target: string;
  sourceHandle?: string | null;
  targetHandle?: string | null;
  isSubmitPrimary?: boolean;
  style?: Record<string, string | number>;
};

type FlowNode = {
  id: string;
  type: 'flow-task';
  position: {
    x: number;
    y: number;
  };
  data: {
    taskId: string;
    title: string;
    screenId: string;
    isPopupTask?: boolean;
  };
};

type FlowTaskPromptNavigation = {
  id: string;
  name: string;
  route: string;
  isPopupTask: boolean;
};

type FlowTaskPreviewState = {
  component: Component | null;
  isLoading: boolean;
  error: string;
  screenId: string;
  cleanup?: (() => void) | null;
};


const promptText: Ref<string> = ref('');
const promptInput = ref<HTMLTextAreaElement | null>(null);
const conversation: Ref<ChatMessage[]> = ref([]);
const isConversationVisible = ref(false);
const isBuilderPanelMinimized = ref(true);
const isGenerating = ref(false);
const didUseInspiration = ref(false);
const message = ref('Escribe una descripción y pulsa "Generar pantalla".');
const generatedState: Ref<GeneratedViewState | null> = ref(null);
const generatedComponent: Ref<Component | null> = ref(null);
type PopupRuntimeState = {
  isOpen: boolean;
  screenId: string;
  title: string;
  component: Component | null;
  isLoading: boolean;
  error: string;
  cleanup: (() => void) | null;
};
const popupState = ref<PopupRuntimeState>({
  isOpen: false,
  screenId: '',
  title: '',
  component: null,
  isLoading: false,
  error: '',
  cleanup: null,
});
const uxEvaluations: Ref<UXEvaluatorResultLine[]> = ref([]);
const screens = ref<SessionScreenSummary[]>([]);
const activeScreenId = ref('');
const isSessionLoading = ref(false);
const isSaving = ref(false);
const lastGeneratedOutput = ref<GenerationPipelineResult | null>(null);
const isHydratingSession = ref(false);
const isScreenDirty = ref(false);
const isFlowNavigationPromptOpen = ref(false);
const unsavedNavigationScreenName = ref('');
const isSavingBeforeFlowNavigation = ref(false);
const dataEditorJson = ref('');
const dataEditorError = ref('');
const isApplyingData = ref(false);
const isApplyingDataGeneration = ref(false);
const dataInstructionText = ref('');
const dataGenerationError = ref('');
const dataGenerationHistory = ref<DataGenerationHistoryEntry[]>([]);
const dataGenerationRedoStack = ref<string[]>([]);
const dataGenerationConversation = ref<GenerationMessage[]>([]);
const isApplyingPug = ref(false);
const pugInstructionText = ref('');
const pugEditorPug = ref('');
const pugEditorError = ref('');
const isApplyingPugGeneration = ref(false);
const pugGenerationError = ref('');
const pugGenerationHistory = ref<PugGenerationHistoryEntry[]>([]);
const pugGenerationRedoStack = ref<string[]>([]);
const pugGenerationConversation = ref<GenerationMessage[]>([]);
const cssEditorCss = ref('');
const cssEditorError = ref('');
const isApplyingCss = ref(false);
const isApplyingCssGeneration = ref(false);
const cssInstructionText = ref('');
const cssGenerationError = ref('');
const cssGenerationHistory = ref<CssGenerationHistoryEntry[]>([]);
const cssGenerationRedoStack = ref<string[]>([]);
const cssGenerationConversation = ref<GenerationMessage[]>([]);
type PrimaryNav = 'builder' | 'flows' | 'execution' | 'components' | 'library' | 'settings';
type EditorWorkspaceTab = 'canvas' | 'data' | 'pug' | 'css' | 'states';
type FlowWorkspaceTab = 'canvas' | 'data' | 'states';

const primaryNav = ref<PrimaryNav>('builder');
const executionSubmitInterceptor = ref<((event: Event) => void) | null>(null);
const railCollapsed = ref(true);
const editorWorkspaceTab = ref<EditorWorkspaceTab>('canvas');
const flowWorkspaceTab = ref<FlowWorkspaceTab>('canvas');
const vueFlowRef = ref<InstanceType<typeof VueFlow> | null>(null);
const flowZoomPercent = ref(100);
const flowTaskCounter = ref(1);
const flowTasks = ref<FlowTask[]>([]);
const flowEdges = ref<FlowEdge[]>([]);
const selectedFlowEdgeId = ref('');
const flowTaskPreviews = ref<Record<string, FlowTaskPreviewState>>({});
const flowNodes = ref<FlowNode[]>([]);
const isFlowDiagramHydrating = ref(false);
const isFlowDiagramHydrated = ref(false);
const flowDiagramSaveTimer = ref<ReturnType<typeof setTimeout> | null>(null);
const explodingBubbleId = ref<string | null>(null);
const uxEvaluationStatus = ref<'idle' | 'loading' | 'ready' | 'error'>('idle');
const uxEvaluationMessage = ref('');
const cleanupStyle = ref<(() => void) | null>(null);
const screenRevision = ref(0);
const isProcessingHashNavigation = ref(false);
const popupRuntimeCounter = ref(0);
const BOOTSWATCH_VERSION = '5.3.8';
const BOOTSWATCH_LINK_ID = 'bootswatch-theme-runtime';
const FLOW_DIAGRAM_AUTO_SAVE_MS = 500;

const themeOptions: { value: string; label: string }[] = [
  { value: 'bootstrap', label: 'Bootstrap (default)' },
  { value: 'cerulean', label: 'Cerulean' },
  { value: 'cosmo', label: 'Cosmo' },
  { value: 'darkly', label: 'Darkly' },
  { value: 'flatly', label: 'Flatly' },
  { value: 'journal', label: 'Journal' },
  { value: 'litera', label: 'Litera' },
  { value: 'lux', label: 'Lux' },
  { value: 'lumen', label: 'Lumen' },
  { value: 'pulse', label: 'Pulse' },
  { value: 'sandstone', label: 'Sandstone' },
  { value: 'simplex', label: 'Simplex' },
  { value: 'sketchy', label: 'Sketchy' },
  { value: 'slate', label: 'Slate' },
  { value: 'solar', label: 'Solar' },
  { value: 'superhero', label: 'Superhero' },
  { value: 'united', label: 'United' },
  { value: 'vapor', label: 'Vapor' },
  { value: 'yeti', label: 'Yeti' },
];

type ThemeOption = (typeof themeOptions)[number]['value'];
type ThemeDirection = 'left' | 'right';

const activeTheme = ref<ThemeOption>('bootstrap');
const themeTransitionDirection = ref<ThemeDirection>('right');
const themeSwipeStartX = ref<number | null>(null);
const THEME_SWIPE_THRESHOLD = 60;

const activeThemeIndex = computed(() => {
  return themeOptions.findIndex((theme) => theme.value === activeTheme.value);
});

const activeThemeLabel = computed(() => {
  const index = activeThemeIndex.value;
  if (index < 0) {
    return 'Tema';
  }
  return themeOptions[index]?.label ?? 'Tema';
});

const themeTransitionKey = computed(() => `${screenRevision.value}-${activeTheme.value}`);

const activeScreenLabel = computed(() => {
  const match = screens.value.find((screen) => screen.id === activeScreenId.value);
  return match?.name ?? 'Sin pantalla';
});
const executionCurrentTask = computed(() => {
  if (primaryNav.value !== 'execution') {
    return null;
  }

  const currentScreenId = activeScreenId.value.trim();
  if (!currentScreenId) {
    return null;
  }

  return flowTasks.value.find((task) => task.screenId === currentScreenId) ?? null;
});
const executionStartTask = computed(() => {
  const tasksWithScreen = flowTasks.value.filter((task) => task.screenId?.trim());
  return getFlowStartTask(tasksWithScreen);
});
const executionCurrentTaskLabel = computed(() => {
  if (!executionCurrentTask.value) {
    return 'Tarea activa: sin tarea';
  }

  const taskIndex = flowTasks.value.findIndex((task) => task.id === executionCurrentTask.value?.id);
  const taskName = executionCurrentTask.value.title || `Tarea ${taskIndex + 1}`;
  return `Tarea activa: ${taskName}`;
});
const executionStartTaskLabel = computed(() => {
  if (!executionStartTask.value) {
    return '';
  }

  const taskIndex = flowTasks.value.findIndex((task) => task.id === executionStartTask.value?.id);
  const taskName = executionStartTask.value.title || `Tarea ${taskIndex + 1}`;
  return `Primera tarea: ${taskName}`;
});

const browserLocale = computed(() => (typeof navigator !== 'undefined' ? navigator.language : '—'));
const selectedFlowEdge = computed(() => flowEdges.value.find((edge) => edge.id === selectedFlowEdgeId.value) ?? null);

const FLOW_COLUMNS = 3;
const FLOW_COLUMN_GAP = 340;
const FLOW_ROW_GAP = 300;

const flowNodesWithPreviews = computed(() => {
  const taskLookup = new Map<string, FlowTask>();
  for (const task of flowTasks.value) {
    taskLookup.set(task.id, task);
  }
  return flowNodes.value.map((node) => ({
    ...node,
    task: taskLookup.get(node.id),
    preview: flowTaskPreviews.value[node.id] ?? null,
  }));
});

const flowSnapshotText = computed(() => {
  try {
    return JSON.stringify(
      {
        tasks: flowTasks.value,
        edges: flowEdges.value,
      },
      null,
      2,
    );
  } catch (_error) {
    return '{}';
  }
});

function getThemeByOffset(offset: number) {
  const index = activeThemeIndex.value;
  if (index < 0 || themeOptions.length === 0) {
    return null;
  }
  const nextIndex = (index + offset + themeOptions.length) % themeOptions.length;
  return themeOptions[nextIndex];
}

function formatScreenDataForEditor(data: unknown) {
  try {
    return JSON.stringify(data ?? {}, null, 2);
  } catch (_error) {
    return '{}';
  }
}

function cloneDataValue(value: unknown) {
  try {
    return JSON.parse(JSON.stringify(value ?? {}));
  } catch (_error) {
    return {};
  }
}

watch(lastGeneratedOutput, (output) => {
  dataEditorError.value = '';
  dataGenerationError.value = '';
  pugGenerationError.value = '';
  cssGenerationError.value = '';
  if (output) {
    dataEditorJson.value = formatScreenDataForEditor(output.data);
    pugEditorPug.value = output.sourcePug ?? '';
    cssEditorCss.value = output.css ?? '';
  } else {
    dataEditorJson.value = '';
    pugEditorPug.value = '';
    cssEditorCss.value = '';
  }
}, { immediate: true });

function clearDataGenerationHistory() {
  dataGenerationHistory.value = [];
  dataGenerationRedoStack.value = [];
  dataGenerationConversation.value = [];
  dataGenerationError.value = '';
}

function clearPugGenerationHistory() {
  pugGenerationHistory.value = [];
  pugGenerationRedoStack.value = [];
  pugGenerationConversation.value = [];
  pugGenerationError.value = '';
  pugInstructionText.value = '';
}

function clearCssGenerationHistory() {
  cssGenerationHistory.value = [];
  cssGenerationRedoStack.value = [];
  cssGenerationConversation.value = [];
  cssGenerationError.value = '';
  cssInstructionText.value = '';
}

function getFlowTaskId(): string {
  const taskId = flowTaskCounter.value;
  flowTaskCounter.value += 1;
  return `task-${taskId}`;
}

function getFlowTaskBaseLabel(index = 1): string {
  return `Tarea ${index}`;
}

function getFlowStartTask(allTasks: FlowTask[] = flowTasks.value): FlowTask | null {
  if (allTasks.length === 0) {
    return null;
  }

  const explicitStart = allTasks.find((task) => task.isStartTask === true);
  return explicitStart ?? allTasks[0];
}

function normalizeFlowTaskStartFlags(allTasks: FlowTask[]): FlowTask[] {
  if (allTasks.length === 0) {
    return [];
  }

  const startTaskId = allTasks.find((task) => task.isStartTask === true)?.id || allTasks[0].id;
  return allTasks.map((task) => ({
    ...task,
    isStartTask: task.id === startTaskId,
  }));
}

function buildFlowNodePosition(index: number) {
  const col = index % FLOW_COLUMNS;
  const row = Math.floor(index / FLOW_COLUMNS);
  return {
    x: col * FLOW_COLUMN_GAP + 24,
    y: row * FLOW_ROW_GAP + 24,
  };
}

function sanitizeFlowDiagramHandle(handle: string | null | undefined): string | null {
  const trimmed = (handle ?? '').trim();
  return trimmed === '' ? null : trimmed;
}

function normalizeTaskPosition(taskId: string, index: number, fallbackNodes: Map<string, { x: number; y: number }>): { x: number; y: number } {
  const fallback = fallbackNodes.get(taskId);
  if (fallback) {
    return fallback;
  }
  return buildFlowNodePosition(index);
}

function buildTaskFlowDiagramFromState(): TaskFlowDiagram {
  const nodeLookup = new Map<string, FlowNode>();
  for (const node of flowNodes.value) {
    nodeLookup.set(node.id, node);
  }

  const positionsLookup = new Map<string, { x: number; y: number }>();
  for (const node of flowNodes.value) {
    positionsLookup.set(node.id, { x: node.position.x, y: node.position.y });
  }

  const tasks = flowTasks.value.map((task, index) => {
    const nodePosition = nodeLookup.get(task.id)?.position;
    return {
      id: task.id,
      name: (task.title || getFlowTaskBaseLabel(index + 1)).trim(),
      screenId: task.screenId.trim(),
      isPopupTask: task.isPopupTask === true,
      isStartTask: task.isStartTask === true,
      position: {
        x: nodePosition?.x ?? buildFlowNodePosition(index).x,
        y: nodePosition?.y ?? buildFlowNodePosition(index).y,
      },
    };
  });

  const edges = flowEdges.value.map((edge) => ({
    id: edge.id,
    source: edge.source,
    target: edge.target,
    sourceHandle: sanitizeFlowDiagramHandle(edge.sourceHandle),
    targetHandle: sanitizeFlowDiagramHandle(edge.targetHandle),
    isSubmitPrimary: edge.isSubmitPrimary === true,
  }));

  for (const task of tasks) {
    if (!task.name.trim()) {
      task.name = getFlowTaskBaseLabel(tasks.indexOf(task) + 1);
    }
    if (!task.id || !task.id.trim()) {
      task.id = getFlowTaskId();
    }
    if (Number.isNaN(task.position.x) || Number.isNaN(task.position.y)) {
      const fallback = positionsLookup.get(task.id) ?? buildFlowNodePosition(tasks.indexOf(task));
      task.position = fallback;
    }
    if (task.id && positionsLookup.has(task.id)) {
      positionsLookup.delete(task.id);
    }
  }

  return {
    tasks,
    edges,
  };
}

function queueFlowDiagramPersist() {
  if (!isFlowDiagramHydrated.value || isFlowDiagramHydrating.value) {
    return;
  }

  if (flowDiagramSaveTimer.value) {
    clearTimeout(flowDiagramSaveTimer.value);
  }

  flowDiagramSaveTimer.value = setTimeout(() => {
    flowDiagramSaveTimer.value = null;
    void persistFlowDiagram();
  }, FLOW_DIAGRAM_AUTO_SAVE_MS);
}

async function persistFlowDiagram() {
  try {
    await sessionService.saveFlowDiagram(buildTaskFlowDiagramFromState());
  } catch (_error) {
    // No-op. Diagram persistence should not block editor usage.
  }
}

function applyFlowDiagram(diagram: TaskFlowDiagram) {
  const validScreenIDs = new Set(screens.value.map((screen) => screen.id));
  const nodePositions = new Map<string, { x: number; y: number }>();

  const normalizedTasks: FlowTask[] = [];
  const seenTaskIDs = new Set<string>();
  const diagramTasks = diagram.tasks || [];

  for (const entry of diagramTasks) {
    const taskPosition = {
      x: Number.isFinite(entry.position?.x) ? entry.position!.x : 0,
      y: Number.isFinite(entry.position?.y) ? entry.position!.y : 0,
    };

    let id = (entry.id || '').trim();
    if (!id || seenTaskIDs.has(id)) {
      id = getFlowTaskId();
    }
    seenTaskIDs.add(id);
    if (!taskPositionIsDefault(taskPosition)) {
      nodePositions.set(id, taskPosition);
    }

    normalizedTasks.push({
      id,
      title: entry.name?.trim() || getFlowTaskBaseLabel(normalizedTasks.length + 1),
      screenId: validScreenIDs.has((entry.screenId || '').trim()) ? entry.screenId.trim() : '',
      isPopupTask: entry.isPopupTask === true,
      isStartTask: entry.isStartTask === true,
    });
  }

  if (screens.value.length > 0) {
    const assignedScreenIds = new Set(normalizedTasks.map((task) => task.screenId).filter((screenId) => screenId));
    for (const screen of screens.value) {
      if (!assignedScreenIds.has(screen.id)) {
        normalizedTasks.push({
          id: getFlowTaskId(),
          title: screen.name,
          screenId: screen.id,
          isPopupTask: false,
          isStartTask: false,
        });
      }
    }
  }

  if (normalizedTasks.length > 0) {
    let maxSuffix = 0;
    for (const task of normalizedTasks) {
      const numeric = /^task-(\d+)$/.exec(task.id);
      if (numeric && numeric[1]) {
        const next = Number.parseInt(numeric[1], 10);
        if (Number.isFinite(next) && next > maxSuffix) {
          maxSuffix = next;
        }
      }
    }
    if (maxSuffix >= flowTaskCounter.value) {
      flowTaskCounter.value = maxSuffix + 1;
    }
  }

  const oldPositions = new Map<string, { x: number; y: number }>();
  for (const node of flowNodes.value) {
    oldPositions.set(node.id, { x: node.position.x, y: node.position.y });
  }

  flowTasks.value = normalizedTasks.map((task, index) => ({
    ...task,
    title: task.title || getFlowTaskBaseLabel(index + 1),
    isPopupTask: task.isPopupTask === true,
    isStartTask: task.isStartTask === true,
  }));
  flowTasks.value = normalizeFlowTaskStartFlags(flowTasks.value);

  const validTaskIds = new Set(flowTasks.value.map((task) => task.id));
  const normalizedEdges = (diagram.edges || [])
    .map((edge, index) => {
      const source = (edge.source || '').trim();
      const target = (edge.target || '').trim();
      if (!source || !target) {
        return null;
      }
      if (!validTaskIds.has(source) || !validTaskIds.has(target)) {
        return null;
      }
      return {
        id: (edge.id || `edge-${source}-${target}-${index + 1}`).trim(),
        source,
        target,
        sourceHandle: sanitizeFlowDiagramHandle(edge.sourceHandle),
        targetHandle: sanitizeFlowDiagramHandle(edge.targetHandle),
        isSubmitPrimary: edge.isSubmitPrimary === true,
      } as FlowEdge;
    })
    .filter((edge): edge is FlowEdge => edge !== null);

  flowEdges.value = normalizedEdges.map((edge) => ({
    ...edge,
    style: buildFlowEdgeStyle(edge.id),
  }));

  flowNodes.value = flowTasks.value.map((task, index) => {
    const position = normalizeTaskPosition(task.id, index, oldPositions);
    const restoredPosition = nodePositions.get(task.id);
    return {
      id: task.id,
      type: 'flow-task',
      position: restoredPosition ?? position,
      data: {
        taskId: task.id,
        title: task.title,
        screenId: task.screenId,
        isPopupTask: task.isPopupTask === true,
      },
    };
  });

  selectedFlowEdgeId.value = '';
  syncFlowEdgeSelectionStyle();

  for (const task of flowTasks.value) {
    void ensureFlowTaskPreview(task.id, task.screenId);
  }
}

function taskPositionIsDefault(position: { x: number; y: number }) {
  return position.x === 0 && position.y === 0;
}

async function hydrateFlowDiagramFromSession() {
  isFlowDiagramHydrating.value = true;
  try {
    const loaded = await sessionService.loadFlowDiagram();
    applyFlowDiagram(loaded?.diagram || { tasks: [], edges: [] });
  } catch (_error) {
    applyFlowDiagram({ tasks: [], edges: [] });
  } finally {
    isFlowDiagramHydrating.value = false;
    isFlowDiagramHydrated.value = true;
    queueFlowDiagramPersist();
  }
}

function clearFlowTaskPreviews() {
  for (const state of Object.values(flowTaskPreviews.value)) {
    if (state.cleanup) {
      state.cleanup();
    }
  }
  flowTaskPreviews.value = {};
}

function removeFlowTaskById(taskId: string) {
  const nextTasks = flowTasks.value.filter((task) => task.id !== taskId);
  const nextPreviews = { ...flowTaskPreviews.value };
  const cleanup = nextPreviews[taskId]?.cleanup;
  if (cleanup) {
    cleanup();
  }
  delete nextPreviews[taskId];
  flowTaskPreviews.value = nextPreviews;
  flowNodes.value = flowNodes.value.filter((node) => node.id !== taskId);
  const taskIdSet = new Set(nextTasks.map((task) => task.id));
  flowEdges.value = flowEdges.value.filter(
    (edge) => taskIdSet.has(edge.source) && taskIdSet.has(edge.target),
  );
  selectedFlowEdgeId.value = '';
  syncFlowEdgeSelectionStyle();
  flowTasks.value = normalizeFlowTaskStartFlags(nextTasks);
}

function buildFlowTaskDefaults(screenId = ''): FlowTask {
  const nextLabel = getFlowTaskBaseLabel(flowTasks.value.length + 1);
  return {
    id: getFlowTaskId(),
    title: nextLabel,
    screenId,
    isPopupTask: false,
    isStartTask: flowTasks.value.length === 0,
  };
}

function toggleFlowTaskPopupType(taskId: string) {
  flowTasks.value = flowTasks.value.map((task) =>
    task.id === taskId ? { ...task, isPopupTask: !task.isPopupTask } : task,
  );
  flowNodes.value = flowNodes.value.map((node) =>
    node.id === taskId
      ? {
          ...node,
          data: {
            ...node.data,
            isPopupTask: !Boolean(node.data.isPopupTask),
          },
        }
      : node,
  );
}

function setFlowTaskAsStart(taskId: string) {
  flowTasks.value = flowTasks.value.map((task) => ({
    ...task,
    isStartTask: task.id === taskId,
  }));
  message.value = `Tarea inicial establecida: ${flowTasks.value.find((task) => task.id === taskId)?.title || 'tarea seleccionada'}.`;
}

function syncFlowTasksToScreens(screenList: SessionScreenSummary[] = screens.value) {
  const validIds = new Set(screenList.map((screen) => screen.id));
  if (screenList.length === 0) {
    flowTaskPreviews.value = {};
    flowNodes.value = [];
    flowEdges.value = [];
    selectedFlowEdgeId.value = '';
    syncFlowEdgeSelectionStyle();
    flowTasks.value = [];
    return;
  }

  if (flowTasks.value.length === 0 && screenList.length > 0) {
    flowTasks.value = screenList.map((screen) => ({
      id: getFlowTaskId(),
      title: `${screen.name}`,
      screenId: screen.id,
      isPopupTask: false,
      isStartTask: false,
    }));
    flowTasks.value = normalizeFlowTaskStartFlags(flowTasks.value);
  } else {
    flowTasks.value = flowTasks.value.filter((task) => !task.screenId || validIds.has(task.screenId));
    flowTasks.value = flowTasks.value.map((task, index) => ({
      ...task,
      title: task.title || getFlowTaskBaseLabel(index + 1),
      isStartTask: task.isStartTask ?? false,
    }));
    const currentIds = new Set(flowTasks.value.map((task) => task.screenId));
    for (const screen of screenList) {
      const hasAssignedScreen = currentIds.has(screen.id);
      if (!hasAssignedScreen) {
        flowTasks.value = [...flowTasks.value, buildFlowTaskDefaults(screen.id)];
        currentIds.add(screen.id);
      }
    }
  }

  flowTasks.value = normalizeFlowTaskStartFlags(flowTasks.value);

  if (screenList.length > 0 && activeScreenId.value) {
    const activeTaskIndex = flowTasks.value.findIndex((task) => task.screenId === activeScreenId.value);
    if (activeTaskIndex < 0 && flowTasks.value.length > 0) {
      const nextTasks = [...flowTasks.value];
      nextTasks[0] = {
        ...nextTasks[0],
        screenId: activeScreenId.value,
      };
      flowTasks.value = nextTasks;
    }
  }

  const validTaskIds = new Set(flowTasks.value.map((task) => task.id));
  flowEdges.value = flowEdges.value.filter(
    (edge) => validTaskIds.has(edge.source) && validTaskIds.has(edge.target),
  );
  if (selectedFlowEdgeId.value) {
    const selectedStillExists = flowEdges.value.some((edge) => edge.id === selectedFlowEdgeId.value);
    if (!selectedStillExists) {
      selectedFlowEdgeId.value = '';
    }
  }
  syncFlowEdgeSelectionStyle();
  flowTaskPreviews.value = Object.fromEntries(
    Object.entries(flowTaskPreviews.value).filter(([taskId]) => validTaskIds.has(taskId)),
  );

  const oldPositions = new Map<string, { x: number; y: number }>();
  for (const node of flowNodes.value) {
    oldPositions.set(node.id, { ...node.position });
  }
  flowNodes.value = flowTasks.value.map((task, index) => {
    const position = oldPositions.get(task.id) ?? buildFlowNodePosition(index);
    return {
      id: task.id,
      type: 'flow-task',
      position,
      data: {
        taskId: task.id,
        title: task.title,
        screenId: task.screenId,
        isPopupTask: task.isPopupTask === true,
      },
    };
  });

  for (const task of flowTasks.value) {
    void ensureFlowTaskPreview(task.id, task.screenId);
  }
}

async function ensureFlowTaskPreview(taskId: string, screenId: string) {
  const previous = flowTaskPreviews.value[taskId];
  if (previous?.screenId === screenId && previous.component) {
    return;
  }
  if (previous?.cleanup) {
    previous.cleanup();
  }

  flowTaskPreviews.value = {
    ...flowTaskPreviews.value,
    [taskId]: {
      component: null,
      isLoading: true,
      error: '',
      screenId,
      cleanup: previous?.cleanup ?? null,
    },
  };

  if (!screenId) {
    flowTaskPreviews.value = {
      ...flowTaskPreviews.value,
      [taskId]: {
        component: null,
        isLoading: false,
        error: 'Asigna una pantalla para ver el preview.',
        screenId: '',
      },
    };
    return;
  }

  try {
    const state = await sessionService.loadLatestState(screenId);
    if (!state) {
      flowTaskPreviews.value = {
        ...flowTaskPreviews.value,
        [taskId]: {
          component: null,
          isLoading: false,
          error: 'Esta pantalla aún no tiene versión guardada.',
          screenId,
        },
      };
      return;
    }
    const pipelineOutput = await pipelineService.renderFromStoredState({
      pug: state.screenPayload.sourcePug,
      css: state.screenPayload.css,
      data: state.screenPayload.data,
      messages: state.screenPayload.messages,
    });
    const rendered = await buildGeneratedScreen(pipelineOutput, {
      componentLoaders,
      styleId: `flow-screen-${taskId}`,
      runtimeContext: createFlowPreviewRuntimeContext(),
    });
    const rawCleanup = rendered.installStyles();
    flowTaskPreviews.value = {
      ...flowTaskPreviews.value,
      [taskId]: {
        component: markRaw(rendered.component),
        isLoading: false,
        error: '',
        screenId,
        cleanup: rawCleanup,
      },
    };
  } catch (_error) {
    flowTaskPreviews.value = {
      ...flowTaskPreviews.value,
      [taskId]: {
        component: null,
        isLoading: false,
        error: 'No fue posible renderizar el preview.',
        screenId,
      },
    };
  }
}

function addFlowTask() {
  const defaultScreenId = screens.value[0]?.id ?? '';
  const task = buildFlowTaskDefaults(defaultScreenId);
  flowTasks.value = [...flowTasks.value, task];
  flowNodes.value = [
    ...flowNodes.value,
    {
      id: task.id,
      type: 'flow-task',
      position: buildFlowNodePosition(flowTasks.value.length - 1),
      data: {
        taskId: task.id,
        title: task.title,
        screenId: task.screenId,
        isPopupTask: task.isPopupTask === true,
      },
    },
  ];
  if (task.screenId) {
    void ensureFlowTaskPreview(task.id, task.screenId);
  }
}

function removeFlowTask(taskId: string) {
  removeFlowTaskById(taskId);
}

function setFlowTaskTitle(taskId: string, title: string) {
  flowTasks.value = flowTasks.value.map((task) => (task.id === taskId ? { ...task, title } : task));
  flowNodes.value = flowNodes.value.map((node) =>
    node.id === taskId
      ? {
          ...node,
          data: {
            ...node.data,
            taskId: taskId,
            title,
            isPopupTask: node.data.isPopupTask,
          },
        }
      : node,
  );
}

function onFlowTaskScreenChange(taskId: string, event: Event) {
  const selectedScreenId = (event.target as HTMLSelectElement).value;
  flowTasks.value = flowTasks.value.map((task) =>
    task.id === taskId ? { ...task, screenId: selectedScreenId } : task,
  );
  flowNodes.value = flowNodes.value.map((node) =>
    node.id === taskId
      ? {
          ...node,
          data: {
            ...node.data,
            taskId,
            title: node.data.title,
            screenId: selectedScreenId,
            isPopupTask: node.data.isPopupTask,
          },
        }
      : node,
  );
  void ensureFlowTaskPreview(taskId, selectedScreenId);
}

function onFlowConnect(connection: FlowConnection) {
  if (!connection.source || !connection.target) {
    return;
  }
  if (connection.source === connection.target) {
    return;
  }
  const sourceHandle = connection.sourceHandle ?? null;
  const targetHandle = connection.targetHandle ?? null;

  const exists = flowEdges.value.some(
    (edge) =>
      edge.source === connection.source &&
      edge.target === connection.target &&
      (edge.sourceHandle ?? null) === sourceHandle &&
      (edge.targetHandle ?? null) === targetHandle,
  );
  if (exists) {
    return;
  }

  flowEdges.value = [
    ...flowEdges.value,
    {
      id: `edge-${Date.now()}-${connection.source}-${sourceHandle ?? 'default'}-${connection.target}-${targetHandle ?? 'default'}`,
      source: connection.source,
      target: connection.target,
      sourceHandle,
      targetHandle,
      isSubmitPrimary: false,
    },
  ];
  selectedFlowEdgeId.value = '';
  syncFlowEdgeSelectionStyle();
}

function buildFlowEdgeStyle(edgeId: string) {
  const isSelected = selectedFlowEdgeId.value === edgeId;
  const edge = flowEdges.value.find((entry) => entry.id === edgeId);
  const isPrimary = edge?.isSubmitPrimary === true;
  return isSelected
    ? {
        stroke: 'var(--rp-primary-hover)',
        strokeWidth: isPrimary ? 5 : 3,
        markerEnd: 'url(#rp-task-flow-arrow)',
      }
    : {
        stroke: isPrimary ? 'var(--rp-primary)' : 'var(--rp-primary)',
        strokeWidth: isPrimary ? 4 : 2,
        markerEnd: 'url(#rp-task-flow-arrow)',
      };
}

function setSelectedFlowEdgeSubmitPrimary() {
  if (!selectedFlowEdge.value) {
    return;
  }

  const shouldPromote = !selectedFlowEdge.value.isSubmitPrimary;
  const sourceTaskId = selectedFlowEdge.value.source;
  const selectedEdgeId = selectedFlowEdge.value.id;

  flowEdges.value = flowEdges.value.map((edge) => {
    if (edge.id === selectedEdgeId) {
      return { ...edge, isSubmitPrimary: shouldPromote };
    }
    if (edge.source === sourceTaskId) {
      return { ...edge, isSubmitPrimary: false };
    }
    return edge;
  });
  syncFlowEdgeSelectionStyle();
}

function syncFlowEdgeSelectionStyle() {
  flowEdges.value = flowEdges.value.map((edge) => ({
    ...edge,
    style: {
      ...buildFlowEdgeStyle(edge.id),
    },
  }));
}

function removeFlowEdgeById(edgeId: string) {
  flowEdges.value = flowEdges.value.filter((edge) => edge.id !== edgeId);
  if (selectedFlowEdgeId.value === edgeId) {
    selectedFlowEdgeId.value = '';
  }
  syncFlowEdgeSelectionStyle();
}

function removeSelectedFlowEdge() {
  if (!selectedFlowEdgeId.value) {
    return;
  }
  removeFlowEdgeById(selectedFlowEdgeId.value);
}

function onFlowEdgeClick(mouseEvent: EdgeMouseEvent) {
  const edgeId = mouseEvent.edge?.id ?? '';
  if (edgeId) {
    selectedFlowEdgeId.value = edgeId;
    syncFlowEdgeSelectionStyle();
  }
}

function onFlowPaneClick() {
  selectedFlowEdgeId.value = '';
  syncFlowEdgeSelectionStyle();
}

function onFlowNodeInput(taskId: string, event: Event) {
  const nextTitle = (event.target as HTMLInputElement).value;
  setFlowTaskTitle(taskId, nextTitle);
}

async function onFlowNodeOpen(taskId: string) {
  await focusFlowTask(taskId);
}

function getFlowNodeView(taskId: string) {
  return flowNodesWithPreviews.value.find((node) => node.id === taskId);
}

function navigateToBuilder() {
  primaryNav.value = 'builder';
}

function canPromptForUnsavedScreenChanges(): boolean {
  return primaryNav.value === 'builder' && isScreenDirty.value && !!activeScreenId.value.trim();
}

function getCurrentScreenNameForPrompt() {
  const activeScreen = screens.value.find((screen) => screen.id === activeScreenId.value);
  return activeScreen?.name || 'esta pantalla';
}

function proceedToFlows() {
  isFlowNavigationPromptOpen.value = false;
  primaryNav.value = 'flows';
  syncFlowTasksToScreens(screens.value);
}

function cancelUnsavedChangesPrompt() {
  if (isSavingBeforeFlowNavigation.value) {
    return;
  }
  isFlowNavigationPromptOpen.value = false;
}

async function saveAndContinueToFlows() {
  if (isSavingBeforeFlowNavigation.value) {
    return;
  }

  isSavingBeforeFlowNavigation.value = true;
  message.value = '';
  try {
    await saveCurrentScreen();
    proceedToFlows();
  } catch (_error) {
    message.value = 'No se pudo guardar la pantalla antes de cambiar a Flujos.';
  } finally {
    isSavingBeforeFlowNavigation.value = false;
  }
}

function declineSaveAndContinueToFlows() {
  proceedToFlows();
}

async function confirmAndSaveBeforeFlowsNavigation(): Promise<boolean> {
  if (!canPromptForUnsavedScreenChanges()) {
    return true;
  }

  unsavedNavigationScreenName.value = getCurrentScreenNameForPrompt();
  isFlowNavigationPromptOpen.value = true;
  return false;
}

function getExecutionStartScreenId(): string {
  const firstTask = getFlowStartTask(flowTasks.value.filter((task) => task.screenId?.trim()));
  return firstTask?.screenId?.trim() ?? '';
}

async function navigateToExecution() {
  syncFlowTasksToScreens(screens.value);
  primaryNav.value = 'execution';
  const startScreenId = getExecutionStartScreenId();
  if (!startScreenId) {
    message.value = flowTasks.value.length
      ? 'La primera tarea no tiene pantalla asociada para iniciar la ejecución.'
      : 'No hay tareas en el flujo para ejecutar.';
    return;
  }
  await openScreen(startScreenId, { force: true });
}

function navigateToPlaceholderNav(nav: Exclude<PrimaryNav, 'builder' | 'flows' | 'execution'>) {
  primaryNav.value = nav;
}

async function navigateToFlows() {
  const canLeave = await confirmAndSaveBeforeFlowsNavigation();
  if (!canLeave) {
    return;
  }

  proceedToFlows();
}

function onTopbarPlay() {
  void navigateToExecution();
}

function onExportClick() {
  message.value = 'La exportación del proyecto estará disponible próximamente.';
}

async function onShareClick() {
  const url = typeof window !== 'undefined' ? window.location.href : '';
  try {
    if (url && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(url);
      message.value = 'Enlace del prototipo copiado al portapapeles.';
    } else {
      message.value = url || 'No hay URL para compartir.';
    }
  } catch (_error) {
    message.value = 'No se pudo copiar el enlace.';
  }
}

function onFlowViewportChangeEnd() {
  const viewport = vueFlowRef.value?.getViewport?.();
  const z = viewport?.zoom;
  if (typeof z === 'number' && Number.isFinite(z)) {
    flowZoomPercent.value = Math.round(z * 100);
  }
}

function flowZoomIn() {
  vueFlowRef.value?.zoomIn?.();
}

function flowZoomOut() {
  vueFlowRef.value?.zoomOut?.();
}

function flowFitView() {
  void vueFlowRef.value?.fitView?.();
}

async function focusFlowTask(taskId: string) {
  const task = flowTasks.value.find((item) => item.id === taskId);
  if (!task || !task.screenId) {
    return;
  }

  navigateToBuilder();
  activeScreenId.value = task.screenId;
  try {
    await openScreen(task.screenId, { force: true });
  } catch (_error) {
    message.value = 'No se pudo abrir la pantalla desde el flujo.';
  }
}

function buildPugGenerationContext() {
  return buildGenerationContextForAI();
}

function buildGenerationContextForAI() {
  const flowContext = buildFlowTaskPromptNavigationContext();
  return {
    locale: navigator.language || 'es-ES',
    theme: activeTheme.value,
    targetDensity: 'compact',
    enabledPacks: ['advanced-inputs', 'files', 'charts'],
    flowTasks: flowContext.length > 0 ? flowContext : undefined,
  };
}

function buildFlowTaskPromptNavigationContext(): FlowTaskPromptNavigation[] {
  return flowTasks.value
    .map((task) => {
      if (!task.screenId || !task.title) {
        return null;
      }
      const route = buildTaskHashForTask(task);
      return {
        id: task.id,
        name: task.title,
        route,
        isPopupTask: task.isPopupTask === true,
      };
    })
    .filter((entry): entry is FlowTaskPromptNavigation => entry !== null);
}

function resetPugEditorDraft() {
  pugEditorError.value = '';
  const output = lastGeneratedOutput.value;
  if (!output) {
    message.value = 'No hay una pantalla generada para editar el pug.';
    pugEditorPug.value = '';
    return;
  }
  pugEditorPug.value = output.sourcePug ?? '';
  if (pugGenerationConversation.value.length === 0) {
    pugGenerationConversation.value = toApiMessages(conversation.value);
  }
  return;
}

function resetCssEditorDraft() {
  cssEditorError.value = '';
  const output = lastGeneratedOutput.value;
  if (!output) {
    message.value = 'No hay una pantalla generada para editar el CSS.';
    cssEditorCss.value = '';
    return;
  }
  cssEditorCss.value = output.css ?? '';
  if (cssGenerationConversation.value.length === 0) {
    cssGenerationConversation.value = toApiMessages(conversation.value);
  }
  return;
}

function resetDataEditorDraft() {
  dataEditorError.value = '';
  dataGenerationError.value = '';
  const output = lastGeneratedOutput.value;
  if (output) {
    dataEditorJson.value = formatScreenDataForEditor(output.data);
    return;
  }
  dataEditorJson.value = '';
}

async function applyDataToCurrentOutput(parsedData: unknown) {
  const output = lastGeneratedOutput.value;
  if (!output || !generatedState.value) {
    message.value = 'No hay una pantalla cargada para aplicar los cambios.';
    return;
  }

  const nextStyleId = `pipeline-runtime-data-${screenRevision.value + 1}`;
  const updatedOutput: GenerationPipelineResult = {
    ...output,
    data: parsedData === undefined ? {} : parsedData,
  };

  const previousStyleCleanup = cleanupStyle.value;
  const renderedView = await buildGeneratedScreen(updatedOutput, {
    componentLoaders,
    styleId: nextStyleId,
    runtimeContext: createRuntimeContext(),
  });

  cleanupStyle.value = renderedView.installStyles;
  generatedState.value = {
    view: renderedView,
    component: renderedView.component,
  };
  generatedComponent.value = markRaw(renderedView.component);
  lastGeneratedOutput.value = updatedOutput;
  screenRevision.value += 1;
  isScreenDirty.value = true;

  if (previousStyleCleanup) {
    previousStyleCleanup();
  }
}

async function applyCssToCurrentOutput(css: string) {
  const output = lastGeneratedOutput.value;
  if (!output || !generatedState.value) {
    message.value = 'No hay una pantalla cargada para aplicar los cambios.';
    return;
  }

  const nextStyleId = `pipeline-runtime-css-${screenRevision.value + 1}`;
  const updatedOutput: GenerationPipelineResult = {
    ...output,
    css,
  };

  const previousStyleCleanup = cleanupStyle.value;
  const renderedView = await buildGeneratedScreen(updatedOutput, {
    componentLoaders,
    styleId: nextStyleId,
    runtimeContext: createRuntimeContext(),
  });

  cleanupStyle.value = renderedView.installStyles;
  generatedState.value = {
    view: renderedView,
    component: renderedView.component,
  };
  generatedComponent.value = markRaw(renderedView.component);
  lastGeneratedOutput.value = updatedOutput;
  screenRevision.value += 1;
  isScreenDirty.value = true;

  if (previousStyleCleanup) {
    previousStyleCleanup();
  }
}

async function applyDataEditorChanges() {
  if (isApplyingData.value) {
    return;
  }
  const output = lastGeneratedOutput.value;
  if (!output || !generatedState.value) {
    message.value = 'No hay una pantalla cargada para aplicar los cambios.';
    return;
  }

  isApplyingData.value = true;
  dataEditorError.value = '';

  try {
    const parsedData = JSON.parse(dataEditorJson.value);
    await applyDataToCurrentOutput(parsedData);
    clearDataGenerationHistory();
    message.value = 'Data actualizada en el estado actual de la pantalla.';
  } catch (error) {
    if (error instanceof SyntaxError) {
      dataEditorError.value = 'JSON inválido. Corrige el formato antes de aplicar.';
      return;
    }
    dataEditorError.value = error instanceof Error ? error.message : 'No se pudo aplicar la data.';
    message.value = dataEditorError.value;
  } finally {
    isApplyingData.value = false;
  }
}

async function applyPugToCurrentOutput(pugTemplate: string) {
  const output = lastGeneratedOutput.value;
  if (!output || !generatedState.value) {
    message.value = 'No hay una pantalla cargada para aplicar los cambios.';
    return;
  }

  const nextStyleId = `pipeline-runtime-data-${screenRevision.value + 1}`;
  const pipelineOutput = await pipelineService.renderFromStoredState({
    pug: pugTemplate,
    css: output.css ?? '',
    data: output.data,
    messages: output.messages,
  });
  const previousStyleCleanup = cleanupStyle.value;
  const renderedView = await buildGeneratedScreen(pipelineOutput, {
    componentLoaders,
    styleId: nextStyleId,
    runtimeContext: createRuntimeContext(),
  });

  cleanupStyle.value = renderedView.installStyles;
  generatedState.value = {
    view: renderedView,
    component: renderedView.component,
  };
  generatedComponent.value = markRaw(renderedView.component);
  lastGeneratedOutput.value = pipelineOutput;
  screenRevision.value += 1;
  isScreenDirty.value = true;

  if (previousStyleCleanup) {
    previousStyleCleanup();
  }
}

async function applyPugEditorChanges() {
  if (isApplyingPug.value) {
    return;
  }

  const output = lastGeneratedOutput.value;
  if (!output || !generatedState.value) {
    message.value = 'No hay una pantalla cargada para aplicar los cambios.';
    return;
  }

  isApplyingPug.value = true;
  pugEditorError.value = '';

  try {
    await applyPugToCurrentOutput(pugEditorPug.value);
    clearPugGenerationHistory();
    message.value = 'Pug actualizado en el estado actual de la pantalla.';
    isScreenDirty.value = true;
  } catch (error) {
    pugEditorError.value = error instanceof Error ? error.message : 'No se pudo aplicar el pug.';
    message.value = pugEditorError.value;
  } finally {
    isApplyingPug.value = false;
  }
}

async function applyCssEditorChanges() {
  if (isApplyingCss.value) {
    return;
  }

  const output = lastGeneratedOutput.value;
  if (!output || !generatedState.value) {
    message.value = 'No hay una pantalla cargada para aplicar el CSS.';
    return;
  }

  isApplyingCss.value = true;
  cssEditorError.value = '';

  try {
    await applyCssToCurrentOutput(cssEditorCss.value);
    clearCssGenerationHistory();
    conversation.value = normalizeChatMessages([
      ...conversation.value,
      {
        role: 'user',
        content: 'He actualizado el CSS de la pantalla manualmente.',
      },
      {
        role: 'assistant',
        content: 'CSS actualizado correctamente.',
      },
    ]);
    message.value = 'CSS actualizado en el estado actual de la pantalla.';
  } catch (error) {
    cssEditorError.value = error instanceof Error ? error.message : 'No se pudo aplicar el CSS.';
    message.value = cssEditorError.value;
  } finally {
    isApplyingCss.value = false;
  }
}

function switchTheme(direction: ThemeDirection) {
  const nextTheme = getThemeByOffset(direction === 'right' ? 1 : -1);
  if (!nextTheme) {
    return;
  }
  if (nextTheme.value === activeTheme.value) {
    return;
  }
  themeTransitionDirection.value = direction;
  activeTheme.value = nextTheme.value;
}

function isThemeHotkey(event: KeyboardEvent) {
  const key = event.key;
  if (key !== 'ArrowLeft' && key !== 'ArrowRight') {
    return;
  }

  if (event.target instanceof HTMLElement) {
    const tagName = event.target.tagName.toLowerCase();
    const editable =
      tagName === 'input' ||
      tagName === 'textarea' ||
      tagName === 'select' ||
      event.target.isContentEditable;
    if (editable) {
      return;
    }
  }

  if (key === 'ArrowLeft') {
    event.preventDefault();
    switchTheme('left');
  } else {
    event.preventDefault();
    switchTheme('right');
  }
}

function onThemeSwipeStart(event: TouchEvent) {
  const point = event.changedTouches[0];
  if (!point) {
    return;
  }
  themeSwipeStartX.value = point.clientX;
}

function onThemeSwipeEnd(event: TouchEvent) {
  const startX = themeSwipeStartX.value;
  themeSwipeStartX.value = null;

  if (startX === null) {
    return;
  }
  const point = event.changedTouches[0];
  if (!point) {
    return;
  }
  const deltaX = point.clientX - startX;
  if (Math.abs(deltaX) < THEME_SWIPE_THRESHOLD) {
    return;
  }
  if (deltaX > 0) {
    switchTheme('left');
  } else {
    switchTheme('right');
  }
}

function getBootswatchHref(theme: string): string | null {
  if (!theme || theme === 'bootstrap') {
    return null;
  }
  return `https://cdn.jsdelivr.net/npm/bootswatch@${BOOTSWATCH_VERSION}/dist/${theme}/bootstrap.min.css`;
}

function applyThemeRuntime(theme: string) {
  if (typeof document === 'undefined') {
    return;
  }

  document.documentElement.setAttribute('data-theme', theme);
  document.body.setAttribute('data-theme', theme);

  const targetHref = getBootswatchHref(theme);
  const existing = document.getElementById(BOOTSWATCH_LINK_ID) as HTMLLinkElement | null;

  if (!targetHref) {
    if (existing) {
      existing.remove();
    }
    return;
  }

  if (existing) {
    if (existing.getAttribute('href') !== targetHref) {
      existing.href = targetHref;
    }
    return;
  }

  const styleLink = document.createElement('link');
  styleLink.id = BOOTSWATCH_LINK_ID;
  styleLink.rel = 'stylesheet';
  styleLink.href = targetHref;
  styleLink.crossOrigin = 'anonymous';
  document.head.appendChild(styleLink);
}

watch(activeTheme, async (theme) => {
  applyThemeRuntime(theme);

  if (isHydratingSession.value) {
    return;
  }

  try {
    await sessionService.updateTheme(theme);
  } catch (_error) {
    // Keep UI resilient; theme remains local if persistence fails.
  }
});

onMounted(async () => {
  window.addEventListener('keydown', isThemeHotkey);
  window.addEventListener('hashchange', onHashChange);
  try {
    isHydratingSession.value = true;
    await restoreLastSession();
    await hydrateFlowDiagramFromSession();
    const hashScreenId = resolveScreenIdFromHashValue(window.location.hash);
    if (hashScreenId && hashScreenId !== activeScreenId.value) {
      await onHashChange();
    }
  } catch (_error) {
    message.value = 'No se pudo cargar la última sesión.';
    try {
      await createNewScreen();
      await hydrateFlowDiagramFromSession();
    } catch (_createError) {
      clearGeneratedState('No se pudo restaurar ni crear una pantalla. Refresca la página.');
    }
  } finally {
    isHydratingSession.value = false;
  }
  applyThemeRuntime(activeTheme.value);
});

const lastUserMessageIndex = computed(() => {
  for (let i = conversation.value.length - 1; i >= 0; i -= 1) {
    if (conversation.value[i]?.role === 'user') {
      return i;
    }
  }
  return -1;
});

const lastUserMessage = computed(() => {
  const index = lastUserMessageIndex.value;
  if (index < 0) {
    return '';
  }
  return conversation.value[index]?.content ?? '';
});

const promptPlaceholder = computed(() => {
  return lastUserMessage.value.trim() || 'Ejemplo: crea una pantalla con header, botón y tabla de tareas';
});

function normalizeChatMessages(messages: ChatMessage[]): ChatMessage[] {
  const normalized: ChatMessage[] = [];
  for (const message of messages) {
    if (!message || (message.role !== 'user' && message.role !== 'assistant')) {
      continue;
    }
    const content = message.content.trim();
    if (!content) {
      continue;
    }
    normalized.push({ role: message.role, content });
  }
  return normalized;
}

function toApiMessages(messages: ChatMessage[]): GenerationMessage[] {
  return normalizeChatMessages(messages).map((entry) => ({
    role: entry.role,
    content: entry.content,
  }));
}

function syncConversationFromBackend(messages: GenerationMessage[]) {
  const normalized = normalizeChatMessages(
    messages
      .filter((entry) => entry.role === 'user' || entry.role === 'assistant')
      .map((entry) => ({
        role: entry.role as ChatRole,
        content: String(entry.content ?? '').trim(),
      }))
      .filter((entry) => entry.content),
  );
  conversation.value = normalized;
}

function buildDataGenerationContext() {
  return buildGenerationContextForAI();
}

function popRedoInstruction(): string {
  if (dataGenerationHistory.value.length > 0) {
    return dataGenerationHistory.value[dataGenerationHistory.value.length - 1]?.instruction ?? '';
  }
  return dataGenerationRedoStack.value.pop() ?? '';
}

function clearGeneratedState(reason = 'Pantalla vacía. Genera para visualizar.'){ 
  if (cleanupStyle.value) {
    cleanupStyle.value();
    cleanupStyle.value = null;
  }
  generatedState.value = null;
  generatedComponent.value = null;
  lastGeneratedOutput.value = null;
  isScreenDirty.value = false;
  message.value = reason;
}

function resetForEmptyScreen(reason = 'Pantalla nueva vacía. Genera para visualizarla.') {
  clearGeneratedState(reason);
  conversation.value = [];
  uxEvaluations.value = [];
  clearDataGenerationHistory();
  clearPugGenerationHistory();
  clearCssGenerationHistory();
}

function getFallbackScreenIdForDeletion(removedScreenId: string): string | null {
  const ordered = [...screens.value];
  const removedIndex = ordered.findIndex((screen) => screen.id === removedScreenId);
  if (removedIndex < 0) {
    return null;
  }
  if (ordered.length === 1) {
    return null;
  }
  if (removedIndex + 1 < ordered.length) {
    return ordered[removedIndex + 1].id;
  }
  return ordered[removedIndex - 1].id;
}

async function hydrateFromSessionState(state: SessionScreenState | null) {
  if (!state) {
    clearGeneratedState('Esta pantalla aún no tiene estado guardado. Genera una versión para persistirla.');
    conversation.value = [];
    uxEvaluations.value = [];
    didUseInspiration.value = false;
    clearPugGenerationHistory();
    clearDataGenerationHistory();
    return;
  }

  const pipelineOutput = await pipelineService.renderFromStoredState({
    pug: state.screenPayload.sourcePug || '',
    css: state.screenPayload.css || '',
    data: state.screenPayload.data,
    messages: state.screenPayload.messages,
  });

  const renderedView = await buildGeneratedScreen(pipelineOutput, {
    componentLoaders,
    styleId: `pipeline-runtime-restored-${screenRevision.value + 1}`,
    runtimeContext: createRuntimeContext(),
  });

  if (cleanupStyle.value) {
    cleanupStyle.value();
  }
  cleanupStyle.value = renderedView.installStyles;
  generatedState.value = {
    view: renderedView,
    component: renderedView.component,
  };
  generatedComponent.value = markRaw(renderedView.component);
  screenRevision.value += 1;
  lastGeneratedOutput.value = pipelineOutput;
  conversation.value = normalizeChatMessages(state.conversation as ChatMessage[]);
  uxEvaluations.value = state.recommendations || [];
  isScreenDirty.value = false;
  didUseInspiration.value = state.conversation.length > 0;
  message.value = renderedView.missingComponents.length
    ? `Pantalla restaurada con componentes faltantes: ${renderedView.missingComponents.join(', ')}`
    : 'Pantalla restaurada correctamente.';
  clearDataGenerationHistory();
  clearPugGenerationHistory();
}

async function refreshScreensFromSession() {
  const session = await sessionService.getSession();
  screens.value = session.screens || [];
  syncFlowTasksToScreens(screens.value);
  if (session.theme && session.theme !== activeTheme.value) {
    activeTheme.value = session.theme;
  }
  return session;
}

function resolveScreenIdFromHashValue(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return '';
  }

  const withoutHash = trimmed.startsWith('#') ? trimmed.slice(1) : trimmed;
  const base = withoutHash.split('?')[0]?.replace(/^\/+/, '') ?? '';
  const normalized = base.trim();

  const taskMatch = normalized.match(/^task\/([^/]+)(?:\/.*)?$/i);
  if (taskMatch?.[1]) {
    const safeTaskId = decodeURIComponentSafe(taskMatch[1]);
    const targetTask = flowTasks.value.find((task) => task.id === safeTaskId);
    if (targetTask?.screenId) {
      return targetTask.screenId;
    }
  }

  const match = normalized.match(/^screen\/(.+)$/i);
  const candidate = (match?.[1] || normalized).trim();
  if (!candidate) {
    return '';
  }

  const safeCandidate = decodeURIComponentSafe(candidate);
  if (isKnownScreenId(safeCandidate)) {
    return safeCandidate;
  }

  const byName = screens.value.find((screen) => screen.name.toLowerCase() === safeCandidate.toLowerCase());
  return byName?.id ?? '';
}

function resolveTaskIdFromHashValue(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return '';
  }

  const withoutHash = trimmed.startsWith('#') ? trimmed.slice(1) : trimmed;
  const base = withoutHash.split('?')[0]?.replace(/^\/+/, '') ?? '';
  const normalized = base.trim();
  const taskMatch = normalized.match(/^task\/([^/]+)(?:\/.*)?$/i);
  if (!taskMatch?.[1]) {
    return '';
  }
  const safeTaskId = decodeURIComponentSafe(taskMatch[1]);
  return flowTasks.value.some((task) => task.id === safeTaskId) ? safeTaskId : '';
}

function buildTaskRouteSlug(raw: string): string {
  return raw
    .trim()
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/gi, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
}

function buildTaskHashForTask(task: FlowTask | null | undefined): string {
  if (!task) {
    return '#/task/';
  }
  const routeTaskId = encodeURIComponent(task.id.trim());
  const slug = buildTaskRouteSlug(task.title);
  return slug ? `#/task/${routeTaskId}/${encodeURIComponent(slug)}` : `#/task/${routeTaskId}`;
}

function isKnownTaskId(taskId: string): boolean {
  return flowTasks.value.some((task) => task.id === taskId);
}

function resolveTaskIdFromRouteReference(rawReference?: string): string {
  const trimmed = (rawReference ?? '').trim();
  if (!trimmed) {
    return '';
  }

  const fromHash = resolveTaskIdFromHashValue(trimmed);
  if (fromHash) {
    return fromHash;
  }

  const safeCandidate = decodeURIComponentSafe(trimmed);
  if (isKnownTaskId(safeCandidate)) {
    return safeCandidate;
  }

  const byName = flowTasks.value.find((task) => task.title.toLowerCase() === safeCandidate.toLowerCase());
  if (byName) {
    return byName.id;
  }

  const bySlug = flowTasks.value.find((task) => buildTaskRouteSlug(task.title) === buildTaskRouteSlug(safeCandidate));
  if (bySlug) {
    return bySlug.id;
  }

  const screenMatch = resolveScreenIdFromHashValue(trimmed);
  if (!screenMatch) {
    return '';
  }

  return flowTasks.value.find((task) => task.screenId === screenMatch)?.id ?? '';
}

function decodeURIComponentSafe(raw: string): string {
  try {
    return decodeURIComponent(raw);
  } catch (_error) {
    return raw;
  }
}

function buildScreenHash(screenId: string): string {
  const task = flowTasks.value.find((item) => item.screenId === screenId);
  if (task) {
    return buildTaskHashForTask(task);
  }
  return `#/screen/${encodeURIComponent(screenId)}`;
}

function isKnownScreenId(screenId: string): boolean {
  return screens.value.some((screen) => screen.id === screenId);
}

function resolveScreenIdFromRouteReference(rawReference?: string): string {
  const trimmed = (rawReference ?? '').trim();
  if (!trimmed) {
    return '';
  }

  const fromTaskId = resolveTaskIdFromRouteReference(trimmed);
  if (fromTaskId) {
    return flowTasks.value.find((task) => task.id === fromTaskId)?.screenId ?? '';
  }

  const fromHash = resolveScreenIdFromHashValue(trimmed);
  if (fromHash) {
    return fromHash;
  }
  if (isKnownScreenId(trimmed)) {
    return trimmed;
  }

  const byName = screens.value.find((screen) => screen.name.toLowerCase() === trimmed.toLowerCase());
  return byName?.id ?? '';
}

function syncBrowserHashForScreen(screenId: string) {
  if (typeof window === 'undefined') {
    return;
  }
  const hash = buildScreenHash(screenId);
  if (window.location.hash === hash) {
    return;
  }
  isProcessingHashNavigation.value = true;
  window.location.hash = hash;
  window.setTimeout(() => {
    if (isProcessingHashNavigation.value) {
      isProcessingHashNavigation.value = false;
    }
  }, 500);
}

function getCurrentFlowTaskForScreen(screenId: string): FlowTask | null {
  return flowTasks.value.find((task) => task.screenId === screenId) ?? null;
}

function resolveSubmitTargetScreenId(sourceScreenId: string, routeOverride?: string): string {
  const sourceTask = getCurrentFlowTaskForScreen(sourceScreenId);
  if (!sourceTask) {
    return '';
  }

  const outgoing = flowEdges.value.filter((edge) => edge.source === sourceTask.id);
  if (outgoing.length === 0) {
    return '';
  }

  if (routeOverride) {
    const explicitTaskId = resolveTaskIdFromRouteReference(routeOverride);
    if (explicitTaskId) {
      const explicitEdge = outgoing.find((edge) => edge.target === explicitTaskId);
      if (explicitEdge) {
        const explicitTargetTask = flowTasks.value.find((task) => task.id === explicitEdge.target);
        return explicitTargetTask?.screenId ?? '';
      }
    }
    const explicitScreenId = resolveScreenIdFromRouteReference(routeOverride);
    if (isKnownScreenId(explicitScreenId)) {
      return explicitScreenId;
    }
  }

  const primary = outgoing.find((edge) => edge.isSubmitPrimary === true) ?? outgoing[0];
  const targetTask = flowTasks.value.find((task) => task.id === primary?.target);
  return targetTask?.screenId ?? '';
}

async function submitCurrentScreen(routeOrScreenId?: string): Promise<void> {
  const currentScreenId = activeScreenId.value.trim();
  if (!currentScreenId) {
    message.value = 'No hay pantalla activa para ejecutar submit().';
    return;
  }

  const nextScreenId = resolveSubmitTargetScreenId(currentScreenId, routeOrScreenId);
  if (!nextScreenId) {
    message.value = 'No se encontró destino para submit(). Verifica conexiones en flujo.';
    return;
  }
  await openScreen(nextScreenId, { force: true });
}

async function openPopupScreen(routeOrScreenId?: string): Promise<void> {
  const targetScreenId = resolveScreenIdFromRouteReference(routeOrScreenId);
  if (!targetScreenId) {
    closePopupScreen();
    popupState.value = {
      isOpen: true,
      screenId: '',
      title: 'Popup',
      component: null,
      isLoading: false,
      error: 'No se encontró la pantalla para abrir como popup.',
      cleanup: null,
    };
    return;
  }

  if (popupState.value.cleanup) {
    popupState.value.cleanup();
  }

  popupRuntimeCounter.value += 1;
  const requestId = popupRuntimeCounter.value;

  popupState.value = {
    isOpen: true,
    screenId: targetScreenId,
    title: screens.value.find((screen) => screen.id === targetScreenId)?.name || targetScreenId,
    component: null,
    isLoading: true,
    error: '',
    cleanup: null,
  };

  try {
    const state = await sessionService.loadLatestState(targetScreenId);
    if (requestId !== popupRuntimeCounter.value) {
      return;
    }
    if (!state) {
      popupState.value = {
        ...popupState.value,
        isLoading: false,
        error: 'La pantalla no tiene versión guardada para mostrar.',
      };
      return;
    }

    const pipelineOutput = await pipelineService.renderFromStoredState({
      pug: state.screenPayload.sourcePug || '',
      css: state.screenPayload.css || '',
      data: state.screenPayload.data,
      messages: state.screenPayload.messages,
    });

    const rendered = await buildGeneratedScreen(pipelineOutput, {
      componentLoaders,
      styleId: `popup-screen-${targetScreenId}`,
      runtimeContext: {
        popup: openPopupScreen,
      },
    });

    if (requestId !== popupRuntimeCounter.value) {
      const cleanup = rendered.installStyles();
      cleanup();
      return;
    }

    if (popupState.value.cleanup) {
      popupState.value.cleanup();
    }
    const cleanup = rendered.installStyles();
    popupState.value = {
      ...popupState.value,
      isLoading: false,
      component: markRaw(rendered.component),
      error: '',
      cleanup,
    };
  } catch (_error) {
    popupState.value = {
      ...popupState.value,
      isLoading: false,
      error: 'No se pudo renderizar la pantalla del popup.',
      component: null,
    };
  }
}

function closePopupScreen() {
  if (popupState.value.cleanup) {
    popupState.value.cleanup();
  }
  popupState.value = {
    isOpen: false,
    screenId: '',
    title: '',
    component: null,
    isLoading: false,
    error: '',
    cleanup: null,
  };
}

async function onHashChange() {
  if (isProcessingHashNavigation.value) {
    isProcessingHashNavigation.value = false;
    return;
  }

  const screenId = resolveScreenIdFromHashValue(window.location.hash);
  if (!screenId || screenId === activeScreenId.value) {
    return;
  }
  if (!isKnownScreenId(screenId)) {
    message.value = 'Pantalla no encontrada para este identificador de hash.';
    return;
  }

  await openScreen(screenId, { force: true });
}

function createRuntimeContext() {
  return {
    popup: openPopupScreen,
  };
}

function createFlowPreviewRuntimeContext() {
  const noop = () => {
    message.value = 'Acciones de navegación no disponibles en vista previa de flujo.';
  };
  return {
    popup: noop,
  };
}

function onExecutionSubmitCapture(event: Event) {
  if (primaryNav.value !== 'execution') {
    return;
  }
  if (event.defaultPrevented) {
    return;
  }
  if (!(event.target instanceof HTMLFormElement)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
  const routeOrScreenId = event.target.getAttribute('action')?.trim() || undefined;
  void submitCurrentScreen(routeOrScreenId);
}

function attachExecutionSubmitInterceptor() {
  if (executionSubmitInterceptor.value || typeof document === 'undefined') {
    return;
  }

  const handler = (event: Event) => onExecutionSubmitCapture(event);
  document.addEventListener('submit', handler, true);
  executionSubmitInterceptor.value = handler;
}

function detachExecutionSubmitInterceptor() {
  if (!executionSubmitInterceptor.value || typeof document === 'undefined') {
    return;
  }

  document.removeEventListener('submit', executionSubmitInterceptor.value, true);
  executionSubmitInterceptor.value = null;
}

type OpenScreenOptions = { force?: boolean };

async function openScreen(screenId: string, options: OpenScreenOptions = {}) {
  const trimmed = screenId.trim();
  if (!trimmed || (isSessionLoading.value && !options.force)) {
    return;
  }

  isSessionLoading.value = true;
  try {
    resetForEmptyScreen('Cargando pantalla...');
    closePopupScreen();
    if (!isKnownScreenId(trimmed)) {
      message.value = 'Pantalla no encontrada. No se pudo abrir.';
      return;
    }
    await sessionService.activateScreen(trimmed);
    activeScreenId.value = trimmed;
    const state = await sessionService.loadLatestState(trimmed);
    await hydrateFromSessionState(state);
    const session = await refreshScreensFromSession();
    screens.value = session.screens || screens.value;
    isScreenDirty.value = state === null && screens.value.find((screen) => screen.id === trimmed)?.lastRevision === 0;
    activeScreenId.value = trimmed;
    syncFlowTasksToScreens(screens.value);
    syncBrowserHashForScreen(trimmed);
  } finally {
    isSessionLoading.value = false;
  }
}

async function restoreLastSession() {
  try {
    const session = await refreshScreensFromSession();
    activeTheme.value = session.theme || activeTheme.value;
    if (session.activeScreenId) {
      activeScreenId.value = session.activeScreenId;
      await hydrateFromSessionState(session.activeState);
      syncBrowserHashForScreen(session.activeScreenId);
    } else if (screens.value.length > 0) {
      await openScreen(screens.value[0]?.id ?? '');
    } else {
      await createNewScreen();
    }
    isScreenDirty.value = screens.value.find((screen) => screen.id === activeScreenId.value)?.lastRevision === 0;
  } catch (_error) {
    message.value = 'No se pudo cargar la sesión. Iniciando con pantalla limpia.';
    await createNewScreen();
  }
}

async function createNewScreen() {
  const nextIndex = screens.value.length + 1;
  const screenName = `Pantalla ${nextIndex}`;
  resetForEmptyScreen('Nueva pantalla creada. Genera contenido para empezar.');
  const created = await sessionService.createScreen(screenName);
  const session = await refreshScreensFromSession();
  screens.value = session.screens || screens.value;
  activeScreenId.value = created.id;
  await openScreen(created.id, { force: true });
  isScreenDirty.value = false;
  syncFlowTasksToScreens(screens.value);
}

async function saveCurrentScreen() {
  const currentScreenId = activeScreenId.value.trim();
  if (!currentScreenId) {
    message.value = 'Crea o selecciona una pantalla antes de guardar.';
    return;
  }

  isSaving.value = true;
  try {
    const output = lastGeneratedOutput.value;
    const payload = {
      conversation: conversation.value.map((entry) => ({
        role: entry.role,
        content: entry.content,
      })),
      recommendations: uxEvaluations.value,
      screenPayload: {
        sourcePug: output?.sourcePug ?? '',
        css: output?.css ?? '',
        data: output?.data ?? {},
        messages: output?.messages ?? buildUserPayloadMessages(conversation.value),
        metadata: output?.metadata,
      },
    };

    await sessionService.saveScreenState(currentScreenId, payload);
    const session = await refreshScreensFromSession();
    screens.value = session.screens || screens.value;
    isScreenDirty.value = false;
    const activeScreen = screens.value.find((screen) => screen.id === currentScreenId);
    if (activeScreen) {
      activeScreen.lastRevision += 1;
    }
    message.value = 'Estado de pantalla guardado.';
    for (const task of flowTasks.value) {
      if (task.screenId === currentScreenId) {
        void ensureFlowTaskPreview(task.id, task.screenId);
      }
    }
  } finally {
    isSaving.value = false;
  }
}

function parseUxRecommendation(observation: string) {
  const trimmed = observation.trim();
  const match = trimmed.match(/^\s*\[?\s*(high|medium|low)\s*\]?\s*(?:-|:)?\s*(.*)$/i);
  if (!match) {
    return null;
  }

  const severity = match[1].toLowerCase() as UXRecommendationSeverity;
  const payload = match[2].trim();
  if (!payload) {
    return null;
  }

  const separatorIndex = payload.indexOf(' - ');
  const recommendation =
    separatorIndex >= 0 ? payload.slice(separatorIndex + 3).trim() : payload;

  return {
    severity,
    text: payload,
    requestText:
      `${recommendation || payload}`,
  };
}

function getPendingUxRecommendationsForEvaluation(): string[] {
  const existing = uxEvaluations.value
    .map((entry) => entry.trim())
    .filter((entry) => entry);
  if (existing.length === 0) {
    return [];
  }
  return existing;
}

const actionableUxRecommendations = computed<UXRecommendationBubble[]>(() => {
  const severityPriority: Record<UXRecommendationSeverity, number> = {
    high: 0,
    medium: 1,
    low: 2,
  };

  return uxEvaluations.value
    .map((observation, index): UXRecommendationBubble | null => {
      const parsed = parseUxRecommendation(observation);
      if (!parsed) {
        return null;
      }

      return {
        id: `recommendation-${parsed.severity}-${index}`,
        severity: parsed.severity,
        text: parsed.text,
        requestText: parsed.requestText,
      };
    })
    .filter((entry): entry is UXRecommendationBubble => entry !== null)
    .sort((a, b) => {
      const priorityDelta = severityPriority[a.severity] - severityPriority[b.severity];
      return priorityDelta;
    });
});

const statusBarValidation = computed(() => {
  if (uxEvaluationStatus.value === 'loading') {
    return 'Evaluando interfaz…';
  }
  if (uxEvaluationStatus.value === 'error') {
    return uxEvaluationMessage.value || 'Error en validación UX';
  }
  if (actionableUxRecommendations.value.length > 0) {
    return `${actionableUxRecommendations.value.length} sugerencia(s) UX pendientes`;
  }
  if (uxEvaluationStatus.value === 'ready') {
    return 'Validaciones UX al día';
  }
  return 'Sin evaluación UX reciente';
});

function getUxRecommendationPriority(recommendation: string): number {
  const parsed = parseUxRecommendation(recommendation);
  if (!parsed) {
    return 99;
  }
  if (parsed.severity === 'high') {
    return 0;
  }
  if (parsed.severity === 'medium') {
    return 1;
  }
  return 2;
}

function sortUxRecommendationsByPriority(recommendations: string[]): string[] {
  return [...recommendations]
    .map((item) => item.trim())
    .filter((item) => item)
    .sort((a, b) => getUxRecommendationPriority(a) - getUxRecommendationPriority(b));
}

function getScreenSaveState(screen: SessionScreenSummary) {
  if (screen.id === activeScreenId.value && isScreenDirty.value) {
    return 'sin guardar';
  }
  return screen.lastRevision > 0 ? 'guardada' : 'sin guardar';
}

function buildUserPayloadMessages(history: ChatMessage[]): GenerationMessage[] {
  return toApiMessages(history);
}

async function renderPipeline(prompt: string, history: ChatMessage[]) {
  if (!prompt.trim()) {
    message.value = 'El prompt no puede estar vacío.';
    return;
  }

  isGenerating.value = true;
  message.value = 'Generando pantalla...';
  uxEvaluationStatus.value = 'idle';
  uxEvaluationMessage.value = '';
  const previousStyleCleanup = cleanupStyle.value;
  const nextStyleId = `pipeline-runtime-generated-${screenRevision.value + 1}`;

  const payload: InspirationRequest = {
    prompt,
    context: buildGenerationContextForAI(),
    messages: buildUserPayloadMessages(history),
  };

  try {
    const shouldUseInspirationEndpoint = !didUseInspiration.value;
    const pipelineOutput = shouldUseInspirationEndpoint
      ? await pipelineService.generateFromInspiration(payload)
      : await pipelineService.generate(payload);
    if (shouldUseInspirationEndpoint) {
      didUseInspiration.value = true;
    }
    if (pipelineOutput.messages.length > 0) {
      syncConversationFromBackend(pipelineOutput.messages);
    } else {
      conversation.value = [
        ...normalizeChatMessages(history),
        { role: 'assistant', content: 'Respuesta generada por la IA.' },
      ];
    }

    uxEvaluationStatus.value = 'loading';
    uxEvaluationMessage.value = 'Evaluando UX...';
    try {
      const existingRecommendations = getPendingUxRecommendationsForEvaluation();
      const recommendations = await pipelineService.evaluateUX({
        pug: pipelineOutput.sourcePug,
        css: pipelineOutput.css,
        data: pipelineOutput.data,
        previousRecommendations: existingRecommendations,
      });
      uxEvaluations.value = sortUxRecommendationsByPriority(recommendations);
      uxEvaluationStatus.value = 'ready';
      uxEvaluationMessage.value = '';
    } catch (error) {
      uxEvaluationStatus.value = 'error';
      uxEvaluationMessage.value = error instanceof Error ? error.message : 'No se pudo obtener las recomendaciones UX.';
      uxEvaluations.value = sortUxRecommendationsByPriority(getPendingUxRecommendationsForEvaluation());
    }

    const renderedView = await buildGeneratedScreen(pipelineOutput, {
      componentLoaders,
      styleId: nextStyleId,
      runtimeContext: createRuntimeContext(),
    });

    cleanupStyle.value = renderedView.installStyles;

    generatedState.value = {
      view: renderedView,
      component: renderedView.component,
    };
    generatedComponent.value = markRaw(renderedView.component);
    lastGeneratedOutput.value = pipelineOutput;
    clearDataGenerationHistory();
    clearPugGenerationHistory();
    screenRevision.value += 1;

    if (previousStyleCleanup) {
      previousStyleCleanup();
    }

    message.value = renderedView.missingComponents.length
      ? `Pantalla renderizada con componentes faltantes: ${renderedView.missingComponents.join(', ')}`
      : 'Pantalla renderizada correctamente.';
  isScreenDirty.value = true;
  } catch (error) {
    message.value = error instanceof Error ? error.message : 'No se pudo generar la pantalla.';
  } finally {
    isGenerating.value = false;
  }
}

async function onGenerate() {
  const trimmed = promptText.value.trim();
  if (!trimmed || isGenerating.value) {
    if (!trimmed) {
      message.value = 'El prompt no puede estar vacío.';
    }
    return;
  }

  await runGenerationFromPrompt(trimmed);
}

async function onCreateScreenClick() {
  if (isSessionLoading.value || isSaving.value || isGenerating.value) {
    return;
  }
  try {
    isSessionLoading.value = true;
    await createNewScreen();
    message.value = 'Pantalla creada.';
  } catch (_error) {
    message.value = 'No se pudo crear la pantalla.';
  } finally {
    isSessionLoading.value = false;
  }
}

async function onDeleteScreenClick() {
  const targetScreenId = activeScreenId.value.trim();
  if (!targetScreenId || isSessionLoading.value || isSaving.value || isGenerating.value) {
    return;
  }

  const targetScreen = screens.value.find((screen) => screen.id === targetScreenId);
  const confirmMessage = `¿Eliminar "${targetScreen?.name ?? 'esta pantalla'}"? Esta acción no se puede deshacer.`;
  if (!window.confirm(confirmMessage)) {
    return;
  }

  isSessionLoading.value = true;
  try {
    const nextScreenId = getFallbackScreenIdForDeletion(targetScreenId);
    await sessionService.deleteScreen(targetScreenId);
    const session = await refreshScreensFromSession();
    screens.value = session.screens || [];
    resetForEmptyScreen('Pantalla eliminada. Selecciona o crea otra pantalla.');

    if (screens.value.length === 0) {
      await createNewScreen();
      return;
    }

    const target = nextScreenId && screens.value.some((screen) => screen.id === nextScreenId) ? nextScreenId : screens.value[0]?.id;
    if (!target) {
      await createNewScreen();
      return;
    }

    activeScreenId.value = target;
    await openScreen(target, { force: true });
    message.value = 'Pantalla eliminada.';
  } catch (_error) {
    message.value = 'No se pudo eliminar la pantalla.';
  } finally {
    isSessionLoading.value = false;
  }
}

async function onSaveCurrentScreenClick() {
  if (isSaving.value || isGenerating.value) {
    return;
  }
  try {
    await saveCurrentScreen();
  } catch (_error) {
    message.value = 'No se pudo guardar la pantalla.';
  }
}

async function onSelectScreenChange() {
  if (!activeScreenId.value) {
    return;
  }
  try {
    await openScreen(activeScreenId.value);
  } catch (_error) {
    message.value = 'No se pudo abrir la pantalla.';
  }
}

async function runGenerationFromPrompt(prompt: string) {
  const normalizedPrompt = prompt.trim();
  if (!normalizedPrompt || isGenerating.value) {
    if (!normalizedPrompt) {
      message.value = 'El prompt no puede estar vacío.';
    }
    return;
  }

  conversation.value = [...normalizeChatMessages(conversation.value), { role: 'user', content: normalizedPrompt }];
  promptText.value = '';
  await renderPipeline(normalizedPrompt, conversation.value);
  focusPromptTextarea();
}

function canGenerateDataWithAI(): boolean {
  return !!lastGeneratedOutput.value && !isApplyingDataGeneration.value && !isGenerating.value;
}

async function generateDataWithPrompt(prompt: string) {
  const output = lastGeneratedOutput.value;
  if (!output) {
    dataGenerationError.value = 'No hay una pantalla cargada para actualizar data.';
    message.value = dataGenerationError.value;
    return;
  }
  if (!canGenerateDataWithAI()) {
    return;
  }

  const normalizedPrompt = prompt.trim();
  if (!normalizedPrompt) {
    dataGenerationError.value = 'La instrucción no puede estar vacía.';
    return;
  }

  isApplyingDataGeneration.value = true;
  dataGenerationError.value = '';

  const previousData = cloneDataValue(output.data);
  const payload: DataGenerationRequest = {
    prompt: normalizedPrompt,
    currentPug: output.sourcePug,
    currentData: cloneDataValue(output.data),
    context: buildDataGenerationContext(),
    messages: dataGenerationConversation.value,
  };

  try {
    const result = await pipelineService.generateData(payload);
    const updatedData = cloneDataValue(result.data);
    await applyDataToCurrentOutput(updatedData);
    dataEditorJson.value = formatScreenDataForEditor(updatedData);
    dataGenerationHistory.value.push({
      instruction: normalizedPrompt,
      previousData,
      previousMessages: dataGenerationConversation.value.map((entry) => ({
        role: entry.role,
        content: entry.content,
      })),
    });
    dataGenerationRedoStack.value = [];
    if (result.messages.length > 0) {
      dataGenerationConversation.value = result.messages;
    }
    dataGenerationError.value = '';
    message.value = 'JSON actualizado con IA y reaplicado en la pantalla actual.';
  } catch (error) {
    dataGenerationError.value = error instanceof Error ? error.message : 'No se pudo actualizar la data con IA.';
    message.value = dataGenerationError.value;
  } finally {
    isApplyingDataGeneration.value = false;
  }
}

async function onGenerateDataFromPrompt() {
  await generateDataWithPrompt(dataInstructionText.value);
}

async function onRedoDataGeneration() {
  const instruction = popRedoInstruction();
  if (!instruction) {
    message.value = 'No hay una instrucción para volver a ejecutar.';
    return;
  }
  if (!canGenerateDataWithAI()) {
    return;
  }

  dataInstructionText.value = instruction;
  await onGenerateDataFromPrompt();
}

async function rollbackDataGeneration() {
  if (!lastGeneratedOutput.value || isApplyingDataGeneration.value) {
    return;
  }

  const entry = dataGenerationHistory.value.pop();
  if (!entry) {
    message.value = 'No hay cambios de data de IA para deshacer.';
    return;
  }

  dataGenerationRedoStack.value.push(entry.instruction);
  dataGenerationConversation.value = entry.previousMessages.map((message) => ({ ...message }));

  try {
    await applyDataToCurrentOutput(entry.previousData);
    dataEditorJson.value = formatScreenDataForEditor(entry.previousData);
    message.value = 'Se descartó el último cambio de data por IA.';
  } catch (_error) {
    message.value = 'No se pudo deshacer el último cambio de data.';
  }
}

function buildCssGenerationContext() {
  return buildGenerationContextForAI();
}

function canGenerateCssWithAI(): boolean {
  return (
    !!lastGeneratedOutput.value &&
    !isApplyingCssGeneration.value &&
    !isApplyingCss.value &&
    !isGenerating.value
  );
}

function popCssRedoInstruction(): string {
  if (cssGenerationHistory.value.length > 0) {
    return cssGenerationHistory.value[cssGenerationHistory.value.length - 1]?.instruction ?? '';
  }
  return cssGenerationRedoStack.value.pop() ?? '';
}

async function generateCssWithPrompt(prompt: string) {
  const output = lastGeneratedOutput.value;
  if (!output) {
    cssGenerationError.value = 'No hay una pantalla cargada para actualizar el css.';
    message.value = cssGenerationError.value;
    return;
  }
  if (!canGenerateCssWithAI()) {
    return;
  }

  const normalizedPrompt = prompt.trim();
  if (!normalizedPrompt) {
    cssGenerationError.value = 'La instrucción para CSS no puede estar vacía.';
    return;
  }

  isApplyingCssGeneration.value = true;
  cssGenerationError.value = '';

  const previousCss = output.css ?? '';
  const currentCss = output.css ?? '';
  const requestMessages: GenerationMessage[] = [
    ...cssGenerationConversation.value,
    {
      role: 'user',
      content: `Actualiza CSS: ${normalizedPrompt}`,
    },
  ];

  const payload: GenerationRequest = {
    prompt: `Actualiza únicamente el CSS de esta pantalla sin cambiar el PUG ni el data.

Instrucción de usuario: ${normalizedPrompt}

CSS actual:
${currentCss}`,
    context: buildCssGenerationContext(),
    messages: requestMessages,
  };

  try {
    const result = await pipelineService.generate(payload);
    const updatedCss = typeof result.css === 'string' ? result.css : '';
    await applyCssToCurrentOutput(updatedCss);
    cssEditorCss.value = updatedCss;
    cssGenerationHistory.value.push({
      instruction: normalizedPrompt,
      previousCss,
      previousMessages: cssGenerationConversation.value.map((message) => ({
        role: message.role,
        content: message.content,
      })),
    });
    cssGenerationRedoStack.value = [];
    if (result.messages.length > 0) {
      cssGenerationConversation.value = result.messages;
      syncConversationFromBackend(result.messages);
    } else {
      conversation.value = normalizeChatMessages([
        ...conversation.value,
        {
          role: 'assistant',
          content: 'CSS actualizado con IA.',
        },
      ]);
    }

    cssGenerationError.value = '';
    message.value = 'CSS actualizado con IA y reaplicado en la pantalla actual.';
  } catch (error) {
    cssGenerationError.value = error instanceof Error ? error.message : 'No se pudo actualizar el css con IA.';
    message.value = cssGenerationError.value;
  } finally {
    isApplyingCssGeneration.value = false;
  }
}

async function onGenerateCssFromPrompt() {
  await generateCssWithPrompt(cssInstructionText.value);
}

async function onRedoCssGeneration() {
  const instruction = popCssRedoInstruction();
  if (!instruction) {
    message.value = 'No hay una instrucción para volver a ejecutar.';
    return;
  }
  if (!canGenerateCssWithAI()) {
    return;
  }

  cssInstructionText.value = instruction;
  await onGenerateCssFromPrompt();
}

async function rollbackCssGeneration() {
  if (!lastGeneratedOutput.value || isApplyingCssGeneration.value) {
    return;
  }

  const entry = cssGenerationHistory.value.pop();
  if (!entry) {
    message.value = 'No hay cambios de css de IA para deshacer.';
    return;
  }

  cssGenerationRedoStack.value.push(entry.instruction);
  cssGenerationConversation.value = entry.previousMessages.map((message) => ({ ...message }));

  try {
    await applyCssToCurrentOutput(entry.previousCss);
    cssEditorCss.value = entry.previousCss;
    syncConversationFromBackend(cssGenerationConversation.value);
    message.value = 'Se descartó el último cambio de CSS por IA.';
  } catch (_error) {
    message.value = 'No se pudo deshacer el último cambio de CSS.';
  }
}

function canGeneratePugWithAI(): boolean {
  return !!lastGeneratedOutput.value && !isApplyingPugGeneration.value && !isGenerating.value;
}

function popPugRedoInstruction(): string {
  if (pugGenerationHistory.value.length > 0) {
    return pugGenerationHistory.value[pugGenerationHistory.value.length - 1]?.instruction ?? '';
  }
  return pugGenerationRedoStack.value.pop() ?? '';
}

async function generatePugWithPrompt(prompt: string) {
  const output = lastGeneratedOutput.value;
  if (!output) {
    pugGenerationError.value = 'No hay una pantalla cargada para actualizar el pug.';
    message.value = pugGenerationError.value;
    return;
  }
  if (!canGeneratePugWithAI()) {
    return;
  }

  const normalizedPrompt = prompt.trim();
  if (!normalizedPrompt) {
    pugGenerationError.value = 'La instrucción para el pug no puede estar vacía.';
    return;
  }

  isApplyingPugGeneration.value = true;
  pugGenerationError.value = '';

  const previousPug = output.sourcePug ?? '';
  const payload: PugGenerationRequest = {
    prompt: normalizedPrompt,
    currentPug: output.sourcePug,
    currentCss: output.css ?? '',
    currentData: cloneDataValue(output.data),
    context: buildPugGenerationContext(),
    messages: pugGenerationConversation.value,
  };

  try {
    const result = await pipelineService.generatePug(payload);
    const updatedPug = (result.pug ?? '').toString();
    await applyPugToCurrentOutput(updatedPug);
    pugEditorPug.value = updatedPug;

    pugGenerationHistory.value.push({
      instruction: normalizedPrompt,
      previousPug,
      previousMessages: pugGenerationConversation.value.map((message) => ({
        role: message.role,
        content: message.content,
      })),
    });
    pugGenerationRedoStack.value = [];
    if (result.messages.length > 0) {
      pugGenerationConversation.value = result.messages;
      syncConversationFromBackend(result.messages);
    }

    pugGenerationError.value = '';
    message.value = 'Pug actualizado con IA y reaplicado en la pantalla actual.';
  } catch (error) {
    pugGenerationError.value = error instanceof Error ? error.message : 'No se pudo actualizar el pug con IA.';
    message.value = pugGenerationError.value;
  } finally {
    isApplyingPugGeneration.value = false;
  }
}

async function onGeneratePugFromPrompt() {
  await generatePugWithPrompt(pugInstructionText.value);
}

async function onRedoPugGeneration() {
  const instruction = popPugRedoInstruction();
  if (!instruction) {
    message.value = 'No hay una instrucción para volver a ejecutar.';
    return;
  }
  if (!canGeneratePugWithAI()) {
    return;
  }

  pugInstructionText.value = instruction;
  await onGeneratePugFromPrompt();
}

async function rollbackPugGeneration() {
  if (!lastGeneratedOutput.value || isApplyingPugGeneration.value) {
    return;
  }

  const entry = pugGenerationHistory.value.pop();
  if (!entry) {
    message.value = 'No hay cambios de pug de IA para deshacer.';
    return;
  }

  pugGenerationRedoStack.value.push(entry.instruction);
  pugGenerationConversation.value = entry.previousMessages.map((message) => ({ ...message }));

  try {
    await applyPugToCurrentOutput(entry.previousPug);
    pugEditorPug.value = entry.previousPug;
    syncConversationFromBackend(pugGenerationConversation.value);
    message.value = 'Se descartó el último cambio de pug por IA.';
  } catch (_error) {
    message.value = 'No se pudo deshacer el último cambio de pug.';
  }
}

async function onUxSuggestionClick(suggestion: UXRecommendationBubble) {
  if (isGenerating.value) {
    return;
  }

  const bubbleId = suggestion.id;
  explodingBubbleId.value = bubbleId;

  try {
    await runGenerationFromPrompt(suggestion.requestText);
  } finally {
    if (explodingBubbleId.value === bubbleId) {
      explodingBubbleId.value = null;
    }
  }
}


async function onRefresh(messageIndex: number) {
  if (isGenerating.value || messageIndex !== lastUserMessageIndex.value) {
    return;
  }
  const targetMessage = conversation.value[messageIndex];
  if (!targetMessage || targetMessage.role !== 'user') {
    return;
  }

  const truncated = conversation.value.slice(0, messageIndex + 1);
  conversation.value = normalizeChatMessages(truncated);
  await renderPipeline(targetMessage.content, conversation.value);
  focusPromptTextarea();
}

async function onRollback() {
  if (isGenerating.value || lastUserMessageIndex.value < 0) {
    return;
  }

  const truncatedConversation = normalizeChatMessages(conversation.value.slice(0, lastUserMessageIndex.value));
  conversation.value = truncatedConversation;

  const previousUserIndex = lastUserMessageIndex.value;
  if (previousUserIndex < 0) {
    clearGeneratedState('Rollback aplicado. Escribe un nuevo mensaje del usuario para generar otra respuesta.');
    focusPromptTextarea();
    return;
  }
  await onRefresh(previousUserIndex);
  return;
}

function focusPromptTextarea() {
  nextTick(() => {
    promptInput.value?.focus();
  });
}

function toggleConversationVisibility() {
  isConversationVisible.value = !isConversationVisible.value;
}

function toggleBuilderPanelMinimized() {
  isBuilderPanelMinimized.value = !isBuilderPanelMinimized.value;
  nextTick(() => {
    focusPromptTextarea();
  });
}

onBeforeUnmount(() => {
  window.removeEventListener('keydown', isThemeHotkey);
  window.removeEventListener('hashchange', onHashChange);
  if (popupState.value.cleanup) {
    popupState.value.cleanup();
  }
  if (cleanupStyle.value) {
    cleanupStyle.value();
    cleanupStyle.value = null;
  }
  detachExecutionSubmitInterceptor();
  if (flowDiagramSaveTimer.value) {
    clearTimeout(flowDiagramSaveTimer.value);
    flowDiagramSaveTimer.value = null;
  }
  clearFlowTaskPreviews();
});

watch([flowTasks, flowNodes, flowEdges], queueFlowDiagramPersist, {
  deep: true,
});
watch(
  primaryNav,
  (nextNav) => {
    if (nextNav === 'execution') {
      attachExecutionSubmitInterceptor();
      return;
    }
    detachExecutionSubmitInterceptor();
  },
  {
    flush: 'post',
  },
);

function onPromptKeydown(event: KeyboardEvent) {
  if (!(event.target instanceof HTMLTextAreaElement)) {
    return;
  }

  if ((event.ctrlKey || event.metaKey) && event.shiftKey && event.key === 'Enter') {
    event.preventDefault();
    if (!isGenerating.value && lastUserMessageIndex.value >= 0) {
      onRollback();
    }
    return;
  }

  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault();
    if (!isGenerating.value && lastUserMessageIndex.value >= 0) {
      onRefresh(lastUserMessageIndex.value);
    }
    return;
  }

  if (event.key !== 'Enter' || event.shiftKey) {
    return;
  }

  event.preventDefault();
  if (!isGenerating.value) {
    onGenerate();
  }
}
</script>

<template>
  <main class="builder-root" :data-theme="activeTheme">
    <header class="app-topbar">
      <div class="app-topbar-brand">
        <span class="app-topbar-logo" aria-hidden="true">✦</span>
        <div class="app-topbar-titles">
          <span class="app-topbar-name">Rapid Prototype Builder</span>
          <span class="app-topbar-tagline">Vista previa y flujos con IA</span>
        </div>
      </div>
      <div class="app-topbar-actions">
        <button type="button" class="app-icon-btn" title="Ejecutar prototype" aria-label="Ejecutar prototype" @click="onTopbarPlay">
          <i class="bi bi-play-fill" aria-hidden="true"></i>
        </button>
        <button type="button" class="app-text-btn" @click="onExportClick">
          <i class="bi bi-download" aria-hidden="true"></i>
          Exportar
        </button>
        <button type="button" class="app-text-btn" @click="onShareClick">
          <i class="bi bi-share" aria-hidden="true"></i>
          Compartir
        </button>
        <div class="app-avatar" aria-hidden="true">RP</div>
      </div>
    </header>

    <div class="app-body">
      <aside class="app-rail" :class="{ 'app-rail--collapsed': railCollapsed }">
        <nav class="app-rail-nav" aria-label="Secciones principales">
          <button
            type="button"
            class="app-rail-item"
            :class="{ 'app-rail-item--active': primaryNav === 'builder' }"
            @click="navigateToBuilder()"
          >
            <i class="bi bi-brush" aria-hidden="true"></i>
            <span class="app-rail-label">Builder</span>
          </button>
          <button
            type="button"
            class="app-rail-item"
            :class="{ 'app-rail-item--active': primaryNav === 'flows' }"
            @click="navigateToFlows()"
          >
            <i class="bi bi-diagram-3" aria-hidden="true"></i>
            <span class="app-rail-label">Flujos</span>
          </button>
        <button
          type="button"
          class="app-rail-item"
          :class="{ 'app-rail-item--active': primaryNav === 'execution' }"
          @click="navigateToExecution()"
        >
          <i class="bi bi-play-btn-fill" aria-hidden="true"></i>
          <span class="app-rail-label">Ejecución</span>
        </button>
          <button
            type="button"
            class="app-rail-item"
            :class="{ 'app-rail-item--active': primaryNav === 'components' }"
            @click="navigateToPlaceholderNav('components')"
          >
            <i class="bi bi-grid-1x2" aria-hidden="true"></i>
            <span class="app-rail-label">Componentes</span>
          </button>
          <button
            type="button"
            class="app-rail-item"
            :class="{ 'app-rail-item--active': primaryNav === 'library' }"
            @click="navigateToPlaceholderNav('library')"
          >
            <i class="bi bi-collection" aria-hidden="true"></i>
            <span class="app-rail-label">Biblioteca</span>
          </button>
          <button
            type="button"
            class="app-rail-item"
            :class="{ 'app-rail-item--active': primaryNav === 'settings' }"
            @click="navigateToPlaceholderNav('settings')"
          >
            <i class="bi bi-gear" aria-hidden="true"></i>
            <span class="app-rail-label">Ajustes</span>
          </button>
        </nav>
        <button
          type="button"
          class="app-rail-collapse"
          :aria-expanded="!railCollapsed"
          :title="railCollapsed ? 'Expandir barra lateral' : 'Contraer barra lateral'"
          @click="railCollapsed = !railCollapsed"
        >
          <i class="bi" :class="railCollapsed ? 'bi-chevron-double-right' : 'bi-chevron-double-left'" aria-hidden="true"></i>
          <span class="visually-hidden">{{ railCollapsed ? 'Expandir navegación' : 'Contraer navegación' }}</span>
        </button>
      </aside>

      <aside
        v-if="primaryNav === 'builder'"
        class="builder-lateral"
        :class="{ 'builder-lateral--minimized': isBuilderPanelMinimized }"
        aria-label="Panel del builder"
      >
        <div class="builder-lateral-header">
          <div class="builder-lateral-title-wrap">
            <h2 class="builder-lateral-title">Builder</h2>
            <p v-if="!isBuilderPanelMinimized" class="builder-lateral-sub">Describe la pantalla que quieres generar.</p>
          </div>
          <button
            type="button"
            class="builder-lateral-minimize-btn"
            :aria-expanded="!isBuilderPanelMinimized"
            :title="isBuilderPanelMinimized ? 'Expandir panel' : 'Minimizar panel'"
            :aria-label="isBuilderPanelMinimized ? 'Expandir panel del builder' : 'Minimizar panel del builder'"
            @click="toggleBuilderPanelMinimized"
          >
            <i class="bi" :class="isBuilderPanelMinimized ? 'bi-arrows-angle-expand' : 'bi-arrows-angle-contract'" aria-hidden="true"></i>
          </button>
        </div>

        <div class="floating-prompt-title">
          <h3 class="builder-lateral-section-title">Prompt</h3>
          <button
            type="button"
            class="conversation-toggle-btn"
            :aria-expanded="isConversationVisible"
            aria-controls="conversation-list"
            :title="isConversationVisible ? 'Ocultar historial' : 'Mostrar historial'"
            :aria-label="isConversationVisible ? 'Ocultar historial de conversación' : 'Mostrar historial de conversación'"
            @click="toggleConversationVisibility"
          >
            <i class="bi bi-chat-left-text" aria-hidden="true"></i>
          </button>
        </div>
        <div v-if="isConversationVisible" id="conversation-list" class="conversation-list">
          <div v-if="conversation.length === 0" class="conversation-empty">
            Aún no hay mensajes. Escribe uno y pulsa ▶ para comenzar.
          </div>
          <div
            v-for="(entry, index) in conversation"
            :key="`${entry.role}-${index}`"
            class="conversation-row"
            :class="entry.role"
          >
            <div class="conversation-content">
              <span v-if="entry.role === 'user'">{{ entry.content }}</span>
              <span v-else class="assistant-icon">📟</span>
            </div>
          </div>
        </div>
        <div v-if="actionableUxRecommendations.length > 0 || isGenerating" class="ux-recommendation-bubbles">
          <TransitionGroup
            name="ux-bubble"
            tag="div"
            class="ux-recommendation-bubble-list"
            appear
          >
            <b-button
              v-for="suggestion in actionableUxRecommendations"
              :key="suggestion.id"
              type="button"
              class="ux-recommendation-bubble"
              :variant="suggestion.severity === 'high' ? 'danger' : suggestion.severity === 'medium' ? 'warning' : 'dark'"
              v-b-tooltip="{ title: suggestion.text }"
              :class="{
                'ux-recommendation-bubble--high': suggestion.severity === 'high',
                'ux-recommendation-bubble--medium': suggestion.severity === 'medium',
                'ux-recommendation-bubble--low': suggestion.severity === 'low',
                'ux-recommendation-bubble--burst': explodingBubbleId === suggestion.id,
              }"
              :aria-label="suggestion.text"
              @click="onUxSuggestionClick(suggestion)"
            >
              <span class="ux-recommendation-bubble-letter">
                {{ suggestion.severity === 'high' ? 'H' : suggestion.severity === 'medium' ? 'M' : 'L' }}
              </span>
              <span class="ux-recommendation-text-visually-hidden">{{ suggestion.text }}</span>
            </b-button>
          </TransitionGroup>
        </div>
        <textarea
          ref="promptInput"
          v-model="promptText"
          :rows="isBuilderPanelMinimized ? 3 : 5"
          class="builder-prompt-textarea"
          :placeholder="promptPlaceholder"
          :disabled="isGenerating"
          @keydown="onPromptKeydown"
        ></textarea>
        <div class="prompt-actions">
          <button
            type="button"
            class="prompt-action-generate prompt-action-btn"
            :disabled="isGenerating"
            title="Generar pantalla (Enter)"
            aria-label="Generar pantalla"
            @click="onGenerate"
          >
            <span v-if="isGenerating" class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span>
            <i v-else class="bi bi-play-fill" aria-hidden="true"></i>
            <span class="visually-hidden">Generar pantalla</span>
          </button>
          <button
            type="button"
            class="conversation-refresh prompt-action-btn"
            :disabled="isGenerating || lastUserMessageIndex < 0"
            title="Regenerar desde el último mensaje (Ctrl + Enter)"
            aria-label="Regenerar desde el último mensaje"
            @click="onRefresh(lastUserMessageIndex)"
          >
            <i class="bi bi-arrow-clockwise" aria-hidden="true"></i>
            <span class="visually-hidden">Regenerar desde el último mensaje</span>
          </button>
          <button
            type="button"
            class="conversation-rollback prompt-action-btn"
            :disabled="isGenerating || lastUserMessageIndex < 0"
            title="Quitar último mensaje del usuario (Ctrl + Shift + Enter)"
            aria-label="Quitar último mensaje del usuario y respuestas siguientes"
            @click="onRollback"
          >
            <i class="bi bi-arrow-counterclockwise" aria-hidden="true"></i>
            <span class="visually-hidden">Quitar último mensaje del usuario y respuestas siguientes</span>
          </button>
        </div>

        <div v-if="!isBuilderPanelMinimized" class="builder-context">
          <div class="builder-context-header">
            <span class="builder-context-heading">Contexto</span>
          </div>
          <dl class="builder-context-list">
            <div class="builder-context-row">
              <dt>Locale</dt>
              <dd>{{ browserLocale }}</dd>
            </div>
            <div class="builder-context-row">
              <dt>Packs</dt>
              <dd class="builder-context-packs">
                <span class="builder-pack-chip">advanced-inputs</span>
                <span class="builder-pack-chip">files</span>
                <span class="builder-pack-chip">charts</span>
              </dd>
            </div>
            <div class="builder-context-row builder-context-row--theme">
              <dt>Tema</dt>
              <dd>
                <label class="theme-control theme-control--compact" @touchstart="onThemeSwipeStart" @touchend="onThemeSwipeEnd">
                  <div class="theme-switch" aria-label="Cambio rápido de tema">
                    <button type="button" class="theme-switch-btn" @click="switchTheme('left')" title="Tema anterior (←)">
                      ◀
                    </button>
                    <span class="theme-current">{{ activeThemeLabel }}</span>
                    <button type="button" class="theme-switch-btn" @click="switchTheme('right')" title="Tema siguiente (→)">
                      ▶
                    </button>
                  </div>
                  <small class="theme-hint">← / →</small>
                </label>
              </dd>
            </div>
          </dl>
        </div>

        <div
          v-if="!isBuilderPanelMinimized && uxEvaluationStatus === 'ready' && actionableUxRecommendations.length === 0 && !isGenerating"
          class="builder-feedback-ok"
        >
          <i class="bi bi-check-circle-fill" aria-hidden="true"></i>
          <span>Flujo de pantalla sin sugerencias UX pendientes.</span>
        </div>
        <p class="prompt-msg builder-prompt-msg">{{ message }}</p>
      </aside>

      <div class="app-main">
        <section v-if="primaryNav === 'builder'" class="canvas-wrap">
      <header class="canvas-header">
        <div class="canvas-workspace-head">
          <div class="workspace-tabs" role="tablist" aria-label="Vista del lienzo">
            <button
              type="button"
              role="tab"
              class="workspace-tab"
              :class="{ 'workspace-tab--active': editorWorkspaceTab === 'canvas' }"
              :aria-selected="editorWorkspaceTab === 'canvas'"
              @click="editorWorkspaceTab = 'canvas'"
            >
              Lienzo
            </button>
            <button
              type="button"
              role="tab"
              class="workspace-tab"
              :class="{ 'workspace-tab--active': editorWorkspaceTab === 'data' }"
              :aria-selected="editorWorkspaceTab === 'data'"
              @click="editorWorkspaceTab = 'data'"
            >
              Datos
            </button>
            <button
              type="button"
              role="tab"
              class="workspace-tab"
              :class="{ 'workspace-tab--active': editorWorkspaceTab === 'pug' }"
              :aria-selected="editorWorkspaceTab === 'pug'"
              @click="editorWorkspaceTab = 'pug'"
            >
              PUG
            </button>
            <button
              type="button"
              role="tab"
              class="workspace-tab"
              :class="{ 'workspace-tab--active': editorWorkspaceTab === 'css' }"
              :aria-selected="editorWorkspaceTab === 'css'"
              @click="editorWorkspaceTab = 'css'"
            >
              CSS
            </button>
            <button
              type="button"
              role="tab"
              class="workspace-tab"
              :class="{ 'workspace-tab--active': editorWorkspaceTab === 'states' }"
              :aria-selected="editorWorkspaceTab === 'states'"
              @click="editorWorkspaceTab = 'states'"
            >
              Estados
            </button>
          </div>
          <div class="screen-toolbar">
            <label>
              <i class="bi bi-collection" aria-hidden="true"></i>
              Pantallas
              <select
                v-model="activeScreenId"
                class="screen-select"
                :disabled="isSessionLoading || isSaving || screens.length === 0"
                @change="onSelectScreenChange"
              >
                <option v-for="screen in screens" :key="screen.id" :value="screen.id">
                  {{ screen.name }} ({{ getScreenSaveState(screen) }})
                </option>
              </select>
            </label>
            <button type="button" class="screen-action-btn" :disabled="isSessionLoading || isSaving" @click="onCreateScreenClick">
              <i class="bi bi-plus-lg" aria-hidden="true"></i>
              Nueva
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isSessionLoading || isSaving || !activeScreenId"
              @click="onDeleteScreenClick"
            >
              <i class="bi bi-trash3" aria-hidden="true"></i>
              Eliminar
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isSessionLoading || isSaving || !activeScreenId"
              @click="onSaveCurrentScreenClick"
            >
              <i class="bi bi-save2" aria-hidden="true"></i>
              {{ isSaving ? 'Guardando...' : 'Guardar' }}
            </button>
          </div>
        </div>
      </header>

      <article v-show="editorWorkspaceTab === 'canvas'" class="canvas-surface">
        <Transition :name="themeTransitionDirection === 'left' ? 'canvas-swipe-left' : 'canvas-swipe-right'" mode="out-in">
          <div v-if="generatedComponent" :key="themeTransitionKey" class="canvas-content">
            <component :is="generatedComponent" />
          </div>
          <div v-else :key="`empty-${activeTheme}`" class="canvas-state">{{ message }}</div>
        </Transition>
        <div v-if="popupState.isOpen" class="screen-popup-backdrop" role="dialog" aria-modal="true" aria-label="Pantalla modal" @click="closePopupScreen">
          <div class="screen-popup-panel" @click.stop>
            <header class="screen-popup-header">
              <strong>{{ popupState.title || 'Popup' }}</strong>
              <button type="button" class="screen-popup-close" @click="closePopupScreen">Cerrar</button>
            </header>
            <div class="screen-popup-content">
              <p v-if="popupState.isLoading" class="screen-popup-message">Cargando pantalla popup...</p>
              <p v-else-if="popupState.error" class="screen-popup-message screen-popup-error">{{ popupState.error }}</p>
              <component v-else-if="popupState.component" :is="popupState.component" />
              <p v-else class="screen-popup-message">No hay contenido para mostrar.</p>
            </div>
          </div>
        </div>
        <div v-if="isGenerating" class="canvas-status-layer">
          <div class="canvas-status-chip">
            <span class="canvas-status-dot" aria-hidden="true"></span>
            {{ generatedComponent ? 'Actualizando pantalla...' : 'Generando pantalla...' }}
          </div>
        </div>
      </article>

      <article v-show="editorWorkspaceTab === 'data'" class="canvas-surface editor-tab-panel editor-tab-panel--data">
        <template v-if="!lastGeneratedOutput">
          <p class="editor-data-empty">Genera o abre una pantalla para editar el JSON de datos y usar el asistente de IA.</p>
        </template>
        <div
          v-else
          class="data-editor-panel"
          role="region"
          aria-label="Editor de data JSON"
        >
          <header class="data-editor-header data-editor-header--embedded">
            <h3>Data JSON</h3>
          </header>
          <label class="data-editor-input-label" for="dataInstructionInput">Instrucción para IA</label>
          <textarea
            id="dataInstructionInput"
            v-model="dataInstructionText"
            rows="3"
            class="data-editor-instruction-textarea"
            placeholder="Ej: Agrega 3 productos al arreglo de productos"
            :disabled="isApplyingDataGeneration || isApplyingData"
          ></textarea>
          <div class="data-editor-inline-actions">
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isApplyingDataGeneration || isGenerating || !dataInstructionText.trim().length"
              @click="onGenerateDataFromPrompt"
            >
              {{ isApplyingDataGeneration ? 'Llamando IA...' : 'Aplicar con IA' }}
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isApplyingDataGeneration || dataGenerationHistory.length === 0"
              @click="rollbackDataGeneration"
            >
              Rollback
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isApplyingDataGeneration || (dataGenerationHistory.length === 0 && dataGenerationRedoStack.length === 0)"
              @click="onRedoDataGeneration"
            >
              Re-do
            </button>
          </div>
          <p v-if="dataGenerationError" class="data-editor-error">{{ dataGenerationError }}</p>
          <textarea
            v-model="dataEditorJson"
            rows="14"
            class="data-editor-textarea data-editor-textarea--embedded"
            :disabled="isApplyingData"
          ></textarea>
          <p v-if="dataEditorError" class="data-editor-error">{{ dataEditorError }}</p>
          <div class="data-editor-actions">
            <button type="button" class="screen-action-btn" :disabled="isApplyingData" @click="resetDataEditorDraft">
              Cancelar
            </button>
            <button
              type="button"
              class="screen-action-btn data-editor-apply-btn"
              :disabled="isApplyingData || !dataEditorJson.trim().length"
              @click="applyDataEditorChanges"
            >
              {{ isApplyingData ? 'Aplicando...' : 'Aplicar cambios' }}
            </button>
          </div>
        </div>
      </article>

      <article v-show="editorWorkspaceTab === 'pug'" class="canvas-surface editor-tab-panel">
        <template v-if="!lastGeneratedOutput">
          <p class="editor-data-empty">Genera o abre una pantalla para editar el PUG y usar el asistente de IA.</p>
        </template>
        <div
          v-else
          class="data-editor-panel"
          role="region"
          aria-label="Editor de PUG"
        >
          <header class="data-editor-header data-editor-header--embedded">
            <h3>Editar PUG</h3>
          </header>
          <label class="data-editor-input-label" for="pugInstructionInput">Instrucción para IA</label>
          <textarea
            id="pugInstructionInput"
            v-model="pugInstructionText"
            rows="3"
            class="data-editor-instruction-textarea"
            placeholder="Ej: Sustituye el formulario actual por una tabla con paginación"
            :disabled="isApplyingPugGeneration || isApplyingPug"
          ></textarea>
          <div class="data-editor-inline-actions">
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isApplyingPugGeneration || isGenerating || !pugInstructionText.trim().length"
              @click="onGeneratePugFromPrompt"
            >
              {{ isApplyingPugGeneration ? 'Llamando IA...' : 'Aplicar con IA' }}
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isApplyingPugGeneration || pugGenerationHistory.length === 0"
              @click="rollbackPugGeneration"
            >
              Rollback
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isApplyingPugGeneration || (pugGenerationHistory.length === 0 && pugGenerationRedoStack.length === 0)"
              @click="onRedoPugGeneration"
            >
              Re-do
            </button>
          </div>
          <p v-if="pugGenerationError" class="data-editor-error">{{ pugGenerationError }}</p>
          <textarea
            v-model="pugEditorPug"
            rows="14"
            class="data-editor-textarea data-editor-textarea--embedded"
            :disabled="isApplyingPug"
          ></textarea>
          <p v-if="pugEditorError" class="data-editor-error">{{ pugEditorError }}</p>
          <div class="data-editor-actions">
            <button type="button" class="screen-action-btn" :disabled="isApplyingPug" @click="resetPugEditorDraft">
              Cancelar
            </button>
            <button
              type="button"
              class="screen-action-btn data-editor-apply-btn"
              :disabled="isApplyingPug || !pugEditorPug.trim().length"
              @click="applyPugEditorChanges"
            >
              {{ isApplyingPug ? 'Aplicando...' : 'Aplicar cambios' }}
            </button>
          </div>
        </div>
      </article>

      <article v-show="editorWorkspaceTab === 'css'" class="canvas-surface editor-tab-panel">
        <template v-if="!lastGeneratedOutput">
          <p class="editor-data-empty">Genera o abre una pantalla para editar el CSS y usar el asistente de IA.</p>
        </template>
        <div
          v-else
          class="data-editor-panel"
          role="region"
          aria-label="Editor de CSS"
        >
          <header class="data-editor-header data-editor-header--embedded">
            <h3>Editar CSS</h3>
          </header>
          <label class="data-editor-input-label" for="cssInstructionInput">Instrucción para IA</label>
          <textarea
            id="cssInstructionInput"
            v-model="cssInstructionText"
            rows="3"
            class="data-editor-instruction-textarea"
            placeholder="Ej: Cambia el fondo del contenedor principal y mejora la legibilidad de texto"
            :disabled="isApplyingCssGeneration || isApplyingCss"
          ></textarea>
          <div class="data-editor-inline-actions">
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isApplyingCssGeneration || isGenerating || !cssInstructionText.trim().length"
              @click="onGenerateCssFromPrompt"
            >
              {{ isApplyingCssGeneration ? 'Llamando IA...' : 'Aplicar con IA' }}
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isApplyingCssGeneration || cssGenerationHistory.length === 0"
              @click="rollbackCssGeneration"
            >
              Rollback
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isApplyingCssGeneration || (cssGenerationHistory.length === 0 && cssGenerationRedoStack.length === 0)"
              @click="onRedoCssGeneration"
            >
              Re-do
            </button>
          </div>
          <p v-if="cssGenerationError" class="data-editor-error">{{ cssGenerationError }}</p>
          <textarea
            v-model="cssEditorCss"
            rows="16"
            class="data-editor-textarea data-editor-textarea--embedded"
            :disabled="isApplyingCss"
          ></textarea>
          <p v-if="cssEditorError" class="data-editor-error">{{ cssEditorError }}</p>
          <div class="data-editor-actions">
            <button type="button" class="screen-action-btn" :disabled="isApplyingCss" @click="resetCssEditorDraft">
              Cancelar
            </button>
            <button
              type="button"
              class="screen-action-btn data-editor-apply-btn"
              :disabled="isApplyingCss"
              @click="applyCssEditorChanges"
            >
              {{ isApplyingCss ? 'Aplicando...' : 'Aplicar cambios' }}
            </button>
          </div>
        </div>
      </article>

      <article v-show="editorWorkspaceTab === 'states'" class="canvas-surface editor-tab-panel">
        <p class="editor-states-hint">
          Los estados de navegación entre pantallas se gestionan en la vista <strong>Flujos</strong> del menú lateral.
        </p>
      </article>
    </section>

    <section v-else-if="primaryNav === 'flows'" class="canvas-wrap canvas-wrap--flow">
      <svg aria-hidden="true" class="flow-edge-marker-defs">
        <defs>
          <marker
            id="rp-task-flow-arrow"
            viewBox="0 0 12 12"
            refX="11"
            refY="6"
            markerWidth="8"
            markerHeight="8"
            orient="auto"
            markerUnits="strokeWidth"
          >
            <path d="M 0 0 L 12 6 L 0 12 z" />
          </marker>
        </defs>
      </svg>
      <article class="canvas-surface flow-surface">
        <div class="flow-workspace-head">
          <div class="flow-toolbar flow-toolbar--split">
            <div class="flow-toolbar-left">
              <div class="workspace-tabs workspace-tabs--flow" role="tablist" aria-label="Vista del flujo">
                <button
                  type="button"
                  role="tab"
                  class="workspace-tab"
                  :class="{ 'workspace-tab--active': flowWorkspaceTab === 'canvas' }"
                  :aria-selected="flowWorkspaceTab === 'canvas'"
                  @click="flowWorkspaceTab = 'canvas'"
                >
                  Lienzo
                </button>
                <button
                  type="button"
                  role="tab"
                  class="workspace-tab"
                  :class="{ 'workspace-tab--active': flowWorkspaceTab === 'data' }"
                  :aria-selected="flowWorkspaceTab === 'data'"
                  @click="flowWorkspaceTab = 'data'"
                >
                  Datos
                </button>
                <button
                  type="button"
                  role="tab"
                  class="workspace-tab"
                  :class="{ 'workspace-tab--active': flowWorkspaceTab === 'states' }"
                  :aria-selected="flowWorkspaceTab === 'states'"
                  @click="flowWorkspaceTab = 'states'"
                >
                  Estados
                </button>
              </div>
            </div>
            <div class="flow-toolbar-actions">
              <div class="flow-zoom-controls" aria-label="Zoom del lienzo">
                <span class="flow-zoom-readout">{{ flowZoomPercent }}%</span>
                <button type="button" class="screen-action-btn flow-zoom-btn" title="Alejar" aria-label="Alejar" @click="flowZoomOut">
                  <i class="bi bi-zoom-out" aria-hidden="true"></i>
                </button>
                <button type="button" class="screen-action-btn flow-zoom-btn" title="Acercar" aria-label="Acercar" @click="flowZoomIn">
                  <i class="bi bi-zoom-in" aria-hidden="true"></i>
                </button>
                <button
                  type="button"
                  class="screen-action-btn flow-zoom-btn"
                  title="Ajustar a la vista"
                  aria-label="Ajustar a la vista"
                  @click="flowFitView"
                >
                  <i class="bi bi-arrows-fullscreen" aria-hidden="true"></i>
                </button>
              </div>
              <button
                type="button"
                class="screen-action-btn flow-toolbar-btn"
                :disabled="screens.length === 0"
                @click="addFlowTask"
              >
                <i class="bi bi-plus-lg" aria-hidden="true"></i>
                Nueva tarea
              </button>
              <button
                type="button"
                class="screen-action-btn flow-toolbar-btn flow-toolbar-btn-soft"
                :disabled="!selectedFlowEdgeId"
                @click="removeSelectedFlowEdge"
              >
                <i class="bi bi-trash3" aria-hidden="true"></i>
                Eliminar flecha seleccionada
              </button>
              <button
                type="button"
                class="screen-action-btn flow-toolbar-btn flow-toolbar-btn-soft"
                :disabled="!selectedFlowEdgeId"
                @click="setSelectedFlowEdgeSubmitPrimary"
              >
                <i class="bi bi-flag-fill" aria-hidden="true"></i>
                {{ selectedFlowEdge?.isSubmitPrimary ? 'Quitar submit principal' : 'Marcar submit principal' }}
              </button>
            </div>
          </div>
        </div>
        <template v-if="flowWorkspaceTab === 'canvas'">
        <div v-if="flowNodes.length === 0" class="canvas-state">
          No hay pantallas aún. Crea o asigna una pantalla para iniciar.
        </div>
        <div v-else class="flow-canvas">
          <VueFlow
            ref="vueFlowRef"
            v-model:nodes="flowNodes"
            v-model:edges="flowEdges"
            :default-zoom="1"
            :fit-view-on-init="true"
            :pan-on-drag="false"
            :zoom-on-scroll="false"
            :connection-mode="ConnectionMode.Loose"
            :nodes-draggable="true"
            :snap-to-grid="true"
            :snap-grid="[20, 20]"
            class="flow-canvas-instance"
            @connect="onFlowConnect"
            @edge-click="onFlowEdgeClick"
            @pane-click="onFlowPaneClick"
            @viewport-change-end="onFlowViewportChangeEnd"
          >
            <template #node-flow-task="{ id }">
              <div class="flow-task">
                <Handle type="source" id="anchor-top-1" :position="Position.Top" class="flow-handle" :style="{ left: '32%' }" />
                <Handle
                  type="source"
                  id="anchor-top-2"
                  :position="Position.Top"
                  class="flow-handle"
                  :style="{ left: '68%' }"
                />
                <Handle
                  type="source"
                  id="anchor-bottom-1"
                  :position="Position.Bottom"
                  class="flow-handle"
                  :style="{ left: '32%' }"
                />
                <Handle
                  type="source"
                  id="anchor-bottom-2"
                  :position="Position.Bottom"
                  class="flow-handle"
                  :style="{ left: '68%' }"
                />
                <Handle
                  type="source"
                  id="anchor-left-1"
                  :position="Position.Left"
                  class="flow-handle"
                  :style="{ top: '32%' }"
                />
                <Handle
                  type="source"
                  id="anchor-left-2"
                  :position="Position.Left"
                  class="flow-handle"
                  :style="{ top: '68%' }"
                />
                <Handle
                  type="source"
                  id="anchor-right-1"
                  :position="Position.Right"
                  class="flow-handle"
                  :style="{ top: '32%' }"
                />
                <Handle
                  type="source"
                  id="anchor-right-2"
                  :position="Position.Right"
                  class="flow-handle"
                  :style="{ top: '68%' }"
                />
                <header class="flow-task-header">
                  <input
                    class="flow-task-title"
                    type="text"
                    :value="getFlowNodeView(id)?.task?.title ?? ''"
                    @input="onFlowNodeInput(id, $event)"
                    placeholder="Nombre de tarea"
                  />
                <button
                  type="button"
                  class="screen-action-btn flow-task-start-btn"
                  :disabled="getFlowNodeView(id)?.task?.isStartTask"
                  :class="{ 'flow-task-start-btn--active': getFlowNodeView(id)?.task?.isStartTask }"
                  :aria-label="
                    getFlowNodeView(id)?.task?.isStartTask
                      ? 'Esta tarea ya es el inicio'
                      : 'Marcar como tarea inicial'
                  "
                  :title="
                    getFlowNodeView(id)?.task?.isStartTask
                      ? 'Esta tarea ya es el inicio'
                      : 'Marcar como tarea inicial'
                  "
                  @click="setFlowTaskAsStart(id)"
                >
                  <i
                    class="bi"
                    :class="getFlowNodeView(id)?.task?.isStartTask ? 'bi-circle-fill' : 'bi-circle'"
                    aria-hidden="true"
                  ></i>
                </button>
                <span
                  v-if="getFlowNodeView(id)?.task?.isStartTask"
                  class="flow-task-start-badge"
                >
                  Inicio
                </span>
                  <button type="button" class="screen-action-btn flow-task-remove" @click="removeFlowTask(id)">×</button>
                </header>
                <label class="flow-task-screen-label">Pantalla asociada</label>
                <select
                  class="flow-task-screen-select"
                  :value="getFlowNodeView(id)?.task?.screenId ?? ''"
                  @change="onFlowTaskScreenChange(id, $event)"
                >
                  <option value="">Sin pantalla</option>
                  <option v-for="screen in screens" :key="screen.id" :value="screen.id">{{ screen.name }}</option>
                </select>
                <label class="flow-task-popup-check">
                  <input
                    type="checkbox"
                    :checked="getFlowNodeView(id)?.task?.isPopupTask ?? false"
                    @change="toggleFlowTaskPopupType(id)"
                  />
                  <span>Marcar como popup</span>
                </label>
                <div class="flow-task-preview">
                  <div v-if="getFlowNodeView(id)?.preview?.isLoading" class="flow-preview-placeholder">
                    Cargando vista previa...
                  </div>
                  <p v-else-if="getFlowNodeView(id)?.preview?.error" class="flow-preview-error">
                    {{ getFlowNodeView(id)?.preview?.error }}
                  </p>
                  <component
                    v-else-if="getFlowNodeView(id)?.preview?.component"
                    :is="getFlowNodeView(id)?.preview?.component"
                    class="flow-preview-component"
                  />
                  <p v-else class="flow-preview-placeholder">Sin vista previa. Guarda la pantalla o asigna una pantalla.</p>
                </div>
                <footer class="flow-task-footer">
                  <button type="button" class="screen-action-btn flow-task-open-btn" @click="onFlowNodeOpen(id)">
                    Abrir pantalla
                  </button>
                </footer>
              </div>
            </template>
          </VueFlow>
        </div>
        <p v-if="flowEdges.length > 0" class="flow-status">
          Conexiones activas: {{ flowEdges.length }}
        </p>
        </template>
        <div v-show="flowWorkspaceTab === 'data'" class="flow-tab-panel flow-aux-panel">
          <pre class="editor-data-preview">{{ flowSnapshotText }}</pre>
        </div>
        <div v-show="flowWorkspaceTab === 'states'" class="flow-tab-panel flow-aux-panel flow-tab-panel--muted">
          <p class="editor-states-hint">
            Conecta nodos en el lienzo para marcar transiciones. Próximamente: estados explícitos y condiciones en cada arista.
          </p>
        </div>
      </article>
    </section>

    <section v-else-if="primaryNav === 'execution'" class="canvas-wrap">
      <article class="canvas-surface">
        <header class="canvas-header">
          <div class="canvas-header-top">
            <div>
              <h1>Ejecución</h1>
              <p>{{ executionStartTaskLabel }}</p>
              <p>{{ executionCurrentTaskLabel }}</p>
            </div>
            <div class="screen-toolbar">
              <button type="button" class="screen-action-btn" @click="navigateToBuilder()">Volver al builder</button>
            </div>
          </div>
        </header>
        <Transition :name="themeTransitionDirection === 'left' ? 'canvas-swipe-left' : 'canvas-swipe-right'" mode="out-in">
          <div v-if="generatedComponent" :key="themeTransitionKey" class="canvas-content">
            <component :is="generatedComponent" />
          </div>
          <div v-else :key="`execution-empty-${activeTheme}`" class="canvas-state">
            {{ message }}
          </div>
        </Transition>
        <div v-if="popupState.isOpen" class="screen-popup-backdrop" role="dialog" aria-modal="true" aria-label="Pantalla modal" @click="closePopupScreen">
          <div class="screen-popup-panel" @click.stop>
            <header class="screen-popup-header">
              <strong>{{ popupState.title || 'Popup' }}</strong>
              <button type="button" class="screen-popup-close" @click="closePopupScreen">Cerrar</button>
            </header>
            <div class="screen-popup-content">
              <p v-if="popupState.isLoading" class="screen-popup-message">Cargando pantalla popup...</p>
              <p v-else-if="popupState.error" class="screen-popup-message screen-popup-error">{{ popupState.error }}</p>
              <component v-else-if="popupState.component" :is="popupState.component" />
              <p v-else class="screen-popup-message">No hay contenido para mostrar.</p>
            </div>
          </div>
        </div>
        <div v-if="isGenerating" class="canvas-status-layer">
          <div class="canvas-status-chip">
            <span class="canvas-status-dot" aria-hidden="true"></span>
            {{ generatedComponent ? 'Actualizando pantalla...' : 'Cargando pantalla...' }}
          </div>
        </div>
      </article>
    </section>

    <section v-else class="canvas-wrap nav-placeholder">
      <div class="nav-placeholder-inner">
        <template v-if="primaryNav === 'components'">
          <h2 class="nav-placeholder-title">Componentes</h2>
          <p class="nav-placeholder-text">Exploración del catálogo BootstrapVue y packs extra. Sección en preparación.</p>
        </template>
        <template v-else-if="primaryNav === 'library'">
          <h2 class="nav-placeholder-title">Biblioteca</h2>
          <p class="nav-placeholder-text">Inspiración y plantillas reutilizables. Sección en preparación.</p>
        </template>
        <template v-else>
          <h2 class="nav-placeholder-title">Ajustes</h2>
          <p class="nav-placeholder-text">Preferencias de proyecto y cuenta. Sección en preparación.</p>
        </template>
        <button type="button" class="screen-action-btn nav-placeholder-back" @click="navigateToBuilder()">Volver al builder</button>
      </div>
    </section>
      </div>
    </div>

    <div
      v-if="isFlowNavigationPromptOpen"
      class="screen-confirm-backdrop"
      role="dialog"
      aria-modal="true"
      aria-label="Confirmar navegación con cambios sin guardar"
      @click.self="cancelUnsavedChangesPrompt"
    >
      <div class="screen-confirm-modal">
        <header class="screen-confirm-header">
          <span class="screen-confirm-icon" aria-hidden="true">⟳</span>
          <div>
            <h3 class="screen-confirm-title">Cambios sin guardar</h3>
            <p class="screen-confirm-subtitle">La pantalla <strong>{{ unsavedNavigationScreenName }}</strong> tiene cambios pendientes.</p>
          </div>
          <button
            type="button"
            class="screen-confirm-close"
            :disabled="isSavingBeforeFlowNavigation"
            @click="cancelUnsavedChangesPrompt"
            aria-label="Cerrar confirmación"
          >
            Cerrar
          </button>
        </header>
        <div class="screen-confirm-body">
          <p>
            Tienes cambios sin guardar. ¿Quieres guardarlos antes de ir a
            <strong>Flujos</strong>?
          </p>
        </div>
        <div class="screen-confirm-actions">
          <button
            type="button"
            class="screen-action-btn screen-confirm-btn"
            :disabled="isSavingBeforeFlowNavigation"
            @click="cancelUnsavedChangesPrompt"
          >
            Quedarse en builder
          </button>
          <button
            type="button"
            class="screen-action-btn screen-confirm-btn screen-confirm-btn--ghost"
            :disabled="isSavingBeforeFlowNavigation"
            @click="declineSaveAndContinueToFlows"
          >
            Ir sin guardar
          </button>
          <button
            type="button"
            class="screen-action-btn screen-confirm-btn screen-confirm-btn--primary"
            :disabled="isSavingBeforeFlowNavigation"
            @click="saveAndContinueToFlows"
          >
            {{ isSavingBeforeFlowNavigation ? 'Guardando...' : 'Guardar y continuar' }}
          </button>
        </div>
      </div>
    </div>

    <footer class="app-statusbar">
      <div class="app-statusbar-left">
        <template v-if="primaryNav === 'builder'">
          <span class="app-status-dot app-status-dot--ok" aria-hidden="true"></span>
          <span class="app-status-name">{{ activeScreenLabel }}</span>
        </template>
        <template v-else-if="primaryNav === 'flows'">
          <span class="app-status-dot app-status-dot--ok" aria-hidden="true"></span>
          <span class="app-status-name">Flujo de tareas</span>
        </template>
        <template v-else-if="primaryNav === 'execution'">
          <span class="app-status-dot app-status-dot--ok" aria-hidden="true"></span>
          <span class="app-status-name">Ejecución de prototype</span>
        </template>
        <template v-else>
          <span class="app-status-name">Rapid Prototype Builder</span>
        </template>
      </div>
      <div class="app-statusbar-center">
        <span>{{ screens.length }} pantalla(s)</span>
        <span class="app-statusbar-sep">·</span>
        <span>{{ flowEdges.length }} conexión(es)</span>
        <span class="app-statusbar-sep">·</span>
        <span>{{ flowTasks.length }} tarea(s)</span>
      </div>
      <div class="app-statusbar-right">
        <i class="bi bi-check2-circle app-statusbar-ic" aria-hidden="true"></i>
        <span>{{ statusBarValidation }}</span>
      </div>
    </footer>

  </main>
</template>

<style scoped>
.builder-root {
  --rp-primary: #1a66ff;
  --rp-primary-hover: #1454d9;
  --rp-primary-soft: rgba(26, 102, 255, 0.12);
  --rp-bg-app: #f8f9fb;
  --rp-bg-panel: #ffffff;
  --rp-bg-canvas: var(--bs-body-bg, #f8f9fb);
  --rp-bg-subtle: #f3f4f6;
  --rp-text: #111827;
  --rp-text-muted: #6b7280;
  --rp-border: #e5e7eb;
  --rp-shadow-sm: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06);
  --rp-shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.08);
  --rp-success-bg: #f0fdf4;
  --rp-success-border: #bbf7d0;
  --rp-success-text: #16a34a;

  min-height: 100vh;
  height: 100vh;
  margin: 0;
  background: var(--rp-bg-app);
  color: var(--rp-text);
  font-family:
    ui-sans-serif,
    system-ui,
    -apple-system,
    BlinkMacSystemFont,
    'Segoe UI',
    Roboto,
    'Helvetica Neue',
    Arial,
    sans-serif;
  display: flex;
  flex-direction: column;
  position: relative;
  overflow: hidden;
}

.builder-root *,
.builder-root *::before,
.builder-root *::after {
  box-sizing: border-box;
}

.app-topbar {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.55rem 1.1rem;
  background: var(--rp-bg-panel);
  border-bottom: 1px solid var(--rp-border);
  box-shadow: 0 1px 0 rgba(15, 23, 42, 0.04);
}

.app-topbar-brand {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
}

.app-topbar-logo {
  display: inline-grid;
  place-items: center;
  width: 2.1rem;
  height: 2.1rem;
  border-radius: 10px;
  background: var(--rp-primary-soft);
  color: var(--rp-primary);
  font-size: 1.1rem;
  line-height: 1;
}

.app-topbar-titles {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}

.app-topbar-name {
  font-weight: 700;
  font-size: 1.02rem;
  letter-spacing: -0.02em;
  color: var(--rp-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-topbar-tagline {
  font-size: 0.72rem;
  color: var(--rp-text-muted);
}

.app-topbar-actions {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-shrink: 0;
}

.app-icon-btn {
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 10px;
  border: 1px solid var(--rp-border);
  background: var(--rp-bg-panel);
  color: var(--rp-primary);
  display: inline-grid;
  place-items: center;
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.app-icon-btn:hover {
  background: var(--rp-bg-subtle);
}

.app-text-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.38rem 0.65rem;
  border-radius: 10px;
  border: 1px solid var(--rp-border);
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.app-text-btn:hover {
  background: var(--rp-bg-subtle);
}

.app-avatar {
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 999px;
  background: linear-gradient(135deg, var(--rp-primary), #6366f1);
  color: #fff;
  font-size: 0.7rem;
  font-weight: 700;
  display: inline-grid;
  place-items: center;
  margin-left: 0.25rem;
}

.app-body {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  align-items: stretch;
  width: 100%;
}

.app-rail {
  flex: 0 0 auto;
  width: 200px;
  display: flex;
  flex-direction: column;
  background: var(--rp-bg-panel);
  border-right: 1px solid var(--rp-border);
  transition: width 0.18s ease;
}

.app-rail--collapsed {
  width: 64px;
}

.app-rail-nav {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.6rem 0.5rem;
  min-height: 0;
  overflow: auto;
}

.app-rail-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.55rem;
  width: 100%;
  border: none;
  border-radius: 10px;
  padding: 0.45rem 0.55rem;
  background: transparent;
  color: var(--rp-text-muted);
  font-size: 0.84rem;
  font-weight: 600;
  cursor: pointer;
  text-align: left;
  border-left: 3px solid transparent;
}

.app-rail-item i {
  font-size: 1.05rem;
  flex-shrink: 0;
}

.app-rail-item:hover {
  background: var(--rp-bg-subtle);
  color: var(--rp-text);
}

.app-rail-item--active {
  background: var(--rp-primary-soft);
  color: var(--rp-primary);
  border-left-color: var(--rp-primary);
}

.app-rail--collapsed .app-rail-label {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
}

.app-rail-collapse {
  flex: 0 0 auto;
  border: none;
  border-top: 1px solid var(--rp-border);
  background: var(--rp-bg-panel);
  padding: 0.45rem;
  cursor: pointer;
  color: var(--rp-text-muted);
  font-size: 1rem;
}

.app-rail-collapse:hover {
  background: var(--rp-bg-subtle);
  color: var(--rp-text);
}

.builder-lateral {
  flex: 0 0 auto;
  width: min(380px, 36vw);
  min-width: 280px;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: auto;
  background: var(--rp-bg-panel);
  border-right: 1px solid var(--rp-border);
  padding: 1rem 1rem 1.25rem;
  gap: 0.35rem;
}

.builder-lateral--minimized {
  position: fixed;
  right: 0.8rem;
  bottom: 0.8rem;
  width: min(400px, calc(100vw - 1.6rem));
  max-height: min(70vh, calc(100vh - 2rem));
  overflow: auto;
  z-index: 45;
  border-right: 1px solid color-mix(in srgb, var(--rp-border) 72%, transparent);
  border-radius: 16px;
  background: color-mix(in srgb, var(--rp-bg-panel) 84%, transparent);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  box-shadow:
    0 18px 35px rgba(15, 23, 42, 0.28),
    0 10px 24px rgba(15, 23, 42, 0.18),
    var(--rp-shadow-md);
  animation: builder-lateral-morph 160ms ease;
}

.builder-lateral--minimized .builder-lateral-header {
  margin-bottom: 0.5rem;
  gap: 0.6rem;
  align-items: center;
}

.builder-lateral-header {
  margin-bottom: 0.35rem;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
}

.builder-lateral-title-wrap {
  min-width: 0;
}

.builder-lateral-minimize-btn {
  width: 1.95rem;
  height: 1.95rem;
  border-radius: 999px;
  border: 1px solid var(--rp-border);
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  flex-shrink: 0;
}

.builder-lateral-minimize-btn:hover:not(:disabled) {
  background: var(--rp-bg-subtle);
}

.builder-lateral--minimized .builder-lateral-sub,
.builder-lateral--minimized .builder-context,
.builder-lateral--minimized .builder-feedback-ok {
  display: none;
}

.builder-lateral--minimized .ux-recommendation-bubbles {
  max-width: 100%;
  white-space: normal;
}

.builder-lateral--minimized .ux-recommendation-bubble-list {
  flex-wrap: wrap;
}

.builder-lateral--minimized .builder-prompt-textarea {
  min-height: 94px;
}

.builder-lateral--minimized .prompt-actions {
  margin-top: 0.45rem;
}

@keyframes builder-lateral-morph {
  from {
    transform: translateY(8px) scale(0.985);
    opacity: 0.65;
  }
  to {
    transform: translateY(0) scale(1);
    opacity: 1;
  }
}

.builder-lateral-header {
  margin-bottom: 0.35rem;
}

.builder-lateral-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--rp-text);
}

.builder-lateral-sub {
  margin: 0.25rem 0 0;
  font-size: 0.8rem;
  color: var(--rp-text-muted);
  line-height: 1.4;
}

.builder-lateral-section-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--rp-text);
}

.builder-context {
  margin-top: 0.75rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid var(--rp-border);
  border-radius: 12px;
  background: var(--rp-bg-canvas);
}

.builder-context-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.5rem;
}

.builder-context-heading {
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--rp-text-muted);
}

.builder-context-list {
  margin: 0;
}

.builder-context-row {
  display: grid;
  grid-template-columns: 4.5rem minmax(0, 1fr);
  gap: 0.35rem 0.65rem;
  font-size: 0.8rem;
  padding: 0.35rem 0;
  border-top: 1px solid var(--rp-border);
}

.builder-context-row:first-of-type {
  border-top: none;
  padding-top: 0;
}

.builder-context-row dt {
  margin: 0;
  font-weight: 600;
  color: var(--rp-text-muted);
}

.builder-context-row dd {
  margin: 0;
  color: var(--rp-text);
  word-break: break-word;
}

.builder-context-packs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.builder-pack-chip {
  font-size: 0.68rem;
  padding: 0.15rem 0.4rem;
  border-radius: 999px;
  background: var(--rp-primary-soft);
  color: var(--rp-primary);
  font-weight: 600;
}

.builder-context-row--theme dd {
  overflow: hidden;
}

.theme-control--compact {
  flex-direction: column;
  align-items: flex-start;
  gap: 0.2rem;
  white-space: normal;
}

.theme-control--compact .theme-current {
  min-width: 7.5rem;
  font-size: 0.78rem;
}

.theme-control--compact .theme-hint {
  margin-top: 0;
}

.builder-feedback-ok {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  margin-top: 0.5rem;
  padding: 0.55rem 0.65rem;
  border-radius: 10px;
  background: var(--rp-success-bg);
  border: 1px solid var(--rp-success-border);
  color: var(--rp-success-text);
  font-size: 0.82rem;
  line-height: 1.35;
}

.builder-feedback-ok i {
  flex-shrink: 0;
  margin-top: 0.08rem;
}

.builder-prompt-msg {
  margin-top: 0.5rem;
}

.app-main {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--rp-bg-app);
  padding: 0.65rem 0.85rem 0.85rem;
  overflow: hidden;
}

.canvas-workspace-head {
  padding-bottom: 0.65rem;
  margin-bottom: 0.75rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.canvas-workspace-head .screen-toolbar {
  margin-left: auto;
}

.workspace-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.workspace-tab {
  border: 1px solid transparent;
  background: var(--rp-bg-subtle);
  color: var(--rp-text-muted);
  font-size: 0.8rem;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  height: 2rem;
  padding: 0 0.75rem;
  border-radius: 999px;
  cursor: pointer;
  line-height: 1;
}

.workspace-tab:hover {
  color: var(--rp-text);
}

.workspace-tab--active {
  background: var(--rp-primary-soft);
  border-color: rgba(26, 102, 255, 0.28);
  color: var(--rp-primary);
}

.canvas-header-top--tools {
  justify-content: flex-start;
  margin-top: 0.25rem;
  padding-top: 0.65rem;
  border-top: none;
}

.editor-tab-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1rem 1.25rem;
  overflow: auto;
}

.editor-tab-panel--data {
  align-items: stretch;
}

.editor-data-empty {
  margin: 0;
  color: var(--rp-text-muted);
  font-size: 0.9rem;
  line-height: 1.55;
  max-width: 40rem;
}

.data-editor-panel {
  width: 100%;
  max-width: min(960px, 100%);
  margin: 0 auto;
  box-sizing: border-box;
  background: var(--rp-bg-panel);
  border: 1px solid var(--rp-border);
  border-radius: 14px;
  padding: 1rem;
  display: grid;
  gap: 0.75rem;
  color: var(--rp-text);
  box-shadow: var(--rp-shadow-md);
}

.data-editor-header--embedded {
  justify-content: flex-start;
}

.data-editor-textarea--embedded {
  min-height: min(360px, 45vh);
}

.editor-data-preview {
  margin: 0;
  padding: 0.85rem;
  border-radius: 12px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 0.72rem;
  line-height: 1.45;
  overflow: auto;
  max-height: min(560px, 55vh);
  border: 1px solid var(--rp-border);
}

.editor-states-hint {
  margin: 0;
  max-width: 42rem;
  color: var(--rp-text-muted);
  line-height: 1.55;
  font-size: 0.9rem;
}

.flow-workspace-head {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  padding: 1.25rem 1.5rem 1rem;
}

.workspace-tabs--flow {
  padding-bottom: 0;
}

.flow-toolbar--split {
  flex-wrap: wrap;
  padding: 0.35rem 0.45rem;
}

.flow-zoom-controls {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding-right: 0.35rem;
  margin-right: 0.15rem;
  border-right: 1px solid var(--rp-border);
}

.flow-zoom-readout {
  min-width: 2.75rem;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--rp-text-muted);
  text-align: right;
}

.flow-zoom-btn {
  min-width: 2rem;
  padding: 0.28rem 0.4rem;
}

.flow-tab-panel {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
}

.flow-aux-panel {
  padding: 0.75rem 0;
}

.flow-tab-panel--muted {
  padding: 1rem;
}

.nav-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
}

.nav-placeholder-inner {
  text-align: center;
  max-width: 24rem;
  padding: 2rem 1rem;
}

.nav-placeholder-title {
  margin: 0 0 0.5rem;
  font-size: 1.25rem;
  color: var(--rp-text);
}

.nav-placeholder-text {
  margin: 0;
  color: var(--rp-text-muted);
  line-height: 1.5;
  font-size: 0.9rem;
}

.nav-placeholder-back {
  margin-top: 1.25rem;
}

.app-statusbar {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.35rem 1rem;
  background: var(--rp-bg-panel);
  border-top: 1px solid var(--rp-border);
  font-size: 0.78rem;
  color: var(--rp-text-muted);
}

.app-statusbar-left,
.app-statusbar-center,
.app-statusbar-right {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  min-width: 0;
}

.app-statusbar-center {
  flex: 0 1 auto;
  justify-content: center;
  flex-wrap: wrap;
}

.app-statusbar-right {
  flex-shrink: 0;
  color: var(--rp-text);
  font-weight: 500;
}

.app-statusbar-sep {
  opacity: 0.45;
}

.app-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: #9ca3af;
  flex-shrink: 0;
}

.app-status-dot--ok {
  background: #22c55e;
}

.app-status-name {
  font-weight: 600;
  color: var(--rp-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-statusbar-ic {
  color: #22c55e;
  flex-shrink: 0;
}

.builder-prompt-textarea {
  margin-top: 0.65rem;
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--rp-border);
  border-radius: 10px;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  padding: 0.6rem;
  min-height: 130px;
  resize: vertical;
}

.builder-prompt-textarea::placeholder {
  color: #9ca3af;
}

.canvas-wrap {
  border: 1px solid var(--rp-border);
  padding: 0;
  background: var(--rp-bg-panel);
  box-shadow: var(--rp-shadow-sm);
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  width: 100%;
  overflow: hidden;
}

.canvas-header {
  padding: 1.25rem 1.5rem 1rem;
  border-bottom: 1px solid var(--rp-border);
  background: var(--rp-bg-panel);
}

.canvas-header h1 {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--rp-text);
  letter-spacing: -0.02em;
}

.canvas-header-top {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: center;
  flex-wrap: wrap;
}

.screen-toolbar {
  display: flex;
  gap: 0.55rem;
  align-items: center;
  flex-wrap: wrap;
  color: var(--rp-text-muted);
  font-size: 0.875rem;
}

.screen-toolbar > label {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.screen-toolbar .screen-action-btn i {
  margin-right: 0.25rem;
  font-size: 0.92rem;
}

.screen-select {
  margin-left: 0.45rem;
  min-width: 170px;
  border: 1px solid var(--rp-border);
  border-radius: 8px;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  padding: 0.38rem 0.56rem;
}

.screen-action-btn {
  border: 1px solid var(--rp-border);
  border-radius: 8px;
  min-width: 86px;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  padding: 0.38rem 0.6rem;
  cursor: pointer;
  font-weight: 500;
}

.screen-action-btn:hover {
  background: var(--rp-bg-subtle);
}

.screen-action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.theme-control {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--rp-text-muted);
  font-size: 0.82rem;
  white-space: nowrap;
}

.theme-select {
  min-width: 156px;
  border: 1px solid var(--rp-border);
  border-radius: 8px;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  padding: 0.38rem 0.56rem;
}

.theme-switch {
  display: inline-flex;
  align-items: stretch;
  gap: 0.45rem;
}

.theme-switch-btn {
  border: 1px solid var(--rp-border);
  border-radius: 8px;
  width: 2rem;
  height: 2rem;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  cursor: pointer;
  display: inline-grid;
  place-items: center;
  padding: 0;
  line-height: 1;
}

.theme-switch-btn:hover {
  background: var(--rp-bg-subtle);
}

.theme-current {
  min-width: 10rem;
  display: inline-grid;
  place-items: center;
  border: 1px solid var(--rp-border);
  border-radius: 8px;
  background: var(--rp-bg-subtle);
  color: var(--rp-text);
  padding: 0.38rem 0.6rem;
  font-size: 0.86rem;
  font-weight: 500;
}

.theme-switch-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.theme-hint {
  display: block;
  color: var(--rp-text-muted);
  font-size: 0.75rem;
  margin-top: 0.2rem;
}

.canvas-header p {
  margin: 0.4rem 0 0.9rem;
  color: var(--rp-text-muted);
  line-height: 1.5;
}

.canvas-surface {
  background: var(--rp-bg-canvas);
  border: none;
  border-radius: 0;
  min-height: 0;
  overflow: auto;
  color: var(--bs-body-color);
  position: relative;
  flex: 1 1 auto;
}

.flow-surface {
  overflow: hidden;
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-height: 0;
  flex: 1;
  border-radius: 0;
  background: var(--rp-bg-canvas);
}

.flow-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.75rem;
  padding: 0;
}

.flow-toolbar-left {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
  flex: 1 1 auto;
  min-width: 0;
}

.workspace-tabs {
  align-items: stretch;
}

.flow-toolbar-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  flex: 0 0 auto;
}

.flow-toolbar-actions .screen-action-btn i {
  margin-right: 0.25rem;
  font-size: 0.92rem;
}

.flow-toolbar-btn-soft {
  background: var(--rp-bg-subtle);
}


.flow-canvas {
  position: relative;
  overflow: auto;
  border: 1px solid var(--rp-border);
  background-color: var(--rp-bg-canvas);
  background-image: radial-gradient(circle, #d1d5db 1px, transparent 1px);
  background-size: 20px 20px;
  min-height: 0;
  flex: 1;
}

.flow-canvas-instance {
  width: 100%;
  min-height: 0;
  height: 100%;
}

.flow-canvas-instance :deep(.vue-flow) {
  width: 100%;
  height: 100%;
  border-radius: 12px;
  background: transparent;
}

.flow-canvas-instance :deep(.vue-flow__node) {
  background: transparent !important;
}

.flow-canvas-instance :deep(.vue-flow__edge path) {
  stroke: var(--rp-primary);
  stroke-width: 2;
  marker-end: url(#rp-task-flow-arrow);
  transition:
    stroke 0.18s ease,
    stroke-width 0.18s ease;
}

.flow-edge-marker-defs {
  width: 0;
  height: 0;
  position: absolute;
  pointer-events: none;
}

.flow-edge-marker-defs path {
  fill: var(--rp-primary);
}

.flow-handle {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  background: var(--rp-primary);
  border: 2px solid var(--rp-bg-panel);
}

.flow-task {
  position: relative;
  width: 100%;
  min-width: 280px;
  min-height: 260px;
  background: linear-gradient(155deg, #ffffff 0%, #f5f7fb 100%);
  border: 1px solid var(--rp-border);
  border-radius: 14px;
  display: grid;
  gap: 0.55rem;
  padding: 0.6rem;
  color: var(--rp-text);
  box-shadow: var(--rp-shadow-md);
}

.flow-task-header {
  display: flex;
  gap: 0.45rem;
  align-items: center;
}

.flow-task-start-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 2.2rem;
  border-radius: 999px;
  padding: 0.2rem 0.5rem;
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--rp-primary);
  background: color-mix(in srgb, var(--rp-primary-soft) 80%, transparent);
  border: 1px solid color-mix(in srgb, var(--rp-primary) 35%, var(--rp-border));
}

.flow-task-title {
  flex: 1;
  border: 1px solid var(--rp-border);
  border-radius: 8px;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  padding: 0.35rem 0.55rem;
  box-shadow: inset 0 1px 2px rgba(15, 23, 42, 0.08);
}

.flow-task-title:focus {
  outline: none;
  border-color: var(--rp-primary);
  box-shadow: 0 0 0 2px var(--rp-primary-soft);
}

.flow-task-title::placeholder {
  color: var(--rp-text-muted);
}

.flow-task-remove {
  width: 2rem;
  min-width: 2rem;
  padding: 0;
}

.flow-task-start-btn {
  width: 1.9rem;
  min-width: 1.9rem;
  height: 1.9rem;
  min-height: 1.9rem;
  padding: 0;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition:
    color 0.18s ease,
    border-color 0.18s ease,
    background-color 0.18s ease;
}

.flow-task-start-btn .bi {
  font-size: 0.78rem;
  line-height: 1;
}

.flow-task-start-btn:hover:not(:disabled) {
  color: #fff;
  background: var(--rp-primary);
  border-color: var(--rp-primary);
}

.flow-task-start-btn--active {
  color: #fff;
  background: var(--rp-primary);
  border-color: var(--rp-primary);
}

.flow-task-screen-label {
  font-size: 0.8rem;
  color: var(--rp-text-muted);
  font-weight: 600;
}

.flow-task-screen-select {
  border: 1px solid var(--rp-border);
  border-radius: 8px;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  padding: 0.35rem 0.55rem;
}

.flow-task-screen-select:focus {
  outline: none;
  border-color: var(--rp-primary);
  box-shadow: 0 0 0 2px var(--rp-primary-soft);
}

.flow-task-popup-check {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.82rem;
  color: var(--rp-text-muted);
}

.flow-task-preview {
  border: 1px solid var(--rp-border);
  border-radius: 8px;
  background: var(--rp-bg-panel);
  width: 300px;
  height: 200px;
  overflow: hidden;
  position: relative;
  padding: 0.2rem;
  box-shadow: inset 0 0 0 1px rgba(15, 23, 42, 0.04);
}

.flow-preview-component {
  transform: scale(0.28);
  transform-origin: top left;
  width: 1024px;
  height: 676px;
  overflow: hidden;
  pointer-events: none;
}

.flow-preview-placeholder {
  margin: 0;
  color: var(--rp-text-muted);
  font-size: 0.78rem;
  padding: 0.45rem;
}

.flow-preview-error {
  margin: 0;
  color: #dc2626;
  font-size: 0.76rem;
  padding: 0.45rem;
}

.flow-task-footer {
  display: flex;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.flow-task-open-btn {
  margin-left: auto;
}

.flow-status {
  margin: 0;
  color: var(--rp-text-muted);
  font-size: 0.82rem;
}

.canvas-state {
  min-height: 100%;
  display: grid;
  place-items: center;
  font-size: 1.05rem;
  text-align: center;
  padding: 1rem;
  color: var(--rp-text-muted);
}

.canvas-content {
  padding: 1rem;
}

.screen-confirm-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  display: grid;
  place-items: center;
  z-index: 60;
}

.screen-confirm-modal {
  width: min(520px, calc(100vw - 2rem));
  background: var(--rp-bg-panel);
  border: 1px solid var(--rp-border);
  border-radius: 16px;
  box-shadow: var(--rp-shadow-md);
  overflow: hidden;
}

.screen-confirm-header {
  padding: 0.85rem 1rem;
  border-bottom: 1px solid var(--rp-border);
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  background: color-mix(in srgb, var(--rp-bg-subtle) 78%, transparent);
}

.screen-confirm-icon {
  color: var(--rp-primary);
  font-size: 1.35rem;
  line-height: 1;
  margin-top: 0.1rem;
}

.screen-confirm-title {
  margin: 0;
  font-size: 1.05rem;
  color: var(--rp-text);
}

.screen-confirm-subtitle {
  margin: 0.25rem 0 0;
  color: var(--rp-text-muted);
  font-size: 0.85rem;
}

.screen-confirm-close {
  align-self: flex-start;
  border: 1px solid var(--rp-border);
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  border-radius: 10px;
  padding: 0.32rem 0.6rem;
  font-size: 0.74rem;
  cursor: pointer;
}

.screen-confirm-close:hover {
  background: var(--rp-bg-subtle);
}

.screen-confirm-body {
  padding: 0.85rem 1rem 0.6rem;
  color: var(--rp-text);
}

.screen-confirm-body p {
  margin: 0;
  line-height: 1.4;
}

.screen-confirm-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  justify-content: flex-end;
  padding: 0 1rem 1rem;
}

.screen-confirm-btn {
  min-width: 0;
  width: auto;
}

.screen-confirm-btn--ghost {
  background: var(--rp-bg-subtle);
}

.screen-confirm-btn--ghost:hover {
  background: color-mix(in srgb, var(--rp-bg-subtle) 68%, #fff);
}

.screen-confirm-btn--primary {
  background: var(--rp-primary);
  border-color: var(--rp-primary);
  color: #fff;
}

.screen-confirm-btn--primary:hover {
  background: var(--rp-primary-hover);
  border-color: var(--rp-primary-hover);
}

.screen-confirm-btn--primary:disabled {
  background: color-mix(in srgb, var(--rp-primary) 65%, #d1d5db);
  border-color: color-mix(in srgb, var(--rp-primary) 65%, #d1d5db);
}

.screen-popup-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.35);
  display: grid;
  place-items: center;
  z-index: 50;
}

.screen-popup-panel {
  width: min(860px, calc(100vw - 2rem));
  max-height: min(88vh, calc(100vh - 2rem));
  background: var(--rp-bg-panel);
  border: 1px solid var(--rp-border);
  border-radius: 16px;
  box-shadow: var(--rp-shadow-md);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.screen-popup-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.55rem 0.75rem;
  background: color-mix(in srgb, var(--rp-bg-subtle) 76%, transparent);
  border-bottom: 1px solid var(--rp-border);
  gap: 0.5rem;
  font-size: 0.93rem;
  font-weight: 600;
}

.screen-popup-close {
  border: 1px solid var(--rp-border);
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  border-radius: 10px;
  padding: 0.35rem 0.65rem;
  font-size: 0.74rem;
  cursor: pointer;
}

.screen-popup-close:hover {
  background: var(--rp-bg-subtle);
}

.screen-popup-content {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding: 0.85rem;
}

.screen-popup-message {
  margin: 0;
  color: var(--rp-text-muted);
}

.screen-popup-error {
  color: #b91c1c;
}

.canvas-status-layer {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: rgba(248, 249, 251, 0.75);
  backdrop-filter: blur(2px);
  pointer-events: none;
}

.canvas-status-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.45rem 0.8rem;
  border-radius: 999px;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  font-size: 0.9rem;
  font-weight: 500;
  border: 1px solid var(--rp-border);
  box-shadow: var(--rp-shadow-sm);
}

.canvas-status-dot {
  width: 0.6rem;
  height: 0.6rem;
  border-radius: 999px;
  background: var(--rp-primary);
  animation: canvas-pulse 1.1s infinite;
}

.canvas-screen-enter-active,
.canvas-screen-leave-active {
  transition:
    opacity 0.32s ease,
    transform 0.32s ease,
    filter 0.32s ease;
}

.canvas-screen-enter-from,
.canvas-screen-leave-to {
  opacity: 0;
  transform: translateY(8px);
  filter: blur(4px);
}

.canvas-swipe-right-enter-active,
.canvas-swipe-right-leave-active,
.canvas-swipe-left-enter-active,
.canvas-swipe-left-leave-active {
  transition:
    opacity 0.24s ease,
    transform 0.24s ease,
    filter 0.24s ease;
}

.canvas-swipe-right-enter-from,
.canvas-swipe-left-leave-to {
  opacity: 0;
  transform: translateX(24px);
  filter: blur(4px);
}

.canvas-swipe-left-enter-from,
.canvas-swipe-right-leave-to {
  opacity: 0;
  transform: translateX(-24px);
  filter: blur(4px);
}

@keyframes canvas-pulse {
  0% {
    transform: scale(0.92);
    opacity: 0.45;
  }
  40% {
    transform: scale(1);
    opacity: 1;
  }
  100% {
    transform: scale(0.92);
    opacity: 0.45;
  }
}

.canvas-meta {
  margin-top: 0;
  padding: 1rem 1.5rem 1.25rem;
  border-top: 1px solid var(--rp-border);
  background: var(--rp-bg-panel);
  color: var(--rp-text-muted);
  font-size: 0.9rem;
}

.canvas-meta p {
  margin: 0.2rem 0;
}

.canvas-meta strong {
  color: var(--rp-text);
}

.ux-evaluator {
  margin-top: 0.5rem;
  padding-top: 0.45rem;
  border-top: 1px solid var(--rp-border);
}

.ux-evaluator-title {
  margin: 0;
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-wrap: wrap;
  color: var(--rp-text);
}

.ux-evaluator-status {
  color: var(--rp-primary);
  font-size: 0.8rem;
  font-weight: 500;
}

.ux-evaluator-status-error {
  color: #dc2626;
}

.ux-evaluator-message {
  margin: 0.3rem 0 0;
  color: var(--rp-text-muted);
}

.ux-recommendation-bubbles {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 0.35rem;
  overflow-x: auto;
  overflow-y: hidden;
  white-space: nowrap;
  scrollbar-width: none;
  padding: 0.24rem 0 0;
}

.ux-bubble-enter-active,
.ux-bubble-leave-active,
.ux-bubble-move {
  transition: all 0.24s ease;
}

.ux-bubble-enter-from {
  opacity: 0;
  transform: translateY(-4px) scale(0.95);
}

.ux-bubble-leave-to {
  opacity: 0;
  transform: translateY(4px) scale(0.95);
}

.ux-bubble-leave-active {
  position: absolute;
}

.ux-recommendation-bubble {
  align-self: center;
  width: auto;
  min-width: 1rem;
  min-height: 1rem;
  border-radius: 9999px;
  padding: 0 0.45rem 0 0.35rem;
  height: 1.2rem;
  display: inline-flex;
  appearance: none;
  border: 0;
  color: #ffffff;
  cursor: pointer;
  align-items: center;
  justify-content: center;
  text-align: center;
  border: 1px solid transparent;
  background: rgba(107, 114, 128, 0.25);
  box-shadow: var(--rp-shadow-sm);
  transition: transform 140ms ease, opacity 140ms ease, box-shadow 140ms ease;
  margin-top: 0.12rem;
  margin-right: 0.12rem;
}

.ux-recommendation-bubble:hover:not(:disabled) {
  transform: translateY(-1px) scale(1.08);
  box-shadow: var(--rp-shadow-md);
}

.ux-recommendation-bubble:focus-visible {
  outline: 2px solid var(--rp-primary);
  outline-offset: 2px;
}

.ux-recommendation-bubble--high {
  border-color: rgba(var(--bs-danger-rgb), 0.65);
  background: color-mix(in srgb, rgba(var(--bs-danger-rgb), 0.85) 40%, var(--rp-bg-panel));
  color: #ffffff;
}

.ux-recommendation-bubble--medium {
  border-color: rgba(var(--bs-warning-rgb), 0.8);
  background: color-mix(in srgb, rgba(var(--bs-warning-rgb), 0.55) 45%, var(--rp-bg-panel));
  color: var(--rp-text);
}

.ux-recommendation-bubble--low {
  border-color: rgba(var(--bs-secondary-rgb), 0.7);
  background: color-mix(in srgb, rgba(var(--bs-secondary-rgb), 0.5) 42%, var(--rp-bg-panel));
  color: var(--rp-text);
}

.ux-recommendation-bubble-letter {
  font-size: 0.58rem;
  line-height: 1;
  font-weight: 700;
  font-family: var(--bs-font-sans-serif);
  letter-spacing: 0.01em;
  display: inline-flex;
  align-items: center;
}

.ux-recommendation-text-visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.ux-recommendation-bubbles::-webkit-scrollbar {
  display: none;
}

.ux-recommendation-bubble--burst {
  animation: ux-bubble-recommendation-pulse 0.8s ease-in-out infinite;
}

@keyframes ux-bubble-recommendation-pulse {
  0% {
    transform: scale(1);
    box-shadow: 0 0 0 0 rgba(26, 102, 255, 0.28);
  }
  50% {
    transform: scale(1.07);
    box-shadow: 0 0 0 8px rgba(26, 102, 255, 0);
  }
  100% {
    transform: scale(1);
    box-shadow: 0 0 0 0 rgba(26, 102, 255, 0.28);
  }
}

.ux-evaluator-list {
  margin: 0.5rem 0 0;
  padding-left: 1.2rem;
  display: grid;
  gap: 0.55rem;
  color: var(--rp-text-muted);
}

.ux-evaluator-item {
  display: grid;
  gap: 0.2rem;
}

.ux-evaluator-severity {
  justify-self: start;
  border-radius: 999px;
  border: 1px solid var(--rp-border);
  font-size: 0.72rem;
  padding: 0.12rem 0.45rem;
  text-transform: uppercase;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.ux-evaluator-severity.severity-high {
  color: #b91c1c;
  background: #fef2f2;
  border-color: #fecaca;
}

.ux-evaluator-severity.severity-medium {
  color: #b45309;
  background: #fffbeb;
  border-color: #fde68a;
}

.ux-evaluator-severity.severity-low {
  color: var(--rp-primary);
  background: var(--rp-primary-soft);
  border-color: rgba(26, 102, 255, 0.25);
}

.ux-evaluator-issue {
  margin: 0;
  color: var(--rp-text);
  font-weight: 700;
}

.ux-evaluator-recommendation {
  margin: 0;
  color: var(--rp-text-muted);
}

.conversation-list {
  margin-top: 0.65rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-height: 240px;
  overflow: auto;
  border: 1px solid var(--rp-border);
  border-radius: 10px;
  padding: 0.55rem;
  background: var(--rp-bg-canvas);
}

.floating-prompt-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.conversation-toggle-btn {
  height: 1.5rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0 0.5rem;
  border: 1px solid var(--rp-border);
  border-width: 1px;
  cursor: pointer;
  font-size: 0.72rem;
  line-height: 1;
}

.conversation-toggle-icon {
  display: inline-grid;
  place-items: center;
  line-height: 1;
}

.conversation-empty {
  color: var(--rp-text-muted);
  font-size: 0.85rem;
}

.conversation-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.conversation-row.user {
  color: var(--rp-text);
}

.conversation-row.assistant {
  color: var(--rp-primary);
}

.conversation-content {
  display: block;
  flex: 1;
  min-width: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

.assistant-icon {
  font-size: 1.2rem;
}

.prompt-action-btn {
  border: 0;
  border-radius: 10px;
  background: var(--rp-primary);
  color: #fff;
  cursor: pointer;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.prompt-action-btn:hover:not(:disabled) {
  background: var(--rp-primary-hover);
}

.prompt-action-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.prompt-action-btn i {
  font-size: 1.08rem;
}

.data-editor-overlay {
  position: fixed;
  inset: 0;
  background: rgba(17, 24, 39, 0.45);
  z-index: 1100;
  display: grid;
  place-items: center;
  padding: 1rem;
}

.data-editor-modal {
  width: min(760px, calc(100vw - 2.5rem));
  max-height: calc(100vh - 2.5rem);
  background: var(--rp-bg-panel);
  border: 1px solid var(--rp-border);
  border-radius: 14px;
  padding: 1rem;
  display: grid;
  gap: 0.75rem;
  color: var(--rp-text);
  box-shadow: var(--rp-shadow-md);
}

.data-editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.data-editor-header h3 {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
}

.data-editor-close {
  border: 1px solid var(--rp-border);
  border-radius: 8px;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  padding: 0.35rem 0.7rem;
  cursor: pointer;
  font-weight: 500;
}

.data-editor-close:hover:not(:disabled) {
  background: var(--rp-bg-subtle);
}

.data-editor-close:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.data-editor-textarea {
  margin: 0;
  width: 100%;
  min-height: 260px;
  resize: vertical;
  border: 1px solid var(--rp-border);
  border-radius: 10px;
  background: var(--rp-bg-canvas);
  color: var(--rp-text);
  padding: 0.65rem;
  font-family: 'Fira Code', Menlo, Monaco, Consolas, 'Courier New', monospace;
  font-size: 0.8rem;
  line-height: 1.35;
  box-sizing: border-box;
}

.data-editor-input-label {
  margin-bottom: -0.4rem;
  color: var(--rp-text-muted);
  font-size: 0.9rem;
  font-weight: 500;
}

.data-editor-instruction-textarea {
  width: 100%;
  margin: 0;
  min-height: 72px;
  resize: vertical;
  border: 1px solid var(--rp-border);
  border-radius: 10px;
  background: var(--rp-bg-panel);
  color: var(--rp-text);
  padding: 0.65rem;
  font-family: inherit;
  font-size: 0.9rem;
  line-height: 1.35;
  box-sizing: border-box;
}

.data-editor-inline-actions {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.data-editor-inline-actions .screen-action-btn {
  width: auto;
  min-width: 120px;
  margin: 0;
}

.data-editor-error {
  margin: 0;
  color: #dc2626;
  font-size: 0.9rem;
}

.data-editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.6rem;
}

.data-editor-actions .screen-action-btn {
  width: auto;
  margin-top: 0;
  min-width: 120px;
}

.data-editor-apply-btn {
  background: var(--rp-primary);
  border-color: var(--rp-primary);
  color: #fff;
}

.data-editor-apply-btn:hover:not(:disabled) {
  background: var(--rp-primary-hover);
  border-color: var(--rp-primary-hover);
}

.prompt-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.6rem;
}

.prompt-actions button {
  margin-top: 0;
  width: auto;
}

.prompt-actions button:first-child {
  flex: 1;
}

.prompt-actions .conversation-refresh {
  width: 42px;
  height: 42px;
  border-radius: 10px;
  flex: 0 0 auto;
}

.prompt-actions .conversation-rollback {
  width: 42px;
  height: 42px;
  border-radius: 10px;
  flex: 0 0 auto;
}

.prompt-action-generate {
  display: inline-flex;
  flex: 1;
  min-height: 42px;
}

.builder-lateral .conversation-toggle-btn {
  border-radius: 999px;
  background: var(--rp-bg-panel);
  color: var(--rp-text-muted);
}

.builder-lateral .conversation-toggle-btn:hover:not(:disabled) {
  background: var(--rp-bg-subtle);
  color: var(--rp-text);
}

.builder-lateral .prompt-actions button:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.prompt-msg {
  margin: 0.65rem 0 0;
  font-size: 0.87rem;
  color: var(--rp-text-muted);
}

.pipeline-missing {
  border: 1px dashed #ca8a04;
  border-radius: 10px;
  background: linear-gradient(160deg, #fffbeb, #fef3c7);
  color: #713f12;
  padding: 0.75rem;
}

.pipeline-missing-title {
  margin: 0;
  font-size: 0.9rem;
  font-weight: 700;
}

.pipeline-missing-subtitle {
  margin: 0.2rem 0 0;
  font-size: 0.78rem;
}
</style>
