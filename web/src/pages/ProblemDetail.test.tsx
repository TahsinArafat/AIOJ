import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { expect, test, vi } from 'vitest'
import ProblemDetail from './ProblemDetail'
import { ToastProvider } from '../components/Toast'
import { ConfirmProvider } from '../components/ConfirmDialog'
import { api } from '../lib/api'

vi.mock('../lib/api', () => ({
    api: {
        problems: {
            get: vi.fn().mockResolvedValue({
                id: 'prob-1',
                slug: 'two-sum',
                title: 'Two Sum',
                description: 'Solve the sum problem.',
                time_limit: 1000,
                memory_limit: 262144,
                difficulty: 'easy',
                sample_cases: []
            }),
            listI18n: vi.fn().mockResolvedValue([]),
        },
        editorials: {
            getByProblem: vi.fn().mockResolvedValue({ data: [] })
        },
        submissions: {
            list: vi.fn().mockResolvedValue({
                data: [
                    { id: 'sub-1', language: 'cpp-gpp-64', status: 'ac', time_used: 12, memory_used: 2048 }
                ],
                total: 1
            }),
            create: vi.fn(),
            get: vi.fn(),
        },
        remoteLanguages: {
            list: vi.fn().mockResolvedValue({ data: [] })
        }
    },
    getAccessToken: vi.fn().mockReturnValue('mock-token')
}))

test('renders submissions tab and handles list load clicks', async () => {
    render(
        <MemoryRouter initialEntries={['/problems/two-sum']}>
            <ToastProvider>
                <ConfirmProvider>
                    <Routes>
                        <Route path="/problems/:slug" element={<ProblemDetail />} />
                    </Routes>
                </ConfirmProvider>
            </ToastProvider>
        </MemoryRouter>
    )

    await waitFor(() => {
        expect(screen.getByText('Two Sum')).toBeInTheDocument()
    })

    const subTab = screen.getByText('My Submissions')
    expect(subTab).toBeInTheDocument()
    fireEvent.click(subTab)

    await waitFor(() => {
        expect(api.submissions.list).toHaveBeenCalled()
    })
})

test('shows the submission id and live verdict immediately after submit', async () => {
    const submissionId = 'abcdef12-3456-7890-abcd-ef1234567890'
    vi.mocked(api.submissions.create).mockResolvedValue({
        id: submissionId,
        status: 'pending',
        language: 'cpp-gpp-64',
        time_used: 0,
        memory_used: 0,
    } as never)
    // The poller kicks in two seconds later; keep it harmless so the test does
    // not leave a rejected promise behind.
    vi.mocked(api.submissions.get).mockResolvedValue({
        id: submissionId,
        status: 'ac',
        language: 'cpp-gpp-64',
        time_used: 12,
        memory_used: 2048,
    } as never)

    render(
        <MemoryRouter initialEntries={['/problems/two-sum']}>
            <ToastProvider>
                <ConfirmProvider>
                    <Routes>
                        <Route path="/problems/:slug" element={<ProblemDetail />} />
                    </Routes>
                </ConfirmProvider>
            </ToastProvider>
        </MemoryRouter>
    )

    await waitFor(() => {
        expect(screen.getByText('Two Sum')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: 'Submit' }))

    // The receipt lands before the first poll: the id and the pending verdict.
    const link = await screen.findByRole('link', { name: '#abcdef12' })
    expect(link).toHaveAttribute('href', `/submissions/${submissionId}`)
    expect(screen.getByText('Pending')).toBeInTheDocument()
    expect(screen.getByText(/updates automatically/)).toBeInTheDocument()
})
