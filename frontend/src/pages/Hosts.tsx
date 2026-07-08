import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { Host, Profile } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

export function Hosts() {
  const { t } = useTranslation()
  const [hosts, setHosts] = useState<Host[] | null>(null)
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [error, setError] = useState<unknown>(null)
  const [adding, setAdding] = useState(false)

  function reload() {
    Promise.all([api.get<Host[]>('/hosts'), api.get<Profile[]>('/profiles')])
      .then(([h, p]) => {
        setHosts(h)
        setProfiles(p)
        setError(null)
      })
      .catch(setError)
  }

  useEffect(reload, [])

  async function remove(host: Host) {
    if (!window.confirm(t('hosts.confirmDelete'))) return
    try {
      await api.delete(`/hosts/${host.id}`)
      reload()
    } catch (err) {
      setError(err)
    }
  }

  const statusLabel: Record<Host['status'], string> = {
    discovered: t('hosts.statusDiscovered'),
    ready: t('hosts.statusReady'),
    installing: t('hosts.statusInstalling'),
    error: t('hosts.statusError'),
  }

  return (
    <>
      <div className="toolbar">
        <h1>{t('hosts.title')}</h1>
        <button className="primary" onClick={() => setAdding(true)}>
          {t('hosts.add')}
        </button>
      </div>
      <ErrorMessage error={error} />
      {adding && (
        <HostForm
          profiles={profiles}
          onSaved={() => {
            setAdding(false)
            reload()
          }}
          onCancel={() => setAdding(false)}
        />
      )}
      {hosts && hosts.length === 0 && !adding && <p className="muted">{t('hosts.empty')}</p>}
      {hosts && hosts.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>{t('hosts.mac')}</th>
              <th>{t('hosts.hostname')}</th>
              <th>{t('hosts.vendor')}</th>
              <th>{t('hosts.profile')}</th>
              <th>{t('hosts.status')}</th>
              <th>{t('common.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {hosts.map((h) => (
              <tr key={h.id}>
                <td>
                  <code>{h.mac}</code>
                </td>
                <td>{h.hostname}</td>
                <td>{h.vendor}</td>
                <td>{profiles.find((p) => p.id === h.profileId)?.name ?? t('common.none')}</td>
                <td>
                  <span className="badge">{statusLabel[h.status]}</span>
                </td>
                <td>
                  <button className="danger" onClick={() => remove(h)}>
                    {t('common.delete')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}

function HostForm({
  profiles,
  onSaved,
  onCancel,
}: {
  profiles: Profile[]
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const [mac, setMac] = useState('')
  const [hostname, setHostname] = useState('')
  const [tags, setTags] = useState('')
  const [profileId, setProfileId] = useState<number | ''>('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await api.post('/hosts', {
        mac,
        hostname,
        tags: tags
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
        profileId: profileId === '' ? null : profileId,
      })
      onSaved()
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <form className="card form-narrow" onSubmit={submit} style={{ marginBottom: '1rem' }}>
      <div className="field">
        <label htmlFor="host-mac">{t('hosts.mac')}</label>
        <input
          id="host-mac"
          value={mac}
          onChange={(e) => setMac(e.target.value)}
          placeholder="aa:bb:cc:dd:ee:ff"
          required
        />
      </div>
      <div className="field">
        <label htmlFor="host-name">{t('hosts.hostname')}</label>
        <input id="host-name" value={hostname} onChange={(e) => setHostname(e.target.value)} />
      </div>
      <div className="field">
        <label htmlFor="host-tags">{t('hosts.tags')}</label>
        <input
          id="host-tags"
          value={tags}
          onChange={(e) => setTags(e.target.value)}
          placeholder={t('hosts.tagsHint')}
        />
      </div>
      <div className="field">
        <label htmlFor="host-profile">{t('hosts.profile')}</label>
        <select
          id="host-profile"
          value={profileId}
          onChange={(e) => setProfileId(e.target.value === '' ? '' : Number(e.target.value))}
        >
          <option value="">{t('common.none')}</option>
          {profiles.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </div>
      <ErrorMessage error={error} />
      <div className="wizard-nav">
        <button type="button" onClick={onCancel} disabled={busy}>
          {t('common.cancel')}
        </button>
        <button className="primary" type="submit" disabled={busy}>
          {t('common.create')}
        </button>
      </div>
    </form>
  )
}
