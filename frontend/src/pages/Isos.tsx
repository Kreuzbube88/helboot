import { useTranslation } from 'react-i18next'

export function Isos() {
  const { t } = useTranslation()
  return (
    <>
      <h1>{t('isos.title')}</h1>
      <div className="card">
        <p className="muted">{t('isos.empty')}</p>
        <p className="muted">{t('isos.comingSoon')}</p>
      </div>
    </>
  )
}
