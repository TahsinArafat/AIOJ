import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ContestRatingLedger from './ContestRatingLedger'
import { ConfirmProvider } from './ConfirmDialog'

/**
 * The rating ledger panel.
 *
 * Three things have to be true for an operator to trust it: a round is only paid
 * out once (so the button is "rate", then "recalculate" or "withdraw"), a
 * disqualification names a person from the whole field rather than from whatever
 * page is on screen, and the panel says how many rounds a replay touched — the
 * answer is rarely one, and an operator who does not know that will think nothing
 * happened.
 */

const mocks = vi.hoisted(() => ({
  calculateRatings: vi.fn(),
  recalculateRatings: vi.fn(),
  voidRatings: vi.fn(),
  unvoidRatings: vi.fn(),
  ratingExclusions: vi.fn(),
  ratingRuns: vi.fn(),
  addRatingExclusion: vi.fn(),
  removeRatingExclusion: vi.fn(),
  scoreboard: vi.fn(),
}))

vi.mock('../lib/api', () => ({ api: { contests: mocks } }))

const boardEntry = (userID: string, username: string) => ({
  rank: 1, user_id: userID, username, total_solved: 1, total_penalty: 1,
  total_score: 1, problems: {},
})

function renderPanel(over: Partial<React.ComponentProps<typeof ContestRatingLedger>> = {}) {
  const onChanged = vi.fn()
  const utils = render(
    <ConfirmProvider>
      <ContestRatingLedger
        contestId="c1"
        applied={false}
        voided={false}
        participantCount={2}
        onChanged={onChanged}
        {...over}
      />
    </ConfirmProvider>,
  )
  return { ...utils, onChanged }
}

/** The dialog's affirmative button. It carries the same label as the trigger. */
function confirmInDialog() {
  const dialog = screen.getByRole('dialog')
  return within(dialog).getAllByRole('button').at(-1) as HTMLElement
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.ratingExclusions.mockResolvedValue({ data: [] })
  mocks.ratingRuns.mockResolvedValue({ data: [] })
  mocks.scoreboard.mockResolvedValue({
    entries: [boardEntry('u1', 'alice'), boardEntry('u2', 'bob')],
  })
})

describe('ContestRatingLedger', () => {
  it('offers a payout only until the round has been rated', async () => {
    const { rerender } = renderPanel()

    expect(await screen.findByRole('button', { name: 'Rate this round' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Recalculate from here' })).toBeNull()

    rerender(
      <ConfirmProvider>
        <ContestRatingLedger contestId="c1" applied voided={false} participantCount={2} onChanged={vi.fn()} />
      </ConfirmProvider>,
    )

    expect(screen.queryByRole('button', { name: 'Rate this round' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Recalculate from here' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Withdraw ratings' })).toBeInTheDocument()
  })

  it('reports how many entrants a payout rated, not just that it finished', async () => {
    mocks.calculateRatings.mockResolvedValue({ rated: 22, participants: 25 })
    const { onChanged } = renderPanel()

    fireEvent.click(await screen.findByRole('button', { name: 'Rate this round' }))

    expect(await screen.findByText('Rated 22 of 25 entrants.')).toBeInTheDocument()
    await waitFor(() => expect(onChanged).toHaveBeenCalled())
  })

  it('reports the number of rounds a recalculation replayed', async () => {
    mocks.recalculateRatings.mockResolvedValue({ contests: 3 })
    renderPanel({ applied: true })

    fireEvent.click(await screen.findByRole('button', { name: 'Recalculate from here' }))
    // The confirmation explains the replay before anything is written.
    fireEvent.click(await screen.findByRole('button', { name: 'Recalculate' }))

    expect(await screen.findByText('Replayed 3 rounds.')).toBeInTheDocument()
  })

  it('asks before withdrawing a round, and says the standings survive', async () => {
    mocks.voidRatings.mockResolvedValue({ contests: 2 })
    renderPanel({ applied: true })

    fireEvent.click(await screen.findByRole('button', { name: 'Withdraw ratings' }))

    expect(await screen.findByText(/standings stay exactly as they are/i)).toBeInTheDocument()
    expect(mocks.voidRatings).not.toHaveBeenCalled()

    fireEvent.click(confirmInDialog())

    await waitFor(() => expect(mocks.voidRatings).toHaveBeenCalledWith('c1'))
    expect(await screen.findByText('Replayed 2 rounds.')).toBeInTheDocument()
  })

  it('offers every entrant on the board, not only the page on screen', async () => {
    renderPanel({ participantCount: 400 })

    await waitFor(() => expect(mocks.scoreboard).toHaveBeenCalled())
    // The whole field is requested in one page, sized to the participant count.
    expect(mocks.scoreboard).toHaveBeenCalledWith('c1', 'public', 1, 400)

    const select = screen.getByLabelText('Entrant to disqualify')
    expect(select).toHaveTextContent('alice')
    expect(select).toHaveTextContent('bob')
  })

  it('records a reason with a disqualification and replays from there', async () => {
    mocks.addRatingExclusion.mockResolvedValue({ contests: 2 })
    renderPanel()

    fireEvent.change(await screen.findByLabelText('Entrant to disqualify'), { target: { value: 'u1' } })
    fireEvent.change(screen.getByLabelText('Reason for disqualification'), { target: { value: 'plagiarism' } })
    fireEvent.click(screen.getByRole('button', { name: 'Disqualify' }))
    // The confirm names the person, so the operator cannot disqualify the wrong one.
    expect(await screen.findByText('Disqualify alice?')).toBeInTheDocument()
    fireEvent.click(confirmInDialog())

    await waitFor(() => expect(mocks.addRatingExclusion).toHaveBeenCalledWith(
      'c1', { user_id: 'u1', reason: 'plagiarism' },
    ))
    expect(await screen.findByText('alice disqualified; replayed 2 rounds.')).toBeInTheDocument()
  })

  it('lists who is disqualified and offers to reinstate them', async () => {
    mocks.ratingExclusions.mockResolvedValue({
      data: [{ user_id: 'u2', contest_id: 'c1', reason: 'plagiarism', created_at: '2026-01-01T00:00:00Z' }],
    })
    mocks.removeRatingExclusion.mockResolvedValue({ contests: 1 })
    renderPanel()

    expect(await screen.findByText('bob')).toBeInTheDocument()
    expect(screen.getByText('— plagiarism')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Reinstate' }))
    fireEvent.click(confirmInDialog())

    await waitFor(() => expect(mocks.removeRatingExclusion).toHaveBeenCalledWith('c1', 'u2'))
  })

  it('surfaces the reason a rating operation was refused', async () => {
    mocks.calculateRatings.mockRejectedValue(new Error('ratings already applied to this contest'))
    renderPanel()

    fireEvent.click(await screen.findByRole('button', { name: 'Rate this round' }))

    expect(await screen.findByText('ratings already applied to this contest')).toBeInTheDocument()
  })

  it('shows the run history with the operator behind each change', async () => {
    mocks.ratingRuns.mockResolvedValue({
      data: [{
        id: 'r1', contest_id: 'c1', actor_name: 'admin', kind: 'recalculate',
        reason: 'round ratings voided', created_at: '2026-01-01T00:00:00Z',
      }],
    })
    renderPanel({ applied: true })

    fireEvent.click(await screen.findByRole('button', { name: /History/i }))

    expect(await screen.findByText('Recalculated')).toBeInTheDocument()
    expect(screen.getByText('by admin')).toBeInTheDocument()
    expect(screen.getByText('— round ratings voided')).toBeInTheDocument()
  })
})
