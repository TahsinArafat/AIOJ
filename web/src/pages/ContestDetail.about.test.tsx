import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { expect, test, vi } from 'vitest'
import ContestDetail from './ContestDetail'
import { ToastProvider } from '../components/Toast'
import { ConfirmProvider } from '../components/ConfirmDialog'

/**
 * The About tab, and the participant count that shares its data source.
 *
 * The count is the reported bug: the page read `registration_count` off the contest
 * response and the API never sent it, so the "Participants" row rendered a
 * hardcoded 0 however many people had entered. The fix is server-side, and this
 * asserts the client half — that a count which arrives is actually rendered rather
 * than overwritten by the initial 0.
 *
 * The About tab exists because the facts a newcomer wants (what this round is, who
 * runs it) were spread across a badge strip and a sidebar, and the organizer list
 * had no home at all. It shows the description and the organizers; the schedule and
 * format stay in the sidebar, which already renders them for every tab. Repeating
 * them here would be a second copy of the same data that can disagree with the
 * first.
 */

const mocks = vi.hoisted(() => ({
    get: vi.fn(),
    standings: vi.fn().mockResolvedValue({ entries: [], problems: [], frozen: false, pagination: null }),
    announcements: vi.fn().mockResolvedValue({ data: [] }),
    clarifications: vi.fn().mockResolvedValue({ data: [] }),
    submissions: vi.fn().mockResolvedValue({ data: [], total: 0, is_judge: false }),
}))

vi.mock('../lib/api', async () => {
    const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api')
    return {
        ...actual,
        api: {
            ...actual.api,
            contests: {
                ...actual.api.contests,
                get: mocks.get,
                standings: mocks.standings,
                announcements: mocks.announcements,
                submissions: mocks.submissions,
                listRegistrations: vi.fn().mockResolvedValue({ data: [], count: 0 }),
            },
            clarifications: { ...actual.api.clarifications, list: mocks.clarifications },
        },
        getAccessToken: vi.fn().mockReturnValue('token'),
    }
})

function roundWith(extra: Record<string, unknown> = {}) {
    return {
        contest: {
            id: 'c1',
            title: 'Test Round',
            start_time: '2026-11-13T03:50:00Z',
            end_time: '2026-11-13T06:40:00Z',
            format: 'acm',
            type: 'acm',
            registration_required: true,
            max_participants: 200,
            description: '',
            ...extra,
        },
        problems: [],
    }
}

function renderAt(hash: string) {
    // The tab is read from window.location.hash in a useState initializer, not from
    // the router — that is how this page has always done it, and in a browser the
    // two are the same string. Under MemoryRouter they are not, so the window hash
    // has to be set too or every test lands on Problems whatever entry it was given.
    window.location.hash = hash
    return render(
        <MemoryRouter initialEntries={[`/contests/c1${hash}`]}>
            <ToastProvider>
                <ConfirmProvider>
                    <Routes>
                        <Route path="/contests/:id" element={<ContestDetail />} />
                    </Routes>
                </ConfirmProvider>
            </ToastProvider>
        </MemoryRouter>,
    )
}

test('the participant count reflects what the API reports', async () => {
    // The reported bug: this row was permanently 0. The response now carries the
    // number; the assertion is that it reaches the screen rather than being replaced
    // by the initial state.
    mocks.get.mockResolvedValue(roundWith({ registration_count: 18 }))

    renderAt('#about')

    expect(await screen.findByText('18 / 200')).toBeInTheDocument()
    expect(screen.queryByText('0 / 200')).not.toBeInTheDocument()
})

test('a missing count still renders as zero rather than breaking', async () => {
    // The field is optional on the wire: a build without the registration store
    // omits it, and the page's fallback is what renders. This is the pre-fix shape,
    // kept as a test so the fallback is not removed along with the bug.
    mocks.get.mockResolvedValue(roundWith())

    renderAt('#about')

    expect(await screen.findByText('0 / 200')).toBeInTheDocument()
})

test('the About tab shows the round description', async () => {
    mocks.get.mockResolvedValue(roundWith({
        description: 'A two-hour round for the campus cup. Bring your own keyboard.',
    }))

    renderAt('#about')

    expect(await screen.findByText(/campus cup/)).toBeInTheDocument()
})

test('the About tab says so when there is no description', async () => {
    // An empty section would look like a loading state. Saying the organizer has not
    // written one is the honest rendering.
    mocks.get.mockResolvedValue(roundWith({ description: '   ' }))

    renderAt('#about')

    expect(await screen.findByText(/has not added a description/)).toBeInTheDocument()
})

test('the About tab lists the organizers by username', async () => {
    mocks.get.mockResolvedValue(roundWith({
        judges: [
            { user_id: 'u1', username: 'team_e8a001bc', access_level: 'manager' },
            { user_id: 'u2', username: 'grader_two', access_level: 'judge' },
        ],
    }))

    renderAt('#about')

    expect(await screen.findByText('team_e8a001bc')).toBeInTheDocument()
    expect(screen.getByText('grader_two')).toBeInTheDocument()
    // The role is labelled, so two judges are distinguishable from a manager.
    expect(screen.getByText('manager')).toBeInTheDocument()
    expect(screen.getByText('judge')).toBeInTheDocument()
})

test('testers are not named on the About tab', async () => {
    // The server does not send them. Asserted here so a client that started rendering
    // whatever it was given would fail rather than quietly widening the roster.
    mocks.get.mockResolvedValue(roundWith({
        judges: [{ user_id: 'u2', username: 'grader_two', access_level: 'judge' }],
    }))

    renderAt('#about')

    await screen.findByText('grader_two')
    expect(screen.queryByText('tester')).not.toBeInTheDocument()
})

test('an absent roster renders nothing rather than an empty heading', async () => {
    // `judges` is undefined when the caller may not read the round as content — the
    // server's decision, not the page's. The section must not appear at all, because
    // an "Organizers: none" heading on a round that has some would be a claim.
    mocks.get.mockResolvedValue(roundWith())

    renderAt('#about')

    expect(await screen.findByText(/About this round/)).toBeInTheDocument()
    expect(screen.queryByText('Organizers')).not.toBeInTheDocument()
})

test('the About tab does not repeat the sidebar schedule', async () => {
    // The sidebar's Contest Info box already renders start, end, freeze, format and
    // problem count on every tab. A second copy here is how the two drift.
    mocks.get.mockResolvedValue(roundWith({
        description: 'd',
        judges: [],
    }))

    renderAt('#about')

    await waitFor(() => expect(screen.getByText('Contest Info')).toBeInTheDocument())
    // One "Format" label, from the sidebar.
    expect(screen.getAllByText('Format')).toHaveLength(1)
})

test('the About tab is reachable by hash', async () => {
    // The hash has to name a real tab, or an old bookmark falls through to Problems
    // and the tab strip shows nothing selected.
    mocks.get.mockResolvedValue(roundWith({ description: 'd', judges: [] }))

    renderAt('#about')

    expect(await screen.findByText(/About this round/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'About' })).toHaveClass('text-brand')
})