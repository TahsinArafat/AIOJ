import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { expect, test, vi } from 'vitest'
import ProblemDetail from './ProblemDetail'
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
            })
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
            })
        }
    },
    getAccessToken: vi.fn().mockReturnValue('mock-token')
}))

test('renders submissions tab and handles list load clicks', async () => {
    render(
        <MemoryRouter initialEntries={['/problems/two-sum']}>
            <Routes>
                <Route path="/problems/:slug" element={<ProblemDetail />} />
            </Routes>
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
