// agentation-overlay.ts
// Mounts the Agentation feedback toolbar as a React overlay on the vanilla TS app.
// React is only used for this thin shim — the rest of the app remains vanilla TS.
//
// Agentation lets users click any element, annotate it, and generate structured
// markdown output that describes the UI change in terms an AI agent can act on.
// Submissions are stored in localStorage and optionally POSTed to /api/feedback
// when the backend API is available.

import React from 'react'
import ReactDOM from 'react-dom/client'
import { Agentation } from 'agentation'
import type { Annotation } from 'agentation'

const STORAGE_KEY = 'acb:agentation:feedback'
const MAX_STORED = 50

function handleSubmit(markdown: string, annotations: Annotation[]): void {
  console.log('[agentation] Feedback submitted')

  // Persist locally
  const existing: Array<{ markdown: string; annotations: Annotation[]; submittedAt: number }> =
    JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
  existing.push({ markdown, annotations, submittedAt: Date.now() })
  localStorage.setItem(STORAGE_KEY, JSON.stringify(existing.slice(-MAX_STORED)))

  // POST to the API if available (non-blocking, best-effort)
  const apiBase = (window as unknown as Record<string, string>)['ACB_API_BASE'] ?? '/api'
  fetch(`${apiBase}/ui-feedback`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ markdown, annotations, submitted_at: new Date().toISOString() }),
  }).catch(() => {
    // API not available yet — localStorage fallback is sufficient
  })
}

export function initAgentation(): void {
  const container = document.createElement('div')
  container.id = 'agentation-root'
  document.body.appendChild(container)

  const root = ReactDOM.createRoot(container)
  // Cast through ElementType so createElement accepts the props without JSX support
  root.render(
    React.createElement(Agentation as React.ElementType, {
      onSubmit: handleSubmit,
      copyToClipboard: true,
    })
  )
}
