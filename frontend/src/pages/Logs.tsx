import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { LogEntry } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

const levels = ['debug', 'info', 'warn', 'error'] as const

export function Logs() {
  const { t } = useTranslation()
  const [level, setLevel] = useState<(typeof levels)[number]>('info')
  const [entries, setEntries] = useState<LogEntry[] | null>(null)
  const [error, setError] = useState<unknown>(null)

  const reload = useCallback(() => {
    api
      .get<LogEntry[]>(`/logs?level=${level}&limit=200`)
      .then((e) => {
        setEntries(e)
        setError(null)
      })
      .catch(setError)
  }, [level])

  useEffect(reload, [reload])

  return (
    <>
      <div className="toolbar">
        <h1>{t('logs.title')}</h1>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
          <label htmlFor="log-level" style={{ margin: 0 }}>
            {t('logs.level')}
          </label>
          <select
            id="log-level"
            value={level}
            onChange={(e) => setLevel(e.target.value as (typeof levels)[number])}
            style={{ width: 'auto' }}
          >
            {levels.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </select>
          <button onClick={reload}>{t('logs.refresh')}</button>
        </div>
      </div>
      <ErrorMessage error={error} />
      {entries && entries.length === 0 && <p className="muted">{t('logs.empty')}</p>}
      {entries && entries.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>{t('logs.time')}</th>
              <th>{t('logs.level')}</th>
              <th>{t('logs.message')}</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e, i) => (
              <tr key={i}>
                <td className="muted">{new Date(e.time).toLocaleTimeString()}</td>
                <td>
                  <span className="badge">{e.level}</span>
                </td>
                <td>
                  {e.message}
                  {e.attrs && Object.keys(e.attrs).length > 0 && (
                    <span className="muted">
                      {' '}
                      {Object.entries(e.attrs)
                        .map(([k, v]) => `${k}=${v}`)
                        .join(' ')}
                    </span>
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
