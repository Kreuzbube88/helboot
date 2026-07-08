// Types mirroring the OpenAPI schema (backend/api/openapi.yaml).

export type Role = 'admin' | 'operator' | 'viewer'

export interface User {
  id: number
  username: string
  role: Role
  locale: string
  createdAt: string
}

export interface SessionInfo {
  user: User
  csrfToken: string
}

export interface Provider {
  name: string
  displayName: string
  family: string
  capabilities: Record<string, boolean>
  answerFile: { format: string; template?: string }
  notes?: string
}

export type HostStatus = 'discovered' | 'ready' | 'installing' | 'error'

export interface Host {
  id: number
  mac: string
  hostname: string
  vendor: string
  model: string
  serial: string
  assetId: string
  tags: string[]
  firmware: '' | 'bios' | 'uefi'
  arch: string
  profileId: number | null
  status: HostStatus
  createdAt: string
  updatedAt: string
}

export interface Profile {
  id: number
  name: string
  provider: string
  isoId: number | null
  currentVersion: number
  createdAt: string
  updatedAt: string
}

export interface SystemInfo {
  version: string
  setupCompleted: boolean
  networkMode: '' | 'proxy_dhcp' | 'dhcp'
  providers: number
}
