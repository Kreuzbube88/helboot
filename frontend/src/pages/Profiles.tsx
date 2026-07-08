import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { Profile, Provider } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

export function Profiles() {
  const { t } = useTranslation()
  const [profiles, setProfiles] = useState<Profile[] | null>(null)
  const [providers, setProviders] = useState<Provider[]>([])
  const [error, setError] = useState<unknown>(null)
  const [notice, setNotice] = useState('')
  const [adding, setAdding] = useState(false)
  const importInput = useRef<HTMLInputElement>(null)

  function reload() {
    Promise.all([api.get<Profile[]>('/profiles'), api.get<Provider[]>('/providers')])
      .then(([p, prov]) => {
        setProfiles(p)
        setProviders(prov)
        setError(null)
      })
      .catch(setError)
  }

  useEffect(reload, [])

  async function remove(profile: Profile) {
    if (!window.confirm(t('profiles.confirmDelete'))) return
    try {
      await api.delete(`/profiles/${profile.id}`)
      reload()
    } catch (err) {
      setError(err)
    }
  }

  async function clone(profile: Profile) {
    const name = window.prompt(t('profiles.clonePrompt'), `${profile.name} (2)`)
    if (!name) return
    try {
      await api.post(`/profiles/${profile.id}/clone`, { name })
      reload()
    } catch (err) {
      setError(err)
    }
  }

  async function importProfile(file: File) {
    setNotice('')
    try {
      const doc = JSON.parse(await file.text())
      await api.post('/profiles/import', doc)
      setNotice(t('profiles.imported'))
      reload()
    } catch (err) {
      setError(err)
    } finally {
      if (importInput.current) importInput.current.value = ''
    }
  }

  return (
    <>
      <div className="toolbar">
        <h1>{t('profiles.title')}</h1>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button onClick={() => importInput.current?.click()}>{t('profiles.import')}</button>
          <input
            ref={importInput}
            type="file"
            accept=".json,application/json"
            style={{ display: 'none' }}
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) importProfile(file)
            }}
          />
          <button className="primary" onClick={() => setAdding(true)}>
            {t('profiles.add')}
          </button>
        </div>
      </div>
      <ErrorMessage error={error} />
      {notice && <p className="muted">{notice}</p>}
      {adding && (
        <ProfileForm
          providers={providers}
          onSaved={() => {
            setAdding(false)
            reload()
          }}
          onCancel={() => setAdding(false)}
        />
      )}
      {profiles && profiles.length === 0 && !adding && (
        <p className="muted">{t('profiles.empty')}</p>
      )}
      {profiles && profiles.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>{t('profiles.name')}</th>
              <th>{t('profiles.provider')}</th>
              <th>{t('profiles.version')}</th>
              <th>{t('common.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {profiles.map((p) => (
              <tr key={p.id}>
                <td>{p.name}</td>
                <td>{providers.find((x) => x.name === p.provider)?.displayName ?? p.provider}</td>
                <td>v{p.currentVersion}</td>
                <td>
                  <button onClick={() => clone(p)}>{t('profiles.clone')}</button>{' '}
                  <a href={`/api/v1/profiles/${p.id}/export`} download>
                    <button>{t('profiles.export')}</button>
                  </a>{' '}
                  <button className="danger" onClick={() => remove(p)}>
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

function ProfileForm({
  providers,
  onSaved,
  onCancel,
}: {
  providers: Provider[]
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [provider, setProvider] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await api.post('/profiles', { name, provider })
      onSaved()
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <form className="card form-narrow" onSubmit={submit} style={{ marginBottom: '1rem' }}>
      <div className="field">
        <label htmlFor="profile-name">{t('profiles.name')}</label>
        <input id="profile-name" value={name} onChange={(e) => setName(e.target.value)} required />
      </div>
      <div className="field">
        <label htmlFor="profile-provider">{t('profiles.provider')}</label>
        <select
          id="profile-provider"
          value={provider}
          onChange={(e) => setProvider(e.target.value)}
          required
        >
          <option value="" disabled>
            —
          </option>
          {providers.map((p) => (
            <option key={p.name} value={p.name}>
              {p.displayName}
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
