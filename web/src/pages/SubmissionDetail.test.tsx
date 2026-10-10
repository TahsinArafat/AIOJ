import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import SubmissionDetail from './SubmissionDetail'
import { ToastProvider } from '../components/Toast'
import { api } from '../lib/api'

vi.mock('../lib/api', async (importOriginal) => {
    const actual = await importOriginal<typeof import('../lib/api')>()
    return {
        ...actual,
        api: {
            ...actual.api,
            submissions: {
                ...actual.api.submissions,
                get: vi.fn(),
                retryRemote: vi.fn(),
                recheckRemote: vi.fn(),
            },
        },
    }
})

vi.mock('../lib/problemSlugResolver', () => ({
    resolveProblemSlug: vi.fn().mockResolvedValue('two-sum'),
    resolveProblemTitle: vi.fn().mockResolvedValue('Two Sum'),
}))

function renderPage() {
    return render(
        <MemoryRouter initialEntries={['/submissions/sub-1']}>
            <ToastProvider>
                <Routes>
                    <Route path="/submissions/:id" element={<SubmissionDetail />} />
                </Routes>
            </ToastProvider>
        </MemoryRouter>
    )
}

async function openTestsTab() {
    await waitFor(() => expect(screen.getByText(/Submission sub-1/)).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /^tests/ }))
}

function baseSubmission(overrides: Record<string, unknown> = {}) {
    return {
        id: 'sub-1',
        problem_id: 'prob-1',
        user_id: 'u1',
        username: 'alice',
        language: 'cpp-gpp-64',
        status: 'ac',
        score: 100,
        time_used: 12,
        memory_used: 2048,
        code_size: 120,
        created_at: new Date().toISOString(),
        judge_result: [],
        ...overrides,
    }
}

beforeEach(() => {
    vi.mocked(api.submissions.get).mockReset()
})

test('renders judge cases in the fallback (non-subtask) view', async () => {
    vi.mocked(api.submissions.get).mockResolvedValue(
        baseSubmission({
            judge_result: [
                { case_name: 'small-1', status: 'ac', time: 5, memory: 1024, score: 50 },
                { case_name: 'big-1', status: 'wa', time: 9, memory: 2048, score: 0, detail: 'wrong answer on line 3' },
            ],
        }) as never
    )

    renderPage()
    await openTestsTab()

    expect(screen.getByText('Test Case 1: small-1')).toBeInTheDocument()
    expect(screen.getByText('Test Case 2: big-1')).toBeInTheDocument()
    expect(screen.getByText('wrong answer on line 3')).toBeInTheDocument()
})

test('groups cases by subtask when subtask ids are present', async () => {
    vi.mocked(api.submissions.get).mockResolvedValue(
        baseSubmission({
            judge_result: [
                { case_name: 's1-1', status: 'ac', time: 1, memory: 512, score: 30, subtask_id: 1 },
                { case_name: 's1-2', status: 'ac', time: 2, memory: 512, score: 30, subtask_id: 1 },
                { case_name: 's2-1', status: 'wa', time: 3, memory: 512, score: 0, subtask_id: 2 },
            ],
        }) as never
    )

    renderPage()
    await openTestsTab()

    expect(screen.getByText('Subtask 1')).toBeInTheDocument()
    expect(screen.getByText('Subtask 2')).toBeInTheDocument()
    expect(screen.getByText('s1-1')).toBeInTheDocument()
    expect(screen.getByText('s2-1')).toBeInTheDocument()
    expect(screen.queryByText(/^Test Case 1:/)).not.toBeInTheDocument()
})

test('shows not-found state when the submission is missing', async () => {
    vi.mocked(api.submissions.get).mockRejectedValue(new Error('404'))

    renderPage()

    await waitFor(() => expect(screen.getByText('Submission not found.')).toBeInTheDocument())
})
