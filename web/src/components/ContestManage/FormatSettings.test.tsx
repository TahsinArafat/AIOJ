import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import { FormatSettings } from './FormatSettings'

/**
 * The selected format's own settings.
 *
 * Extracted from ContestManage's settings tab with the access table. The fields
 * come from the format registry rather than being hand-written per format, so a
 * format added there gets its settings UI for free.
 */

test('renders the registry fields for a known format', () => {
    render(<FormatSettings format="acm" config={{}} onChange={vi.fn()} />)

    // acm's penalty-per-wrong is a registry field; if the registry lost it this
    // would render an empty fieldset and the setter could not tune it.
    const inputs = screen.getAllByRole('spinbutton')
    expect(inputs.length).toBeGreaterThan(0)
})

test('an unknown format says so rather than rendering an empty fieldset', () => {
    render(<FormatSettings format="no-such-format" config={{ penalty: '20' }} onChange={vi.fn()} />)

    // An empty fieldset would read as "no settings", and a save would then clear
    // the stored values. The stored values must be left untouched and said to be.
    expect(screen.getByText(/no settings for/)).toBeInTheDocument()
    expect(screen.getByText(/left untouched/)).toBeInTheDocument()
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument()
})

test('changing a field reports that key only', () => {
    const onChange = vi.fn()
    render(<FormatSettings format="acm" config={{}} onChange={onChange} />)

    const input = screen.getAllByRole('spinbutton')[0]
    fireEvent.change(input, { target: { value: '30' } })

    expect(onChange).toHaveBeenCalled()
    // The parent merges this into the form's config, so the component must report
    // the single key rather than the whole map.
    expect(onChange.mock.calls[0][0]).toBeTypeOf('string')
    expect(onChange.mock.calls[0][1]).toBe('30')
})

test('a value the registry does not know is still rendered', () => {
    render(
        <FormatSettings
            format="acm"
            config={{ penalty_per_wrong: '20' }}
            onChange={vi.fn()}
        />
    )

    // The stored value must appear even if the registry's field set changed, or
    // saving would silently drop a number the round is using.
    expect(screen.getByDisplayValue('20')).toBeInTheDocument()
})
