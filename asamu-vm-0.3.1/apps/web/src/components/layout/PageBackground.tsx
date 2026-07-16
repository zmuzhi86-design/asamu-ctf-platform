import { useLocation } from 'react-router-dom'
import { useAssetSystem } from '../../contexts/AssetProvider'

function pageKey(pathname: string) {
  if (pathname === '/') return 'home'
  if (pathname.startsWith('/admin')) return 'admin'
  if (pathname.startsWith('/challenges/')) return 'challenge_detail'
  if (pathname.startsWith('/challenges')) return 'challenges'
  if (pathname.startsWith('/competitions')) return 'competitions'
  if (pathname.startsWith('/learning')) return 'learning'
  if (pathname.startsWith('/teams/')) return 'team_detail'
  if (pathname.startsWith('/teams')) return 'team_list'
  if (pathname.startsWith('/leaderboard')) return 'leaderboard'
  if (pathname.startsWith('/writeups')) return 'writeups'
  if (pathname.startsWith('/profile')) return 'profile'
  if (pathname === '/login' || pathname === '/register' || pathname.includes('password') || pathname.includes('verify-email')) return 'login'
  return 'global'
}

export function PageBackground() {
  const { pathname } = useLocation()
  const { backgrounds, resolve } = useAssetSystem()
  const background = backgrounds.find((item) => item.pageKey === pageKey(pathname) && item.enabled && item.status === 'published') ?? backgrounds.find((item) => item.pageKey === 'global' && item.status === 'published')
  if (!background) return null
  const dark = document.documentElement.classList.contains('theme-dark')
  const mobile = window.matchMedia('(max-width: 760px)').matches
  const assetKey = dark ? (mobile ? background.darkMobileAssetKey ?? background.darkAssetKey : background.darkAssetKey) : (mobile ? background.mobileAssetKey ?? background.lightAssetKey : background.lightAssetKey)
  const asset = resolve(assetKey ?? background.lightAssetKey, { theme: dark ? 'dark' : 'light', mobile })
  const repeat = background.fit === 'repeat' || background.fit === 'repeat-x'
  return <div className="pointer-events-none fixed inset-0 z-0 overflow-hidden" aria-hidden="true">
    <div className="absolute inset-0" style={{ backgroundImage: `url("${asset.url}")`, backgroundRepeat: repeat ? background.fit : 'no-repeat', backgroundSize: repeat ? 'auto' : background.fit, backgroundPosition: background.position, opacity: background.assetOpacity, filter: background.blur ? `blur(${background.blur}px)` : undefined, transform: background.blur ? 'scale(1.03)' : undefined }} />
    <div className="absolute inset-0" style={{ backgroundColor: background.overlayColor, opacity: background.overlayOpacity }} />
  </div>
}
