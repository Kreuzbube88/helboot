import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api, ApiError } from '../api/client'
import type { AnswerPreview, ISOImage, Profile, ProfileVersion, Provider } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'
import { SchemaForm, initialValues, toConfig, type ConfigValues } from '../components/SchemaForm'

export function Profiles() {
  const { t } = useTranslation()
  const [profiles, setProfiles] = useState<Profile[] | null>(null)
  const [providers, setProviders] = useState<Provider[]>([])
  const [isos, setIsos] = useState<ISOImage[]>([])
  const [error, setError] = useState<unknown>(null)
  const [notice, setNotice] = useState('')
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Profile | null>(null)
  const importInput = useRef<HTMLInputElement>(null)

  function reload() {
    Promise.all([
      api.get<Profile[]>('/profiles'),
      api.get<Provider[]>('/providers'),
      api.get<ISOImage[]>('/isos').catch(() => [] as ISOImage[]),
    ])
      .then(([p, prov, i]) => {
        setProfiles(p)
        setProviders(prov)
        setIsos(i)
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

  if (editing) {
    return (
      <ProfileEditor
        profile={editing}
        provider={providers.find((p) => p.name === editing.provider)}
        isos={isos}
        onClose={() => {
          setEditing(null)
          reload()
        }}
      />
    )
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
        <ProfileCreateForm
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
                  <button onClick={() => setEditing(p)}>{t('common.edit')}</button>{' '}
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

/** Creation form: name + provider + the schema-generated settings. */
function ProfileCreateForm({
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
  const [providerName, setProviderName] = useState('')
  const [values, setValues] = useState<ConfigValues>({})
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  const provider = providers.find((p) => p.name === providerName)
  const schema = provider?.settingsSchema ?? []

  function selectProvider(pname: string) {
    setProviderName(pname)
    const next = providers.find((p) => p.name === pname)
    setValues(initialValues(next?.settingsSchema ?? []))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await api.post('/profiles', { name, provider: providerName, config: toConfig(values) })
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
          value={providerName}
          onChange={(e) => selectProvider(e.target.value)}
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
      {schema.length > 0 && (
        <SchemaForm
          schema={schema}
          values={values}
          onChange={(key, value) => setValues((v) => ({ ...v, [key]: value }))}
        />
      )}
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

/** Full-page editor: schema-generated settings, explicit versioning
 * (ADR-0013), version history and the answer-file preview/override
 * (ADR-0014). */
function ProfileEditor({
  profile,
  provider,
  isos,
  onClose,
}: {
  profile: Profile
  provider: Provider | undefined
  isos: ISOImage[]
  onClose: () => void
}) {
  const { t } = useTranslation()
  const schema = provider?.settingsSchema ?? []
  const [name, setName] = useState(profile.name)
  const [isoId, setIsoId] = useState<number | ''>(profile.isoId ?? '')
  const [values, setValues] = useState<ConfigValues>({})
  const [versions, setVersions] = useState<ProfileVersion[]>([])
  const [asNewVersion, setAsNewVersion] = useState(false)
  const [versionInUse, setVersionInUse] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)

  const loadVersions = useCallback(async () => {
    const list = await api.get<ProfileVersion[]>(`/profiles/${profile.id}/versions`)
    setVersions(list)
    const current = list.find((v) => v.version === profile.currentVersion) ?? list[list.length - 1]
    setValues(initialValues(schema, current?.config))
    return list
    // schema is derived from the (stable) provider prop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profile.id, profile.currentVersion])

  useEffect(() => {
    loadVersions().catch(setError)
  }, [loadVersions])

  async function save(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    setNotice('')
    try {
      await api.put(`/profiles/${profile.id}`, {
        name,
        isoId: isoId === '' ? null : isoId,
        config: toConfig(values),
        saveAsNewVersion: asNewVersion,
      })
      setNotice(t('profiles.saved'))
      onClose()
    } catch (err) {
      if (err instanceof ApiError && err.code === 'profile.version_in_use') {
        setVersionInUse(true)
        setAsNewVersion(true)
      }
      setError(err)
      setBusy(false)
    }
  }

  const providerISOs = isos.filter((i) => i.provider === profile.provider && i.status === 'ready')

  return (
    <>
      <div className="toolbar">
        <h1>
          {t('profiles.editTitle', { name: profile.name })}{' '}
          <span className="badge">v{profile.currentVersion}</span>
        </h1>
        <button onClick={onClose}>{t('common.back')}</button>
      </div>
      <ErrorMessage error={error} />
      {versionInUse && <p className="muted">{t('profiles.versionInUseHint')}</p>}
      {notice && <p className="muted">{notice}</p>}

      <form className="card form-narrow" onSubmit={save} style={{ marginBottom: '1.5rem' }}>
        <div className="field">
          <label htmlFor="edit-name">{t('profiles.name')}</label>
          <input id="edit-name" value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div className="field">
          <label htmlFor="edit-iso">{t('profiles.iso')}</label>
          <select
            id="edit-iso"
            value={isoId}
            onChange={(e) => setIsoId(e.target.value === '' ? '' : Number(e.target.value))}
          >
            <option value="">{t('common.none')}</option>
            {providerISOs.map((i) => (
              <option key={i.id} value={i.id}>
                {i.filename}
              </option>
            ))}
          </select>
          <small className="muted">{t('profiles.isoHint')}</small>
        </div>
        {schema.length > 0 ? (
          <SchemaForm
            schema={schema}
            values={values}
            onChange={(key, value) => setValues((v) => ({ ...v, [key]: value }))}
          />
        ) : (
          <p className="muted">{t('profiles.noSettings')}</p>
        )}
        <div className="field checkbox-field">
          <label htmlFor="edit-newversion">
            <input
              id="edit-newversion"
              type="checkbox"
              checked={asNewVersion}
              onChange={(e) => setAsNewVersion(e.target.checked)}
            />{' '}
            {t('profiles.saveAsNewVersion')}
          </label>
          <small className="muted">{t('profiles.saveAsNewVersionHint')}</small>
        </div>
        <button className="primary" type="submit" disabled={busy}>
          {t('common.save')}
        </button>
      </form>

      <AnswerFileCard profile={profile} onOverrideChanged={() => loadVersions().catch(setError)} />

      {versions.length > 0 && (
        <div className="card" style={{ marginBottom: '1.5rem' }}>
          <h2>{t('profiles.versions')}</h2>
          <table>
            <thead>
              <tr>
                <th>{t('profiles.version')}</th>
                <th>{t('profiles.versionCreated')}</th>
                <th>{t('profiles.answerFile')}</th>
              </tr>
            </thead>
            <tbody>
              {versions.map((v) => (
                <tr key={v.id}>
                  <td>
                    v{v.version}
                    {v.version === profile.currentVersion && (
                      <span className="badge" style={{ marginLeft: '0.5rem' }}>
                        {t('profiles.current')}
                      </span>
                    )}
                  </td>
                  <td>{new Date(v.createdAt).toLocaleString()}</td>
                  <td>
                    {v.answerOverride
                      ? t('profiles.answerOverridden')
                      : t('profiles.answerGenerated')}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}

/** Preview of the generated answer file for the current version, with
 * manual override editing (ADR-0014). */
function AnswerFileCard({
  profile,
  onOverrideChanged,
}: {
  profile: Profile
  onOverrideChanged: () => void
}) {
  const { t } = useTranslation()
  const [preview, setPreview] = useState<AnswerPreview | null>(null)
  const [content, setContent] = useState('')
  const [editing, setEditing] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const [unavailable, setUnavailable] = useState(false)

  const version = profile.currentVersion

  const load = useCallback(async () => {
    try {
      const p = await api.get<AnswerPreview>(`/profiles/${profile.id}/versions/${version}/answer`)
      setPreview(p)
      setContent(p.content)
      setError(null)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'profile.no_answer_file') {
        setUnavailable(true)
        return
      }
      setError(err)
    }
  }, [profile.id, version])

  useEffect(() => {
    load()
  }, [load])

  async function setOverride(body: string) {
    setBusy(true)
    setError(null)
    try {
      await api.put(`/profiles/${profile.id}/versions/${version}/answer-override`, {
        content: body,
      })
      setEditing(false)
      await load()
      onOverrideChanged()
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  if (unavailable) return null

  return (
    <div className="card" style={{ marginBottom: '1.5rem' }}>
      <h2>
        {t('profiles.answerFile')}
        {preview && <span className="badge" style={{ marginLeft: '0.5rem' }}>{preview.format}</span>}
        {preview?.overridden && (
          <span className="badge" style={{ marginLeft: '0.5rem' }}>
            {t('profiles.answerOverridden')}
          </span>
        )}
      </h2>
      <p className="muted">{t('profiles.answerHint')}</p>
      <ErrorMessage error={error} />
      {preview && (
        <>
          <textarea
            rows={14}
            value={content}
            readOnly={!editing}
            onChange={(e) => setContent(e.target.value)}
            style={{ fontFamily: 'monospace', width: '100%' }}
          />
          <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.5rem' }}>
            {!editing && (
              <button type="button" onClick={() => setEditing(true)}>
                {t('profiles.answerEdit')}
              </button>
            )}
            {editing && (
              <>
                <button
                  type="button"
                  className="primary"
                  disabled={busy}
                  onClick={() => setOverride(content)}
                >
                  {t('profiles.answerSaveOverride')}
                </button>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    setEditing(false)
                    setContent(preview.content)
                  }}
                >
                  {t('common.cancel')}
                </button>
              </>
            )}
            {preview.overridden && (
              <button type="button" className="danger" disabled={busy} onClick={() => setOverride('')}>
                {t('profiles.answerClearOverride')}
              </button>
            )}
            <button type="button" disabled={busy} onClick={load}>
              {t('profiles.answerRefresh')}
            </button>
          </div>
        </>
      )}
    </div>
  )
}
