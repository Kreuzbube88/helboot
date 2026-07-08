import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import { ErrorMessage } from '../components/ErrorMessage'
import { setLanguage, supportedLanguages, type Language } from '../i18n'

type NetworkMode = 'proxy_dhcp' | 'dhcp'

const steps = ['stepLanguage', 'stepAdmin', 'stepNetwork', 'stepSummary'] as const

/**
 * First-run wizard (§24): language → admin account → network mode →
 * summary. ISO upload and the first profile are optional follow-ups
 * inside the app once it is set up.
 */
export function SetupWizard({ onDone }: { onDone: () => void }) {
  const { t, i18n } = useTranslation()
  const [step, setStep] = useState(0)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [mode, setMode] = useState<NetworkMode>('proxy_dhcp')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  const language = (supportedLanguages.includes(i18n.language as Language)
    ? i18n.language
    : 'en') as Language

  const stepValid =
    step === 1 ? username.trim().length >= 3 && password.length >= 10 : true

  async function finish() {
    setBusy(true)
    setError(null)
    try {
      await api.post('/setup', {
        language,
        admin: { username: username.trim(), password },
        network: { mode },
      })
      onDone()
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <div className="center-screen">
      <div className="card wizard-card">
        <h1>{t('setup.title')}</h1>
        <p className="muted">{t('setup.intro')}</p>
        <div className="wizard-steps">
          {steps.map((s, i) => (
            <span key={s} className={i === step ? 'current' : ''}>
              {i + 1}. {t(`setup.${s}`)}
            </span>
          ))}
        </div>

        {step === 0 && (
          <div className="field">
            <label>{t('common.language')}</label>
            {supportedLanguages.map((lang) => (
              <label
                key={lang}
                className={`radio-option ${language === lang ? 'selected' : ''}`}
              >
                <input
                  type="radio"
                  name="language"
                  checked={language === lang}
                  onChange={() => setLanguage(lang)}
                />{' '}
                {lang === 'de' ? 'Deutsch' : 'English'}
              </label>
            ))}
          </div>
        )}

        {step === 1 && (
          <>
            <div className="field">
              <label htmlFor="admin-user">{t('setup.adminUsername')}</label>
              <input
                id="admin-user"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
              />
            </div>
            <div className="field">
              <label htmlFor="admin-pass">{t('setup.adminPassword')}</label>
              <input
                id="admin-pass"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="new-password"
              />
              <small className="muted">{t('setup.adminPasswordHint')}</small>
            </div>
          </>
        )}

        {step === 2 && (
          <div className="field">
            <label>{t('setup.networkQuestion')}</label>
            <label className={`radio-option ${mode === 'proxy_dhcp' ? 'selected' : ''}`}>
              <input
                type="radio"
                name="mode"
                checked={mode === 'proxy_dhcp'}
                onChange={() => setMode('proxy_dhcp')}
              />{' '}
              {t('setup.networkProxy')}
              <div className="muted">{t('setup.networkProxyHint')}</div>
            </label>
            <label className={`radio-option ${mode === 'dhcp' ? 'selected' : ''}`}>
              <input
                type="radio"
                name="mode"
                checked={mode === 'dhcp'}
                onChange={() => setMode('dhcp')}
              />{' '}
              {t('setup.networkDhcp')}
              <div className="muted">{t('setup.networkDhcpHint')}</div>
            </label>
          </div>
        )}

        {step === 3 && (
          <>
            <p>{t('setup.summaryIntro')}</p>
            <table>
              <tbody>
                <tr>
                  <th>{t('setup.summaryLanguage')}</th>
                  <td>{language === 'de' ? 'Deutsch' : 'English'}</td>
                </tr>
                <tr>
                  <th>{t('setup.summaryAdmin')}</th>
                  <td>{username}</td>
                </tr>
                <tr>
                  <th>{t('setup.summaryNetwork')}</th>
                  <td>{mode === 'proxy_dhcp' ? t('setup.modeProxyDhcp') : t('setup.modeDhcp')}</td>
                </tr>
              </tbody>
            </table>
            <ErrorMessage error={error} />
          </>
        )}

        <div className="wizard-nav">
          <button onClick={() => setStep(step - 1)} disabled={step === 0 || busy}>
            {t('common.back')}
          </button>
          {step < steps.length - 1 ? (
            <button className="primary" onClick={() => setStep(step + 1)} disabled={!stepValid}>
              {t('common.next')}
            </button>
          ) : (
            <button className="primary" onClick={finish} disabled={busy}>
              {t('common.finish')}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
