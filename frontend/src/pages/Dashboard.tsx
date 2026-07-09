import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { Host, HostStatus, Installation, NetworkStatus, Profile, SystemInfo } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

const hostStatuses: HostStatus[] = ['discovered', 'ready', 'installing', 'error']

export function Dashboard() {
  const { t } = useTranslation()
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [hosts, setHosts] = useState<Host[] | null>(null)
  const [profiles, setProfiles] = useState<Profile[] | null>(null)
  const [installs, setInstalls] = useState<Installation[]>([])
  const [netStatus, setNetStatus] = useState<NetworkStatus | null>(null)
  const [error, setError] = useState<unknown>(null)

  useEffect(() => {
    Promise.all([
      api.get<SystemInfo>('/system/info'),
      api.get<Host[]>('/hosts'),
      api.get<Profile[]>('/profiles'),
      api.get<Installation[]>('/installations').catch(() => [] as Installation[]),
      api.get<NetworkStatus>('/network/status').catch(() => null),
    ])
      .then(([i, h, p, inst, n]) => {
        setInfo(i)
        setHosts(h)
        setProfiles(p)
        setInstalls(inst)
        setNetStatus(n)
      })
      .catch(setError)
  }, [])

  if (error) return <ErrorMessage error={error} />
  if (!info || !hosts || !profiles) return <p className="muted">{t('common.loading')}</p>

  const statusLabel: Record<HostStatus, string> = {
    discovered: t('hosts.statusDiscovered'),
    ready: t('hosts.statusReady'),
    installing: t('hosts.statusInstalling'),
    error: t('hosts.statusError'),
  }
  const maxStatusCount = Math.max(1, ...hostStatuses.map((s) => hosts.filter((h) => h.status === s).length))
  const recentInstalls = [...installs].slice(0, 5)

  return (
    <>
      <h1>{t('dashboard.title')}</h1>
      {netStatus?.warnings.map((w) => (
        <div key={w.code} className="warning-banner">
          <strong>{t(`warnings.${w.code}`, { defaultValue: w.message })}</strong>
          {netStatus.dhcpServers.length > 0 && (
            <div className="muted">
              {t('dashboard.dhcpServersSeen')}:{' '}
              {netStatus.dhcpServers.map((s) => s.ip).join(', ')}
            </div>
          )}
        </div>
      ))}

      <div className="cards">
        <div className="card">
          <div className="stat">{hosts.length}</div>
          <div className="muted">{t('dashboard.hosts')}</div>
        </div>
        <div className="card">
          <div className="stat">{profiles.length}</div>
          <div className="muted">{t('dashboard.profiles')}</div>
        </div>
        <div className="card">
          <div className="stat">{info.providers}</div>
          <div className="muted">{t('dashboard.providers')}</div>
        </div>
        <div className="card">
          <div className="stat">{info.version}</div>
          <div className="muted">{t('dashboard.version')}</div>
        </div>
        <div className="card">
          <div className="stat">
            {info.networkMode === 'dhcp' ? t('setup.modeDhcp') : t('setup.modeProxyDhcp')}
          </div>
          <div className="muted">{t('dashboard.networkMode')}</div>
        </div>
      </div>

      <div className="overview-grid">
        <div className="card">
          <h2>{t('dashboard.hostsByStatus')}</h2>
          {hosts.length === 0 ? (
            <p className="muted">{t('hosts.empty')}</p>
          ) : (
            <div className="status-breakdown">
              {hostStatuses.map((s) => {
                const count = hosts.filter((h) => h.status === s).length
                return (
                  <div key={s} className={`status-row${s === 'error' ? ' is-error' : ''}`}>
                    <span className="status-label">{statusLabel[s]}</span>
                    <span className="status-track">
                      <span
                        className="status-fill"
                        style={{ width: `${(count / maxStatusCount) * 100}%` }}
                      />
                    </span>
                    <span className="status-count">{count}</span>
                  </div>
                )
              })}
            </div>
          )}
        </div>

        <div className="card">
          <h2>{t('dashboard.recentInstallations')}</h2>
          {recentInstalls.length === 0 ? (
            <p className="muted">{t('installations.empty')}</p>
          ) : (
            <div className="recent-list">
              {recentInstalls.map((inst) => {
                const host = hosts.find((h) => h.id === inst.hostId)
                return (
                  <div key={inst.id} className="recent-item">
                    <span className="recent-host">
                      {host ? host.hostname || host.mac : `#${inst.hostId}`}
                    </span>
                    <span className="badge">{t(`installations.status${capitalize(inst.status)}`)}</span>
                    <span className="recent-time">
                      {new Date(inst.createdAt).toLocaleString()}
                    </span>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>

      <h2>{t('dashboard.quickLinks')}</h2>
      <div className="quick-links">
        <Link to="/hosts" className="quick-link-card">
          <div className="quick-link-title">{t('nav.hosts')}</div>
          <p>{t('dashboard.quickLinkHosts')}</p>
        </Link>
        <Link to="/profiles" className="quick-link-card">
          <div className="quick-link-title">{t('nav.profiles')}</div>
          <p>{t('dashboard.quickLinkProfiles')}</p>
        </Link>
        <Link to="/isos" className="quick-link-card">
          <div className="quick-link-title">{t('nav.isos')}</div>
          <p>{t('dashboard.quickLinkIsos')}</p>
        </Link>
        <Link to="/installations" className="quick-link-card">
          <div className="quick-link-title">{t('nav.installations')}</div>
          <p>{t('dashboard.quickLinkInstallations')}</p>
        </Link>
      </div>
    </>
  )
}

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1)
}
