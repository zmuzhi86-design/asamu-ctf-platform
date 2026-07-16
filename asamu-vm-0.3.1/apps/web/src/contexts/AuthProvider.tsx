import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { currentUser, login as loginRequest, logout as logoutRequest, register as registerRequest, type AuthUser } from '../services/authApi'

const adminRoles = new Set(['super_admin', 'site_admin', 'visual_operator', 'competition_admin', 'challenge_author', 'reviewer'])
type AuthContextValue = { user: AuthUser | null; loading: boolean; canAccessAdmin: boolean; login: (login: string, password: string) => Promise<void>; register: (email: string, username: string, password: string) => Promise<void>; logout: () => Promise<void>; hasPermission: (permission: string) => boolean }
const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [loading, setLoading] = useState(true)
  useEffect(() => { currentUser().then(setUser).catch(() => setUser(null)).finally(() => setLoading(false)) }, [])
  const value = useMemo<AuthContextValue>(() => ({
    user, loading, canAccessAdmin: Boolean(user?.roles.some((role) => adminRoles.has(role))),
    login: async (login, password) => { const session = await loginRequest(login, password); setUser(session.user) },
    register: async (email, username, password) => { const session = await registerRequest(email, username, password); setUser(session.user) },
    logout: async () => { try { await logoutRequest() } finally { setUser(null) } },
    hasPermission: (permission) => Boolean(user?.permissions.includes('*') || user?.permissions.includes(permission)),
  }), [loading, user])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() { const value = useContext(AuthContext); if (!value) throw new Error('useAuth must be used inside AuthProvider'); return value }
