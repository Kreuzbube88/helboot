import { NavLink, Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { User } from '../api/types'
import { LanguageSwitcher } from './LanguageSwitcher'

export function Layout({ user, onLogout }: { user: User; onLogout: () => void }) {
  const { t } = useTranslation()

  async function logout() {
    try {
      await api.post('/auth/logout')
    } finally {
      onLogout()
    }
  }

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="brand">{t('app.title')}</div>
        <nav>
          <NavLink to="/" end>
            {t('nav.dashboard')}
          </NavLink>
          <NavLink to="/hosts">{t('nav.hosts')}</NavLink>
          <NavLink to="/profiles">{t('nav.profiles')}</NavLink>
          <NavLink to="/isos">{t('nav.isos')}</NavLink>
          <NavLink to="/installations">{t('nav.installations')}</NavLink>
          <NavLink to="/logs">{t('nav.logs')}</NavLink>
          {user.role === 'admin' && <NavLink to="/users">{t('nav.users')}</NavLink>}
          <NavLink to="/settings">{t('nav.settings')}</NavLink>
        </nav>
        <LanguageSwitcher />
        <div className="muted">{user.username}</div>
        <button onClick={logout}>{t('nav.logout')}</button>
      </aside>
      <main className="content">
        <Outlet />
      </main>
    </div>
  )
}
