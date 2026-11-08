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
