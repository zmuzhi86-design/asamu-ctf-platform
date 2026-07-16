import { apiRequest } from './apiClient'

export type RegistryCredential = {
  id: string
  name: string
  registryHost: string
  username: string
  enabled: boolean
  tokenConfigured: boolean
  lastUsedAt?: string
  createdAt: string
  updatedAt: string
  version: number
}

export const registryCredentialApi = {
  list: () => apiRequest<RegistryCredential[]>('/admin/registry-credentials'),
  create: (input: { name: string; registryHost: string; username: string; token: string }) => apiRequest<RegistryCredential>('/admin/registry-credentials', { method: 'POST', body: JSON.stringify(input) }),
  update: (credential: RegistryCredential, input: { name: string; username: string; token?: string; enabled: boolean; reason: string }) => apiRequest<RegistryCredential>(`/admin/registry-credentials/${credential.id}`, { method: 'PUT', body: JSON.stringify({ ...input, expectedVersion: credential.version }) }),
}
