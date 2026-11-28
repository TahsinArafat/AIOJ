import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ContestCountdown, ContestRunningClock } from './ContestClock'
import { contestClockAt } from '../lib/contestTime'

/**
 * The contest clock's rendering.
 *
 * The old version was a full-width saturated gradient (`from-emerald-600 to
 * teal-600`, `text-white`, `text-3xl`, pulsing dot) that ignored the design
 * tokens entirely, so it was the one surface on the page that was wrong in dark
 * mode. It also had no urgency state: a round two minutes from ending looked
 * identical to one with three hours left.
 *
 * These components no longer own a timer — the page hands them a `ContestClock`
 * from `useContestClock`, so the digits and the phase are the same instant (see
 * useContestClock.test.ts for the ticking). Here they are pure: given a clock,
 * do they render the right thing?
 */

const HOUR = 3_600_000
const NOW = new Date('2026-06-01T12:00:00Z').getTime()

function at(offsetMs: number): string {
    return new Date(NOW + offsetMs).toISOString()
}

function clockFor(startOffset: number, endOffset: number) {
    return contestClockAt(at(startOffset), at(endOffset), null, NOW)
}

describe('ContestCountdown', () => {
    it('renders the time remaining until the round starts', () => {
        render(<ContestCountdown clock={clockFor(2 * HOUR, 5 * HOUR)} label="Starts in" />)

        expect(screen.getByText('Starts in')).toBeInTheDocument()
        expect(screen.getByText(/^02:00:00$/)).toBeInTheDocument()
    })

    it('renders nothing once the round has started', () => {
        // The running clock takes over; a zeroed countdown beside a live contest
        // says nothing useful.
        const { container } = render(<ContestCountdown clock={clockFor(-1000, 2 * HOUR)} />)
        expect(container).toBeEmptyDOMElement()
    })

    it('turns urgent as the start approaches', () => {
        const { container, rerender } = render(<ContestCountdown clock={clockFor(2 * HOUR, 5 * HOUR)} />)
        expect(container.innerHTML).not.toContain('border-warn')

        rerender(<ContestCountdown clock={clockFor(10 * 60_000, 5 * HOUR)} />)
        expect(container.innerHTML).toContain('border-warn')

        rerender(<ContestCountdown clock={clockFor(2 * 60_000, 5 * HOUR)} />)
        expect(container.innerHTML).toContain('border-bad')
    })

    it('uses design tokens rather than a hardcoded gradient', () => {
        const { container } = render(<ContestCountdown clock={clockFor(2 * HOUR, 5 * HOUR)} />)
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
        render(<ContestRunningClock clock={clockFor(-HOUR, HOUR)} />)

        expect(screen.getByText('Running')).toBeInTheDocument()
        expect(screen.getByText(/^01:00:00/)).toBeInTheDocument()
        // Halfway through a two-hour round. The elapsed span and the percentage
        // are on the bar's title rather than in a third row.
        expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50')
        expect(screen.getByRole('progressbar')).toHaveAttribute('title', '50% elapsed')
    })

    it('renders nothing once the round has ended', () => {
        const { container } = render(<ContestRunningClock clock={clockFor(-2 * HOUR, -1000)} />)
        expect(container).toBeEmptyDOMElement()
    })

    it('stays up while the scoreboard is frozen', () => {
        // The freeze holds the standings back; the round is still running and
        // still submittable, so the clock keeps counting.
        const clock = contestClockAt(at(-HOUR), at(HOUR), at(-10 * 60_000), NOW)
        render(<ContestRunningClock clock={clock} />)

        expect(clock.status).toBe('frozen')
        expect(screen.getByText('Running')).toBeInTheDocument()
    })

    it('turns urgent inside the last 5 minutes', () => {
        const { container, rerender } = render(<ContestRunningClock clock={clockFor(-HOUR, HOUR)} />)
        expect(container.innerHTML).not.toContain('border-bad')

        rerender(<ContestRunningClock clock={clockFor(-119 * 60_000, 60_000)} />)
        const html = container.innerHTML
        expect(html).toContain('border-bad')
        expect(html).toContain('bg-bad')
    })

    it('uses design tokens rather than a hardcoded gradient', () => {
        const { container } = render(<ContestRunningClock clock={clockFor(-HOUR, HOUR)} />)
        const html = container.innerHTML
        // The old version was a saturated emerald gradient that ignored the theme.
        expect(html).not.toContain('from-emerald-600')
        expect(html).not.toContain('to-teal-600')
        expect(html).toContain('bg-surface')
    })
})
