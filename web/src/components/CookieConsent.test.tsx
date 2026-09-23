import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import CookieConsent from './CookieConsent'

describe('CookieConsent', () => {
    beforeEach(() => localStorage.clear())

    it('shows the banner when no choice is stored', () => {
        render(<CookieConsent />)
        expect(screen.getByText(/cookies/i)).toBeInTheDocument()
    })

    it('hides after accepting', () => {
        render(<CookieConsent />)
        fireEvent.click(screen.getByRole('button', { name: /accept/i }))
        expect(screen.queryByText(/cookies/i)).not.toBeInTheDocument()
        expect(localStorage.getItem('cookie-consent')).toBe('accepted')
    })

    it('hides after declining', () => {
        render(<CookieConsent />)
        fireEvent.click(screen.getByRole('button', { name: /decline/i }))
        expect(screen.queryByText(/cookies/i)).not.toBeInTheDocument()
        expect(localStorage.getItem('cookie-consent')).toBe('declined')
    })

    it('does not show when a choice already exists', () => {
        localStorage.setItem('cookie-consent', 'accepted')
        render(<CookieConsent />)
        expect(screen.queryByText(/cookies/i)).not.toBeInTheDocument()
    })
})
