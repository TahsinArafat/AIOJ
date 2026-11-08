import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, beforeEach } from 'vitest'
import { DocumentTitle } from './DocumentTitle'

beforeEach(() => {
    document.title = 'AIOJ — Online Judge'
})

function renderAt(path: string) {
    return render(
        <MemoryRouter initialEntries={[path]}>
            <DocumentTitle />
        </MemoryRouter>,
    )
}

// End-to-end through the router: the title must follow the URL, which is the
// behaviour a bookmark or a browser tab actually shows.
test('DocumentTitle sets the title for the current route', () => {
    renderAt('/problems')
    expect(document.title).toBe('Problems — AIOJ')
})

test('DocumentTitle updates when the route changes', () => {
    const { unmount } = renderAt('/rankings')
    expect(document.title).toBe('Rankings — AIOJ')
    unmount()

    renderAt('/login')
    expect(document.title).toBe('Sign in — AIOJ')
})

test('DocumentTitle names an unknown route instead of leaving the home title', () => {
    renderAt('/definitely/not/a/route')
    expect(document.title).toBe('Page not found — AIOJ')
})
