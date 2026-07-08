import { useTranslation } from 'react-i18next'
import { setLanguage, supportedLanguages, type Language } from '../i18n'

const languageNames: Record<Language, string> = {
  en: 'English',
  de: 'Deutsch',
}

export function LanguageSwitcher() {
  const { i18n, t } = useTranslation()
  return (
    <select
      aria-label={t('common.language')}
      value={i18n.language}
      onChange={(e) => setLanguage(e.target.value as Language)}
    >
      {supportedLanguages.map((lang) => (
        <option key={lang} value={lang}>
          {languageNames[lang]}
        </option>
      ))}
    </select>
  )
}
