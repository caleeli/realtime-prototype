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
import { VueFlow, Handle, Position } from '@vue-flow/core';
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
import {
  buildGeneratedScreen,
  type GeneratedScreenView,
  type GenerationRenderOptions,
} from './services/generationRenderService';
import {
  ProjectSessionService,
  type SessionScreenSummary,
  type SessionScreenState,
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
};

type FlowEdge = {
  id: string;
  source: string;
  target: string;
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
  };
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
const isGenerating = ref(false);
const didUseInspiration = ref(false);
const message = ref('Escribe una descripción y pulsa "Generar pantalla".');
const generatedState: Ref<GeneratedViewState | null> = ref(null);
const generatedComponent: Ref<Component | null> = ref(null);
const uxEvaluations: Ref<UXEvaluatorResultLine[]> = ref([]);
const screens = ref<SessionScreenSummary[]>([]);
const activeScreenId = ref('');
const isSessionLoading = ref(false);
const isSaving = ref(false);
const lastGeneratedOutput = ref<GenerationPipelineResult | null>(null);
const isHydratingSession = ref(false);
const isScreenDirty = ref(false);
const isDataEditorVisible = ref(false);
const dataEditorJson = ref('{}');
const dataEditorError = ref('');
const isApplyingData = ref(false);
const isApplyingDataGeneration = ref(false);
const dataInstructionText = ref('');
const dataGenerationError = ref('');
const dataGenerationHistory = ref<DataGenerationHistoryEntry[]>([]);
const dataGenerationRedoStack = ref<string[]>([]);
const dataGenerationConversation = ref<GenerationMessage[]>([]);
const isPugEditorVisible = ref(false);
const isApplyingPug = ref(false);
const pugInstructionText = ref('');
const pugEditorPug = ref('');
const pugEditorError = ref('');
const isApplyingPugGeneration = ref(false);
const pugGenerationError = ref('');
const pugGenerationHistory = ref<PugGenerationHistoryEntry[]>([]);
const pugGenerationRedoStack = ref<string[]>([]);
const pugGenerationConversation = ref<GenerationMessage[]>([]);
const isCssEditorVisible = ref(false);
const cssEditorCss = ref('');
const cssEditorError = ref('');
const isApplyingCss = ref(false);
const isApplyingCssGeneration = ref(false);
const cssInstructionText = ref('');
const cssGenerationError = ref('');
const cssGenerationHistory = ref<CssGenerationHistoryEntry[]>([]);
const cssGenerationRedoStack = ref<string[]>([]);
const cssGenerationConversation = ref<GenerationMessage[]>([]);
const isBuilderMinimized = ref(false);
const flowTaskCounter = ref(1);
const flowTasks = ref<FlowTask[]>([]);
const flowEdges = ref<FlowEdge[]>([]);
const flowTaskPreviews = ref<Record<string, FlowTaskPreviewState>>({});
const flowNodes = ref<FlowNode[]>([]);
const explodingBubbleId = ref<string | null>(null);
const uxEvaluationStatus = ref<'idle' | 'loading' | 'ready' | 'error'>('idle');
const uxEvaluationMessage = ref('');
const cleanupStyle = ref<(() => void) | null>(null);
const screenRevision = ref(0);
const BOOTSWATCH_VERSION = '5.3.8';
const BOOTSWATCH_LINK_ID = 'bootswatch-theme-runtime';

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

function buildFlowNodePosition(index: number) {
  const col = index % FLOW_COLUMNS;
  const row = Math.floor(index / FLOW_COLUMNS);
  return {
    x: col * FLOW_COLUMN_GAP + 24,
    y: row * FLOW_ROW_GAP + 24,
  };
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
  flowTasks.value = nextTasks;
}

function buildFlowTaskDefaults(screenId = ''): FlowTask {
  const nextLabel = getFlowTaskBaseLabel(flowTasks.value.length + 1);
  return {
    id: getFlowTaskId(),
    title: nextLabel,
    screenId,
  };
}

