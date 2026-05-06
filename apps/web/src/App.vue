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

import {
  GenerationPipelineService,
  type UXEvaluatorResultLine,
  type GenerationMessage,
  type InspirationRequest,
  type GenerationPipelineResult,
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

function getThemeByOffset(offset: number) {
  const index = activeThemeIndex.value;
  if (index < 0 || themeOptions.length === 0) {
    return null;
  }
  const nextIndex = (index + offset + themeOptions.length) % themeOptions.length;
  return themeOptions[nextIndex];
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
}

async function refreshScreensFromSession() {
  const session = await sessionService.getSession();
  screens.value = session.screens || [];
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

function onRollback() {
  if (isGenerating.value || lastUserMessageIndex.value < 0) {
    return;
  }

  conversation.value = normalizeChatMessages(conversation.value.slice(0, lastUserMessageIndex.value));
  message.value = 'Rollback aplicado. Escribe un nuevo mensaje del usuario para generar otra respuesta.';
  focusPromptTextarea();
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
    <section class="canvas-wrap">
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

    <section class="floating-prompt">
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
        <button
          v-for="suggestion in actionableUxRecommendations"
          :key="suggestion.id"
          type="button"
          class="ux-recommendation-bubble"
          :class="{
            'ux-recommendation-bubble--high': suggestion.severity === 'high',
            'ux-recommendation-bubble--medium': suggestion.severity === 'medium',
            'ux-recommendation-bubble--burst': explodingBubbleId === suggestion.id,
          }"
          :title="`Aplicar sugerencia ${suggestion.text}`"
          @click="onUxSuggestionClick(suggestion)"
        >
          <span class="ux-recommendation-severity">
            {{ suggestion.severity.toUpperCase() }}
          </span>
          <span class="ux-recommendation-text">{{ suggestion.text }}</span>
        </button>
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
          v-b-tooltip.hover="'Generar pantalla'"
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
  </main>
</template>

<style scoped>
.builder-root {
  min-height: 100vh;
  margin: 0;
  padding: 2rem;
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
  padding: 1rem;
  background: rgba(16, 19, 36, 0.9);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.35);
  min-height: calc(100vh - 4rem);
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
  border-radius: 14px;
  min-height: calc(100vh - 14rem);
  max-height: calc(100vh - 14rem);
  overflow: auto;
  color: var(--bs-body-color);
  position: relative;
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
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.7rem;
}

.ux-recommendation-bubble {
  appearance: none;
  border: 0;
  border-radius: 999px;
  padding: 0.45rem 0.62rem;
  color: #f4f7ff;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.45rem;
  max-width: 100%;
  text-align: left;
  background: rgba(15, 23, 54, 0.9);
  border: 1px solid rgba(255, 255, 255, 0.16);
  box-shadow: 0 10px 20px rgba(0, 0, 0, 0.22);
  transition: transform 140ms ease, box-shadow 140ms ease;
}

.ux-recommendation-bubble:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 14px 26px rgba(0, 0, 0, 0.35);
}

.ux-recommendation-bubble:focus-visible {
  outline: 2px solid rgba(255, 255, 255, 0.6);
  outline-offset: 2px;
}

.ux-recommendation-bubble--high {
  border-color: rgba(255, 118, 118, 0.58);
  background: rgba(87, 22, 22, 0.55);
}

.ux-recommendation-bubble--medium {
  border-color: rgba(240, 180, 42, 0.58);
  background: rgba(83, 55, 5, 0.45);
}

.ux-recommendation-bubble--high .ux-recommendation-severity {
  color: #ffcdcd;
  background: rgba(255, 95, 95, 0.2);
  border-color: rgba(255, 145, 145, 0.58);
}

.ux-recommendation-bubble--medium .ux-recommendation-severity {
  color: #ffdfaf;
  background: rgba(255, 188, 64, 0.2);
  border-color: rgba(255, 205, 104, 0.58);
}

.ux-recommendation-severity {
  text-transform: uppercase;
  font-size: 0.66rem;
  border-radius: 999px;
  border: 1px solid currentColor;
  padding: 0.08rem 0.34rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  white-space: nowrap;
}

.ux-recommendation-text {
  font-size: 0.78rem;
  color: #f7f9ff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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
