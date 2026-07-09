import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { Host, Profile, ProfileVersion } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

export function Hosts() {
  const { t } = useTranslation()
  const [hosts, setHosts] = useState<Host[] | null>(null)
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [error, setError] = useState<unknown>(null)
  const [notice, setNotice] = useState('')
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Host | null>(null)

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

  async function queueInstall(host: Host) {
    setNotice('')
    try {
      await api.post('/installations', { hostId: host.id })
      setNotice(t('installations.queued'))
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

  function profileLabel(h: Host): string {
    if (h.profileId == null) return t('common.none')
    const name = profiles.find((p) => p.id === h.profileId)?.name ?? `#${h.profileId}`
    return h.profileVersion > 0 ? `${name} (v${h.profileVersion})` : name
  }

  const form = (host: Host | null) => (
    <HostForm
      profiles={profiles}
      host={host}
      onSaved={() => {
        setAdding(false)
        setEditing(null)
        reload()
      }}
      onCancel={() => {
        setAdding(false)
        setEditing(null)
      }}
    />
  )

  return (
    <>
      <div className="toolbar">
        <h1>{t('hosts.title')}</h1>
        <button className="primary" onClick={() => setAdding(true)}>
          {t('hosts.add')}
        </button>
      </div>
      <ErrorMessage error={error} />
      {notice && <p className="muted">{notice}</p>}
      {adding && form(null)}
      {editing && form(editing)}
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
                <td>{profileLabel(h)}</td>
                <td>
                  <span className="badge">{statusLabel[h.status]}</span>
                </td>
                <td>
                  {h.profileId != null && h.status !== 'installing' && (
                    <button onClick={() => queueInstall(h)}>{t('installations.queue')}</button>
                  )}{' '}
                  <button onClick={() => setEditing(h)}>{t('common.edit')}</button>{' '}
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

/** Create/edit form. The pinned profile version (ADR-0013) is visible
 * and changeable: assigning a profile defaults to its current version. */
function HostForm({
  profiles,
  host,
  onSaved,
  onCancel,
}: {
  profiles: Profile[]
  host: Host | null
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const [mac, setMac] = useState(host?.mac ?? '')
  const [hostname, setHostname] = useState(host?.hostname ?? '')
  const [tags, setTags] = useState(host?.tags.join(', ') ?? '')
  const [profileId, setProfileId] = useState<number | ''>(host?.profileId ?? '')
  const [profileVersion, setProfileVersion] = useState<number>(host?.profileVersion ?? 0)
  const [versions, setVersions] = useState<ProfileVersion[]>([])
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (profileId === '') {
      setVersions([])
      return
    }
    api
      .get<ProfileVersion[]>(`/profiles/${profileId}/versions`)
      .then(setVersions)
      .catch(setError)
  }, [profileId])

  const selectedProfile = profiles.find((p) => p.id === profileId)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    const payload = {
      mac,
      hostname,
      vendor: host?.vendor ?? '',
      model: host?.model ?? '',
      serial: host?.serial ?? '',
      assetId: host?.assetId ?? '',
      firmware: host?.firmware ?? '',
      arch: host?.arch ?? '',
      tags: tags
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
      profileId: profileId === '' ? null : profileId,
      profileVersion: profileId === '' ? 0 : profileVersion,
    }
    try {
      if (host) {
        await api.put(`/hosts/${host.id}`, payload)
      } else {
        await api.post('/hosts', payload)
      }
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
          onChange={(e) => {
            const id = e.target.value === '' ? '' : Number(e.target.value)
            setProfileId(id)
            const p = profiles.find((x) => x.id === id)
            setProfileVersion(p?.currentVersion ?? 0)
          }}
        >
          <option value="">{t('common.none')}</option>
          {profiles.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </div>
      {profileId !== '' && versions.length > 0 && (
        <div className="field">
          <label htmlFor="host-profile-version">{t('hosts.profileVersion')}</label>
          <select
            id="host-profile-version"
            value={profileVersion}
            onChange={(e) => setProfileVersion(Number(e.target.value))}
          >
            {versions.map((v) => (
              <option key={v.id} value={v.version}>
                v{v.version}
                {v.version === selectedProfile?.currentVersion
                  ? ` (${t('profiles.current')})`
                  : ''}
              </option>
            ))}
          </select>
          <small className="muted">{t('hosts.profileVersionHint')}</small>
        </div>
      )}
      <ErrorMessage error={error} />
      <div className="wizard-nav">
        <button type="button" onClick={onCancel} disabled={busy}>
          {t('common.cancel')}
        </button>
        <button className="primary" type="submit" disabled={busy}>
          {host ? t('common.save') : t('common.create')}
        </button>
      </div>
    </form>
  )
}
