import { createI18n } from 'vue-i18n';
import en from './locales/en.json';
import es from './locales/es.json';

const DEFAULT_LOCALE = 'en';
const FALLBACK_LOCALE = 'en';

export const i18n = createI18n({
  legacy: false,
  locale: DEFAULT_LOCALE,
  fallbackLocale: FALLBACK_LOCALE,
  messages: {
    en,
    es,
  },
});

