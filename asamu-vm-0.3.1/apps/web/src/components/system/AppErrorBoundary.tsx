import { Component, type ErrorInfo, type ReactNode } from 'react'

type Props = { children: ReactNode }
type State = { failed: boolean }

export class AppErrorBoundary extends Component<Props, State> {
  state: State = { failed: false }

  static getDerivedStateFromError(): State {
    return { failed: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('asamu UI failed to render', error, info.componentStack)
  }

  render() {
    if (!this.state.failed) return this.props.children
    return <main className="grid min-h-screen place-items-center px-4 text-asamu-ink">
      <section className="w-full max-w-xl border-2 border-asamu-ink bg-asamu-card p-6 shadow-pixel">
        <p className="text-xs font-black tracking-[.2em] text-asamu-blue">ASAMU RECOVERY</p>
        <h1 className="mt-3 font-display text-2xl font-black">页面加载失败</h1>
        <p className="mt-3 text-sm font-semibold leading-6 text-asamu-muted">请刷新页面重试。如果问题持续存在，请运行服务器上的 docker-doctor.sh 并检查 Web/API 日志。</p>
        <button className="pixel-button pixel-button-primary pixel-button-md mt-5" onClick={() => window.location.reload()}>重新加载</button>
      </section>
    </main>
  }
}
