import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import { ProblemMeta } from './ProblemMeta'
import { StatementSection } from './StatementSection'

test('a statement section renders its heading and math body', () => {
    const { container } = render(
        <StatementSection title="Constraints" body={'$1 \\le n \\le 10$\n$1 \\le q \\le 10$'} />
    )

    expect(screen.getByRole('heading', { name: 'Constraints' })).toBeInTheDocument()
    // Two bounds, each its own KaTeX run, separated by a line break.
    expect(container.querySelectorAll('.katex')).toHaveLength(2)
    expect(container.querySelectorAll('br')).toHaveLength(1)
})

test('limits are shown in seconds and megabytes', () => {
    render(<ProblemMeta timeLimit={1500} memoryLimit={262144} />)

    expect(screen.getByText('1.5 s')).toBeInTheDocument()
    expect(screen.getByText('256 MB')).toBeInTheDocument()
})

test('an unrated problem shows no rating chip', () => {
    render(<ProblemMeta timeLimit={1000} memoryLimit={262144} rating={0} />)
    expect(screen.queryByText('Rating')).not.toBeInTheDocument()
})

test('a rated problem shows its rating and tags', () => {
    render(<ProblemMeta timeLimit={1000} memoryLimit={262144} rating={1500} tags={['dp', 'math']} difficulty="hard" />)

    expect(screen.getByText('Rating')).toBeInTheDocument()
    expect(screen.getByText('1500')).toBeInTheDocument()
    expect(screen.getByText('dp')).toBeInTheDocument()
    expect(screen.getByText('hard')).toBeInTheDocument()
})

test('a stack limit the language actually grants is shown', () => {
    render(<ProblemMeta timeLimit={1000} memoryLimit={262144} stackLimit={524288} />)

    const stack = screen.getByText('Stack').parentElement
    expect(stack).toHaveTextContent('512 MB')
})

test('the sandbox default stack is not advertised', () => {
    // Every language without stack_limit_kb runs under the same 8 MB default,
    // which is not a promise the platform makes — naming it would be noise.
    render(<ProblemMeta timeLimit={1000} memoryLimit={262144} stackLimit={0} />)

    expect(screen.queryByText('Stack')).not.toBeInTheDocument()
})
