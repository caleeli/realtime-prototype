<script setup lang="ts">
import { ref, watch, onMounted } from 'vue';
import { BButton, BFormTextarea, BFormGroup } from 'bootstrap-vue-next';
import type { ProjectSettings } from '../services/projectSessionService';

interface Props {
  settings: ProjectSettings | null;
  isLoading?: boolean;
  isSaving?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  isLoading: false,
  isSaving: false,
});

const emit = defineEmits<{
  (e: 'save', settings: Omit<ProjectSettings, 'projectId' | 'updatedAt'>): void;
}>();

const localSettings = ref({
  designStyle: '',
  colorPalette: '',
  brandGuidelines: '',
  componentExamples: '',
  technicalConstraints: '',
  layoutPreferences: '',
  imageGenerationNotes: '',
  generationContext: '',
});

const fieldLabels: Record<string, string> = {
  designStyle: 'Estilo de diseño',
  colorPalette: 'Paleta de colores',
  brandGuidelines: 'Guías de marca',
  componentExamples: 'Ejemplos de componentes',
  technicalConstraints: 'Restricciones técnicas',
  layoutPreferences: 'Preferencias de layout',
  imageGenerationNotes: 'Notas para generación de imágenes',
  generationContext: 'Contexto adicional de generación',
};

const fieldPlaceholders: Record<string, string> = {
  designStyle: 'Ej: Minimalista, moderno, corporativo, playful...',
  colorPalette: 'Ej: Primary: #3B82F6, Secondary: #10B981, Neutral: #6B7280...',
  brandGuidelines: 'Ej: Tipografía, tono de voz, principios de diseño...',
  componentExamples: 'Ej: Preferir cards con sombras suaves, botones redondeados...',
  technicalConstraints: 'Ej: Soporte para IE11, accesibilidad WCAG 2.1 AA...',
  layoutPreferences: 'Ej: Densidad compacta, espaciado amplio, grid de 12 columnas...',
  imageGenerationNotes: 'Ej: Estilo ilustrativo, fotográfico, flat design...',
  generationContext: 'Ej: Contexto específico del proyecto, usuarios target, industria...',
};

const fieldDescriptions: Record<string, string> = {
  designStyle: 'Define el estilo visual general que se aplicará a las pantallas generadas.',
  colorPalette: 'Especifica los colores principales y secundarios del proyecto.',
  brandGuidelines: 'Describe las guías de marca que deben respetarse en el diseño.',
  componentExamples: 'Proporciona ejemplos de componentes preferidos o patrones comunes.',
  technicalConstraints: 'Indica restricciones técnicas o requisitos especiales.',
  layoutPreferences: 'Define preferencias de espaciado, densidad y estructura.',
  imageGenerationNotes: 'Guías específicas para la generación de imágenes de inspiración.',
  generationContext: 'Contexto adicional que se incluirá en todos los prompts de generación.',
};

watch(
  () => props.settings,
  (newSettings) => {
    if (newSettings) {
      localSettings.value = {
        designStyle: newSettings.designStyle || '',
        colorPalette: newSettings.colorPalette || '',
        brandGuidelines: newSettings.brandGuidelines || '',
        componentExamples: newSettings.componentExamples || '',
        technicalConstraints: newSettings.technicalConstraints || '',
        layoutPreferences: newSettings.layoutPreferences || '',
        imageGenerationNotes: newSettings.imageGenerationNotes || '',
        generationContext: newSettings.generationContext || '',
      };
    }
  },
  { immediate: true }
);

function handleSave() {
  emit('save', { ...localSettings.value });
}

function getFieldId(field: string): string {
  return `project-settings-${field}`;
}
</script>

