<script setup lang="ts">
import { ref, watch } from 'vue';
import { BButton, BFormTextarea, BFormGroup } from 'bootstrap-vue-next';
import { useI18n } from 'vue-i18n';
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

const { t } = useI18n();

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
      <h2 class="project-settings-title">{{ t('projectSettings.title') }}</h2>
      <p class="project-settings-subtitle">
        {{ t('projectSettings.subtitle') }}
      </p>
    </header>

    <div v-if="isLoading" class="project-settings-loading">
      <div class="spinner-border spinner-border-sm text-primary" role="status">
        <span class="visually-hidden">{{ t('projectSettings.loading') }}</span>
      </div>
      <span class="loading-text">{{ t('projectSettings.loadingConfig') }}</span>
    </div>

    <form v-else class="project-settings-form" @submit.prevent="handleSave">
      <div class="project-settings-sections">
        <section class="settings-section">
          <h3 class="settings-section-title">{{ t('projectSettings.section.visualDesign') }}</h3>
          <p class="settings-section-desc">{{ t('projectSettings.section.visualDesignDesc') }}</p>

          <BFormGroup>
            <label :for="getFieldId('designStyle')" class="form-label">{{ t('projectSettings.fields.designStyle') }}</label>
            <BFormTextarea
              :id="getFieldId('designStyle')"
              v-model="localSettings.designStyle"
              :placeholder="t('projectSettings.placeholders.designStyle')"
              rows="2"
            />
            <small class="form-text text-muted">{{ t('projectSettings.descriptions.designStyle') }}</small>
          </BFormGroup>

          <BFormGroup>
            <label :for="getFieldId('colorPalette')" class="form-label">{{ t('projectSettings.fields.colorPalette') }}</label>
            <BFormTextarea
              :id="getFieldId('colorPalette')"
              v-model="localSettings.colorPalette"
              :placeholder="t('projectSettings.placeholders.colorPalette')"
              rows="3"
            />
            <small class="form-text text-muted">{{ t('projectSettings.descriptions.colorPalette') }}</small>
          </BFormGroup>

          <BFormGroup>
            <label :for="getFieldId('brandGuidelines')" class="form-label">{{ t('projectSettings.fields.brandGuidelines') }}</label>
            <BFormTextarea
              :id="getFieldId('brandGuidelines')"
              v-model="localSettings.brandGuidelines"
              :placeholder="t('projectSettings.placeholders.brandGuidelines')"
              rows="4"
            />
            <small class="form-text text-muted">{{ t('projectSettings.descriptions.brandGuidelines') }}</small>
          </BFormGroup>
        </section>

        <section class="settings-section">
          <h3 class="settings-section-title">{{ t('projectSettings.section.componentsPatterns') }}</h3>
          <p class="settings-section-desc">{{ t('projectSettings.section.componentsPatternsDesc') }}</p>

          <BFormGroup>
            <label :for="getFieldId('componentExamples')" class="form-label">{{ t('projectSettings.fields.componentExamples') }}</label>
            <BFormTextarea
              :id="getFieldId('componentExamples')"
              v-model="localSettings.componentExamples"
              :placeholder="t('projectSettings.placeholders.componentExamples')"
              rows="4"
            />
            <small class="form-text text-muted">{{ t('projectSettings.descriptions.componentExamples') }}</small>
          </BFormGroup>

          <BFormGroup>
            <label :for="getFieldId('layoutPreferences')" class="form-label">{{ t('projectSettings.fields.layoutPreferences') }}</label>
            <BFormTextarea
              :id="getFieldId('layoutPreferences')"
              v-model="localSettings.layoutPreferences"
              :placeholder="t('projectSettings.placeholders.layoutPreferences')"
              rows="3"
            />
            <small class="form-text text-muted">{{ t('projectSettings.descriptions.layoutPreferences') }}</small>
          </BFormGroup>
        </section>

        <section class="settings-section">
          <h3 class="settings-section-title">{{ t('projectSettings.section.constraintsContext') }}</h3>
          <p class="settings-section-desc">{{ t('projectSettings.section.constraintsContextDesc') }}</p>

          <BFormGroup>
            <label :for="getFieldId('technicalConstraints')" class="form-label">{{ t('projectSettings.fields.technicalConstraints') }}</label>
            <BFormTextarea
              :id="getFieldId('technicalConstraints')"
              v-model="localSettings.technicalConstraints"
              :placeholder="t('projectSettings.placeholders.technicalConstraints')"
              rows="3"
            />
            <small class="form-text text-muted">{{ t('projectSettings.descriptions.technicalConstraints') }}</small>
          </BFormGroup>

          <BFormGroup>
            <label :for="getFieldId('generationContext')" class="form-label">{{ t('projectSettings.fields.generationContext') }}</label>
            <BFormTextarea
              :id="getFieldId('generationContext')"
              v-model="localSettings.generationContext"
              :placeholder="t('projectSettings.placeholders.generationContext')"
              rows="4"
            />
            <small class="form-text text-muted">{{ t('projectSettings.descriptions.generationContext') }}</small>
          </BFormGroup>
        </section>

        <section class="settings-section">
          <h3 class="settings-section-title">{{ t('projectSettings.section.inspirationImages') }}</h3>
          <p class="settings-section-desc">{{ t('projectSettings.section.inspirationImagesDesc') }}</p>

          <BFormGroup>
            <label :for="getFieldId('imageGenerationNotes')" class="form-label">{{ t('projectSettings.fields.imageGenerationNotes') }}</label>
            <BFormTextarea
              :id="getFieldId('imageGenerationNotes')"
              v-model="localSettings.imageGenerationNotes"
              :placeholder="t('projectSettings.placeholders.imageGenerationNotes')"
              rows="3"
            />
            <small class="form-text text-muted">{{ t('projectSettings.descriptions.imageGenerationNotes') }}</small>
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
          {{ isSaving ? t('projectSettings.saving') : t('projectSettings.save') }}
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
