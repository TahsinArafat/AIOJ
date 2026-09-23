import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import TwoFactorSetup from './TwoFactorSetup'
import { api } from '../../lib/api'

vi.mock('../../lib/api', () => ({
    api: {
        twoFactor: {
            begin: vi.fn(),
            enable: vi.fn(),
            disable: vi.fn(),
            verify: vi.fn(),
        },
    },
}))

describe('TwoFactorSetup', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        // i18next must be initialized for useTranslation in component
    })

    it('starts setup and shows secret after begin', async () => {
        ;(api.twoFactor.begin as ReturnType<typeof vi.fn>).mockResolvedValue({
            secret: 'JBSWY3DPEHPK3PXP',
            uri: 'otpauth://totp/AIOJ:alice?secret=JBSWY3DPEHPK3PXP&issuer=AIOJ',
        })

        render(
            <MemoryRouter>
                <TwoFactorSetup />
            </MemoryRouter>,
        )

        fireEvent.click(screen.getByRole('button', { name: /begin 2fa setup/i }))
        await waitFor(() => {
            expect(screen.getByText('JBSWY3DPEHPK3PXP')).toBeInTheDocument()
        })
        expect(api.twoFactor.begin).toHaveBeenCalled()
        expect(screen.getByPlaceholderText('6-digit code')).toBeInTheDocument()
    })

    it('enables 2FA and shows backup codes', async () => {
        ;(api.twoFactor.begin as ReturnType<typeof vi.fn>).mockResolvedValue({
            secret: 'JBSWY3DPEHPK3PXP',
            uri: 'otpauth://totp/AIOJ:alice?secret=JBSWY3DPEHPK3PXP',
        })
        ;(api.twoFactor.enable as ReturnType<typeof vi.fn>).mockResolvedValue({
            backup_codes: ['abcd1234', 'efgh5678'],
        })

        render(
            <MemoryRouter>
                <TwoFactorSetup />
            </MemoryRouter>,
        )

        fireEvent.click(screen.getByRole('button', { name: /begin 2fa setup/i }))
        await screen.findByPlaceholderText('6-digit code')
        fireEvent.change(screen.getByPlaceholderText('6-digit code'), {
            target: { value: '123456' },
        })
        fireEvent.click(screen.getByRole('button', { name: /enable 2fa/i }))

        await waitFor(() => {
            expect(screen.getByText('abcd1234')).toBeInTheDocument()
            expect(screen.getByText('efgh5678')).toBeInTheDocument()
        })
        expect(api.twoFactor.enable).toHaveBeenCalledWith('123456')
    })
})
