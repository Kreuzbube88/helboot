import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api, setCsrfToken } from '../api/client'
import type { SessionInfo, User } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'
import { LanguageSwitcher } from '../components/LanguageSwitcher'

export function Login({ onLogin }: { onLogin: (user: User) => void }) {
  const { t } = useTranslation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const session = await api.post<SessionInfo>('/auth/login', { username, password })
      setCsrfToken(session.csrfToken)
      onLogin(session.user)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="center-screen">
      <div className="card auth-card">
        <div className="toolbar">
          <h1>{t('login.title')}</h1>
          <LanguageSwitcher />
        </div>
        <form onSubmit={submit}>
          <div className="field">
            <label htmlFor="username">{t('login.username')}</label>
            <input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
            />
          </div>
          <div className="field">
            <label htmlFor="password">{t('login.password')}</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
          </div>
          <ErrorMessage error={error} />
          <button className="primary" type="submit" disabled={busy}>
            {t('login.submit')}
          </button>
        </form>
      </div>
    </div>
  )
}
