import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { ISOImage } from '../api/types'
import { ErrorMessage } from '../components/ErrorMessage'

function formatSize(bytes: number): string {
  if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(1)} GiB`
  if (bytes >= 1 << 20) return `${(bytes / (1 << 20)).toFixed(1)} MiB`
  return `${(bytes / 1024).toFixed(0)} KiB`
}

export function Isos() {
  const { t } = useTranslation()
  const [images, setImages] = useState<ISOImage[] | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const fileInput = useRef<HTMLInputElement>(null)

  function reload() {
    api
      .get<ISOImage[]>('/isos')
      .then((list) => {
        setImages(list)
        setError(null)
      })
      .catch(setError)
  }

  useEffect(reload, [])

  async function uploadFile(file: File) {
    setBusy(true)
    setError(null)
    setNotice(t('isos.uploading'))
    try {
      await api.upload<ISOImage>('/isos/upload', file)
      setNotice('')
      reload()
    } catch (err) {
      setNotice('')
      setError(err)
    } finally {
      setBusy(false)
      if (fileInput.current) fileInput.current.value = ''
    }
  }

  async function scan() {
    setBusy(true)
    setError(null)
    try {
      const added = await api.post<ISOImage[]>('/isos/scan')
      setNotice(t('isos.scanned', { count: added.length }))
      reload()
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  async function remove(img: ISOImage) {
    if (!window.confirm(t('isos.confirmDelete'))) return
    try {
      await api.delete(`/isos/${img.id}`)
      reload()
    } catch (err) {
      setError(err)
    }
  }

  const statusLabel: Record<ISOImage['status'], string> = {
    ready: t('isos.statusReady'),
    unsupported: t('isos.statusUnsupported'),
    uploaded: t('isos.statusUploaded'),
    analyzing: t('isos.statusAnalyzing'),
  }

  return (
    <>
      <div className="toolbar">
        <h1>{t('isos.title')}</h1>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button onClick={scan} disabled={busy}>
            {t('isos.scan')}
          </button>
          <button className="primary" onClick={() => fileInput.current?.click()} disabled={busy}>
            {t('isos.upload')}
          </button>
          <input
            ref={fileInput}
            type="file"
            accept=".iso,.img"
            style={{ display: 'none' }}
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) uploadFile(file)
            }}
          />
        </div>
      </div>
      <ErrorMessage error={error} />
      {notice && <p className="muted">{notice}</p>}
      {images && images.length === 0 && <p className="muted">{t('isos.empty')}</p>}
      {images && images.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>{t('isos.filename')}</th>
              <th>{t('isos.os')}</th>
              <th>{t('isos.bootloader')}</th>
              <th>{t('isos.size')}</th>
              <th>{t('isos.status')}</th>
              <th>{t('common.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {images.map((img) => (
              <tr key={img.id}>
                <td>
                  <code>{img.filename}</code>
                </td>
                <td>{img.osName}</td>
                <td>{img.bootloader}</td>
                <td>{formatSize(img.sizeBytes)}</td>
                <td>
                  <span className="badge">{statusLabel[img.status]}</span>
                </td>
                <td>
                  <button className="danger" onClick={() => remove(img)} disabled={busy}>
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
