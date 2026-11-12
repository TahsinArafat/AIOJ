import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, expect, test, vi } from 'vitest'
import LanguagesPanel from './LanguagesPanel'
import { api } from '../../lib/api'

vi.mock('../../lib/api', () => ({
    api: {
        admin: {
            languages: {
                list: vi.fn(),
                detect: vi.fn(),
                templates: vi.fn(),
            },
        },
    },
}))

const toastError = vi.fn()

vi.mock('../../components/Toast', () => ({
    useToast: () => ({ success: vi.fn(), error: toastError, info: vi.fn() }),
}))

vi.mock('../../components/ConfirmDialog', () => ({
    useConfirm: () => vi.fn(),
}))

function renderPanel() {
    return render(
        <MemoryRouter>
            <LanguagesPanel />
        </MemoryRouter>,
    )
}

beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.admin.languages.list).mockResolvedValue({ data: [] } as never)
})

// The original bug: Detect answered `"compilers": null` on a server with no
// toolchains, the panel called .map() on that null, the render threw, and with
// no error boundary the entire admin page went blank. A response that omits the
// lists entirely must degrade to "none found" instead of white-screening.
test('Detect renders an empty state instead of blanking when the response has no lists', async () => {
    vi.mocked(api.admin.languages.detect).mockResolvedValue({} as never)

    renderPanel()
    await screen.findByText('Judge Languages')

    fireEvent.click(screen.getByRole('button', { name: /Detect Tools/i }))

    expect(await screen.findByText('No compilers detected')).toBeInTheDocument()
    expect(screen.getByText('No interpreters detected')).toBeInTheDocument()
})

// JSON null survives typing as a lie, so keep the guard honest against it too.
test('Detect tolerates explicit null lists', async () => {
    vi.mocked(api.admin.languages.detect).mockResolvedValue({
        compilers: null,
        interpreters: null,
    } as never)

    renderPanel()
    await screen.findByText('Judge Languages')

    fireEvent.click(screen.getByRole('button', { name: /Detect Tools/i }))

    expect(await screen.findByText('No compilers detected')).toBeInTheDocument()
})

// A working detection still has to render the tools it found.
test('Detect lists the tools the judge reported', async () => {
    vi.mocked(api.admin.languages.detect).mockResolvedValue({
        compilers: [{ name: 'G++ (C++)', path: '/usr/bin/g++', version: 'g++ 13.2.0' }],
        interpreters: [{ name: 'Python 3', path: '/usr/bin/python3', version: 'Python 3.11.2' }],
    } as never)

    renderPanel()
    await screen.findByText('Judge Languages')

    fireEvent.click(screen.getByRole('button', { name: /Detect Tools/i }))

    expect(await screen.findByText('G++ (C++)')).toBeInTheDocument()
    expect(screen.getByText('Python 3')).toBeInTheDocument()
    expect(screen.getByText('g++ 13.2.0')).toBeInTheDocument()
})

// A failed detection must surface the reason and not leave a dead panel open.
test('Detect reports failure and closes the panel', async () => {
    vi.mocked(api.admin.languages.detect).mockRejectedValue(new Error('sandbox offline'))

    renderPanel()
    await screen.findByText('Judge Languages')

    fireEvent.click(screen.getByRole('button', { name: /Detect Tools/i }))

    await waitFor(() => expect(toastError).toHaveBeenCalledWith('Detection failed: sandbox offline'))
    expect(screen.queryByText(/Detected Compilers/i)).not.toBeInTheDocument()
})
