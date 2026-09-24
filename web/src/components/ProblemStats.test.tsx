import { render, screen } from '@testing-library/react'
import { expect, test, vi, beforeEach } from 'vitest'
import ProblemStats from './ProblemStats'
import { api } from '../lib/api'
import type { Mock } from 'vitest'

vi.mock('../lib/api', () => ({
    api: {
        stats: {
            getProblemStats: vi.fn(),
        },
    },
}))

beforeEach(() => {
    vi.clearAllMocks()
})

test('shows an EmptyState when statistics fail to load', async () => {
    ; (api.stats.getProblemStats as Mock).mockRejectedValue(new Error('boom'))
    render(<ProblemStats problemId="p1" />)

    expect(await screen.findByText('No statistics available')).toBeInTheDocument()
    expect(screen.getByText(/Submission stats will appear/)).toBeInTheDocument()
})

test('shows the language EmptyState when there are no submissions', async () => {
    ; (api.stats.getProblemStats as Mock).mockResolvedValue({
        total_submissions: 0,
        accepted_submissions: 0,
        acceptance_rate: 0,
        average_attempts: 0,
        language_distribution: {},
    })
    render(<ProblemStats problemId="p1" />)

    expect(await screen.findByText('No language data yet')).toBeInTheDocument()
    expect(api.stats.getProblemStats).toHaveBeenCalledWith('p1')
})

test('renders stat tiles and the language breakdown when data exists', async () => {
    ; (api.stats.getProblemStats as Mock).mockResolvedValue({
        total_submissions: 10,
        accepted_submissions: 5,
        acceptance_rate: 50,
        average_attempts: 2,
        language_distribution: { 'cpp-gpp-64': 6, 'python-3-8': 4 },
    })
    render(<ProblemStats problemId="p1" />)

    expect(await screen.findByText('Languages Used')).toBeInTheDocument()
    expect(screen.getByText('10')).toBeInTheDocument()
    expect(screen.getByText(/6 \(60%\)/)).toBeInTheDocument()
    expect(screen.getByText(/4 \(40%\)/)).toBeInTheDocument()
})
