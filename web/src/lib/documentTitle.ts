import { useEffect } from 'react'

/**
 * Document title management.
 *
 * Every route used to keep whatever `<title>` index.html was built with, so the
 * whole site read "AIOJ — Online Judge" in a browser tab, a bookmark and a
 * history entry. Requirement: the tab has to name the page you are on.
 *
 * Two layers, on purpose:
 *
 *   1. `<DocumentTitle />` (components/DocumentTitle.tsx, mounted once next to
 *      `<Routes>`) derives a title from the path alone via `titleForPath`. That
 *      covers every static page without each component having to declare one.
 *   2. `useDocumentTitle(title)` lets a detail page replace it with the real
 *      entity name once it has loaded ("Two Sum — AIOJ" instead of "Problem").
 *
 * Ordering is what makes layer 2 win: the route-level component runs in a layout
 * effect and therefore before any page's (plain) effect, and a late-arriving
 * fetch only re-runs the page's effect. That is also why an empty title is
 * ignored below — a detail page must not blank the tab mid-fetch.
 */

const SITE = 'AIOJ'
const SUFFIX = ` — ${SITE}`

/** The home page keeps the fuller, searchable marketing title. */
const HOME_TITLE = `${SITE} — Online Judge`

/** Joins a page name with the site name. */
export function formatTitle(title: string): string {
    const next = title.trim()
    if (!next || next === SITE) return HOME_TITLE
    return `${next}${SUFFIX}`
}

/**
 * Sets the document title. An empty/undefined title is ignored rather than
 * clearing the tab — detail pages call this with not-yet-loaded data, and must
 * not blank the title during the fetch.
 */
export function useDocumentTitle(title?: string | null): void {
    useEffect(() => {
        const next = (title ?? '').trim()
        if (!next) return
        const full = formatTitle(next)
        if (document.title !== full) document.title = full
    }, [title])
}

/** Static path → title rules, most specific first. */
const ROUTE_TITLES: { pattern: RegExp; title: string }[] = [
    { pattern: /^\/$/, title: 'Home' },

    { pattern: /^\/problems\/[^/]+$/, title: 'Problem' },
    { pattern: /^\/problems\/?$/, title: 'Problems' },

    { pattern: /^\/contests\/[^/]+\/scoreboard$/, title: 'Scoreboard' },
    { pattern: /^\/contests\/[^/]+\/plagiarism$/, title: 'Plagiarism' },
    { pattern: /^\/contests\/[^/]+\/problem\/[^/]+$/, title: 'Contest problem' },
    { pattern: /^\/contests\/[^/]+$/, title: 'Contest' },
    { pattern: /^\/contests\/?$/, title: 'Contests' },

    { pattern: /^\/gym\/[^/]+$/, title: 'Gym contest' },
    { pattern: /^\/gym\/?$/, title: 'Gym' },

    { pattern: /^\/hack\//, title: 'Hack' },

    { pattern: /^\/groups\/create$/, title: 'Create group' },
    { pattern: /^\/groups\/join$/, title: 'Join group' },
    { pattern: /^\/groups\/[^/]+$/, title: 'Group' },
    { pattern: /^\/groups\/?$/, title: 'Groups' },

    { pattern: /^\/teams\/create$/, title: 'Create team' },
    { pattern: /^\/teams\/[^/]+$/, title: 'Team' },
    { pattern: /^\/teams\/?$/, title: 'Teams' },

    { pattern: /^\/blog\/create$/, title: 'Write a post' },
    { pattern: /^\/blog\/[^/]+$/, title: 'Blog post' },
    { pattern: /^\/blog\/?$/, title: 'Blog' },

    { pattern: /^\/editorials\/[^/]+$/, title: 'Editorial' },
    { pattern: /^\/editorials\/?$/, title: 'Editorials' },

    { pattern: /^\/organizations\/create$/, title: 'Create organization' },
    { pattern: /^\/organizations\/[^/]+$/, title: 'Organization' },
    { pattern: /^\/organizations\/?$/, title: 'Organizations' },

    { pattern: /^\/classes\/[^/]+$/, title: 'Class' },

    { pattern: /^\/training\/create$/, title: 'Create training plan' },
    { pattern: /^\/training\/[^/]+$/, title: 'Training plan' },
    { pattern: /^\/training\/?$/, title: 'Training' },

    { pattern: /^\/submissions\/[^/]+$/, title: 'Submission' },
    { pattern: /^\/submissions\/?$/, title: 'Submissions' },

    { pattern: /^\/user\/[^/]+$/, title: 'User' },
    { pattern: /^\/profile$/, title: 'Profile' },
    { pattern: /^\/settings\/api$/, title: 'API keys' },
    { pattern: /^\/settings\/notifications$/, title: 'Notification settings' },
    { pattern: /^\/settings$/, title: 'Settings' },

    { pattern: /^\/login$/, title: 'Sign in' },
    { pattern: /^\/register$/, title: 'Create account' },
    { pattern: /^\/forgot-password$/, title: 'Reset password' },
    { pattern: /^\/reset-password$/, title: 'Choose a new password' },
    { pattern: /^\/verify-email$/, title: 'Verify email' },
    { pattern: /^\/oauth\/complete$/, title: 'Completing sign-in' },
    { pattern: /^\/auth\/2fa\/setup$/, title: 'Set up two-factor' },
    { pattern: /^\/auth\/2fa\/verify$/, title: 'Two-factor verification' },

    { pattern: /^\/notifications$/, title: 'Notifications' },
    { pattern: /^\/rankings$/, title: 'Rankings' },
    { pattern: /^\/rating-history$/, title: 'Rating history' },
    { pattern: /^\/practice$/, title: 'Practice' },
    { pattern: /^\/virtual$/, title: 'Virtual contest' },
    { pattern: /^\/ide$/, title: 'IDE' },

    { pattern: /^\/admin$/, title: 'Admin' },
    { pattern: /^\/setter\/create$/, title: 'Create problem' },
    { pattern: /^\/setter\/ai\/jobs\/[^/]+$/, title: 'AI generation job' },
    { pattern: /^\/setter\/ai\/jobs$/, title: 'AI generation jobs' },
    { pattern: /^\/setter\/ai$/, title: 'AI problem generation' },
    { pattern: /^\/setter\/contest\/create$/, title: 'Create contest' },
    { pattern: /^\/setter\/contest\/[^/]+\/manage$/, title: 'Manage contest' },
    { pattern: /^\/setter\/[^/]+$/, title: 'Problem workspace' },
    { pattern: /^\/setter\/?$/, title: 'Setter panel' },

    { pattern: /^\/legal\/terms$/, title: 'Terms of Service' },
    { pattern: /^\/legal\/privacy$/, title: 'Privacy Policy' },
    { pattern: /^\/legal\/dmca$/, title: 'DMCA' },

    { pattern: /.*/, title: 'Page not found' },
]

/** Title for a pathname, from the static map. */
export function titleForPath(pathname: string): string {
    const path = pathname.length > 1 && pathname.endsWith('/') ? pathname.slice(0, -1) : pathname
    for (const rule of ROUTE_TITLES) {
        if (rule.pattern.test(path)) return rule.title
    }
    return 'Page not found'
}
