import React, { Suspense } from 'react'
import ReactDOM from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { router } from './app/router'
import { AssetProvider } from './contexts/AssetProvider'
import { AuthProvider } from './contexts/AuthProvider'
import { PlatformProvider } from './contexts/PlatformProvider'
import { AppErrorBoundary } from './components/system/AppErrorBoundary'
import './styles/index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppErrorBoundary><AuthProvider><AssetProvider><PlatformProvider><Suspense fallback={<div className="grid min-h-screen place-items-center bg-asamu-canvas font-black text-asamu-blue">正在进入 asamu…</div>}><RouterProvider router={router} /></Suspense></PlatformProvider></AssetProvider></AuthProvider></AppErrorBoundary>
  </React.StrictMode>,
)
