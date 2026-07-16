import { useEffect } from 'react'
import { Link, useRouteError } from 'react-router-dom'

export function RouteErrorPage() {
  const routeError = useRouteError()
  useEffect(() => { console.error('asamu route failed to render', routeError) }, [routeError])
  return <main className="grid min-h-screen place-items-center px-4 text-asamu-ink">
    <section className="w-full max-w-xl border-2 border-asamu-ink bg-asamu-card p-6 shadow-pixel">
      <p className="text-xs font-black tracking-[.2em] text-asamu-blue">ASAMU RECOVERY</p>
      <h1 className="mt-3 font-display text-2xl font-black">页面加载失败</h1>
      <p className="mt-3 text-sm font-semibold leading-6 text-asamu-muted">页面组件遇到异常，请重新加载。如果问题持续存在，请联系平台管理员检查 Web/API 日志。</p>
      <div className="mt-5 flex flex-wrap gap-3">
        <button className="pixel-button pixel-button-primary pixel-button-md" onClick={() => window.location.reload()}>重新加载</button>
        <Link className="pixel-button pixel-button-secondary pixel-button-md" to="/">返回首页</Link>
      </div>
    </section>
  </main>
}
