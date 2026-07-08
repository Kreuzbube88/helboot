import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import en from './locales/en/translation.json'
import de from './locales/de/translation.json'

export const supportedLanguages = ['en', 'de'] as const
export type Language = (typeof supportedLanguages)[number]

const stored = localStorage.getItem('helboot.language')
const browser = navigator.language.slice(0, 2)
const initial: Language = supportedLanguages.includes(stored as Language)
  ? (stored as Language)
  : supportedLanguages.includes(browser as Language)
    ? (browser as Language)
    : 'en'

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    de: { translation: de },
  },
  lng: initial,
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
})

export function setLanguage(lang: Language) {
  localStorage.setItem('helboot.language', lang)
  i18n.changeLanguage(lang)
}

export default i18n
