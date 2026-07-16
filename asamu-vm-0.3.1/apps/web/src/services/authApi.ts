import { apiRequest, setAccessToken } from './apiClient'

export type AuthUser = { id: string; email: string; username: string; status: string; roles: string[]; permissions: string[]; mustChangePassword: boolean; emailVerified: boolean; pendingEmail?: string }
export type Session = { accessToken: string; accessTokenExpiresAt: string; refreshTokenExpiresAt: string; user: AuthUser }

export async function login(login: string, password: string) { const session = await apiRequest<Session>('/auth/login', { method: 'POST', body: JSON.stringify({ login, password }) }); setAccessToken(session.accessToken); return session }
export async function register(email: string, username: string, password: string) { const session = await apiRequest<Session>('/auth/register', { method: 'POST', body: JSON.stringify({ email, username, password }) }); setAccessToken(session.accessToken); return session }
export async function logout() { try { await apiRequest<void>('/auth/logout', { method: 'POST' }) } finally { setAccessToken() } }
export function currentUser() { return apiRequest<AuthUser>('/auth/me') }
export function requestPasswordReset(email: string) { return apiRequest<{ message: string }>('/auth/forgot-password', { method: 'POST', body: JSON.stringify({ email }) }) }
export function resetPassword(token: string, newPassword: string) { return apiRequest<void>('/auth/reset-password', { method: 'POST', body: JSON.stringify({ token, newPassword }) }) }
export function verifyEmail(token: string) { return apiRequest<{ verified: boolean }>('/auth/verify-email', { method: 'POST', body: JSON.stringify({ token }) }) }
export function resendVerification() { return apiRequest<{ message: string }>('/auth/verification-email', { method: 'POST' }) }
export function requestEmailChange(currentPassword: string, newEmail: string) { return apiRequest<{ message: string }>('/auth/email/change', { method: 'POST', body: JSON.stringify({ currentPassword, newEmail }) }) }
export function confirmEmailChange(token: string) { return apiRequest<{ changed: boolean }>('/auth/confirm-email-change', { method: 'POST', body: JSON.stringify({ token }) }) }
export function changePassword(currentPassword: string, newPassword: string) { return apiRequest<void>('/auth/password', { method: 'POST', body: JSON.stringify({ currentPassword, newPassword }) }) }
