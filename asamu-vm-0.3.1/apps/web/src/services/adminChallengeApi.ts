import { apiRequest } from './apiClient'
import type { ChallengeDto, Page } from './platformApi'

export type AdminChallenge = ChallengeDto & {
  maximumScore: number; dynamicDecay: number; scoreMode: string; visibility: string
  hints: Array<{ id: string; title: string; content: string; cost: number }>
  files: Array<{ id: string; name: string; mimeType: string; size: number; downloadUrl: string }>
  runtime?: { registryCredentialId?: string; imageRef?: string; imageDigest?: string; internalPort: number; protocol: string; flagFormat: 'standard' | 'uuid'; cpuMilli: number; memoryMB: number; pidsLimit?: number; diskMB?: number; ttlSeconds: number; maxTTLSeconds: number; readOnlyRootFS?: boolean; environment?: Record<string, string> }
}

export type ChallengeMutation = {
  slug: string; title: string; categoryKey: string; difficulty: string; summary: string; description: string; author: string
  scoreMode: string; visibility: string; baseScore: number; minimumScore: number; maximumScore: number; dynamicDecay: number; isDynamic: boolean
  tags: string[]; knowledgePoints: string[]; hintConfigs: Array<{ title: string; content: string; cost: number }>
  flags?: Array<{ kind: string; value: string; stage: number }>
  runtime?: { registryCredentialId?: string; imageRef: string; imageDigest: string; internalPort: number; protocol: string; flagFormat: 'standard' | 'uuid'; cpuMilli: number; memoryMB: number; pidsLimit: number; diskMB: number; ttlSeconds: number; maxTTLSeconds: number; readOnlyRootFS: boolean; environment: Record<string, string> }
}

export const listAdminChallenges = () => apiRequest<Page<ChallengeDto>>('/admin/challenges?pageSize=100')
export const getAdminChallenge = (id: string) => apiRequest<AdminChallenge>(`/admin/challenges/${id}`)
export const saveAdminChallenge = (id: string | undefined, input: ChallengeMutation) => apiRequest<AdminChallenge>(id ? `/admin/challenges/${id}` : '/admin/challenges', { method: id ? 'PUT' : 'POST', body: JSON.stringify(input) })
export const publishAdminChallenge = (id: string) => apiRequest<void>(`/admin/challenges/${id}/publish`, { method: 'POST' })
export const archiveAdminChallenge = (id: string) => apiRequest<void>(`/admin/challenges/${id}`, { method: 'DELETE' })
export const deleteChallengeFile = (id: string, fileId: string) => apiRequest<void>(`/admin/challenges/${id}/files/${fileId}`, { method: 'DELETE' })
export async function uploadChallengeFile(id: string, file: File) { const body = new FormData(); body.append('file', file); body.append('public', 'false'); return apiRequest(`/admin/challenges/${id}/files`, { method: 'POST', body }) }
