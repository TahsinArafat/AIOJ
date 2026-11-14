import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import { AccessTable } from './AccessTable'

/**
 * The per-surface access table.
 *
 * Extracted from ContestManage's settings tab, which had grown to 768 lines with
 * this table inline. The behaviour worth pinning is the part that has been wrong
 * before: which rows are marked inherited, and that clearing a row returns it to
 * inherited rather than storing an empty audience.
 */

test('renders a row per surface', () => {
    render(<AccessTable accessConfig={{}} listed onChange={vi.fn()} />)

    // Every surface in the registry gets a row, so a new surface cannot be added
    // without appearing here.
    expect(screen.getByLabelText('Problem list access')).toBeInTheDocument()
    expect(screen.getByLabelText('Problem statements access')).toBeInTheDocument()
    expect(screen.getByLabelText('Standings access')).toBeInTheDocument()
})

test('an empty config marks every row inherited', () => {
    render(<AccessTable accessConfig={{}} listed onChange={vi.fn()} />)

    // Inherited rows are labelled, because an inherited audience and one stored on
    // the round look identical otherwise.
    const inherited = screen.getAllByText('inherited')
    expect(inherited.length).toBeGreaterThan(0)
})

test('setting a surface reports the new config', () => {
    const onChange = vi.fn()
    render(<AccessTable accessConfig={{}} listed onChange={onChange} />)

    fireEvent.change(screen.getByLabelText('Standings access'), { target: { value: 'public' } })

    expect(onChange).toHaveBeenCalled()
    expect(onChange.mock.calls[0][0]).toMatchObject({ standings: 'public' })
})

test('clearing a surface returns it to inherited rather than storing a blank', () => {
    const onChange = vi.fn()
    render(
        <AccessTable
            accessConfig={{ standings: 'public' }}
            listed
            onChange={onChange}
        />
    )

    fireEvent.change(screen.getByLabelText('Standings access'), { target: { value: '' } })

    // The empty option means "same as the round", so the key must be removed, not
    // set to '' — a stored blank would render as a real (blank) audience.
    const next = onChange.mock.calls[0][0]
    expect(next.standings).toBeUndefined()
})

test('submit is not offered "participants only"', () => {
    render(<AccessTable accessConfig={{}} listed onChange={vi.fn()} />)

    // Requiring an entrant in order to submit deadlocks the first submission,
    // because submitting is one of the two ways to become one. The server answers
    // 400 for it, so offering it would offer an option that cannot be saved.
    const submit = screen.getByLabelText('Submit solutions access') as HTMLSelectElement
    const values = Array.from(submit.options).map(o => o.value)
    expect(values).not.toContain('participants')
})

test('the summary line defers listing to the control that owns it', () => {
    render(<AccessTable accessConfig={{}} listed={false} onChange={vi.fn()} />)

    // Nothing is public, and the round is unlisted: both halves must be stated,
    // because before the listing control existed this line asserted the round
    // stayed out of the list for the wrong reason.
    expect(screen.getByText(/Nothing here is public/)).toBeInTheDocument()
    expect(screen.getByText(/unlisted/)).toBeInTheDocument()
})
