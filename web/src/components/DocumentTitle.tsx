import { useLayoutEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { formatTitle, titleForPath } from '../lib/documentTitle'

/**
 * Route-level document title, mounted once next to `<Routes>`.
 *
 * Uses `useLayoutEffect` so the title is correct in the same frame as the new
 * page rather than a tick later — a history entry or a bookmark taken
 * immediately after a click would otherwise capture the previous title.
 *
 * Page-level `useDocumentTitle` calls (from `lib/documentTitle`) are plain
 * effects, which React flushes after layout effects, so a detail page replaces
 * "Problem" with the real problem name without any coordination between the two.
 */
export function DocumentTitle() {
    const { pathname } = useLocation()

    useLayoutEffect(() => {
        const full = formatTitle(titleForPath(pathname))
        if (document.title !== full) document.title = full
    }, [pathname])

    return null
}
