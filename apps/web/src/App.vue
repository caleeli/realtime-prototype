<script setup lang="ts">
import {
  computed,
  defineComponent,
  h,
  markRaw,
  type Component,
  onBeforeUnmount,
  ref,
  type Ref,
} from 'vue';

import {
  GenerationPipelineService,
  type GenerationMessage,
  type GenerationRequest,
} from './services/generationPipelineService';
import {
  buildGeneratedScreen,
  type GeneratedScreenView,
  type GenerationRenderOptions,
} from './services/generationRenderService';

const pipelineService = new GenerationPipelineService({
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

const promptText: Ref<string> = ref('');
const conversation: Ref<ChatMessage[]> = ref([]);
const isGenerating = ref(false);
const message = ref('Escribe una descripción y pulsa "Generar pantalla".');
const generatedState: Ref<GeneratedViewState | null> = ref(null);
const generatedComponent: Ref<Component | null> = ref(null);
const cleanupStyle = ref<(() => void) | null>(null);

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

  if (cleanupStyle.value) {
    cleanupStyle.value();
    cleanupStyle.value = null;
  }

  const payload: GenerationRequest = {
    prompt,
    context: {
      locale: navigator.language || 'es-ES',
      theme: 'light',
      targetDensity: 'compact',
      enabledPacks: ['advanced-inputs', 'files'],
    },
    messages: buildUserPayloadMessages(history),
  };

  try {
    const pipelineOutput = await pipelineService.generate(payload);
    if (pipelineOutput.messages.length > 0) {
      syncConversationFromBackend(pipelineOutput.messages);
    } else {
      conversation.value = [
        ...normalizeChatMessages(history),
        { role: 'assistant', content: 'Respuesta generada por la IA.' },
      ];
    }

    const renderedView = await buildGeneratedScreen(pipelineOutput, {
      componentLoaders,
      styleId: 'pipeline-runtime-generated',
    });

    cleanupStyle.value = renderedView.installStyles;

    generatedState.value = {
      view: renderedView,
      component: renderedView.component,
    };
    generatedComponent.value = markRaw(renderedView.component);

    message.value = renderedView.missingComponents.length
      ? `Pantalla renderizada con componentes faltantes: ${renderedView.missingComponents.join(', ')}`
      : 'Pantalla renderizada correctamente.';
  } catch (error) {
    generatedComponent.value = null;
    generatedState.value = null;
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

  promptText.value = '';
  conversation.value = [...normalizeChatMessages(conversation.value), { role: 'user', content: trimmed }];
  await renderPipeline(trimmed, conversation.value);
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
}

onBeforeUnmount(() => {
  if (cleanupStyle.value) {
    cleanupStyle.value();
    cleanupStyle.value = null;
  }
});
</script>

<template>
  <main class="builder-root">
    <section class="canvas-wrap">
      <header class="canvas-header">
        <h1>Builder Editor</h1>
        <p>Genera pantallas con IA y las dibuja en vivo en el canvas.</p>
      </header>

      <article class="canvas-surface">
        <div v-if="isGenerating" class="canvas-state">Generando...</div>
        <div v-else-if="!generatedComponent" class="canvas-state">{{ message }}</div>
        <div v-else class="canvas-content">
          <component :is="generatedComponent" />
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
      </footer>
    </section>

    <section class="floating-prompt">
      <h2>Prompt</h2>
      <div class="conversation-list">
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
      <textarea
        v-model="promptText"
        rows="4"
        :placeholder="promptPlaceholder"
        :disabled="isGenerating"
      ></textarea>
      <div class="prompt-actions">
        <button type="button" :disabled="isGenerating" @click="onGenerate">
          {{ isGenerating ? 'Generando…' : '▶' }}
        </button>
        <button
          type="button"
          class="conversation-refresh"
          :disabled="isGenerating || lastUserMessageIndex < 0"
          title="Regenerar desde el último mensaje"
          @click="onRefresh(lastUserMessageIndex)"
        >
          ⟳
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

.canvas-header p {
  margin: 0.4rem 0 0.9rem;
  color: #c8d0ff;
}

.canvas-surface {
  background: #fdfefe;
  border: 1px dashed rgba(255, 255, 255, 0.2);
  border-radius: 14px;
  min-height: calc(100vh - 14rem);
  max-height: calc(100vh - 14rem);
  overflow: auto;
  color: #111;
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

.canvas-meta {
  margin-top: 0.8rem;
  color: #bac0dd;
  font-size: 0.9rem;
}

.canvas-meta p {
  margin: 0.2rem 0;
}

.floating-prompt {
  position: fixed;
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

.conversation-refresh {
  border: 0;
  border-radius: 999px;
  width: 28px;
  height: 28px;
  background: #3a82ff;
  color: #fff;
  cursor: pointer;
  font-weight: 700;
}

.conversation-refresh:disabled {
  opacity: 0.45;
  cursor: not-allowed;
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
  width: 100%;
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

.prompt-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.6rem;
}

.prompt-actions button {
  margin-top: 0;
}

.prompt-actions button:first-child {
  flex: 1;
}

.prompt-actions .conversation-refresh {
  width: 44px;
  aspect-ratio: 1 / 1;
  border-radius: 10px;
  flex: 0 0 auto;
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