function syncFlowTasksToScreens(screenList: SessionScreenSummary[] = screens.value) {
  const validIds = new Set(screenList.map((screen) => screen.id));
  if (screenList.length === 0) {
    flowTaskPreviews.value = {};
    flowNodes.value = [];
    flowEdges.value = [];
    flowTasks.value = [];
    return;
  }

  if (flowTasks.value.length === 0 && screenList.length > 0) {
    flowTasks.value = screenList.map((screen) => ({
      id: getFlowTaskId(),
      title: `${screen.name}`,
      screenId: screen.id,
    }));
  } else {
    flowTasks.value = flowTasks.value.filter((task) => !task.screenId || validIds.has(task.screenId));
    flowTasks.value = flowTasks.value.map((task, index) => ({
      ...task,
      title: task.title || getFlowTaskBaseLabel(index + 1),
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
          },
        }
      : node,
  );
  void ensureFlowTaskPreview(taskId, selectedScreenId);
}

function onFlowConnect(connection: { source?: string; target?: string }) {
  if (!connection.source || !connection.target) {
    return;
  }
  if (connection.source === connection.target) {
    return;
  }

  const exists = flowEdges.value.some(
    (edge) => edge.source === connection.source && edge.target === connection.target,
  );
  if (exists) {
    return;
  }

  flowEdges.value = [
    ...flowEdges.value,
    {
      id: `edge-${Date.now()}-${connection.source}-${connection.target}`,
      source: connection.source,
      target: connection.target,
    },
  ];
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

async function focusFlowTask(taskId: string) {
  const task = flowTasks.value.find((item) => item.id === taskId);
  if (!task || !task.screenId) {
    return;
  }

  isBuilderMinimized.value = false;
  activeScreenId.value = task.screenId;
  try {
    await openScreen(task.screenId, { force: true });
  } catch (_error) {
    message.value = 'No se pudo abrir la pantalla desde el flujo.';
  }
}

function openBuilder() {
  isBuilderMinimized.value = false;
}

function toggleBuilderMinimized() {
  isBuilderMinimized.value = !isBuilderMinimized.value;
  if (isBuilderMinimized.value) {
    syncFlowTasksToScreens(screens.value);
  }
}

function buildPugGenerationContext() {
  return {
    locale: navigator.language || 'es-ES',
    theme: activeTheme.value,
    targetDensity: 'compact',
    enabledPacks: ['advanced-inputs', 'files', 'charts'],
  };
}

function openDataEditor() {
  const output = lastGeneratedOutput.value;
  if (!output) {
    message.value = 'No hay una pantalla generada para editar.';
    return;
  }
  dataEditorJson.value = formatScreenDataForEditor(output.data);
  dataEditorError.value = '';
  dataGenerationError.value = '';
  isDataEditorVisible.value = true;
}

function openPugEditor() {
  const output = lastGeneratedOutput.value;
  if (!output) {
    message.value = 'No hay una pantalla generada para editar el pug.';
    return;
  }
  pugEditorPug.value = output.sourcePug ?? '';
  pugEditorError.value = '';
  pugGenerationError.value = '';
  if (pugGenerationConversation.value.length === 0) {
    pugGenerationConversation.value = toApiMessages(conversation.value);
  }
  isPugEditorVisible.value = true;
}

function openCssEditor() {
  const output = lastGeneratedOutput.value;
  if (!output) {
    message.value = 'No hay una pantalla generada para editar el CSS.';
    return;
  }
  cssEditorCss.value = output.css ?? '';
  cssEditorError.value = '';
  cssInstructionText.value = '';
  cssGenerationError.value = '';
  if (cssGenerationConversation.value.length === 0) {
    cssGenerationConversation.value = toApiMessages(conversation.value);
  }
  isCssEditorVisible.value = true;
}

function closeDataEditor() {
  isDataEditorVisible.value = false;
  dataEditorError.value = '';
  dataEditorJson.value = '';
}

function closePugEditor() {
  isPugEditorVisible.value = false;
  pugEditorError.value = '';
  pugEditorPug.value = '';
}

function closeCssEditor() {
  isCssEditorVisible.value = false;
  cssEditorError.value = '';
  cssEditorCss.value = '';
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
  if (!isDataEditorVisible.value || isApplyingData.value) {
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
    isDataEditorVisible.value = false;
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
  if (!isPugEditorVisible.value || isApplyingPug.value) {
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
    isPugEditorVisible.value = false;
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
  if (!isCssEditorVisible.value || isApplyingCss.value) {
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
    isCssEditorVisible.value = false;
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
  try {
    isHydratingSession.value = true;
    await restoreLastSession();
  } catch (_error) {
    message.value = 'No se pudo cargar la última sesión.';
    try {
      await createNewScreen();
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
  return {
    locale: navigator.language || 'es-ES',
    theme: activeTheme.value,
    targetDensity: 'compact',
    enabledPacks: ['advanced-inputs', 'files', 'charts'],
  };
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

type OpenScreenOptions = { force?: boolean };

async function openScreen(screenId: string, options: OpenScreenOptions = {}) {
  const trimmed = screenId.trim();
  if (!trimmed || (isSessionLoading.value && !options.force)) {
    return;
  }

  isSessionLoading.value = true;
  try {
    resetForEmptyScreen('Cargando pantalla...');
    await sessionService.activateScreen(trimmed);
    activeScreenId.value = trimmed;
    const state = await sessionService.loadLatestState(trimmed);
    await hydrateFromSessionState(state);
    const session = await refreshScreensFromSession();
    screens.value = session.screens || screens.value;
    isScreenDirty.value = state === null && screens.value.find((screen) => screen.id === trimmed)?.lastRevision === 0;
    activeScreenId.value = trimmed;
    syncFlowTasksToScreens(screens.value);
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
      `Aplica esta recomendación UX (${severity.toUpperCase()}): ${recommendation || payload}`,
  };
}

const actionableUxRecommendations = computed<UXRecommendationBubble[]>(() => {
  return uxEvaluations.value
    .map((observation, index): UXRecommendationBubble | null => {
      const parsed = parseUxRecommendation(observation);
      if (!parsed || (parsed.severity !== 'high' && parsed.severity !== 'medium')) {
        return null;
      }

      return {
        id: `recommendation-${parsed.severity}-${index}`,
        severity: parsed.severity,
        text: parsed.text,
        requestText: parsed.requestText,
      };
    })
    .filter((entry): entry is UXRecommendationBubble => entry !== null);
});

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
  uxEvaluations.value = [];
  uxEvaluationStatus.value = 'idle';
  uxEvaluationMessage.value = '';
  const previousStyleCleanup = cleanupStyle.value;
  const nextStyleId = `pipeline-runtime-generated-${screenRevision.value + 1}`;

  const payload: InspirationRequest = {
    prompt,
    context: {
      locale: navigator.language || 'es-ES',
      theme: activeTheme.value,
      targetDensity: 'compact',
      enabledPacks: ['advanced-inputs', 'files', 'charts'],
    },
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
      const recommendations = await pipelineService.evaluateUX({
        pug: pipelineOutput.sourcePug,
        css: pipelineOutput.css,
        data: pipelineOutput.data,
      });
      uxEvaluations.value = recommendations;
      uxEvaluationStatus.value = 'ready';
      uxEvaluationMessage.value = '';
    } catch (error) {
      uxEvaluationStatus.value = 'error';
      uxEvaluationMessage.value = error instanceof Error ? error.message : 'No se pudo obtener las recomendaciones UX.';
      uxEvaluations.value = [];
    }

    const renderedView = await buildGeneratedScreen(pipelineOutput, {
      componentLoaders,
      styleId: nextStyleId,
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
  return {
    locale: navigator.language || 'es-ES',
    theme: activeTheme.value,
    targetDensity: 'compact',
    enabledPacks: ['advanced-inputs', 'files', 'charts'],
  };
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
    isCssEditorVisible.value = true;
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
    isCssEditorVisible.value = false;
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
    isPugEditorVisible.value = false;
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
    isPugEditorVisible.value = false;
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
  setTimeout(() => {
    if (explodingBubbleId.value === bubbleId) {
      explodingBubbleId.value = null;
    }
  }, 420);

  await runGenerationFromPrompt(suggestion.requestText);
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

onBeforeUnmount(() => {
  window.removeEventListener('keydown', isThemeHotkey);
  if (cleanupStyle.value) {
    cleanupStyle.value();
    cleanupStyle.value = null;
  }
  clearFlowTaskPreviews();
});

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
    <section v-if="!isBuilderMinimized" class="canvas-wrap">
      <header class="canvas-header">
        <div class="canvas-header-top">
          <div class="screen-toolbar">
            <label>
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
              + Nueva
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isSessionLoading || isSaving || !activeScreenId"
              @click="onDeleteScreenClick"
            >
              Eliminar
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="isSessionLoading || isSaving || !activeScreenId"
              @click="onSaveCurrentScreenClick"
            >
              {{ isSaving ? 'Guardando...' : 'Guardar' }}
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="!generatedComponent || isGenerating || isApplyingData || isApplyingDataGeneration"
              @click="openDataEditor"
            >
              Editar JSON
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="!generatedComponent || isGenerating || isApplyingPug || isApplyingPugGeneration"
              @click="openPugEditor"
            >
              Editar PUG
            </button>
            <button
              type="button"
              class="screen-action-btn"
              :disabled="!generatedComponent || isGenerating || isApplyingCss || isApplyingCssGeneration"
              @click="openCssEditor"
            >
              Editar CSS
            </button>
          <button type="button" class="screen-action-btn" @click="toggleBuilderMinimized">
            {{ isBuilderMinimized ? 'Maximizar' : 'Minimizar' }}
          </button>
          </div>
          <div>
            <h1>Builder Editor</h1>
            <p>Genera pantallas con IA y las dibuja en vivo en el canvas.</p>
          </div>
          <label
            class="theme-control"
            @touchstart="onThemeSwipeStart"
            @touchend="onThemeSwipeEnd"
          >
            Tema
            <div class="theme-switch" aria-label="Cambio rápido de tema">
              <button type="button" class="theme-switch-btn" @click="switchTheme('left')" title="Tema anterior (←)">
                ◀
              </button>
              <span class="theme-current">{{ activeThemeLabel }}</span>
              <button type="button" class="theme-switch-btn" @click="switchTheme('right')" title="Tema siguiente (→)">
                ▶
              </button>
            </div>
            <small class="theme-hint">Hotkeys: ← / →</small>
          </label>
        </div>
      </header>

      <article class="canvas-surface">
        <Transition :name="themeTransitionDirection === 'left' ? 'canvas-swipe-left' : 'canvas-swipe-right'" mode="out-in">
          <div v-if="generatedComponent" :key="themeTransitionKey" class="canvas-content">
            <component :is="generatedComponent" />
          </div>
          <div v-else :key="`empty-${activeTheme}`" class="canvas-state">{{ message }}</div>
        </Transition>
        <div v-if="isGenerating" class="canvas-status-layer">
          <div class="canvas-status-chip">
            <span class="canvas-status-dot" aria-hidden="true"></span>
            {{ generatedComponent ? 'Actualizando pantalla...' : 'Generando pantalla...' }}
          </div>
        </div>
      </article>
      <footer class="canvas-meta">
        <p v-if="generatedState">
          <strong>Tags usados:</strong>
          {{ generatedState.view.usedTags.join(', ') }}
        </p>
        <p v-if="generatedState && generatedState.view.unresolvedTags.length">
          <strong>No resueltos:</strong>
          {{ generatedState.view.unresolvedTags.join(', ') }}
        </p>
        <div v-if="generatedState && uxEvaluationStatus !== 'idle'" class="ux-evaluator">
          <p class="ux-evaluator-title">
            <strong>Recomendaciones UX:</strong>
            <span v-if="uxEvaluationStatus === 'loading'" class="ux-evaluator-status">Evaluando...</span>
            <span v-else-if="uxEvaluationStatus === 'error'" class="ux-evaluator-status ux-evaluator-status-error"
              >No disponible</span
            >
          </p>
          <p v-if="uxEvaluationStatus === 'error' && uxEvaluationMessage" class="ux-evaluator-message">
            {{ uxEvaluationMessage }}
          </p>
          <p v-else-if="uxEvaluationStatus === 'ready' && uxEvaluations.length === 0" class="ux-evaluator-message">
            No se encontraron observaciones de UX.
          </p>
          <ul v-else-if="uxEvaluationStatus === 'ready' && uxEvaluations.length" class="ux-evaluator-list">
            <li
              v-for="(observation, observationIndex) in uxEvaluations"
              :key="`${observationIndex}-${observation}`"
              class="ux-evaluator-item"
            >
              {{ observation }}
            </li>
          </ul>
        </div>
      </footer>
    </section>

    <section v-else class="canvas-wrap">
      <article class="canvas-surface flow-surface">
        <div class="flow-toolbar">
          <h2 class="text-body-emphasis flow-toolbar-title">Flujo de tareas</h2>
          <div class="flow-toolbar-actions">
            <button type="button" class="screen-action-btn flow-toolbar-btn" @click="openBuilder">
              Maximizar builder
            </button>
            <button
              type="button"
              class="screen-action-btn flow-toolbar-btn"
              :disabled="screens.length === 0"
              @click="addFlowTask"
            >
              + Nueva tarea
            </button>
          </div>
        </div>
        <div v-if="flowNodes.length === 0" class="canvas-state">
          No hay pantallas aún. Crea o asigna una pantalla para iniciar.
        </div>
        <div v-else class="flow-canvas">
          <VueFlow
            v-model:nodes="flowNodes"
            v-model:edges="flowEdges"
            :default-zoom="1"
            :fit-view-on-init="true"
            :pan-on-drag="false"
            :zoom-on-scroll="false"
            :nodes-draggable="true"
            :snap-to-grid="true"
            :snap-grid="[20, 20]"
            class="flow-canvas-instance"
            @connect="onFlowConnect"
          >
            <template #node-flow-task="{ id }">
              <div class="flow-task">
                <Handle type="target" :position="Position.Left" id="in" class="flow-handle" />
                <Handle type="source" :position="Position.Right" id="out" class="flow-handle" />
                <header class="flow-task-header">
                  <input
                    class="flow-task-title"
                    type="text"
                    :value="getFlowNodeView(id)?.task?.title ?? ''"
                    @input="onFlowNodeInput(id, $event)"
                    placeholder="Nombre de tarea"
                  />
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
      </article>
    </section>

    <section v-if="!isBuilderMinimized" class="floating-prompt">
      <div class="floating-prompt-title">
        <h2>Prompt</h2>
        <button
          type="button"
          class="conversation-toggle-btn"
          :aria-expanded="isConversationVisible"
          aria-controls="conversation-list"
          :title="isConversationVisible ? 'Ocultar historial' : 'Mostrar historial'"
          :aria-label="isConversationVisible ? 'Ocultar historial de conversación' : 'Mostrar historial de conversación'"
          @click="toggleConversationVisibility"
        >
          <i class="bi conversation-toggle-icon" :class="isConversationVisible ? 'bi-eye-slash' : 'bi-eye'" aria-hidden="true"></i>
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
      <div v-if="actionableUxRecommendations.length > 0" class="ux-recommendation-bubbles">
        <b-button
          v-for="suggestion in actionableUxRecommendations"
          :key="suggestion.id"
          type="button"
          class="ux-recommendation-bubble"
          v-b-tooltip="{ title: suggestion.text }"
          :class="{
            btn: true,
            'btn-danger': suggestion.severity === 'high',
            'btn-warning': suggestion.severity === 'medium',
            'btn-dark': suggestion.severity !== 'high' && suggestion.severity !== 'medium',
            'ux-recommendation-bubble--high': suggestion.severity === 'high',
            'ux-recommendation-bubble--medium': suggestion.severity === 'medium',
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
      </div>
      <textarea
        ref="promptInput"
        v-model="promptText"
        rows="4"
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
      <p class="prompt-msg">{{ message }}</p>
    </section>
      <Teleport to="body">
        <div v-if="isDataEditorVisible" class="data-editor-overlay" @click.self="closeDataEditor">
          <div class="data-editor-modal" role="dialog" aria-modal="true" aria-label="Editor de data JSON">
            <header class="data-editor-header">
              <h3>Editar data JSON</h3>
              <button
                type="button"
                class="data-editor-close"
                :disabled="isApplyingData"
                @click="closeDataEditor"
              >
                Cerrar
              </button>
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
              class="data-editor-textarea"
              :disabled="isApplyingData"
            ></textarea>
            <p v-if="dataEditorError" class="data-editor-error">{{ dataEditorError }}</p>
            <div class="data-editor-actions">
              <button type="button" class="screen-action-btn" :disabled="isApplyingData" @click="closeDataEditor">
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
        </div>
      </Teleport>
      <Teleport to="body">
        <div v-if="isPugEditorVisible" class="data-editor-overlay" @click.self="closePugEditor">
          <div class="data-editor-modal" role="dialog" aria-modal="true" aria-label="Editor de pug">
            <header class="data-editor-header">
              <h3>Editar PUG</h3>
              <button
                type="button"
                class="data-editor-close"
                :disabled="isApplyingPug"
                @click="closePugEditor"
              >
                Cerrar
              </button>
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
              class="data-editor-textarea"
              :disabled="isApplyingPug"
            ></textarea>
            <p v-if="pugEditorError" class="data-editor-error">{{ pugEditorError }}</p>
            <div class="data-editor-actions">
              <button type="button" class="screen-action-btn" :disabled="isApplyingPug" @click="closePugEditor">
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
        </div>
      </Teleport>
      <Teleport to="body">
        <div v-if="isCssEditorVisible" class="data-editor-overlay" @click.self="closeCssEditor">
          <div class="data-editor-modal" role="dialog" aria-modal="true" aria-label="Editor de CSS">
            <header class="data-editor-header">
              <h3>Editar CSS</h3>
              <button
                type="button"
                class="data-editor-close"
                :disabled="isApplyingCss"
                @click="closeCssEditor"
              >
                Cerrar
              </button>
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
              class="data-editor-textarea"
              :disabled="isApplyingCss"
            ></textarea>
            <p v-if="cssEditorError" class="data-editor-error">{{ cssEditorError }}</p>
            <div class="data-editor-actions">
              <button type="button" class="screen-action-btn" :disabled="isApplyingCss" @click="closeCssEditor">
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
        </div>
      </Teleport>
  </main>
</template>

<style scoped>
.builder-root {
  min-height: 100vh;
  margin: 0;
  padding: 0;
  background: radial-gradient(circle at 16% 20%, #2a1b6b 0%, transparent 42%),
    radial-gradient(circle at 84% 10%, #1a7bf7 0%, transparent 38%), #0d1020;
  color: #f5f6ff;
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  position: relative;
  overflow: hidden;
}

.canvas-wrap {
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 18px;
  padding: 0;
  background: rgba(16, 19, 36, 0.9);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.35);
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  width: 100%;
  box-sizing: border-box;
}

.canvas-header h1 {
  margin: 0;
  font-size: 1.5rem;
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
  color: #e4e9ff;
}

.screen-select {
  margin-left: 0.45rem;
  min-width: 170px;
  border: 1px solid rgba(255, 255, 255, 0.24);
  border-radius: 8px;
  background: #0e152f;
  color: #f4f7ff;
  padding: 0.38rem 0.56rem;
}

.screen-action-btn {
  border: 1px solid rgba(255, 255, 255, 0.25);
  border-radius: 8px;
  min-width: 86px;
  background: #0e152f;
  color: #f4f7ff;
  height: 2rem;
  padding: 0.38rem 0.6rem;
  cursor: pointer;
}

.screen-action-btn:hover {
  background: rgba(255, 255, 255, 0.08);
}

.screen-action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.theme-control {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: #e4e9ff;
  font-size: 0.82rem;
  white-space: nowrap;
}

.theme-select {
  min-width: 156px;
  border: 1px solid rgba(255, 255, 255, 0.24);
  border-radius: 8px;
  background: #0e152f;
  color: #f4f7ff;
  padding: 0.38rem 0.56rem;
}

.theme-switch {
  display: inline-flex;
  align-items: stretch;
  gap: 0.45rem;
}

.theme-switch-btn {
  border: 1px solid rgba(255, 255, 255, 0.25);
  border-radius: 8px;
  width: 2rem;
  height: 2rem;
  background: #0e152f;
  color: #f4f7ff;
  cursor: pointer;
  display: inline-grid;
  place-items: center;
  padding: 0;
  line-height: 1;
}

.theme-switch-btn:hover {
  background: rgba(255, 255, 255, 0.08);
}

.theme-current {
  min-width: 10rem;
  display: inline-grid;
  place-items: center;
  border: 1px solid rgba(255, 255, 255, 0.22);
  border-radius: 8px;
  background: #0e152f;
  color: #f4f7ff;
  padding: 0.38rem 0.6rem;
  font-size: 0.86rem;
}

.theme-switch-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.theme-hint {
  display: block;
  color: #aeb8db;
  font-size: 0.75rem;
  margin-top: 0.2rem;
}

.canvas-header p {
  margin: 0.4rem 0 0.9rem;
  color: #c8d0ff;
}

.canvas-surface {
  background: var(--bs-body-bg);
  border: 1px dashed rgba(255, 255, 255, 0.2);
  border-radius: 0;
  min-height: 0;
  overflow: hidden;
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
}

.flow-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.75rem;
}

.flow-toolbar h2 {
  margin: 0;
  font-size: 1.02rem;
}

.flow-toolbar-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.flow-toolbar-btn-soft {
  background: rgba(255, 255, 255, 0.1);
}

.flow-canvas {
  position: relative;
  overflow: auto;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 0;
  background: rgba(8, 11, 28, 0.82);
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
  stroke: #9bc0ff;
  stroke-width: 2;
}

.flow-handle {
  width: 10px;
  height: 10px;
  background: #8ec5ff;
  border: 1px solid #1f3566;
  border-radius: 999px;
}

.flow-task {
  position: relative;
  width: 100%;
  min-width: 280px;
  min-height: 260px;
  background: rgba(17, 23, 52, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 12px;
  display: grid;
  gap: 0.55rem;
  padding: 0.6rem;
  color: #f4f7ff;
  box-shadow: 0 10px 25px rgba(2, 10, 26, 0.34);
}

.flow-task-header {
  display: flex;
  gap: 0.45rem;
  align-items: center;
}

.flow-task-title {
  flex: 1;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 8px;
  background: #0d132f;
  color: #f4f7ff;
  padding: 0.35rem 0.55rem;
}

.flow-task-remove {
  width: 2rem;
  min-width: 2rem;
  padding: 0;
}

.flow-task-screen-label {
  font-size: 0.8rem;
  color: #c7d5ef;
}

.flow-task-screen-select {
  border: 1px solid rgba(255, 255, 255, 0.22);
  border-radius: 8px;
  background: #0d132f;
  color: #f4f7ff;
  padding: 0.35rem 0.55rem;
}

.flow-task-preview {
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 8px;
  background: #0a1024;
  width: 300px;
  height: 200px;
  overflow: hidden;
  position: relative;
  padding: 0.2rem;
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
  color: #9ca9ca;
  font-size: 0.78rem;
  padding: 0.45rem;
}

.flow-preview-error {
  margin: 0;
  color: #ff8f8f;
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
  color: #a9b6d4;
  font-size: 0.82rem;
}

.canvas-state {
  min-height: 100%;
  display: grid;
  place-items: center;
  font-size: 1.05rem;
  text-align: center;
  padding: 1rem;
}

.canvas-content {
  padding: 1rem;
}

.canvas-status-layer {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: rgba(253, 254, 255, 0.7);
  backdrop-filter: blur(2px);
  pointer-events: none;
}

.canvas-status-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.45rem 0.8rem;
  border-radius: 999px;
  background: #141a33;
  color: #edf1ff;
  font-size: 0.9rem;
  box-shadow: 0 8px 20px rgba(18, 21, 40, 0.2);
}

.canvas-status-dot {
  width: 0.6rem;
  height: 0.6rem;
  border-radius: 999px;
  background: #4fc3f7;
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
  margin-top: 0.8rem;
  color: #bac0dd;
  font-size: 0.9rem;
}

.canvas-meta p {
  margin: 0.2rem 0;
}

.ux-evaluator {
  margin-top: 0.5rem;
  padding-top: 0.45rem;
  border-top: 1px solid rgba(255, 255, 255, 0.14);
}

.ux-evaluator-title {
  margin: 0;
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-wrap: wrap;
}

.ux-evaluator-status {
  color: #9fd7ff;
  font-size: 0.8rem;
}

.ux-evaluator-status-error {
  color: #ffb4b4;
}

.ux-evaluator-message {
  margin: 0.3rem 0 0;
  color: #95a2c4;
}

.ux-recommendation-bubbles {
  display: flex;
  flex-wrap: nowrap;
  align-items: flex-end;
  gap: 0.35rem;
  overflow-x: auto;
  overflow-y: hidden;
  white-space: nowrap;
  scrollbar-width: none;
  padding: 0.24rem 0 0;
}

.ux-recommendation-bubble {
  align-self: flex-end;
  flex: 0 0 1rem;
  width: 1rem;
  height: 1rem;
  min-width: 1rem;
  min-height: 1rem;
  border-radius: 9999px;
  padding: 0;
  display: inline-flex;
  appearance: none;
  border: 0;
  color: var(--bs-light);
  cursor: pointer;
  align-items: center;
  justify-content: center;
  text-align: center;
  border: 1px solid transparent;
  background: rgba(var(--bs-body-color-rgb, 248, 249, 250), 0.22);
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.16);
  transition: transform 140ms ease, opacity 140ms ease, box-shadow 140ms ease;
  margin-top: 0.12rem;
}

.ux-recommendation-bubble:hover:not(:disabled) {
  transform: translateY(-1px) scale(1.08);
  box-shadow: 0 14px 26px rgba(0, 0, 0, 0.35);
}

.ux-recommendation-bubble:focus-visible {
  outline: 2px solid rgba(255, 255, 255, 0.6);
  outline-offset: 2px;
}

.ux-recommendation-bubble--high {
  border-color: rgba(var(--bs-danger-rgb), 0.8);
  background: color-mix(in srgb, rgba(var(--bs-danger-rgb), 0.42) 52%, transparent);
  color: var(--bs-light);
}

.ux-recommendation-bubble--medium {
  border-color: rgba(var(--bs-warning-rgb), 0.8);
  background: color-mix(in srgb, rgba(var(--bs-warning-rgb), 0.42) 52%, transparent);
  color: var(--bs-dark);
}

.ux-recommendation-bubble-letter {
  font-size: 0.58rem;
  line-height: 1;
  font-weight: 700;
  font-family: var(--bs-font-sans-serif);
  letter-spacing: 0.01em;
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
  animation: ux-bubble-pop 0.38s cubic-bezier(0.15, 1.1, 0.3, 1) forwards;
}

@keyframes ux-bubble-pop {
  0% {
    transform: scale(1);
  }
  20% {
    transform: scale(1.07);
    box-shadow: 0 0 0 0 rgba(255, 255, 255, 0.45);
  }
  60% {
    transform: scale(1.15);
    box-shadow: 0 0 0 10px rgba(255, 255, 255, 0);
  }
  100% {
    transform: scale(0.92);
    opacity: 0.35;
  }
}

.ux-evaluator-list {
  margin: 0.5rem 0 0;
  padding-left: 1.2rem;
  display: grid;
  gap: 0.55rem;
}

.ux-evaluator-item {
  display: grid;
  gap: 0.2rem;
}

.ux-evaluator-severity {
  justify-self: start;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.22);
  font-size: 0.72rem;
  padding: 0.12rem 0.45rem;
  text-transform: uppercase;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.ux-evaluator-severity.severity-high {
  color: #ffb3b3;
  background: rgba(255, 95, 95, 0.15);
  border-color: rgba(255, 95, 95, 0.52);
}

.ux-evaluator-severity.severity-medium {
  color: #ffd17a;
  background: rgba(255, 187, 82, 0.16);
  border-color: rgba(255, 187, 82, 0.5);
}

.ux-evaluator-severity.severity-low {
  color: #9ad5ff;
  background: rgba(74, 161, 255, 0.16);
  border-color: rgba(74, 161, 255, 0.5);
}

.ux-evaluator-issue {
  margin: 0;
  color: #e2e8ff;
  font-weight: 700;
}

.ux-evaluator-recommendation {
  margin: 0;
  color: #adc2eb;
}

.floating-prompt {
  position: fixed;
  z-index: 1000;
  right: 1.4rem;
  bottom: 1.4rem;
  width: min(420px, calc(100vw - 2.8rem));
  background: #11162b;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 14px;
  padding: 0.9rem;
  box-shadow: 0 12px 26px rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(6px);
}

.floating-prompt h2 {
  margin: 0;
  font-size: 1rem;
}

.conversation-list {
  margin-top: 0.65rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-height: 240px;
  overflow: auto;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 10px;
  padding: 0.55rem;
  background: rgba(11, 15, 30, 0.75);
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
  padding: 0;
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
  color: #98a4c7;
  font-size: 0.85rem;
}

.conversation-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.conversation-row.user {
  color: #f6f8ff;
}

.conversation-row.assistant {
  color: #f6d14f;
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
  background: #3a82ff;
  color: #fff;
  cursor: pointer;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.prompt-action-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.prompt-action-btn i {
  font-size: 1.08rem;
}

.floating-prompt textarea {
  margin-top: 0.65rem;
  width: 100%;
  box-sizing: border-box;
  border: 1px solid rgba(255, 255, 255, 0.25);
  border-radius: 10px;
  background: #0d142e;
  color: #f5f6ff;
  padding: 0.6rem;
  min-height: 130px;
  resize: vertical;
}

.floating-prompt button {
  margin-top: 0.6rem;
  border: 0;
  border-radius: 10px;
  background: #3a82ff;
  color: #fff;
  padding: 0.58rem;
  cursor: pointer;
  font-weight: 700;
}

.floating-prompt button:hover:not(:disabled) {
  background: #5c9bff;
}

.floating-prompt .conversation-toggle-btn {
  width: auto;
  margin-top: 0;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 999px;
  background: transparent;
  color: #dfe7ff;
}

.floating-prompt .conversation-toggle-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.1);
}

.data-editor-overlay {
  position: fixed;
  inset: 0;
  background: rgba(10, 14, 28, 0.76);
  z-index: 1100;
  display: grid;
  place-items: center;
  padding: 1rem;
}

.data-editor-modal {
  width: min(760px, calc(100vw - 2.5rem));
  max-height: calc(100vh - 2.5rem);
  background: #11162b;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 12px;
  padding: 0.9rem;
  display: grid;
  gap: 0.75rem;
  color: #f5f8ff;
}

.data-editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.data-editor-close {
  border: 0;
  border-radius: 8px;
  background: #0e152f;
  color: #f4f7ff;
  padding: 0.35rem 0.7rem;
  cursor: pointer;
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
  border: 1px solid rgba(255, 255, 255, 0.25);
  border-radius: 10px;
  background: #0d142e;
  color: #f5f6ff;
  padding: 0.65rem;
  font-family: 'Fira Code', Menlo, Monaco, Consolas, 'Courier New', monospace;
  font-size: 0.8rem;
  line-height: 1.35;
  box-sizing: border-box;
}

.data-editor-input-label {
  margin-bottom: -0.4rem;
  color: #d5ddff;
  font-size: 0.9rem;
}

.data-editor-instruction-textarea {
  width: 100%;
  margin: 0;
  min-height: 72px;
  resize: vertical;
  border: 1px solid rgba(255, 255, 255, 0.25);
  border-radius: 10px;
  background: #0d142e;
  color: #f5f6ff;
  padding: 0.65rem;
  font-family: 'Inter', sans-serif;
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
  color: #ff6f6f;
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
  background: #5f9dff;
}

.data-editor-apply-btn:hover:not(:disabled) {
  background: #7ab0ff;
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

.floating-prompt button:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.prompt-msg {
  margin: 0.65rem 0 0;
  font-size: 0.87rem;
  color: #a9b5d3;
}

.pipeline-missing {
  border: 1px dashed #fdc100;
  border-radius: 10px;
  background: linear-gradient(160deg, #fff8dd, #fef3c2);
  color: #6b4f00;
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
