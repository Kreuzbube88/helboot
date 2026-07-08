import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { Role, User } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

export function Users({ currentUser }: { currentUser: User }) {
  const { t } = useTranslation()
  const [users, setUsers] = useState<User[] | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [adding, setAdding] = useState(false)

  function reload() {
    api
      .get<User[]>('/users')
      .then((u) => {
        setUsers(u)
        setError(null)
      })
      .catch(setError)
  }

  useEffect(reload, [])

  async function remove(user: User) {
    if (!window.confirm(t('users.confirmDelete'))) return
    try {
      await api.delete(`/users/${user.id}`)
      reload()
    } catch (err) {
      setError(err)
    }
  }

  async function changeRole(user: User, role: Role) {
    try {
      await api.put(`/users/${user.id}`, { role })
      reload()
    } catch (err) {
      setError(err)
      reload()
    }
  }

  async function resetPassword(user: User) {
    const password = window.prompt(t('users.resetPasswordPrompt', { name: user.username }))
    if (!password) return
    try {
      await api.put(`/users/${user.id}/password`, { password })
    } catch (err) {
      setError(err)
    }
  }

  const roleLabel: Record<Role, string> = {
    admin: t('users.roleAdmin'),
    operator: t('users.roleOperator'),
    viewer: t('users.roleViewer'),
  }

  return (
    <>
      <div className="toolbar">
        <h1>{t('users.title')}</h1>
        <button className="primary" onClick={() => setAdding(true)}>
          {t('users.add')}
        </button>
      </div>
      <ErrorMessage error={error} />
      {adding && (
        <UserForm
          onSaved={() => {
            setAdding(false)
            reload()
          }}
          onCancel={() => setAdding(false)}
        />
      )}
      {users && (
        <table>
          <thead>
            <tr>
              <th>{t('users.username')}</th>
              <th>{t('users.role')}</th>
              <th>{t('users.created')}</th>
              <th>{t('common.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td>{u.username}</td>
                <td>
                  <select
                    value={u.role}
                    onChange={(e) => changeRole(u, e.target.value as Role)}
                    style={{ width: 'auto' }}
                    aria-label={t('users.role')}
                  >
                    {(Object.keys(roleLabel) as Role[]).map((r) => (
                      <option key={r} value={r}>
                        {roleLabel[r]}
                      </option>
                    ))}
                  </select>
                </td>
                <td className="muted">{new Date(u.createdAt).toLocaleDateString()}</td>
                <td>
                  <button onClick={() => resetPassword(u)}>{t('users.resetPassword')}</button>{' '}
                  {u.id !== currentUser.id && (
                    <button className="danger" onClick={() => remove(u)}>
                      {t('common.delete')}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}

function UserForm({ onSaved, onCancel }: { onSaved: () => void; onCancel: () => void }) {
  const { t } = useTranslation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('operator')
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await api.post('/users', { username, password, role, locale: 'en' })
      onSaved()
    } catch (err) {
      setError(err)
      setBusy(false)
    }
  }

  return (
    <form className="card form-narrow" onSubmit={submit} style={{ marginBottom: '1rem' }}>
      <div className="field">
        <label htmlFor="new-username">{t('users.username')}</label>
        <input
          id="new-username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="off"
          required
        />
      </div>
      <div className="field">
        <label htmlFor="new-password">{t('users.password')}</label>
        <input
          id="new-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
          required
        />
        <small className="muted">{t('users.passwordHint')}</small>
      </div>
      <div className="field">
        <label htmlFor="new-role">{t('users.role')}</label>
        <select id="new-role" value={role} onChange={(e) => setRole(e.target.value as Role)}>
          <option value="admin">{t('users.roleAdmin')}</option>
          <option value="operator">{t('users.roleOperator')}</option>
          <option value="viewer">{t('users.roleViewer')}</option>
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
