import { apiRequest } from './apiClient'

export type LearningChallenge = { id: string; slug: string; title: string; difficulty: string; score: number; dynamic: boolean; required: boolean; completed: boolean; sortOrder: number }
export type LearningStage = { id: string; title: string; description: string; sortOrder: number; challenges: LearningChallenge[]; completedChallenges: number; totalChallenges: number; completed: boolean }
export type LearningPath = { id: string; slug: string; directionId?: string; directionKey: string; directionName: string; sceneAssetKey: string; title: string; summary: string; description: string; prerequisite: string; estimatedMinutes: number; heroAssetKey: string; status: 'draft' | 'published' | 'archived'; featured: boolean; sortOrder: number; publishedAt?: string; updatedAt: string; stages: LearningStage[]; completedChallenges: number; totalChallenges: number; progress: number }
export type LearningPathMutation = { slug: string; directionKey: string; title: string; summary: string; description: string; prerequisite: string; estimatedMinutes: number; heroAssetKey: string; featured: boolean; sortOrder: number; stages: Array<{ title: string; description: string; sortOrder: number; challengeIds: string[] }> }

export const fetchLearningPaths = () => apiRequest<LearningPath[]>('/learning/paths')
export const fetchAdminLearningPaths = () => apiRequest<LearningPath[]>('/admin/learning/paths')
export const saveLearningPath = (id: string | undefined, input: LearningPathMutation) => apiRequest<LearningPath>(id ? `/admin/learning/paths/${id}` : '/admin/learning/paths', { method: id ? 'PUT' : 'POST', body: JSON.stringify(input) })
export const publishLearningPath = (id: string) => apiRequest<LearningPath>(`/admin/learning/paths/${id}/publish`, { method: 'POST', body: '{}' })
export const archiveLearningPath = (id: string) => apiRequest<void>(`/admin/learning/paths/${id}`, { method: 'DELETE' })
