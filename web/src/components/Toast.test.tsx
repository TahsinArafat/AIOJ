import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { ToastProvider, useToast } from './Toast'

function Harness() {
    const toast = useToast()
    return (
        <>
            <button onClick={() => toast.success('Saved')}>ok</button>
            <button onClick={() => toast.error('Delete failed: not found')}>fail</button>
        </>
    )
}

afterEach(() => vi.useRealTimers())

describe('Toast', () => {
    it('shows a success message', async () => {
        render(<ToastProvider><Harness /></ToastProvider>)
        await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'ok' })) })
        expect(screen.getByText('Saved')).toBeInTheDocument()
    })

    it('renders errors with role=alert so screen readers announce them', async () => {
        render(<ToastProvider><Harness /></ToastProvider>)
        await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'fail' })) })
        expect(screen.getByRole('alert')).toHaveTextContent('Delete failed: not found')
    })

    it('auto-dismisses after the timeout', async () => {
        vi.useFakeTimers()
        render(<ToastProvider><Harness /></ToastProvider>)
        fireEvent.click(screen.getByRole('button', { name: 'ok' }))
        expect(screen.getByText('Saved')).toBeInTheDocument()

        await act(async () => { vi.advanceTimersByTime(5000) })
        expect(screen.queryByText('Saved')).not.toBeInTheDocument()
    })

    it('can be dismissed manually', async () => {
        render(<ToastProvider><Harness /></ToastProvider>)
        await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'ok' })) })
        await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Dismiss notification' })) })
        expect(screen.queryByText('Saved')).not.toBeInTheDocument()
    })

    it('throws a clear error when used outside the provider', () => {
        const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
        expect(() => render(<Harness />)).toThrow(/ToastProvider/)
        spy.mockRestore()
    })
})
