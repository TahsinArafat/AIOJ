import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import TwoFactorVerify from './TwoFactorVerify'
import { api, setTokens } from '../../lib/api'

const navigate = vi.fn()

vi.mock('react-router-dom', async () => {
    const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
    return {
        ...actual,
        useNavigate: () => navigate,
        useSearchParams: () => [new URLSearchParams('challenge_id=ch_1')],
    }
})

vi.mock('../../lib/api', () => ({
    api: {
        twoFactor: {
            begin: vi.fn(),
            enable: vi.fn(),
            disable: vi.fn(),
            verify: vi.fn(),
        },
    },
    setTokens: vi.fn(),
}))

describe('TwoFactorVerify', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    it('submits challenge and code, stores tokens, navigates home', async () => {
        ;(api.twoFactor.verify as ReturnType<typeof vi.fn>).mockResolvedValue({
            access_token: 'a',
            refresh_token: 'r',
            user: { id: '1' },
        })

        render(
            <MemoryRouter>
                <TwoFactorVerify />
            </MemoryRouter>,
        )

        fireEvent.change(screen.getByPlaceholderText(/6-digit code/i), {
            target: { value: '654321' },
        })
        fireEvent.click(screen.getByRole('button', { name: /verify/i }))

        await waitFor(() => {
            expect(api.twoFactor.verify).toHaveBeenCalledWith({
                challenge_id: 'ch_1',
                code: '654321',
            })
            expect(setTokens).toHaveBeenCalledWith('a', 'r')
            expect(navigate).toHaveBeenCalledWith('/')
        })
    })

    it('shows error on invalid code', async () => {
        ;(api.twoFactor.verify as ReturnType<typeof vi.fn>).mockRejectedValue(
            new Error('invalid code'),
        )

        render(
            <MemoryRouter>
                <TwoFactorVerify />
            </MemoryRouter>,
        )

        fireEvent.change(screen.getByPlaceholderText(/6-digit code/i), {
            target: { value: '000000' },
        })
        fireEvent.click(screen.getByRole('button', { name: /verify/i }))

        await waitFor(() => {
            expect(screen.getByText('invalid code')).toBeInTheDocument()
        })
        expect(setTokens).not.toHaveBeenCalled()
    })
})
