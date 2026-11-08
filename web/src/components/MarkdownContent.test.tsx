import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import MarkdownContent from './MarkdownContent'

test('renders Markdown and inline/display TeX as KaTeX', () => {
    const { container } = render(
        <MarkdownContent content={'## Approach\n\nTakes $O(n \\log n)$ time.\n\n$$\n\\sum_{i=1}^{n} i = \\frac{n(n+1)}{2}\n$$'}
        />
    )

    expect(screen.getByRole('heading', { name: 'Approach' })).toBeInTheDocument()
    expect(container.querySelectorAll('.katex').length).toBeGreaterThanOrEqual(2)
    expect(container.querySelector('.katex-display')).not.toBeNull()
    expect(container.querySelector('script')).toBeNull()
})

test('renders nothing for empty content', () => {
    const { container } = render(<MarkdownContent content="" />)
    expect(container).toBeEmptyDOMElement()
})

test('inline mode drops the wrapping paragraph so it can sit in a line of text', () => {
    const { container } = render(<MarkdownContent inline content={'Memory: $O(1)$'} />)

    expect(container.querySelector('p')).toBeNull()
    expect(container.querySelector('.katex')).not.toBeNull()
    expect(container.textContent).toContain('Memory:')
})

// Constraints are stored one bound per line. Markdown folds a lone newline into
// a space, so a two-bound block used to render as one long line.
test('renders a lone newline as a line break', () => {
    const { container } = render(
        <MarkdownContent content={'$1 \\le n \\le 10$\n$1 \\le q \\le 10$'} />
    )

    expect(container.querySelectorAll('br')).toHaveLength(1)
    expect(container.querySelectorAll('.katex')).toHaveLength(2)
})

test('keeps paragraph breaks distinct from line breaks', () => {
    render(<MarkdownContent content={'first paragraph\nsecond line\n\nsecond paragraph'} />)

    expect(screen.getByText(/second line/)).toBeInTheDocument()
    expect(screen.getByText('second paragraph')).toBeInTheDocument()
})

test('leaves code block whitespace alone', () => {
    const { container } = render(<MarkdownContent content={'```\n1 2\n3 4\n```'} />)

    expect(container.querySelector('br')).toBeNull()
    expect(container.querySelector('code')?.textContent).toBe('1 2\n3 4\n')
})
