import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { Host, Installation } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

export function Installations() {
  const { t } = useTranslation()
  const [installs, setInstalls] = useState<Installation[] | null>(null)
  const [hosts, setHosts] = useState<Host[]>([])
  const [error, setError] = useState<unknown>(null)

  function reload() {
    Promise.all([api.get<Installation[]>('/installations'), api.get<Host[]>('/hosts')])
      .then(([i, h]) => {
        setInstalls(i)
        setHosts(h)
        setError(null)
      })
      .catch(setError)
  }

  useEffect(reload, [])

  async function cancel(inst: Installation) {
    if (!window.confirm(t('installations.confirmCancel'))) return
    try {
      await api.delete(`/installations/${inst.id}`)
      reload()
    } catch (err) {
      setError(err)
    }
  }

  const statusLabel: Record<Installation['status'], string> = {
    discovered: t('installations.statusDiscovered'),
    waiting: t('installations.statusWaiting'),
    installing: t('installations.statusInstalling'),
    success: t('installations.statusSuccess'),
    error: t('installations.statusError'),
  }

  const fmt = (s: string | null) => (s ? new Date(s).toLocaleString() : '—')

  return (
    <>
      <h1>{t('installations.title')}</h1>
      <ErrorMessage error={error} />
      {installs && installs.length === 0 && <p className="muted">{t('installations.empty')}</p>}
      {installs && installs.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>{t('installations.host')}</th>
              <th>{t('installations.status')}</th>
              <th>{t('installations.created')}</th>
              <th>{t('installations.started')}</th>
              <th>{t('installations.finished')}</th>
              <th>{t('common.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {installs.map((inst) => {
              const host = hosts.find((h) => h.id === inst.hostId)
              return (
                <tr key={inst.id}>
                  <td>{host ? host.hostname || host.mac : `#${inst.hostId}`}</td>
                  <td>
                    <span className="badge">{statusLabel[inst.status]}</span>
                  </td>
                  <td>{fmt(inst.createdAt)}</td>
                  <td>{fmt(inst.startedAt)}</td>
                  <td>{fmt(inst.finishedAt)}</td>
                  <td>
                    {inst.status === 'waiting' && (
                      <button className="danger" onClick={() => cancel(inst)}>
                        {t('installations.cancel')}
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      )}
    </>
  )
}
