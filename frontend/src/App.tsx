import { useEffect, useState } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { api, setCsrfToken } from './api/client'
import type { SessionInfo, User } from './api/types'
import { Layout } from './components/Layout'
import { Dashboard } from './pages/Dashboard'
import { Hosts } from './pages/Hosts'
import { Installations } from './pages/Installations'
import { Isos } from './pages/Isos'
import { Login } from './pages/Login'
import { Logs } from './pages/Logs'
import { Profiles } from './pages/Profiles'
import { Settings } from './pages/Settings'
import { SetupWizard } from './pages/SetupWizard'

type Boot =
  | { state: 'loading' }
  | { state: 'setup' }
  | { state: 'login' }
  | { state: 'ready'; user: User }

export default function App() {
  const { t } = useTranslation()
  const [boot, setBoot] = useState<Boot>({ state: 'loading' })

  useEffect(() => {
    async function bootstrap() {
      const status = await api.get<{ completed: boolean }>('/setup/status')
      if (!status.completed) {
        setBoot({ state: 'setup' })
        return
      }
      try {
        const session = await api.get<SessionInfo>('/auth/me')
        setCsrfToken(session.csrfToken)
        setBoot({ state: 'ready', user: session.user })
      } catch {
        setBoot({ state: 'login' })
      }
    }
    bootstrap().catch(() => setBoot({ state: 'login' }))
  }, [])

  if (boot.state === 'loading') {
    return <div className="center-screen muted">{t('common.loading')}</div>
  }
  if (boot.state === 'setup') {
    return <SetupWizard onDone={() => setBoot({ state: 'login' })} />
  }
  if (boot.state === 'login') {
    return <Login onLogin={(user) => setBoot({ state: 'ready', user })} />
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout user={boot.user} onLogout={() => setBoot({ state: 'login' })} />}>
          <Route index element={<Dashboard />} />
          <Route path="hosts" element={<Hosts />} />
          <Route path="profiles" element={<Profiles />} />
          <Route path="isos" element={<Isos />} />
          <Route path="installations" element={<Installations />} />
          <Route path="logs" element={<Logs />} />
          <Route path="settings" element={<Settings />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
