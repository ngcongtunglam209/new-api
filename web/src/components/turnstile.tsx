/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useRef } from 'react'

declare global {
  interface Window {
    turnstile?: {
      render: (
        element: HTMLElement,
        options: Record<string, unknown>
      ) => string | undefined
      remove: (widgetId: string) => void
      reset: (widgetId?: string) => void
    }
  }
}

const TURNSTILE_SCRIPT_ID = 'cf-turnstile'
const TURNSTILE_SCRIPT_SRC =
  'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'

let turnstileScriptPromise: Promise<void> | null = null

/**
 * Loads the Cloudflare script once for the whole app. Every widget awaits the
 * same promise, so a second widget mounting while the script is still in
 * flight still renders instead of silently giving up on the existing tag.
 */
function loadTurnstileScript(): Promise<void> {
  if (typeof window === 'undefined') {
    return Promise.reject(new Error('Turnstile requires a browser environment'))
  }
  if (window.turnstile) return Promise.resolve()
  if (!turnstileScriptPromise) {
    turnstileScriptPromise = new Promise<void>((resolve, reject) => {
      document.querySelector(`#${TURNSTILE_SCRIPT_ID}`)?.remove()
      const script = document.createElement('script')
      script.id = TURNSTILE_SCRIPT_ID
      script.src = TURNSTILE_SCRIPT_SRC
      script.async = true
      script.defer = true
      script.addEventListener('load', () => resolve(), { once: true })
      script.addEventListener(
        'error',
        () => reject(new Error('Turnstile script failed to load')),
        { once: true }
      )
      document.head.appendChild(script)
    }).catch((error: unknown) => {
      // Do not cache the failure, otherwise a transient network error blocks
      // the widget for the rest of the session.
      turnstileScriptPromise = null
      throw error
    })
  }
  return turnstileScriptPromise
}

interface TurnstileProps {
  siteKey: string
  onVerify: (token: string) => void
  /** Solved token is no longer valid. Cloudflare auto-refreshes by default. */
  onExpire?: () => void
  /** Challenge or script failed. Cloudflare does NOT auto-retry these. */
  onError?: () => void
  className?: string
}

export function Turnstile({
  siteKey,
  onVerify,
  onExpire,
  onError,
  className,
}: TurnstileProps) {
  const ref = useRef<HTMLDivElement | null>(null)
  const handlers = useRef({ onVerify, onExpire, onError })

  useEffect(() => {
    handlers.current = { onVerify, onExpire, onError }
  }, [onVerify, onExpire, onError])

  // Only siteKey may re-render the widget. Callers routinely pass inline arrow
  // functions, and re-running on those would call render() against an
  // already-rendered container on every keystroke.
  useEffect(() => {
    let cancelled = false
    let widgetId: string | undefined

    loadTurnstileScript()
      .then(() => {
        if (cancelled || !ref.current || !window.turnstile) return
        widgetId = window.turnstile.render(ref.current, {
          sitekey: siteKey,
          callback: (token: string) => handlers.current.onVerify(token),
          'expired-callback': () => handlers.current.onExpire?.(),
          'timeout-callback': () => handlers.current.onExpire?.(),
          'error-callback': () => handlers.current.onError?.(),
        })
        if (widgetId === undefined) {
          throw new Error('Turnstile refused to render the widget')
        }
      })
      .catch((error: unknown) => {
        if (cancelled) return
        // eslint-disable-next-line no-console
        console.error('Turnstile initialization failed', error)
        handlers.current.onError?.()
      })

    return () => {
      cancelled = true
      if (widgetId === undefined) return
      try {
        window.turnstile?.remove(widgetId)
      } catch {
        // The widget was already torn down together with its container.
      }
    }
  }, [siteKey])

  return <div ref={ref} className={className} />
}
