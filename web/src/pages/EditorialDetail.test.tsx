import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { expect, test, vi } from 'vitest'
import EditorialDetail from './EditorialDetail'
import { api } from '../lib/api'

vi.mock('../lib/api', () => ({
    api: {
        editorials: {
            get: vi.fn(),
        },
    },
    getAccessToken: vi.fn().mockReturnValue(null),
}))

vi.mock('../components/CommentSection', () => ({
    default: () => <div data-testid="comment-section" />,
}))

test('renders editorial body and approach with KaTeX math', async () => {
    vi.mocked(api.editorials.get).mockResolvedValue({
        id: 'e1',
        problem_id: 'p1',
        title: 'Logarithmic approach',
        username: 'alice',
        created_at: '2026-01-01T00:00:00Z',
        content: 'The loop runs $O(n \\log n)$.\n\n$$\n\\sum_{i=1}^{n} i\n$$',
        approach: 'Use $O(1)$ extra memory.',
        time_complexity: '$O(n \\log n)$',
        space_complexity: '$O(1)$',
    } as never)

    const { container } = render(
        <MemoryRouter initialEntries={['/editorials/e1']}>
            <Routes>
                <Route path="/editorials/:id" element={<EditorialDetail />} />
            </Routes>
        </MemoryRouter>
    )

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Logarithmic approach' })).toBeInTheDocument())

    expect(screen.getByTestId('comment-section')).toBeInTheDocument()
    expect(container.querySelector('.katex-display')).not.toBeNull()
    // Body math, approach math, time and space complexity.
    expect(container.querySelectorAll('.katex').length).toBeGreaterThanOrEqual(4)
})
