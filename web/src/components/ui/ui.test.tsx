import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import { Button } from './Button'
import { buttonClass } from './buttonStyles'
import { Badge } from './Badge'
import { Field, Input } from './Field'

/**
 * These primitives replaced dozens of hand-written class strings. The tests
 * pin the contract the pages depend on: semantic theme tokens (not raw
 * `bg-white dark:bg-gray-800`) and a safe default `type` so a button inside a
 * form never submits by accident.
 */

test('buttonClass emits semantic tokens for every variant', () => {
    expect(buttonClass({ variant: 'primary' })).toContain('bg-brand')
    expect(buttonClass({ variant: 'secondary' })).toContain('border-hairline-strong')
    expect(buttonClass({ variant: 'ghost' })).toContain('hover:bg-surface-2')
    expect(buttonClass({ variant: 'danger' })).toContain('text-bad')
    // A hard-coded light-only surface would break dark mode.
    expect(buttonClass()).not.toContain('bg-white')
    expect(buttonClass()).not.toContain('dark:')
})

test('Button defaults to type=button and honours an explicit submit', () => {
    render(
        <form>
            <Button>Save</Button>
            <Button type="submit">Submit</Button>
        </form>,
    )
    expect(screen.getByRole('button', { name: 'Save' })).toHaveAttribute('type', 'button')
    expect(screen.getByRole('button', { name: 'Submit' })).toHaveAttribute('type', 'submit')
})

test('Badge maps tone to theme tokens', () => {
    render(
        <>
            <Badge tone="ok">Accepted</Badge>
            <Badge tone="bad">Wrong answer</Badge>
            <Badge tone="brand" dot>Upcoming</Badge>
        </>,
    )
    expect(screen.getByText('Accepted').className).toContain('text-ok')
    expect(screen.getByText('Wrong answer').className).toContain('text-bad')
    expect(screen.getByText('Upcoming').className).toContain('bg-brand-soft')
})

test('Field wires the label to its control and shows the error instead of the hint', () => {
    render(
        <Field label="Password" htmlFor="pw" hint="At least 12 characters." error="Too short">
            <Input id="pw" />
        </Field>,
    )
    expect(screen.getByLabelText('Password')).toHaveAttribute('id', 'pw')
    expect(screen.getByText('Too short')).toBeInTheDocument()
    expect(screen.queryByText('At least 12 characters.')).not.toBeInTheDocument()
})
