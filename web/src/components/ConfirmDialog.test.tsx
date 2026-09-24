import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { ConfirmProvider, useConfirm } from './ConfirmDialog'

function Harness({ onResult }: { onResult: (v: boolean) => void }) {
    const confirm = useConfirm()
    return (
        <button onClick={async () => {
            const result = await confirm({
                title: 'Delete problem?',
                message: 'This cannot be undone.',
                confirmLabel: 'Delete problem',
                variant: 'danger',
            })
            onResult(result)
        }}>Open delete dialog</button>
    )
}

afterEach(() => vi.useRealTimers())

describe('ConfirmDialog', () => {
    it('resolves true when the confirm button is clicked', async () => {
        const onResult = vi.fn()
        render(<ConfirmProvider><Harness onResult={onResult} /></ConfirmProvider>)

        fireEvent.click(screen.getByRole('button', { name: 'Open delete dialog' }))
        expect(screen.getByRole('dialog')).toBeInTheDocument()

        await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Delete problem' })) })
        expect(onResult).toHaveBeenCalledWith(true)
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })

    it('resolves false when cancelled', async () => {
        const onResult = vi.fn()
        render(<ConfirmProvider><Harness onResult={onResult} /></ConfirmProvider>)

        fireEvent.click(screen.getByRole('button', { name: 'Open delete dialog' }))
        await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Cancel' })) })
        expect(onResult).toHaveBeenCalledWith(false)
    })

    it('resolves false on Escape', async () => {
        const onResult = vi.fn()
        render(<ConfirmProvider><Harness onResult={onResult} /></ConfirmProvider>)

        fireEvent.click(screen.getByRole('button', { name: 'Open delete dialog' }))
        await act(async () => { fireEvent.keyDown(document, { key: 'Escape' }) })
        expect(onResult).toHaveBeenCalledWith(false)
    })

    it('resolves false when the backdrop is clicked', async () => {
        const onResult = vi.fn()
        const { container } = render(<ConfirmProvider><Harness onResult={onResult} /></ConfirmProvider>)

        fireEvent.click(screen.getByRole('button', { name: 'Open delete dialog' }))
        const backdrop = document.querySelector('.fixed.inset-0') as HTMLElement
        await act(async () => { fireEvent.click(backdrop) })
        expect(onResult).toHaveBeenCalledWith(false)
        void container
    })

    it('is wired for accessibility', () => {
        render(<ConfirmProvider><Harness onResult={vi.fn()} /></ConfirmProvider>)
        fireEvent.click(screen.getByRole('button', { name: 'Open delete dialog' }))

        const dialog = screen.getByRole('dialog')
        expect(dialog).toHaveAttribute('aria-modal', 'true')
        expect(dialog).toHaveAccessibleName('Delete problem?')
        expect(dialog).toHaveAccessibleDescription('This cannot be undone.')
    })

    it('moves focus to the confirm button on open', () => {
        render(<ConfirmProvider><Harness onResult={vi.fn()} /></ConfirmProvider>)
        fireEvent.click(screen.getByRole('button', { name: 'Open delete dialog' }))
        expect(screen.getByRole('button', { name: 'Delete problem' })).toHaveFocus()
    })

    it('throws a clear error when used outside the provider', () => {
        const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
        expect(() => render(<Harness onResult={vi.fn()} />)).toThrow(/ConfirmProvider/)
        spy.mockRestore()
    })
})
