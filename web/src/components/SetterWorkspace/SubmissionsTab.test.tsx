import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import SubmissionsTab from './SubmissionsTab'
import { ConfirmProvider } from '../ConfirmDialog'
import { ToastProvider } from '../Toast'
import { api } from '../../lib/api'
import type { Submission } from '../../types/api'

vi.mock('../../lib/api', () => ({
    api: {
        problems: {
            submissions: vi.fn(),
            rejudgeSubmissions: vi.fn(),
        },
    },
    getAccessToken: vi.fn(() => 'mock-token'),
}))

function row(overrides: Partial<Submission> = {}): Submission {
    return {
        id: 'aaaaaaaa-1111-4111-8111-111111111111',
        problem_id: 'p1',
        user_id: 'u1',
        username: 'alice',
        language: 'cpp-gpp-64',
        submission_type: 'code',
        code_size: 100,
        status: 'wa',
        score: 0,
        time_used: 20,
        memory_used: 1024,
        judged_by: 'judge',
        is_remote: false,
        can_view: true,
        can_rejudge: true,
        created_at: '2026-01-01T10:00:00Z',
        ...overrides,
    }
}

function renderTab() {
    return render(
        <MemoryRouter>
            <ToastProvider>
                <ConfirmProvider>
                    <SubmissionsTab problemSlug="two-sum" />
                </ConfirmProvider>
            </ToastProvider>
        </MemoryRouter>
    )
}

beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.problems.submissions).mockResolvedValue({ data: [], total: 0 })
    vi.mocked(api.problems.rejudgeSubmissions).mockResolvedValue({ enqueued: 1, total: 1 })
})

test('lists the problem submissions with the submitter', async () => {
    vi.mocked(api.problems.submissions).mockResolvedValue({ data: [row()], total: 1 })
    renderTab()

    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    expect(api.problems.submissions).toHaveBeenCalledWith('two-sum', 0, 50, {
        handle: undefined, status: undefined, language: undefined,
    })
})

test('rejudges a single submission after confirmation', async () => {
    vi.mocked(api.problems.submissions).mockResolvedValue({ data: [row()], total: 1 })
    renderTab()

    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Rejudge aaaaaaaa/ }))

    const confirmButton = await screen.findByRole('button', { name: 'Rejudge' })
    fireEvent.click(confirmButton)

    await waitFor(() => expect(api.problems.rejudgeSubmissions).toHaveBeenCalledWith('two-sum', {
        submission_id: 'aaaaaaaa-1111-4111-8111-111111111111',
    }))
})

test('rejudge all covers the whole problem', async () => {
    vi.mocked(api.problems.submissions).mockResolvedValue({ data: [row()], total: 1 })
    renderTab()

    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Rejudge all' }))

    // The dialog's confirm button shares its label with the page button.
    const buttons = await screen.findAllByRole('button', { name: 'Rejudge all' })
    fireEvent.click(buttons[buttons.length - 1])

    await waitFor(() => expect(api.problems.rejudgeSubmissions).toHaveBeenCalledWith('two-sum', {}))
})

test('filters by handle and verdict', async () => {
    renderTab()
    await waitFor(() => expect(api.problems.submissions).toHaveBeenCalled())

    fireEvent.change(screen.getByLabelText('Handle'), { target: { value: 'bob' } })
    fireEvent.change(screen.getByLabelText('Verdict'), { target: { value: 'ce' } })
    fireEvent.click(screen.getByRole('button', { name: 'Filter' }))

    await waitFor(() => expect(api.problems.submissions).toHaveBeenLastCalledWith(
        'two-sum', 0, 50, expect.objectContaining({ handle: 'bob', status: 'ce' }),
    ))
})

test('a pending row cannot be rejudged again', async () => {
    vi.mocked(api.problems.submissions).mockResolvedValue({ data: [row({ status: 'judging' })], total: 1 })
    renderTab()

    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: /Rejudge aaaaaaaa/ })).toBeDisabled()
})

// A setter scanning the list should see why a submission failed without
// opening each one.
test('renders the judge failure text inline for a system error', async () => {
    vi.mocked(api.problems.submissions).mockResolvedValue({
        data: [row({ status: 'se', judge_error: 'go-judge returned 500: sandbox unavailable' })],
        total: 1,
    })
    renderTab()

    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    expect(screen.getByText('go-judge returned 500: sandbox unavailable')).toBeInTheDocument()
})

test('renders no failure block for a normal verdict', async () => {
    vi.mocked(api.problems.submissions).mockResolvedValue({ data: [row()], total: 1 })
    renderTab()

    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    expect(document.querySelectorAll('tbody tr')).toHaveLength(1)
})
