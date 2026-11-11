import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { expect, test, vi } from 'vitest'
import ContestDetail from './ContestDetail'
import { ToastProvider } from '../components/Toast'
import { ConfirmProvider } from '../components/ConfirmDialog'

/**
 * The contest problem list used to render a bare "Solve" button for every
 * problem: no tick for one already solved, and no sign of which ones had been
 * attempted. A competitor opening the Problems tab could not tell where they
 * stood in the round without going back to the standings.
 *
 * `solved_count` is the round-wide count, `my_solved`/`my_attempted` are the
 * viewer's own state. They are distinct fields on purpose: a signed-out visitor
 * gets the counts but no personal state.
 */

const mocks = vi.hoisted(() => ({
    get: vi.fn(),
    standings: vi.fn().mockResolvedValue({
        entries: [], problems: [], frozen: false, pagination: null,
    }),
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

function contestWith(problems: unknown[]) {
    return {
        contest: {
            id: 'c1',
            title: 'Test Round 1',
            start_time: '2020-01-01T00:00:00Z',
            end_time: '2030-01-01T00:00:00Z',
            format: 'acm',
            type: 'acm',
            problems: problems.length,
        },
        problems,
    }
}

function renderAt(hash: string) {
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

test('marks a solved problem with a tick and shows the round solve count', async () => {
    mocks.get.mockResolvedValue(contestWith([
        { problem_id: 'p1', index: 'A', title: 'Alpha', score: 100, solved_count: 12, my_solved: true, my_attempted: true, my_submissions: 2 },
    ]))

    renderAt('#problems')

    expect(await screen.findByText('Alpha')).toBeInTheDocument()

    // The tick carries a label so it is findable without relying on colour.
    expect(screen.getByLabelText('Solved')).toBeInTheDocument()
    expect(screen.getByText('Solved')).toBeInTheDocument()
    // The solved row offers a review path instead of another "Solve".
    expect(screen.getByRole('link', { name: 'Review' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Solve' })).not.toBeInTheDocument()
    // Round-wide count.
    expect(screen.getByText('12')).toBeInTheDocument()
})

test('marks an attempted-but-unsolved problem and keeps the solve button', async () => {
    mocks.get.mockResolvedValue(contestWith([
        { problem_id: 'p2', index: 'B', title: 'Beta', score: 100, solved_count: 3, my_solved: false, my_attempted: true, my_submissions: 4 },
    ]))

    renderAt('#problems')

    expect(await screen.findByText('Beta')).toBeInTheDocument()
    expect(screen.getByText('Tried')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Solve' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Solved')).not.toBeInTheDocument()
})

test('an untouched problem shows neither tick nor tried', async () => {
    mocks.get.mockResolvedValue(contestWith([
        { problem_id: 'p3', index: 'C', title: 'Gamma', score: 100, solved_count: 0 },
    ]))

    renderAt('#problems')

    expect(await screen.findByText('Gamma')).toBeInTheDocument()
    expect(screen.queryByLabelText('Solved')).not.toBeInTheDocument()
    expect(screen.queryByText('Tried')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Solve' })).toBeInTheDocument()
})

test('keeps the solve count visible when signed out', async () => {
    // No my_* keys at all, which is what the API sends an anonymous caller.
    mocks.get.mockResolvedValue(contestWith([
        { problem_id: 'p4', index: 'D', title: 'Delta', score: 100, solved_count: 7 },
    ]))

    renderAt('#problems')

    expect(await screen.findByText('Delta')).toBeInTheDocument()
    expect(screen.getByText('7')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByLabelText('Solved')).not.toBeInTheDocument())
})
