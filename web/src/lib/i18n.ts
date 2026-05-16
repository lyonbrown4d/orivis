import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import en from '@/locales/en.json';
import zh from '@/locales/zh.json';

const preferred = localStorage.getItem('orivis_lang') || navigator.language;

i18n.use(initReactI18next).init({
  resources: { en: { translation: en }, zh: { translation: zh } },
  lng: preferred?.toLowerCase().startsWith('zh') ? 'zh' : 'en',
  fallbackLng: 'en',
  interpolation: { escapeValue: false }
});

export function setLanguage(code: 'en' | 'zh') {
  localStorage.setItem('orivis_lang', code);
  return i18n.changeLanguage(code);
}

export default i18n;
