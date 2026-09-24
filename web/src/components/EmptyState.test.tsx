import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import { EmptyState } from './EmptyState'

test('renders the headline and actionable description', () => {
    render(<EmptyState text="No balloons yet" description="Balloons appear after accepted submissions." />)
    expect(screen.getByText('No balloons yet')).toBeInTheDocument()
    expect(screen.getByText('Balloons appear after accepted submissions.')).toBeInTheDocument()
})

test('omits the description paragraph when none is given', () => {
    const { container } = render(<EmptyState text="Nothing here" />)
    expect(screen.getByText('Nothing here')).toBeInTheDocument()
    expect(container.querySelectorAll('p')).toHaveLength(1)
})

test('renders the icon slot only when provided', () => {
    const { container, rerender } = render(
        <EmptyState icon={<svg data-testid="glyph" />} text="With icon" />
    )
    expect(screen.getByTestId('glyph')).toBeInTheDocument()
    rerender(<EmptyState text="Without icon" />)
    expect(screen.queryByTestId('glyph')).not.toBeInTheDocument()
    expect(container.querySelector('.text-4xl')).toBeNull()
})
