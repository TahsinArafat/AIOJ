import { render, screen, fireEvent, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ClarificationsPanel, AskClarificationForm } from './ClarificationsPanel'
import type { Clarification } from '../types/api'

/**
 * The clarification queue.
 *
 * A competitor reads the round's questions and answers; a jury works a queue.
 * What matters for both is that the row says enough without opening it: which
 * problem it is about, whether it has been answered, and whether the answer is
 * public (a ruling the whole round sees) or private (only the asker's).
 *
 * The jury's bulk answer is the part that makes a live round survivable — the
 * same ruling answers a run of questions, and it must not silently skip one the
 * person selected.
 */

const problems = [
    { problem_id: 'p1', index: 'A', title: 'Sum problem' },
    { problem_id: 'p2', index: 'B', title: 'Graph problem' },
]

const question = (over: Partial<Clarification> = {}): Clarification => ({
    id: 'c1',
    contest_id: 'contest',
    user_id: 'u1',
    username: 'alice',
    problem_id: 'p1',
    problem_idx: 'A',
    question: 'Is n inclusive?',
    answer: null,
    is_public: true,
    answered_by: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...over,
})

describe('ClarificationsPanel', () => {
    it('labels each question with the problem it is about', () => {
        render(<ClarificationsPanel
            items={[question()]}
            loading={false}
            problems={problems}
            isJudge={false}
        />)

        // The same text is also a filter option, so scope to the question list.
        const list = screen.getByRole('list')
        expect(within(list).getByText('A — Sum problem')).toBeInTheDocument()
    })

    it('shows a question with no problem as unlabelled', () => {
        render(<ClarificationsPanel
            items={[question({ problem_id: null, problem_idx: null })]}
            loading={false}
            problems={problems}
            isJudge={false}
        />)

        const list = screen.getByRole('list')
        expect(within(list).queryByText(/Sum problem/)).not.toBeInTheDocument()
    })

    it('marks an unanswered question, and an answered one as resolved', () => {
        const { rerender } = render(<ClarificationsPanel
            items={[question({ answer: null })]}
            loading={false}
            problems={problems}
            isJudge
            onAnswer={vi.fn()}
        />)
        expect(within(screen.getByRole('list')).getByText('unanswered')).toBeInTheDocument()
        expect(screen.getByLabelText('Your answer')).toBeInTheDocument()

        rerender(<ClarificationsPanel
            items={[question({ answer: 'Yes, n is inclusive.' })]}
            loading={false}
            problems={problems}
            isJudge
            onAnswer={vi.fn()}
        />)
        // The filter's own label still says "Unanswered"; the row badge must go.
        const list = screen.getByRole('list')
        expect(within(list).queryByText('unanswered')).not.toBeInTheDocument()
        expect(within(list).getByText('Yes, n is inclusive.')).toBeInTheDocument()
    })

    it('says an answer is shared with everyone when it is public', () => {
        render(<ClarificationsPanel
            items={[question({ answer: 'Yes.', is_public: true })]}
            loading={false}
            problems={problems}
            isJudge={false}
        />)

        expect(screen.getByText('Answer — shared with all contestants')).toBeInTheDocument()
        expect(screen.queryByText('Answer — private')).not.toBeInTheDocument()
    })

    it('says an answer is private when it is not public', () => {
        render(<ClarificationsPanel
            items={[question({ answer: 'Only for you.', is_public: false })]}
            loading={false}
            problems={problems}
            isJudge={false}
        />)

        // A private reply must not be presented as a ruling the whole round sees.
        expect(screen.getByText('Answer — private')).toBeInTheDocument()
        expect(screen.queryByText('Answer — shared with all contestants')).not.toBeInTheDocument()
    })

    it('filters to unanswered when asked', () => {
        render(<ClarificationsPanel
            items={[
                question({ id: 'c1' }),
                question({ id: 'c2', answer: 'Done.', is_public: true }),
            ]}
            loading={false}
            problems={problems}
            isJudge={false}
        />)

        fireEvent.click(screen.getByRole('button', { name: /Unanswered/ }))

        expect(screen.getByText('Is n inclusive?')).toBeInTheDocument()
        expect(screen.queryByText(/Done\./)).not.toBeInTheDocument()
    })

    it('filters by problem', () => {
        render(<ClarificationsPanel
            items={[
                question({ id: 'c1', problem_id: 'p1' }),
                question({ id: 'c2', problem_id: 'p2', question: 'Does B have weights?' }),
            ]}
            loading={false}
            problems={problems}
            isJudge={false}
        />)

        fireEvent.change(screen.getByLabelText('Filter by problem'), { target: { value: 'p2' } })

        const list = screen.getByRole('list')
        expect(within(list).getByText('Does B have weights?')).toBeInTheDocument()
        expect(within(list).queryByText('Is n inclusive?')).not.toBeInTheDocument()
    })

    it('gives a contestant no answer box and no selection controls', () => {
        render(<ClarificationsPanel
            items={[question()]}
            loading={false}
            problems={problems}
            isJudge={false}
        />)

        expect(screen.queryByLabelText('Your answer')).not.toBeInTheDocument()
        expect(screen.queryByLabelText(/Select clarification about/)).not.toBeInTheDocument()
    })

    it('lets a judge answer one question inline', () => {
        const onAnswer = vi.fn().mockResolvedValue(undefined)
        render(<ClarificationsPanel
            items={[question()]}
            loading={false}
            problems={problems}
            isJudge
            onAnswer={onAnswer}
        />)

        fireEvent.change(screen.getByLabelText('Your answer'), { target: { value: 'Inclusive.' } })
        fireEvent.click(within(screen.getByRole('list')).getByRole('button', { name: 'Answer' }))

        expect(onAnswer).toHaveBeenCalledWith('c1', 'Inclusive.', true)
    })

    it('answers every selected question in one call', async () => {
        const onAnswerMany = vi.fn().mockResolvedValue(undefined)
        render(<ClarificationsPanel
            items={[
                question({ id: 'c1' }),
                question({ id: 'c2', question: 'And for B?' }),
            ]}
            loading={false}
            problems={problems}
            isJudge
            onAnswerMany={onAnswerMany}
        />)

        // Both questions come from the same competitor, so the row checkbox is
        // labelled by the question text to stay unambiguous.
        fireEvent.click(screen.getByLabelText('Select clarification about: Is n inclusive?'))
        fireEvent.click(screen.getByLabelText('Select clarification about: And for B?'))
        fireEvent.change(screen.getByLabelText('Answer for the selected clarifications'), { target: { value: 'Same ruling.' } })
        fireEvent.click(screen.getByRole('button', { name: 'Answer selected' }))

        expect(onAnswerMany).toHaveBeenCalledWith(['c1', 'c2'], 'Same ruling.', true)
    })

    it('selects every visible question at once', () => {
        render(<ClarificationsPanel
            items={[question({ id: 'c1' }), question({ id: 'c2' })]}
            loading={false}
            problems={problems}
            isJudge
            onAnswerMany={vi.fn()}
        />)

        fireEvent.click(screen.getByLabelText('Select all visible'))

        expect(screen.getByText('2 selected')).toBeInTheDocument()
    })

    it('does not answer a question with an empty ruling', () => {
        const onAnswerMany = vi.fn()
        render(<ClarificationsPanel
            items={[question()]}
            loading={false}
            problems={problems}
            isJudge
            onAnswerMany={onAnswerMany}
        />)

        fireEvent.click(screen.getByLabelText('Select clarification about: Is n inclusive?'))
        // The submit button is disabled until there is something to write.
        expect(screen.getByRole('button', { name: 'Answer selected' })).toBeDisabled()
    })
})

