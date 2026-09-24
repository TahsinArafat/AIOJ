import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, expect, test, vi } from 'vitest'
import { ChallengesTab } from './ContestManage'
import { ToastProvider } from '../components/Toast'
import { ConfirmProvider } from '../components/ConfirmDialog'

/**
 * Focused coverage for the Challenges tab: reorder affordance/feedback and
 * the actionable empty state. ChallengesTab is exported solely for these
 * tests; routing-level behavior stays with the default ContestManage export.
 */

const mocks = vi.hoisted(() => ({
    getContest: vi.fn(),
    updateProblem: vi.fn(),
    addProblem: vi.fn(),
    removeProblem: vi.fn(),
    listProblems: vi.fn(),
}))

vi.mock('../lib/api', () => ({
    api: {
        contests: {
            get: mocks.getContest,
            updateProblem: mocks.updateProblem,
            addProblem: mocks.addProblem,
            removeProblem: mocks.removeProblem,
        },
        problems: { list: mocks.listProblems },
    },
    getAccessToken: vi.fn().mockReturnValue('mock-token'),
}))

const twoProblems = [
    { problem_id: 'p1', title: 'Alpha', slug: 'alpha', score: 100 },
    { problem_id: 'p2', title: 'Beta', slug: 'beta', score: 100 },
]

beforeEach(() => {
    vi.clearAllMocks()
    mocks.getContest.mockResolvedValue({ problems: twoProblems })
    mocks.updateProblem.mockResolvedValue({})
})

function renderChallenges() {
    return render(
        <MemoryRouter initialEntries={['/setter/contest/c1/manage']}>
            <ToastProvider>
                <ConfirmProvider>
                    <ChallengesTab contestId="c1" />
                </ConfirmProvider>
            </ToastProvider>
        </MemoryRouter>
    )
}

/** First data row of the problem table (row 0 is the header). */
function firstRowText(): string {
    const rows = screen.getAllByRole('row')
    return rows[1].textContent || ''
}

test('reorder controls expose accessible labels and disable at the edges', async () => {
    renderChallenges()
    await screen.findByRole('link', { name: 'Alpha' })

    expect(screen.getByRole('group', { name: 'Reorder Alpha' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Move Alpha up' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move Alpha down' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Move Beta up' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Move Beta down' })).toBeDisabled()
})

test('moving a problem up persists new indexes and confirms success', async () => {
    renderChallenges()
    await screen.findByRole('link', { name: 'Alpha' })

    fireEvent.click(screen.getByRole('button', { name: 'Move Beta up' }))

    await waitFor(() => {
        expect(mocks.updateProblem).toHaveBeenCalledTimes(2)
    })
    expect(mocks.updateProblem).toHaveBeenCalledWith('c1', 'p1', {
        index: 'B', score: 100, sort_order: 1,
    })
    expect(mocks.updateProblem).toHaveBeenCalledWith('c1', 'p2', {
        index: 'A', score: 100, sort_order: 0,
    })

    await screen.findByText('Challenge order updated')
    expect(firstRowText()).toContain('Beta')
})

test('reorder buttons lock while a reorder is in flight', async () => {
    // One shared gate: both parallel updateProblem calls must settle together.
    let finish!: () => void
    const gate = new Promise<void>(resolve => { finish = resolve })
    mocks.updateProblem.mockImplementation(() => gate)
    renderChallenges()
    await screen.findByRole('link', { name: 'Alpha' })

    fireEvent.click(screen.getByRole('button', { name: 'Move Beta up' }))

    await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Move Alpha up' })).toBeDisabled()
    })
    expect(screen.getByRole('button', { name: 'Move Beta up' })).toHaveAttribute('aria-busy', 'true')

    finish()
    await screen.findByText('Challenge order updated')
    // Beta moved to the first row: its up edge is now locked, Alpha's down edge too.
    expect(screen.getByRole('button', { name: 'Move Beta up' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move Beta up' })).not.toHaveAttribute('aria-busy', 'true')
    expect(screen.getByRole('button', { name: 'Move Alpha down' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move Alpha up' })).toBeEnabled()
})

test('a failed reorder reports the error and re-syncs server order', async () => {
    mocks.updateProblem.mockRejectedValue(new Error('boom'))
    renderChallenges()
    await screen.findByRole('link', { name: 'Alpha' })

    fireEvent.click(screen.getByRole('button', { name: 'Move Beta up' }))

    await screen.findByRole('alert')
    expect(screen.getByRole('alert')).toHaveTextContent(
        'Failed to update challenge order: boom'
    )
    await waitFor(() => {
        expect(mocks.getContest).toHaveBeenCalledTimes(2) // initial load + reconciliation
    })
    expect(firstRowText()).toContain('Alpha')
})

test('empty challenge list points at the Add Problem flow', async () => {
    mocks.getContest.mockResolvedValue({ problems: [] })
    renderChallenges()

    expect(await screen.findByText('No problems added yet')).toBeInTheDocument()
    expect(
        screen.getByText('Use Add Problem to build the contest challenge list.')
    ).toBeInTheDocument()
})
