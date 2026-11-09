import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import { ProblemStatement } from './ProblemStatement'
import { ProblemMeta } from './ProblemMeta'

/**
 * A problem that is still live in a contest shows its statement but not the
 * metadata that gives the intended solution away. The server strips those fields
 * already; these tests pin the second half of the rule, so a client that receives
 * them anyway still refuses to render them.
 */

const liveContestProblem = {
    description: 'Find the number of inversions.',
    input_format: 'The first line contains $n$.',
    output_format: 'Print the answer.',
    constraints: '$1 \\le n \\le 2\\cdot10^5$',
    hint: 'use a Fenwick tree',
    sample_cases: [{ input: '3', output: '0', explanation: null }],
}

test('hides the hint while the problem is live in a contest', () => {
    render(<ProblemStatement problem={liveContestProblem} hideSpoilers />)

    expect(screen.queryByText(/Fenwick/i)).not.toBeInTheDocument()
    expect(screen.queryByText('Hint')).not.toBeInTheDocument()
})

test('shows the hint once the contest is over', () => {
    render(<ProblemStatement problem={liveContestProblem} />)

    // Practice pages render it as a plain Notes section; contests fold it away.
    expect(screen.getByRole('heading', { name: 'Notes' })).toBeInTheDocument()
    expect(screen.getByText(/Fenwick/i)).toBeInTheDocument()
})

test('the contest variant folds the hint behind a toggle', () => {
    render(<ProblemStatement problem={liveContestProblem} collapsibleHint />)

    expect(screen.getByText('Hint')).toBeInTheDocument()
    expect(screen.queryByText(/Fenwick/i)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Hint/ })).toHaveAttribute('aria-expanded', 'false')
})

test('keeps the statement, its sections and the samples visible', () => {
    render(<ProblemStatement problem={liveContestProblem} hideSpoilers />)

    expect(screen.getByText('Find the number of inversions.')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Input format' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Constraints' })).toBeInTheDocument()
    expect(screen.getByText('Sample 1')).toBeInTheDocument()
})

test('can hide the prose but keep the samples', () => {
    render(<ProblemStatement problem={liveContestProblem} showStatement={false} />)

    expect(screen.queryByText('Find the number of inversions.')).not.toBeInTheDocument()
    expect(screen.getByText('Sample 1')).toBeInTheDocument()
})

test('meta strip drops difficulty, rating and tags while live', () => {
    render(
        <ProblemMeta
            timeLimit={2000}
            memoryLimit={262144}
            difficulty="hard"
            rating={2400}
            tags={['dp', 'fenwick-tree']}
            hideSpoilers
        />
    )

    expect(screen.queryByText('hard')).not.toBeInTheDocument()
    expect(screen.queryByText('Rating')).not.toBeInTheDocument()
    expect(screen.queryByText('Tags')).not.toBeInTheDocument()
    // Limits are not a spoiler and stay visible.
    expect(screen.getByText('2.0 s')).toBeInTheDocument()
    expect(screen.getByText('256 MB')).toBeInTheDocument()
})

test('meta strip shows them again once the contest is over', () => {
    render(<ProblemMeta timeLimit={2000} memoryLimit={262144} difficulty="hard" rating={2400} tags={['dp']} />)

    expect(screen.getByText('hard')).toBeInTheDocument()
    expect(screen.getByText('2400')).toBeInTheDocument()
    expect(screen.getByText('dp')).toBeInTheDocument()
})
