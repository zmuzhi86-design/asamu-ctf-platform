import { useEffect, useState } from 'react'
import { Outlet } from 'react-router-dom'
import { TopNavigation } from '../components/layout/TopNavigation'
import { PageBackground } from '../components/layout/PageBackground'
import { usePlatform } from '../contexts/PlatformProvider'

export function AppLayout() {
  const { config } = usePlatform()
  const [dark, setDark] = useState(() => localStorage.getItem('asamu-theme') === 'dark')

  useEffect(() => {
    document.documentElement.classList.toggle('theme-dark', dark)
    localStorage.setItem('asamu-theme', dark ? 'dark' : 'light')
  }, [dark])

  return <div className="relative isolate min-h-screen text-asamu-ink"><PageBackground /><div className="relative z-10">
    <TopNavigation dark={dark} onToggleTheme={() => setDark((value) => !value)} />
    <main><Outlet /></main>
    <footer className="border-t border-asamu-line bg-asamu-card/90 px-4 py-6 text-center text-xs font-semibold text-asamu-muted">{config.profile.footerMarkdown || `© 2026 ${config.profile.platformName}`}</footer>
  </div></div>
}
