import type { ReactNode } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { User } from '../api/types'
import { LanguageSwitcher } from './LanguageSwitcher'

/** Minimal stroke icons for the sidebar nav — inline to avoid a new
 * asset/dependency, sized and colored via CSS (currentColor). */
const icons: Record<string, ReactNode> = {
  dashboard: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="7" height="9" rx="1.5" />
      <rect x="14" y="3" width="7" height="5" rx="1.5" />
      <rect x="14" y="12" width="7" height="9" rx="1.5" />
      <rect x="3" y="16" width="7" height="5" rx="1.5" />
    </svg>
  ),
  hosts: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="4" width="18" height="6" rx="1.5" />
      <rect x="3" y="14" width="18" height="6" rx="1.5" />
      <circle cx="7" cy="7" r="0.6" fill="currentColor" />
      <circle cx="7" cy="17" r="0.6" fill="currentColor" />
    </svg>
  ),
  profiles: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M7 3.5h7l4 4v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1v-16a1 1 0 0 1 1-1Z" />
      <path d="M9.5 12h5M9.5 15.5h5M9.5 8.5h2" />
    </svg>
  ),
  isos: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="8.5" />
      <circle cx="12" cy="12" r="2.4" />
    </svg>
  ),
  installations: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 3v11" />
      <path d="m7.5 10 4.5 4.5L16.5 10" />
      <path d="M4.5 17.5v1.8a1.7 1.7 0 0 0 1.7 1.7h11.6a1.7 1.7 0 0 0 1.7-1.7v-1.8" />
    </svg>
  ),
  logs: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3.5" y="4" width="17" height="16" rx="1.7" />
      <path d="m7.5 9 2.5 2-2.5 2M12 13.5h4.5" />
    </svg>
  ),
  users: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="9" cy="8" r="3" />
      <path d="M3.5 20a5.5 5.5 0 0 1 11 0" />
      <path d="M16 8a3 3 0 1 1 3.9 2.86" />
      <path d="M20.5 20a5 5 0 0 0-3.4-4.74" />
    </svg>
  ),
  settings: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="3.2" />
      <path d="M19.4 13.6a1.8 1.8 0 0 0 .36 1.98l.06.06a2.18 2.18 0 1 1-3.08 3.08l-.06-.06a1.8 1.8 0 0 0-1.98-.36 1.8 1.8 0 0 0-1.1 1.65v.17a2.18 2.18 0 1 1-4.36 0v-.09a1.8 1.8 0 0 0-1.18-1.65 1.8 1.8 0 0 0-1.98.36l-.06.06a2.18 2.18 0 1 1-3.08-3.08l.06-.06a1.8 1.8 0 0 0 .36-1.98 1.8 1.8 0 0 0-1.65-1.1h-.17a2.18 2.18 0 1 1 0-4.36h.09a1.8 1.8 0 0 0 1.65-1.18 1.8 1.8 0 0 0-.36-1.98l-.06-.06a2.18 2.18 0 1 1 3.08-3.08l.06.06a1.8 1.8 0 0 0 1.98.36h.08a1.8 1.8 0 0 0 1.1-1.65v-.17a2.18 2.18 0 1 1 4.36 0v.09a1.8 1.8 0 0 0 1.1 1.65h.08a1.8 1.8 0 0 0 1.98-.36l.06-.06a2.18 2.18 0 1 1 3.08 3.08l-.06.06a1.8 1.8 0 0 0-.36 1.98v.08a1.8 1.8 0 0 0 1.65 1.1h.17a2.18 2.18 0 1 1 0 4.36h-.09a1.8 1.8 0 0 0-1.65 1.1Z" />
    </svg>
  ),
}

function NavIcon({ name }: { name: string }) {
  return icons[name] ?? null
}

export function Layout({ user, onLogout }: { user: User; onLogout: () => void }) {
  const { t } = useTranslation()

  async function logout() {
    try {
      await api.post('/auth/logout')
    } finally {
      onLogout()
    }
  }

  const initial = user.username.slice(0, 1).toUpperCase()

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="sidebar-header">
          <div className="brand-mark">H</div>
          <div className="brand">{t('app.title')}</div>
        </div>
        <nav className="sidebar-nav">
          <NavLink to="/" end>
            <NavIcon name="dashboard" />
            {t('nav.dashboard')}
          </NavLink>
          <NavLink to="/hosts">
            <NavIcon name="hosts" />
            {t('nav.hosts')}
          </NavLink>
          <NavLink to="/profiles">
            <NavIcon name="profiles" />
            {t('nav.profiles')}
          </NavLink>
          <NavLink to="/isos">
            <NavIcon name="isos" />
            {t('nav.isos')}
          </NavLink>
          <NavLink to="/installations">
            <NavIcon name="installations" />
            {t('nav.installations')}
          </NavLink>
          <NavLink to="/logs">
            <NavIcon name="logs" />
            {t('nav.logs')}
          </NavLink>
          {user.role === 'admin' && (
            <NavLink to="/users">
              <NavIcon name="users" />
              {t('nav.users')}
            </NavLink>
          )}
          <NavLink to="/settings">
            <NavIcon name="settings" />
            {t('nav.settings')}
          </NavLink>
        </nav>
        <div className="sidebar-footer">
          <LanguageSwitcher />
          <div className="sidebar-user">
            <span className="user-avatar">{initial}</span>
            {user.username}
          </div>
          <button onClick={logout}>{t('nav.logout')}</button>
        </div>
      </aside>
      <main className="content">
        <Outlet />
      </main>
    </div>
  )
}
