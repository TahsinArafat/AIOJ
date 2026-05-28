import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test } from 'vitest'
import Navbar from './Navbar'

test('renders brand logo and primary navigation links', () => {
    render(
        <MemoryRouter>
            <Navbar />
        </MemoryRouter>
    )

    // Verify brand link exists
    const brandElement = screen.getByText('AIOJ')
    expect(brandElement).toBeInTheDocument()
    expect(brandElement.getAttribute('href')).toBe('/')

    // Verify key navigation paths exist
    expect(screen.getByText('Problems')).toBeInTheDocument()
    expect(screen.getByText('Contests')).toBeInTheDocument()
    expect(screen.getByText('Practice')).toBeInTheDocument()
})
