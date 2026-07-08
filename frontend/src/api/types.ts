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

export interface ISOImage {
  id: number
  filename: string
  provider: string
  osName: string
  version: string
  arch: string
  bootloader: '' | 'bios' | 'uefi' | 'hybrid'
  installMethod: string
  sizeBytes: number
  sha256: string
  status: 'uploaded' | 'analyzing' | 'ready' | 'unsupported'
  createdAt: string
}

export type InstallationStatus = 'discovered' | 'waiting' | 'installing' | 'success' | 'error'

export interface Installation {
  id: number
  hostId: number
  profileVersionId: number
  status: InstallationStatus
  startedAt: string | null
  finishedAt: string | null
  log: string
  createdAt: string
}

export interface LogEntry {
  time: string
  level: string
  message: string
  attrs?: Record<string, string>
}

export interface NetworkConfig {
  mode: 'proxy_dhcp' | 'dhcp'
  serverIp: string
  dhcp: {
    rangeStart: string
    rangeEnd: string
    subnetMask: string
    gateway: string
    dns: string
    leaseMinutes: number
  }
}
