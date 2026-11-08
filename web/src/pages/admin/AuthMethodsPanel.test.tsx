import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, expect, test, vi } from 'vitest'
import AuthMethodsPanel from './AuthMethodsPanel'
import { api } from '../../lib/api'
import type { AdminAuthMethods } from '../../types/api'

vi.mock('../../lib/api', () => ({
    api: {
        admin: {
            authMethods: {
                get: vi.fn(),
                set: vi.fn(),
            },
        },
    },
}))

vi.mock('../../components/Toast', () => ({
    useToast: () => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}))

function view(overrides: Partial<AdminAuthMethods> = {}): AdminAuthMethods {
    return {
        settings: { password_registration: true, google: true, github: true },
        configured: { google: true, github: true },
        default: { password_registration: true, google: true, github: true },
        customized: false,
        ...overrides,
    }
}

function renderPanel() {
    return render(
        <MemoryRouter>
            <AuthMethodsPanel />
        </MemoryRouter>,
    )
}

beforeEach(() => {
    vi.clearAllMocks()
})

test('renders one switch per sign-in method', async () => {
    vi.mocked(api.admin.authMethods.get).mockResolvedValue({ data: view() } as never)

    renderPanel()

    expect(
        await screen.findByRole('switch', { name: /Enable Email and password registration/i }),
    ).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: /Enable Google sign-in/i })).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: /Enable GitHub sign-in/i })).toBeInTheDocument()
})

test('explains a provider that is switched on but has no credentials', async () => {
    // The confusing case: the switch is on, yet the button never appears on the
    // login page. The panel has to say why.
    vi.mocked(api.admin.authMethods.get).mockResolvedValue({
        data: view({ configured: { google: false, github: true } }),
    } as never)

    renderPanel()

    expect(await screen.findByText(/Not available on this deployment/i)).toBeInTheDocument()
    expect(screen.getByText(/GOOGLE_CLIENT_ID/)).toBeInTheDocument()
    expect(screen.getByText(/Switched on, but the login page will not offer it/i)).toBeInTheDocument()
})

test('warns when every route to a new account is closed', async () => {
    vi.mocked(api.admin.authMethods.get).mockResolvedValue({
        data: view({
            settings: { password_registration: false, google: false, github: false },
        }),
    } as never)

    renderPanel()

    expect(await screen.findByText(/New sign-ups are fully closed/i)).toBeInTheDocument()
})

test('does not warn when at least one method is usable', async () => {
    vi.mocked(api.admin.authMethods.get).mockResolvedValue({
        data: view({
            settings: { password_registration: false, google: true, github: false },
            configured: { google: true, github: false },
        }),
    } as never)

    renderPanel()

    await screen.findByRole('switch', { name: /Enable Google sign-in/i })
    expect(screen.queryByText(/New sign-ups are fully closed/i)).not.toBeInTheDocument()
})

test('loads failures surface as an alert instead of an empty panel', async () => {
    vi.mocked(api.admin.authMethods.get).mockRejectedValue(new Error('forbidden'))

    renderPanel()

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
})
