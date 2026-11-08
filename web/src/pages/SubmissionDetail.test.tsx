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
    // The header splits the label and the id into separate nodes for layout, so
    // assert on the heading role rather than a single concatenated string.
    await waitFor(() => expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Submission'))
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('sub-1')
    // Tabs are real tabs now (`role="tab"`), not unlabelled buttons.
    fireEvent.click(screen.getByRole('tab', { name: /^Test cases/ }))
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

    expect(screen.getByText('small-1')).toBeInTheDocument()
    expect(screen.getByText('big-1')).toBeInTheDocument()
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

// A contestant who reports "judge system error" has nothing to act on; the
// judge's own failure text is what explains it, and only privileged viewers
// receive it (the API strips it for the submitter).
test('shows the judge error text when the API provides it', async () => {
    vi.mocked(api.submissions.get).mockResolvedValue(
        baseSubmission({
            status: 'se',
            judge_error: 'execution client error: dial tcp judge:5050: connection refused',
            judged_at: new Date().toISOString(),
        }) as never
    )

    renderPage()

    await waitFor(() => expect(screen.getByText('Judge error')).toBeInTheDocument())
    expect(screen.getByText(/dial tcp judge:5050/)).toBeInTheDocument()
    // It must not be presented as the contestant's compile failure.
    expect(screen.queryByText('Compile error')).not.toBeInTheDocument()
})

test('a submitter without judge access sees the generic note only', async () => {
    vi.mocked(api.submissions.get).mockResolvedValue(baseSubmission({ status: 'se' }) as never)

    renderPage()

    await waitFor(() => expect(screen.getByText(/A system error means the judge itself failed/)).toBeInTheDocument())
    expect(screen.queryByText('Judge error')).not.toBeInTheDocument()
    expect(screen.getByText(/setters and site admins can see the failure details/)).toBeInTheDocument()
})
