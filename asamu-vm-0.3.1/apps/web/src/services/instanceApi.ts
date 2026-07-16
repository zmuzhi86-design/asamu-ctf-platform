import { apiRequest, createIdempotencyKey, mockApiEnabled } from './apiClient'
import { randomUUID } from '../utils/random'

export type InstanceStatus = 'pending' | 'pulling' | 'creating' | 'starting' | 'running' | 'restarting' | 'resetting' | 'stopping' | 'stopped' | 'failed' | 'expired' | 'interrupted' | 'deleted'
export type ChallengeInstance = { id: string; challengeId: string; challengeSlug?: string; status: InstanceStatus; accessUrl?: string; port?: number; internalPort?: number; startedAt?: string; expiresAt?: string; remainingSeconds?: number; errorCode?: string; errorMessage?: string; version: number; generation?: number }
export type InstanceScope = { competitionId?: string; teamId?: string }

const STORAGE_PREFIX = 'asamu-instance-dev:'
const transitional = new Set<InstanceStatus>(['pending', 'pulling', 'creating', 'starting', 'restarting', 'resetting', 'stopping'])
export function isTransitionalInstanceStatus(status: InstanceStatus) { return transitional.has(status) }
function mockStored(challengeId: string): ChallengeInstance { try { const raw = localStorage.getItem(STORAGE_PREFIX + challengeId); if (raw) return JSON.parse(raw) } catch {} return { id: '', challengeId, status: 'stopped', version: 0 } }
function mockSave(value: ChallengeInstance) { localStorage.setItem(STORAGE_PREFIX + value.challengeId, JSON.stringify(value)); return value }
async function mockOperation(challengeId: string, operation: string) { await new Promise((resolve) => setTimeout(resolve, 700)); const current = mockStored(challengeId); if (operation === 'stop') return mockSave({ id: current.id, challengeId, status: 'stopped', version: current.version + 1 }); if (operation === 'start' || operation === 'reset') return mockSave({ ...current, id: randomUUID(), challengeId, status: 'running', accessUrl: `http://127.0.0.1:${operation === 'reset' ? 20881 : 20880}`, port: operation === 'reset' ? 20881 : 20880, internalPort: 8080, startedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 7_200_000).toISOString(), remainingSeconds: 7200, version: current.version + 1, generation: (current.generation ?? 0) + (operation === 'reset' ? 1 : 0) }); return mockSave({ ...current, status: 'running', version: current.version + 1 }) }

function scopeQuery(scope?: InstanceScope) { const query = new URLSearchParams(); if (scope?.competitionId) query.set('competitionId', scope.competitionId); if (scope?.teamId) query.set('teamId', scope.teamId); const value = query.toString(); return value ? `?${value}` : '' }
export async function getChallengeInstance(challengeId: string, scope?: InstanceScope) { if (mockApiEnabled) return mockStored(`${challengeId}:${scope?.competitionId ?? 'global'}`); return apiRequest<ChallengeInstance>(`/challenges/${challengeId}/instance/status${scopeQuery(scope)}`) }
async function waitForStable(challengeId: string, initial: ChallengeInstance, scope?: InstanceScope) { let current = initial; for (let attempt = 0; attempt < 60 && transitional.has(current.status); attempt += 1) { await new Promise((resolve) => setTimeout(resolve, 1000)); current = await getChallengeInstance(challengeId, scope) } return current }
async function operate(challengeId: string, operation: 'start' | 'restart' | 'stop' | 'reset', scope?: InstanceScope) { if (mockApiEnabled) return mockOperation(challengeId, operation); const initial = await apiRequest<ChallengeInstance>(`/challenges/${challengeId}/instance/${operation}`, { method: 'POST', headers: { 'Idempotency-Key': createIdempotencyKey(operation) }, body: JSON.stringify(scope ?? {}) }); return waitForStable(challengeId, initial, scope) }
export function startChallengeInstance(challengeId: string, scope?: InstanceScope) { return operate(challengeId, 'start', scope) }
export function restartChallengeInstance(challengeId: string, scope?: InstanceScope) { return operate(challengeId, 'restart', scope) }
export function stopChallengeInstance(challengeId: string, scope?: InstanceScope) { return operate(challengeId, 'stop', scope) }
export function resetChallengeInstance(challengeId: string, scope?: InstanceScope) { return operate(challengeId, 'reset', scope) }
