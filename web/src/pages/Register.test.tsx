import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, expect, test, vi } from 'vitest'
import Register from './Register'
import { api } from '../lib/api'

vi.mock('../lib/api', () => ({
    api: {
        auth: {
            register: vi.fn(),
            resendVerification: vi.fn(),
            methods: vi.fn(),
        },
    },
    setTokens: vi.fn(),
}))

function methods(data: { password_registration: boolean; google: boolean; github: boolean }) {
    vi.mocked(api.auth.methods).mockResolvedValue({ data } as never)
}

function renderPage() {
    return render(
        <MemoryRouter>
            <Register />
        </MemoryRouter>,
    )
}

beforeEach(() => {
    vi.clearAllMocks()
    methods({ password_registration: true, google: false, github: false })
})

test('renders the email/password form when registration is open', async () => {
    renderPage()

    expect(await screen.findByLabelText('Username')).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Register' })).toBeInTheDocument()
})

test('replaces the form with an explanation when password registration is closed', async () => {
    methods({ password_registration: false, google: true, github: false })

    renderPage()

    expect(
        await screen.findByRole('heading', { name: 'Registration closed' }),
    ).toBeInTheDocument()
    // The form must be gone, not merely disabled: the API would reject it.
    expect(screen.queryByLabelText('Password')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Register' })).not.toBeInTheDocument()
})

test('offers a usable provider when password registration is closed', async () => {
    methods({ password_registration: false, google: true, github: false })

    renderPage()

    expect(await screen.findByRole('link', { name: /Continue with Google/i })).toHaveAttribute(
        'href',
        '/api/auth/oauth/google/start',
    )
    expect(screen.queryByRole('link', { name: /Continue with GitHub/i })).not.toBeInTheDocument()
})

test('explains that sign-ups are closed when nothing at all is available', async () => {
    methods({ password_registration: false, google: false, github: false })

    renderPage()

    expect(
        await screen.findByRole('heading', { name: 'Registration closed' }),
    ).toBeInTheDocument()
    expect(screen.getByText(/contact the site administrator/i)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Continue with/i })).not.toBeInTheDocument()
})

test('shows provider buttons below the form when registration is open', async () => {
    methods({ password_registration: true, google: true, github: true })

    renderPage()

    expect(await screen.findByRole('link', { name: /Continue with Google/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Continue with GitHub/i })).toBeInTheDocument()
    // The form is still the primary path.
    expect(screen.getByRole('button', { name: 'Register' })).toBeInTheDocument()
})
