import { useEffect, useState } from 'react'

/**
 * Returns true when the browser tab is visible (document.visibilityState ===
 * 'visible'), false when the user has switched away.
 *
 * Use this to pause expensive setInterval polls while the user isn't looking —
 * it saves both server DB queries and client CPU cycles with zero UX impact,
 * because the next poll fires immediately when the tab becomes visible again.
 */
export function usePageVisible(): boolean {
  const [visible, setVisible] = useState(() =>
    typeof document !== 'undefined'
      ? document.visibilityState === 'visible'
      : true
  )

  useEffect(() => {
    const handler = () => setVisible(document.visibilityState === 'visible')
    document.addEventListener('visibilitychange', handler)
    return () => document.removeEventListener('visibilitychange', handler)
  }, [])

  return visible
}
