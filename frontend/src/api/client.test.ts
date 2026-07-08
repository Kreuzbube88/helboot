import { describe, expect, it } from 'vitest'
import { ApiError } from './client'

describe('ApiError', () => {
  it('maps the backend error code to an i18n key', () => {
    const err = new ApiError(401, 'auth.invalid_credentials', 'invalid username or password')
    expect(err.i18nKey).toBe('errors.auth.invalid_credentials')
    expect(err.status).toBe(401)
    expect(err.message).toBe('invalid username or password')
  })
})
