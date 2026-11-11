import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ContestCountdown, ContestRunningClock } from './ContestClock'

/**
 * The contest clock.
 *
 * The old rendering was a full-width saturated gradient (`from-emerald-600 to
 * teal-600`, `text-white`, `text-3xl`, pulsing dot) that ignored the design
 * tokens entirely, so it was the one surface on the page that was wrong in dark
 * mode. It also had no urgency state: a round two minutes from ending looked
 * identical to one with three hours left.
 *
 * These tests pin the behaviour that matters: the urgency class changes, the
 * clock disappears when it has nothing left to say, and the markup carries the
 * tokens rather than a hardcoded gradient.
 */

const HOUR = 3_600_000

function at(offsetMs: number): string {
    return new Date(Date.now() + offsetMs).toISOString()
}

beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-01T12:00:00Z'))
})

afterEach(() => {
    vi.useRealTimers()
})

describe('ContestCountdown', () => {
    it('renders the time remaining until the round starts', () => {
        render(<ContestCountdown target={at(2 * HOUR)} label="Starts in" />)

        expect(screen.getByText('Starts in')).toBeInTheDocument()
        expect(screen.getByText(/^02:00:00$/)).toBeInTheDocument()
    })

    it('renders nothing once the round has started', () => {
        // The running clock takes over; a zeroed countdown beside a live contest
        // says nothing useful.
        const { container } = render(<ContestCountdown target={at(-1000)} />)
        expect(container).toBeEmptyDOMElement()
    })

    it('turns urgent as the start approaches', () => {
        const { container, rerender } = render(<ContestCountdown target={at(2 * HOUR)} />)
        expect(container.innerHTML).not.toContain('border-warn')

        rerender(<ContestCountdown target={at(10 * 60_000)} />)
        expect(container.innerHTML).toContain('border-warn')

        rerender(<ContestCountdown target={at(2 * 60_000)} />)
        expect(container.innerHTML).toContain('border-bad')
    })

    it('uses design tokens rather than a hardcoded gradient', () => {
        const { container } = render(<ContestCountdown target={at(2 * HOUR)} />)
        const html = container.innerHTML
        // The old version was bg-gradient-to-r from-blue-600 to-indigo-600 with
        // text-white, which does not adapt to the theme.
        expect(html).not.toContain('from-blue-600')
        expect(html).not.toContain('to-indigo-600')
        expect(html).toContain('bg-surface')
    })
})

describe('ContestRunningClock', () => {
    it('shows the time left and how far through the round it is', () => {
        render(<ContestRunningClock start={at(-HOUR)} end={at(HOUR)} />)

        expect(screen.getByText('Running')).toBeInTheDocument()
        expect(screen.getByText(/00:59:59|01:00:00/)).toBeInTheDocument()
        expect(screen.getByText('Elapsed 1h 0m')).toBeInTheDocument()
        // Halfway through a two-hour round.
        expect(screen.getByText('50%')).toBeInTheDocument()
        expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50')
    })

    it('renders nothing once the round has ended', () => {
        const { container } = render(<ContestRunningClock start={at(-2 * HOUR)} end={at(-1000)} />)
        expect(container).toBeEmptyDOMElement()
    })

    it('turns urgent inside the last 5 minutes', () => {
        const { container, rerender } = render(<ContestRunningClock start={at(-HOUR)} end={at(HOUR)} />)
        expect(container.innerHTML).not.toContain('border-bad')

        rerender(<ContestRunningClock start={at(-119 * 60_000)} end={at(60_000)} />)
        const html = container.innerHTML
        expect(html).toContain('border-bad')
        expect(html).toContain('bg-bad')
    })

    it('uses design tokens rather than a hardcoded gradient', () => {
        const { container } = render(<ContestRunningClock start={at(-HOUR)} end={at(HOUR)} />)
        const html = container.innerHTML
        // The old version was a saturated emerald gradient that ignored the theme.
        expect(html).not.toContain('from-emerald-600')
        expect(html).not.toContain('to-teal-600')
        expect(html).toContain('bg-surface')
    })

    it('ticks down once a second', async () => {
        render(<ContestRunningClock start={at(-HOUR)} end={at(HOUR)} />)
        // The deferred resync lands before the first interval tick.
        await act(async () => { await Promise.resolve() })
        expect(screen.getByText(/^01:00:00/)).toBeInTheDocument()

        // Two seconds on, exactly two seconds gone — not a jump, not a stall.
        act(() => { vi.advanceTimersByTime(2000) })
        expect(screen.getByText(/^00:59:58/)).toBeInTheDocument()
    })
})