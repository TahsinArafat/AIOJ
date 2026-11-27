import { useLayoutEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { canonicalHref, formatTitle, titleForPath } from '../lib/documentTitle'

/**
 * Route-level document title and canonical URL, mounted once next to `<Routes>`.
 *
 * Uses `useLayoutEffect` so the head is correct in the same frame as the new
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

        // The canonical is set here rather than in index.html because the SPA
        // serves every route from one file: a static tag would declare a single
        // URL canonical for the whole site. Crawlers that render JS — Google and
        // Bing — see this and treat the entity page as canonical to itself.
        const href = canonicalHref(pathname, window.location.origin)
        if (!href) return
        let link = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
        if (!link) {
            link = document.createElement('link')
            link.rel = 'canonical'
            document.head.appendChild(link)
        }
        // getAttribute, not .href: the property resolves and normalises, so
        // comparing it against a built string reports a difference every time.
        if (link.getAttribute('href') !== href) link.setAttribute('href', href)
    }, [pathname])

    return null
}