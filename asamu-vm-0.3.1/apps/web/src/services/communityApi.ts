import { apiRequest } from './apiClient'
import type { Page, WriteupDto } from './platformApi'
import type { TeamDto } from './platformApi'

export type NotificationDto = { id: string; type: string; title: string; body: string; link?: string; payload?: Record<string, unknown>; readAt?: string; createdAt: string }
export type WriteupMutation = { title: string; summary: string; contentMarkdown: string; visibility: 'public' | 'unlisted' | 'private'; challengeId: string; competitionId?: string }

export function fetchNotifications(unreadOnly = false) { return apiRequest<Page<NotificationDto>>(`/notifications?pageSize=100${unreadOnly ? '&unread=true' : ''}`) }
export function readNotification(id: string) { return apiRequest<void>(`/notifications/${id}/read`, { method: 'PATCH' }) }
export function readAllNotifications() { return apiRequest<void>('/notifications/read-all', { method: 'POST' }) }
export function fetchMyWriteups() { return apiRequest<Page<WriteupDto>>('/me/writeups?pageSize=100') }
export function fetchMyWriteup(id: string) { return apiRequest<WriteupDto>(`/me/writeups/${id}`) }
export function createWriteup(input: WriteupMutation) { return apiRequest<WriteupDto>('/writeups', { method: 'POST', body: JSON.stringify(input) }) }
export function updateWriteup(id: string, input: WriteupMutation) { return apiRequest<WriteupDto>(`/writeups/${id}`, { method: 'PUT', body: JSON.stringify(input) }) }
export function submitWriteup(id: string) { return apiRequest<void>(`/writeups/${id}/submit-review`, { method: 'POST' }) }

export type TeamManagementDto = { team: TeamDto; myRole: 'captain' | 'manager' | 'member'; joinRequests: Array<{ id: string; userId: string; username: string; message: string; status: string; createdAt: string }>; invitations: Array<{ id: string; userId: string; username: string; status: string; expiresAt: string; createdAt: string }> }
export type TeamMutation = { name: string; slogan: string; description: string; flagAssetKey: string; bannerAssetKey: string; memberLimit: number; recruiting?: boolean }
export function fetchMyTeam() { return apiRequest<TeamManagementDto>('/me/team') }
export function createTeam(input: TeamMutation) { return apiRequest<TeamDto>('/teams', { method: 'POST', body: JSON.stringify(input) }) }
export function updateTeam(id: string, input: TeamMutation) { return apiRequest<TeamDto>(`/teams/${id}`, { method: 'PUT', body: JSON.stringify(input) }) }
export function uploadTeamAvatar(id: string, file: File) { const body = new FormData(); body.append('file', file); return apiRequest<TeamDto>(`/teams/${id}/avatar`, { method: 'POST', body }) }
export function inviteTeamMember(id: string, username: string) { return apiRequest<{ id: string }>(`/teams/${id}/invitations`, { method: 'POST', body: JSON.stringify({ username }) }) }
export function reviewTeamJoin(id: string, requestId: string, approve: boolean) { return apiRequest<void>(`/teams/${id}/join-requests/${requestId}/review`, { method: 'POST', body: JSON.stringify({ approve }) }) }
export function postTeamAnnouncement(id: string, title: string, content: string, pinned: boolean) { return apiRequest<void>(`/teams/${id}/announcements`, { method: 'POST', body: JSON.stringify({ title, content, pinned }) }) }
export function removeTeamMember(id: string, userId: string) { return apiRequest<void>(`/teams/${id}/members/${userId}`, { method: 'DELETE' }) }
export function transferTeamCaptain(id: string, userId: string) { return apiRequest<void>(`/teams/${id}/transfer-captain`, { method: 'POST', body: JSON.stringify({ userId }) }) }
export function leaveTeam(id: string) { return apiRequest<void>(`/teams/${id}/leave`, { method: 'POST' }) }

export type ProfileDto = { id: string; email?: string; username: string; displayName: string; bio: string; organizationName: string; avatarAssetKey: string; characterAssetKey: string; signature: string; status: string; skills: string[]; privacy?: Record<string, boolean>; recentSolves: Array<{ slug: string; title: string; category: string; score: number; solved_at?: string; solvedAt?: string }>; competitionHistory: Array<{ slug: string; name: string; status: string; registered_at?: string }>; favorites: Array<{ slug: string; title: string; summary: string; created_at?: string }> }
export type ProfileMutation = { displayName: string; bio: string; organizationName: string; avatarAssetKey: string; characterAssetKey: string; signature: string; skills: string[]; privacy: Record<string, boolean> }
export function fetchMyProfile() { return apiRequest<ProfileDto>('/me/profile') }
export function updateMyProfile(input: ProfileMutation) { return apiRequest<ProfileDto>('/me/profile', { method: 'PATCH', body: JSON.stringify(input) }) }