describe('AskClarificationForm', () => {
    it('binds the question to the problem the competitor chose', async () => {
        const onSubmit = vi.fn().mockResolvedValue(undefined)
        render(<AskClarificationForm problems={problems} onSubmit={onSubmit} />)

        fireEvent.change(screen.getByLabelText('Which problem is this about'), { target: { value: 'p2' } })
        fireEvent.change(screen.getByLabelText('Your question'), { target: { value: 'Weights?' } })
        fireEvent.click(screen.getByRole('button', { name: 'Ask' }))

        expect(onSubmit).toHaveBeenCalledWith('p2', 'Weights?')
    })

    it('keeps a locked problem fixed', () => {
        render(<AskClarificationForm problems={problems} lockedProblemId="p1" onSubmit={vi.fn()} />)

        const select = screen.getByLabelText('Which problem is this about') as HTMLSelectElement
        expect(select.value).toBe('p1')
        expect(select).toBeDisabled()
    })

    it('says a question with no problem is general', () => {
        render(<AskClarificationForm problems={problems} onSubmit={vi.fn()} />)

        const options = within(screen.getByLabelText('Which problem is this about')).getAllByRole('option')
        expect(options[0].textContent).toContain('not about one problem')
    })

    it('refuses to send an empty question', () => {
        const onSubmit = vi.fn()
        render(<AskClarificationForm problems={problems} onSubmit={onSubmit} />)

        expect(screen.getByRole('button', { name: 'Ask' })).toBeDisabled()
    })

    it('keeps the typed question when the send fails', async () => {
        const onSubmit = vi.fn().mockRejectedValue(new Error('offline'))
        render(<AskClarificationForm problems={problems} onSubmit={onSubmit} />)

        fireEvent.change(screen.getByLabelText('Your question'), { target: { value: 'still typing' } })
        fireEvent.click(screen.getByRole('button', { name: 'Ask' }))

        expect(await screen.findByText('offline')).toBeInTheDocument()
        expect(screen.getByLabelText('Your question')).toHaveValue('still typing')
    })
})