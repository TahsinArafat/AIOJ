import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import CountryFlag, { flagEmoji } from './CountryFlag'

describe('flagEmoji', () => {
  it('maps common country names to regional indicators', () => {
    expect(flagEmoji('Bangladesh')).toBe('\u{1F1E7}\u{1F1E9}')
    expect(flagEmoji('United States')).toBe('\u{1F1FA}\u{1F1F8}')
    expect(flagEmoji('bd')).toBe('\u{1F1E7}\u{1F1E9}')
  })

  it('returns null for empty or unknown', () => {
    expect(flagEmoji('')).toBeNull()
    expect(flagEmoji('Narnia')).toBeNull()
  })
})

describe('CountryFlag', () => {
  it('renders nothing without country', () => {
    const { container } = render(<CountryFlag country="" />)
    expect(container.firstChild).toBeNull()
  })

  it('renders flag with name', () => {
    render(<CountryFlag country="Japan" withName />)
    expect(screen.getByTitle('Japan')).toBeInTheDocument()
    expect(screen.getByText('Japan')).toBeInTheDocument()
  })
})
