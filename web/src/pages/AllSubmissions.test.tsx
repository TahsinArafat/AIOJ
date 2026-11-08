import { render, screen, waitFor, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import AllSubmissions from './AllSubmissions'
import { api } from '../lib/api'
import type { Submission } from '../types/api'

vi.mock('../lib/api', () => ({
    api: {
        submissions: { listAll: vi.fn() },
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
        status: 'ac',
        score: 100,
        time_used: 12,
        memory_used: 2048,
        judged_by: 'judge',
        is_remote: false,
        problem_slug: 'two-sum',
        problem_title: 'Two Sum',
        created_at: '2026-01-01T10:00:00Z',
        ...overrides,
    }
}

function renderPage() {
    return render(
        <MemoryRouter>
            <AllSubmissions />
        </MemoryRouter>
    )
}

beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.submissions.listAll).mockResolvedValue({ data: [], total: 0 })
})

test('renders submissions with problem and submitter', async () => {
    vi.mocked(api.submissions.listAll).mockResolvedValue({ data: [row()], total: 1 })
    renderPage()

    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    expect(screen.getByText('Two Sum')).toBeInTheDocument()
    // Scoped to the row: "Accepted" also appears as a verdict filter option.
    const bodyRow = screen.getAllByRole('row')[1]
    expect(within(bodyRow).getByText('Accepted')).toBeInTheDocument()
})

// The submission API only returns source to the owner, admins and the
// problem's setters — a row the viewer cannot open must not be a link.
test('links a row only when the viewer can open it', async () => {
    vi.mocked(api.submissions.listAll).mockResolvedValue({
        data: [
            row({ id: '11111111-1111-4111-8111-111111111111', can_view: true }),
            row({ id: '22222222-2222-4222-8222-222222222222', can_view: false }),
        ],
        total: 2,
    })
    renderPage()

    await waitFor(() => expect(screen.getByText('11111111')).toBeInTheDocument())
    expect(screen.getByText('11111111').closest('a')).toHaveAttribute('href', '/submissions/11111111-1111-4111-8111-111111111111')
    expect(screen.getByText('22222222').closest('a')).toBeNull()
})

test('applies handle, problem and verdict filters', async () => {
    renderPage()
    await waitFor(() => expect(api.submissions.listAll).toHaveBeenCalled())

    fireEvent.change(screen.getByLabelText('Handle'), { target: { value: 'bob' } })
    fireEvent.change(screen.getByLabelText('Problem'), { target: { value: 'two-sum' } })
    fireEvent.change(screen.getByLabelText('Verdict'), { target: { value: 'wa' } })
    fireEvent.click(screen.getByRole('button', { name: 'Filter' }))

    await waitFor(() => expect(api.submissions.listAll).toHaveBeenLastCalledWith(
        expect.objectContaining({ handle: 'bob', problem: 'two-sum', status: 'wa', offset: 0 }),
    ))
})

test('reset clears the filters', async () => {
    renderPage()
    await waitFor(() => expect(api.submissions.listAll).toHaveBeenCalled())

    fireEvent.change(screen.getByLabelText('Handle'), { target: { value: 'bob' } })
    fireEvent.click(screen.getByRole('button', { name: 'Filter' }))
    await waitFor(() => expect(api.submissions.listAll).toHaveBeenLastCalledWith(
        expect.objectContaining({ handle: 'bob' }),
    ))

    fireEvent.click(screen.getByRole('button', { name: 'Reset' }))
    await waitFor(() => expect(api.submissions.listAll).toHaveBeenLastCalledWith(
        expect.objectContaining({ handle: undefined }),
    ))
})

test('paginates through results', async () => {
    vi.mocked(api.submissions.listAll).mockResolvedValue({ data: [row()], total: 120 })
    renderPage()

    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))

    await waitFor(() => expect(api.submissions.listAll).toHaveBeenLastCalledWith(
        expect.objectContaining({ offset: 50 }),
    ))
})
