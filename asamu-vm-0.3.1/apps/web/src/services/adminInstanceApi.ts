import { apiRequest, createIdempotencyKey } from './apiClient'

export type AdminInstance = {
  id: string
  challengeId: string
  challengeSlug: string
  challengeTitle: string
  ownerScope: 'user' | 'team' | 'instance'
  ownerId: string
  ownerName: string
  competitionId?: string
  competitionName?: string
  status: string
  accessUrl?: string
  hostPort?: number
  internalPort?: number
  runtimeProvider: string
  startedAt?: string
  expiresAt?: string
  stoppedAt?: string
  errorCode?: string
  errorMessage?: string
  version: number
  statusVersion: number
  generation: number
  operationId?: string
  createdAt: string
  updatedAt: string
}

export type AdminInstanceOperation = {
  id: string
  actorId: string
  operation: string
  fromStatus: string
  toStatus: string
  result: string
  errorCode?: string
  requestId?: string
  requestedAt: string
  finishedAt?: string
}

export type AdminRuntimeEvent = {
  id: string
  instanceId: string
  type: string
  providerStatus?: string
  payload: Record<string, unknown>
  createdAt: string
}

export type AdminWorker = {
  workerId: string
  hostname: string
  status: 'online' | 'offline' | 'draining' | 'disabled'
  enabled: boolean
  draining: boolean
  cpuTotalMilli: number
  memoryTotalMb: number
  maxInstances: number
  activeInstances: number
  reservedCpuMilli: number
  reservedMemoryMb: number
  cpuPercent: number
  memoryPercent: number
  supportedProtocols: string[]
  cachedImages: string[]
  lastErrorCode?: string
  lastHeartbeat: string
  version: number
}

type Page<T> = { items: T[]; page: number; pageSize: number; total: number; totalPages: number }
type Detail = { instance: AdminInstance; operations: AdminInstanceOperation[] }

export const adminInstanceApi = {
  list: () => apiRequest<Page<AdminInstance>>('/admin/instances?pageSize=100'),
  detail: (id: string) => apiRequest<Detail>(`/admin/instances/${id}`),
  logs: (id: string) => apiRequest<AdminRuntimeEvent[]>(`/admin/instances/${id}/logs`),
  workers: () => apiRequest<AdminWorker[]>('/admin/runtime/workers'),
  setWorkerDrain: (worker: AdminWorker, draining: boolean, reason: string) => apiRequest<AdminWorker>(`/admin/runtime/workers/${encodeURIComponent(worker.workerId)}/drain`, {
    method: 'PATCH',
    body: JSON.stringify({ draining, reason, expectedVersion: worker.version }),
  }),
  transition: (instance: AdminInstance, operation: 'stop' | 'reset', reason: string) => apiRequest(`/admin/instances/${instance.id}/${operation}`, {
    method: 'POST',
    headers: { 'Idempotency-Key': createIdempotencyKey(`admin-instance-${operation}`) },
    body: JSON.stringify({ reason, expectedVersion: instance.version }),
  }),
}
