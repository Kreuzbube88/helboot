import { useTranslation } from 'react-i18next'
import type { SettingsField } from '../api/types'

/** Values of a profile config document, keyed by schema field key. */
export type ConfigValues = Record<string, unknown>

/** Initial form values: stored config merged over the schema defaults. */
export function initialValues(schema: SettingsField[], configJSON?: string): ConfigValues {
  const values: ConfigValues = {}
  for (const f of schema) {
    if (f.default !== undefined && f.default !== null) values[f.key] = f.default
  }
  if (configJSON) {
    try {
      Object.assign(values, JSON.parse(configJSON) as ConfigValues)
    } catch {
      // Malformed stored config: fall back to the defaults.
    }
  }
  return values
}

/** Serializable config: only fields the schema declares (plus unknown
 * keys already present in values, which are preserved untouched). */
export function toConfig(values: ConfigValues): ConfigValues {
  const out: ConfigValues = {}
  for (const [key, value] of Object.entries(values)) {
    if (value === undefined || value === '') continue
    if (Array.isArray(value) && value.length === 0) continue
    out[key] = value
  }
  return out
}

function dependencySatisfied(f: SettingsField, values: ConfigValues): boolean {
  if (!f.dependsOn) return true
  return values[f.dependsOn.field] === f.dependsOn.value
}

/**
 * Renders a profile configuration form generically from a provider's
 * settings schema (ADR-0012) — no hardcoded per-OS forms. Labels
 * resolve through i18n (profileFields.<key>) with the manifest label
 * as fallback, so new providers work without core changes.
 */
export function SchemaForm({
  schema,
  values,
  onChange,
}: {
  schema: SettingsField[]
  values: ConfigValues
  onChange: (key: string, value: unknown) => void
}) {
  const { t } = useTranslation()

  const groups: { name: string; fields: SettingsField[] }[] = []
  for (const f of schema) {
    const name = f.group ?? ''
    const g = groups.find((x) => x.name === name)
    if (g) g.fields.push(f)
    else groups.push({ name, fields: [f] })
  }

  return (
    <>
      {groups.map((g) => {
        const visible = g.fields.filter((f) => dependencySatisfied(f, values))
        if (visible.length === 0) return null
        return (
          <fieldset key={g.name || 'general'} className="schema-group">
            {g.name && <legend>{t(`profileGroups.${g.name}`, { defaultValue: g.name })}</legend>}
            {visible.map((f) => (
              <SchemaField key={f.key} field={f} value={values[f.key]} onChange={onChange} />
            ))}
          </fieldset>
        )
      })}
    </>
  )
}

function SchemaField({
  field,
  value,
  onChange,
}: {
  field: SettingsField
  value: unknown
  onChange: (key: string, value: unknown) => void
}) {
  const { t } = useTranslation()
  const id = `cfg-${field.key}`
  const label = t(`profileFields.${field.key}`, { defaultValue: field.label })

  if (field.type === 'bool') {
    return (
      <div className="field checkbox-field">
        <label htmlFor={id}>
          <input
            id={id}
            type="checkbox"
            checked={Boolean(value)}
            onChange={(e) => onChange(field.key, e.target.checked)}
          />{' '}
          {label}
        </label>
        {field.help && <small className="muted">{field.help}</small>}
      </div>
    )
  }

  let input
  switch (field.type) {
    case 'text':
      input = (
        <textarea
          id={id}
          rows={4}
          value={String(value ?? '')}
          placeholder={field.placeholder}
          onChange={(e) => onChange(field.key, e.target.value)}
        />
      )
      break
    case 'list':
      input = (
        <textarea
          id={id}
          rows={3}
          value={Array.isArray(value) ? value.join('\n') : String(value ?? '')}
          placeholder={field.placeholder}
          onChange={(e) =>
            onChange(
              field.key,
              e.target.value
                .split('\n')
                .map((s) => s.trim())
                .filter(Boolean),
            )
          }
        />
      )
      break
    case 'int':
      input = (
        <input
          id={id}
          type="number"
          min={field.min}
          max={field.max}
          value={value === undefined || value === '' ? '' : Number(value)}
          onChange={(e) => onChange(field.key, e.target.value === '' ? '' : Number(e.target.value))}
          required={field.required}
        />
      )
      break
    case 'select':
      input = (
        <select
          id={id}
          value={String(value ?? '')}
          onChange={(e) => onChange(field.key, e.target.value)}
          required={field.required}
        >
          {!field.required && <option value="">—</option>}
          {(field.options ?? []).map((opt) => (
            <option key={opt} value={opt}>
              {opt}
            </option>
          ))}
        </select>
      )
      break
    default: // string, password
      input = (
        <input
          id={id}
          type={field.type === 'password' ? 'password' : 'text'}
          value={String(value ?? '')}
          placeholder={field.placeholder}
          onChange={(e) => onChange(field.key, e.target.value)}
          required={field.required}
          autoComplete="off"
        />
      )
  }

  return (
    <div className="field">
      <label htmlFor={id}>
        {label}
        {field.required && ' *'}
      </label>
      {input}
      {field.help && <small className="muted">{field.help}</small>}
      {field.type === 'list' && <small className="muted">{t('profiles.listHint')}</small>}
    </div>
  )
}
