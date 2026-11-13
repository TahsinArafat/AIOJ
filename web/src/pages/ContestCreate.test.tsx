import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import ContestCreate from './ContestCreate'
import { api } from '../lib/api'
import type { Mock } from 'vitest'

vi.mock('../lib/api', () => ({
    api: { contests: { create: vi.fn() } },
    getAccessToken: vi.fn().mockReturnValue('mock-token'),
    contestSlug: (c: { slug?: string; display_id?: number; id: string }) =>
        c.slug && !/^[0-9a-f-]{36}$/.test(c.slug) ? c.slug : String(c.display_id || c.id),
}))

/**
 * The create page asks for a name and hands off to the workspace.
 *
 * Two things are worth pinning, and neither is obvious from reading the component:
 *
 *   1. It sends ONLY the name (plus the slug). This is the whole point of the
 *      reduction — a regression that quietly re-adds `start_time: ""` or a default
 *      format would reintroduce the six-section form this replaced, and the symptom
 *      would be a round created with a schedule the setter never chose rather than
 *      an error.
 *   2. It lands on the workspace SETTINGS tab, not the public contest page. A new
 *      round is a draft, so the public page has nothing to render; the settings tab
 *      is where the schedule and the problems are.
 */
beforeEach(() => {
    vi.clearAllMocks()
    ;(api.contests.create as Mock).mockResolvedValue({ id: 'c1', slug: 'my-round', display_id: 7 })
})

function renderPage() {
    return render(
        <MemoryRouter initialEntries={['/setter/contest/create']}>
            <Routes>
                <Route path="/setter/contest/create" element={<ContestCreate />} />
                {/* Reports the URL it landed on as ONE text node. The hash has to be
                    assembled from the location hook rather than interpolated around
                    `location.hash` in JSX, which testing-library cannot match across
                    element boundaries. */}
                <Route path="/setter/contest/:id/manage" element={<HashProbe />} />
            </Routes>
        </MemoryRouter>,
    )
}

function HashProbe() {
    const loc = useLocation()
    return <div data-testid="landed">{`${loc.pathname}${loc.hash}`}</div>
}

test('creates from the name alone and continues to the workspace', async () => {
    renderPage()
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'My Round' } })
    fireEvent.click(screen.getByRole('button', { name: /Create contest and continue/i }))

    await waitFor(() => expect(api.contests.create).toHaveBeenCalledTimes(1))
    const sent = (api.contests.create as Mock).mock.calls[0][0] as Record<string, unknown>
    expect(sent.title).toBe('My Round')
    // The slug is derived from the name, so it is sent — but nothing else is.
    expect(sent.slug).toBe('my-round')
    expect(Object.keys(sent).sort()).toEqual(['slug', 'title'])

    // Lands on the settings tab of the new round's workspace.
    await waitFor(() => expect(screen.getByTestId('landed')).toHaveTextContent('/setter/contest/my-round/manage#settings'))
})

test('refuses to submit without a name', async () => {
    renderPage()
    fireEvent.click(screen.getByRole('button', { name: /Create contest and continue/i }))
    expect(api.contests.create).not.toHaveBeenCalled()
})

test('a hand-edited slug is sent as typed', async () => {
    renderPage()
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'My Round' } })
    fireEvent.change(screen.getByLabelText('URL Slug'), { target: { value: 'custom-slug' } })
    fireEvent.click(screen.getByRole('button', { name: /Create contest and continue/i }))

    await waitFor(() => expect(api.contests.create).toHaveBeenCalled())
    expect((api.contests.create as Mock).mock.calls[0][0].slug).toBe('custom-slug')
})

test('surfaces a create failure instead of navigating', async () => {
    ;(api.contests.create as Mock).mockRejectedValue(new Error('slug already taken'))
    renderPage()
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'My Round' } })
    fireEvent.click(screen.getByRole('button', { name: /Create contest and continue/i }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(/slug already taken/))
    expect(screen.queryByTestId('landed')).not.toBeInTheDocument()
})