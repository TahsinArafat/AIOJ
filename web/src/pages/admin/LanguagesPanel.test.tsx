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
                reset: vi.fn(),
                get: vi.fn(),
                getRaw: vi.fn(),
                update: vi.fn(),
                create: vi.fn(),
                delete: vi.fn(),
                test: vi.fn(),
            },
        },
    },
}))

// The panel invalidates the SPA's cached language list after a save or reset;
// that is a side effect on a different module, so stub it rather than assert it
// through the real cache.
vi.mock('../../lib/useLanguages', () => ({
    invalidateLanguages: vi.fn(),
    useLanguages: () => ({ languages: [], loading: false, options: [], refresh: vi.fn() }),
}))

const toastError = vi.fn()
const confirmMock = vi.fn()

vi.mock('../../components/Toast', () => ({
    useToast: () => ({ success: vi.fn(), error: toastError, info: vi.fn() }),
}))

vi.mock('../../components/ConfirmDialog', () => ({
    useConfirm: () => confirmMock,
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
    confirmMock.mockResolvedValue(true)
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

// --- Reset to Default ------------------------------------------------------

const oneLang = {
    data: [{
        name: 'C++ (G++ 64-bit)',
        key: 'cpp-gpp-64',
        compile: '/usr/bin/g++ -O2 -o {{exe}} {{src}}',
        runtime: '',
        copy_in: {},
        time_limit_multiplier: 1,
        memory_limit_multiplier: 1,
        seccomp_rule: 'c_cpp',
        extensions: ['.cpp'],
        mono: false,
        file_path: 'cpp-gpp-64.yaml',
    }],
}

function renderWithOneLang() {
    vi.mocked(api.admin.languages.list).mockResolvedValue(oneLang as never)
    return renderPanel()
}

// Reset exists because the panel edits lang/*.yaml in place: a bad save could
// otherwise only be undone by a redeploy.
test('Reset restores the language after confirmation', async () => {
    vi.mocked(api.admin.languages.reset).mockResolvedValue(oneLang.data[0] as never)

    renderWithOneLang()
    await screen.findByText('C++ (G++ 64-bit)')

    fireEvent.click(screen.getByTitle('Reset to Default'))

    await waitFor(() => expect(api.admin.languages.reset).toHaveBeenCalledWith('cpp-gpp-64'))
})

// A destructive action must be cancellable, and a cancelled reset must not
// touch the server.
test('Reset is skipped when the confirmation is declined', async () => {
    confirmMock.mockResolvedValue(false)

    renderWithOneLang()
    await screen.findByText('C++ (G++ 64-bit)')

    fireEvent.click(screen.getByTitle('Reset to Default'))

    await waitFor(() => expect(confirmMock).toHaveBeenCalled())
    expect(api.admin.languages.reset).not.toHaveBeenCalled()
})

// A language an operator created has no shipped default; the server answers 404
// with an explanation, which the panel must show rather than swallow.
test('a failed Reset reports why', async () => {
    vi.mocked(api.admin.languages.reset).mockRejectedValue(
        new Error('language "operator-made" has no built-in default to restore'),
    )

    renderWithOneLang()
    await screen.findByText('C++ (G++ 64-bit)')

    fireEvent.click(screen.getByTitle('Reset to Default'))

    await waitFor(() => expect(toastError).toHaveBeenCalledWith(
        expect.stringContaining('no built-in default'),
    ))
})
