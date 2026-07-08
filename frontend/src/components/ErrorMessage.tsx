import { useTranslation } from 'react-i18next'
import { ApiError } from '../api/client'

/**
 * Renders an API error localized via its stable error code; unknown
 * codes fall back to the server's English message.
 */
export function ErrorMessage({ error }: { error: unknown }) {
  const { t } = useTranslation()
  if (!error) return null
  if (error instanceof ApiError) {
    return <p className="error">{t(error.i18nKey, { defaultValue: error.message })}</p>
  }
  return <p className="error">{t('errors.internal')}</p>
}
