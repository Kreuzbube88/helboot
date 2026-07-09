import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { Host, NetworkStatus, Profile, SystemInfo } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

export function Dashboard() {
  const { t } = useTranslation()
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [hosts, setHosts] = useState<Host[] | null>(null)
  const [profiles, setProfiles] = useState<Profile[] | null>(null)
  const [netStatus, setNetStatus] = useState<NetworkStatus | null>(null)
  const [error, setError] = useState<unknown>(null)

  useEffect(() => {
    Promise.all([
      api.get<SystemInfo>('/system/info'),
      api.get<Host[]>('/hosts'),
      api.get<Profile[]>('/profiles'),
      api.get<NetworkStatus>('/network/status').catch(() => null),
    ])
      .then(([i, h, p, n]) => {
        setInfo(i)
        setHosts(h)
        setProfiles(p)
        setNetStatus(n)
      })
      .catch(setError)
  }, [])

  if (error) return <ErrorMessage error={error} />
  if (!info || !hosts || !profiles) return <p className="muted">{t('common.loading')}</p>

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
    </>
  )
}
