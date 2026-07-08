import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { NetworkConfig } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

export function Settings() {
  const { t } = useTranslation()
  const [cfg, setCfg] = useState<NetworkConfig | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const backupInput = useRef<HTMLInputElement>(null)

  useEffect(() => {
    api
      .get<NetworkConfig>('/network/config')
      .then((c) => {
        // Older instances may not have every field persisted yet.
        if (!c.mode) c.mode = 'proxy_dhcp'
        setCfg(c)
      })
      .catch(setError)
  }, [])

  async function save(e: FormEvent) {
    e.preventDefault()
    if (!cfg) return
    setBusy(true)
    setError(null)
    setNotice('')
    try {
      await api.put('/network/config', cfg)
      setNotice(t('settings.saved'))
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  async function importBackup(file: File) {
    setBusy(true)
    setError(null)
    setNotice('')
    try {
      await api.upload('/backup/import', file)
      setNotice(t('settings.backupImportStaged'))
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
      if (backupInput.current) backupInput.current.value = ''
    }
  }

  const setField = <K extends keyof NetworkConfig>(key: K, value: NetworkConfig[K]) =>
    setCfg(cfg ? { ...cfg, [key]: value } : cfg)
  const setDHCP = <K extends keyof NetworkConfig['dhcp']>(key: K, value: NetworkConfig['dhcp'][K]) =>
    setCfg(cfg ? { ...cfg, dhcp: { ...cfg.dhcp, [key]: value } } : cfg)

  if (!cfg) {
    return (
      <>
        <h1>{t('settings.title')}</h1>
        <ErrorMessage error={error} />
        <p className="muted">{t('common.loading')}</p>
      </>
    )
  }

  return (
    <>
      <h1>{t('settings.title')}</h1>
      <ErrorMessage error={error} />
      {notice && <p className="muted">{notice}</p>}

      <form className="card form-narrow" onSubmit={save} style={{ marginBottom: '1.5rem' }}>
        <h2>{t('settings.network')}</h2>
        <div className="field">
          <label htmlFor="mode">{t('settings.mode')}</label>
          <select
            id="mode"
            value={cfg.mode}
            onChange={(e) => setField('mode', e.target.value as NetworkConfig['mode'])}
          >
            <option value="proxy_dhcp">{t('setup.modeProxyDhcp')}</option>
            <option value="dhcp">{t('setup.modeDhcp')}</option>
          </select>
        </div>
        <div className="field">
          <label htmlFor="server-ip">{t('settings.serverIp')}</label>
          <input
            id="server-ip"
            value={cfg.serverIp}
            onChange={(e) => setField('serverIp', e.target.value)}
            placeholder="192.168.1.10"
          />
          <small className="muted">{t('settings.serverIpHint')}</small>
        </div>

        {cfg.mode === 'dhcp' && (
          <>
            <div className="field">
              <label htmlFor="range-start">{t('settings.rangeStart')}</label>
              <input
                id="range-start"
                value={cfg.dhcp.rangeStart}
                onChange={(e) => setDHCP('rangeStart', e.target.value)}
                placeholder="192.168.1.100"
              />
            </div>
            <div className="field">
              <label htmlFor="range-end">{t('settings.rangeEnd')}</label>
              <input
                id="range-end"
                value={cfg.dhcp.rangeEnd}
                onChange={(e) => setDHCP('rangeEnd', e.target.value)}
                placeholder="192.168.1.199"
              />
            </div>
            <div className="field">
              <label htmlFor="subnet">{t('settings.subnetMask')}</label>
              <input
                id="subnet"
                value={cfg.dhcp.subnetMask}
                onChange={(e) => setDHCP('subnetMask', e.target.value)}
                placeholder="255.255.255.0"
              />
            </div>
            <div className="field">
              <label htmlFor="gateway">{t('settings.gateway')}</label>
              <input
                id="gateway"
                value={cfg.dhcp.gateway}
                onChange={(e) => setDHCP('gateway', e.target.value)}
                placeholder="192.168.1.1"
              />
            </div>
            <div className="field">
              <label htmlFor="dns">{t('settings.dns')}</label>
              <input
                id="dns"
                value={cfg.dhcp.dns}
                onChange={(e) => setDHCP('dns', e.target.value)}
                placeholder="192.168.1.1, 9.9.9.9"
              />
              <small className="muted">{t('settings.dnsHint')}</small>
            </div>
            <div className="field">
              <label htmlFor="lease">{t('settings.leaseMinutes')}</label>
              <input
                id="lease"
                type="number"
                min={1}
                value={cfg.dhcp.leaseMinutes || 60}
                onChange={(e) => setDHCP('leaseMinutes', Number(e.target.value))}
              />
            </div>
          </>
        )}

        <button className="primary" type="submit" disabled={busy}>
          {t('common.save')}
        </button>
      </form>

      <div className="card form-narrow" style={{ marginBottom: '1.5rem' }}>
        <h2>{t('settings.bootMedia')}</h2>
        <p className="muted">{t('settings.bootMediaHint')}</p>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          <a href="/api/v1/bootmedia/iso" download>
            <button type="button">{t('settings.bootMediaIso')}</button>
          </a>
          <a href="/api/v1/bootmedia/img" download>
            <button type="button">{t('settings.bootMediaImg')}</button>
          </a>
        </div>
      </div>

      <PasswordCard />

      <div className="card form-narrow">
        <h2>{t('settings.backup')}</h2>
        <p className="muted">{t('settings.backupHint')}</p>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <a href="/api/v1/backup/export" download>
            <button type="button">{t('settings.backupExport')}</button>
          </a>
          <button type="button" onClick={() => backupInput.current?.click()} disabled={busy}>
            {t('settings.backupImport')}
          </button>
          <input
            ref={backupInput}
            type="file"
            accept=".tar.gz,.gz"
            style={{ display: 'none' }}
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) importBackup(file)
            }}
          />
        </div>
      </div>
    </>
  )
}

/** Self-service password change; a successful change revokes every
 * session, so the app reloads onto the login screen. */
function PasswordCard() {
  const { t } = useTranslation()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await api.post('/auth/password', { currentPassword: current, newPassword: next })
      window.alert(t('account.changed'))
      window.location.reload()
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <form className="card form-narrow" onSubmit={submit} style={{ marginBottom: '1.5rem' }}>
      <h2>{t('account.title')}</h2>
      <div className="field">
        <label htmlFor="pw-current">{t('account.current')}</label>
        <input
          id="pw-current"
          type="password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
          autoComplete="current-password"
          required
        />
      </div>
      <div className="field">
        <label htmlFor="pw-new">{t('account.new')}</label>
        <input
          id="pw-new"
          type="password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          autoComplete="new-password"
          required
        />
        <small className="muted">{t('users.passwordHint')}</small>
      </div>
      <ErrorMessage error={error} />
      <button className="primary" type="submit" disabled={busy}>
        {t('account.submit')}
      </button>
    </form>
  )
}