<template>
  <div class="project-settings-panel">
    <header class="project-settings-header">
      <h2 class="project-settings-title">Configuración del proyecto</h2>
      <p class="project-settings-subtitle">
        Estas propiedades enriquecen los system prompts para generación de UI, inspiración e imágenes.
      </p>
    </header>

    <div v-if="isLoading" class="project-settings-loading">
      <div class="spinner-border spinner-border-sm text-primary" role="status">
        <span class="visually-hidden">Cargando...</span>
      </div>
      <span class="loading-text">Cargando configuración...</span>
    </div>

    <form v-else class="project-settings-form" @submit.prevent="handleSave">
      <div class="project-settings-sections">
        <section class="settings-section">
          <h3 class="settings-section-title">Diseño visual</h3>
          <p class="settings-section-desc">Configuraciones que afectan la apariencia de las pantallas generadas.</p>

          <BFormGroup>
            <label :for="getFieldId('designStyle')" class="form-label">{{ fieldLabels.designStyle }}</label>
            <BFormTextarea
              :id="getFieldId('designStyle')"
              v-model="localSettings.designStyle"
              :placeholder="fieldPlaceholders.designStyle"
              rows="2"
            />
            <small class="form-text text-muted">{{ fieldDescriptions.designStyle }}</small>
          </BFormGroup>

          <BFormGroup>
            <label :for="getFieldId('colorPalette')" class="form-label">{{ fieldLabels.colorPalette }}</label>
            <BFormTextarea
              :id="getFieldId('colorPalette')"
              v-model="localSettings.colorPalette"
              :placeholder="fieldPlaceholders.colorPalette"
              rows="3"
            />
            <small class="form-text text-muted">{{ fieldDescriptions.colorPalette }}</small>
          </BFormGroup>

          <BFormGroup>
            <label :for="getFieldId('brandGuidelines')" class="form-label">{{ fieldLabels.brandGuidelines }}</label>
            <BFormTextarea
              :id="getFieldId('brandGuidelines')"
              v-model="localSettings.brandGuidelines"
              :placeholder="fieldPlaceholders.brandGuidelines"
              rows="4"
            />
            <small class="form-text text-muted">{{ fieldDescriptions.brandGuidelines }}</small>
          </BFormGroup>
        </section>

        <section class="settings-section">
          <h3 class="settings-section-title">Componentes y patrones</h3>
          <p class="settings-section-desc">Preferencias para la selección y estilo de componentes.</p>

          <BFormGroup>
            <label :for="getFieldId('componentExamples')" class="form-label">{{ fieldLabels.componentExamples }}</label>
            <BFormTextarea
              :id="getFieldId('componentExamples')"
              v-model="localSettings.componentExamples"
              :placeholder="fieldPlaceholders.componentExamples"
              rows="4"
            />
            <small class="form-text text-muted">{{ fieldDescriptions.componentExamples }}</small>
          </BFormGroup>

          <BFormGroup>
            <label :for="getFieldId('layoutPreferences')" class="form-label">{{ fieldLabels.layoutPreferences }}</label>
            <BFormTextarea
              :id="getFieldId('layoutPreferences')"
              v-model="localSettings.layoutPreferences"
              :placeholder="fieldPlaceholders.layoutPreferences"
              rows="3"
            />
            <small class="form-text text-muted">{{ fieldDescriptions.layoutPreferences }}</small>
          </BFormGroup>
        </section>

        <section class="settings-section">
          <h3 class="settings-section-title">Restricciones y contexto</h3>
          <p class="settings-section-desc">Requisitos técnicos y contexto adicional para la generación.</p>

          <BFormGroup>
            <label :for="getFieldId('technicalConstraints')" class="form-label">{{ fieldLabels.technicalConstraints }}</label>
            <BFormTextarea
              :id="getFieldId('technicalConstraints')"
              v-model="localSettings.technicalConstraints"
              :placeholder="fieldPlaceholders.technicalConstraints"
              rows="3"
            />
            <small class="form-text text-muted">{{ fieldDescriptions.technicalConstraints }}</small>
          </BFormGroup>

          <BFormGroup>
            <label :for="getFieldId('generationContext')" class="form-label">{{ fieldLabels.generationContext }}</label>
            <BFormTextarea
              :id="getFieldId('generationContext')"
              v-model="localSettings.generationContext"
              :placeholder="fieldPlaceholders.generationContext"
              rows="4"
            />
            <small class="form-text text-muted">{{ fieldDescriptions.generationContext }}</small>
          </BFormGroup>
        </section>

        <section class="settings-section">
          <h3 class="settings-section-title">Inspiración e imágenes</h3>
          <p class="settings-section-desc">Configuraciones específicas para la generación de inspiración visual.</p>

          <BFormGroup>
            <label :for="getFieldId('imageGenerationNotes')" class="form-label">{{ fieldLabels.imageGenerationNotes }}</label>
            <BFormTextarea
              :id="getFieldId('imageGenerationNotes')"
              v-model="localSettings.imageGenerationNotes"
              :placeholder="fieldPlaceholders.imageGenerationNotes"
              rows="3"
            />
            <small class="form-text text-muted">{{ fieldDescriptions.imageGenerationNotes }}</small>
          </BFormGroup>
        </section>
      </div>

      <div class="project-settings-actions">
        <BButton
          type="submit"
          variant="primary"
          :disabled="isSaving"
        >
          <span v-if="isSaving" class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
          {{ isSaving ? 'Guardando...' : 'Guardar configuración' }}
        </BButton>
      </div>
    </form>
  </div>
</template>

<style scoped>
.project-settings-panel {
  padding: 1.5rem;
  height: 100%;
  overflow-y: auto;
}

.project-settings-header {
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--bs-border-color);
}

.project-settings-title {
  font-size: 1.25rem;
  font-weight: 600;
  margin: 0 0 0.5rem;
  color: var(--bs-body-color);
}

.project-settings-subtitle {
  font-size: 0.875rem;
  color: var(--bs-secondary-color);
  margin: 0;
}

.project-settings-loading {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 2rem;
  color: var(--bs-secondary-color);
}

.loading-text {
  font-size: 0.875rem;
}

.project-settings-form {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.project-settings-sections {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.settings-section {
  padding: 1.25rem;
  background: var(--bs-tertiary-bg);
  border-radius: 0.5rem;
  border: 1px solid var(--bs-border-color);
}

.settings-section-title {
  font-size: 1rem;
  font-weight: 600;
  margin: 0 0 0.25rem;
  color: var(--bs-body-color);
}

.settings-section-desc {
  font-size: 0.8125rem;
  color: var(--bs-secondary-color);
  margin: 0 0 1rem;
}

:deep(.form-group) {
  margin-bottom: 1rem;
}

:deep(.form-group:last-child) {
  margin-bottom: 0;
}

.form-label {
  font-size: 0.875rem;
  font-weight: 500;
  margin-bottom: 0.375rem;
  display: inline-block;
}

:deep(.form-text) {
  font-size: 0.75rem;
  margin-top: 0.375rem;
  display: block;
}

.project-settings-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 1rem;
  border-top: 1px solid var(--bs-border-color);
}
</style>
