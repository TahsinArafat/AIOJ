import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { expect, test, vi, beforeEach } from 'vitest'
import BotAccountsPanel from './BotAccountsPanel'
import { ToastProvider } from '../../components/Toast'
import { ConfirmProvider } from '../../components/ConfirmDialog'
import { api } from '../../lib/api'

vi.mock('../../lib/api', async (importOriginal) => {
    const actual = await importOriginal<typeof import('../../lib/api')>()
    return {
        ...actual,
        api: {
            ...actual.api,
            admin: {
                ...actual.api.admin,
                listUsers: vi.fn(),
                botAccounts: {
                    list: vi.fn(),
                    create: vi.fn(),
                    update: vi.fn(),
                    delete: vi.fn(),
                    testLogin: vi.fn(),
                },
            },
        },
    }
})

function renderPanel() {
    return render(
        <ToastProvider>
            <ConfirmProvider>
                <BotAccountsPanel />
            </ConfirmProvider>
        </ToastProvider>
    )
}

beforeEach(() => {
    vi.mocked(api.admin.botAccounts.list).mockReset()
    vi.mocked(api.admin.listUsers).mockReset().mockResolvedValue({ data: [], total: 0 } as never)
})

test('loads and renders the bot account list', async () => {
    vi.mocked(api.admin.botAccounts.list).mockResolvedValue({
        data: [
            {
                id: 'bot-1',
                user_id: 'u1',
                platform: 'codeforces',
                platform_user: 'alice',
                status: 'active',
                rate_limit_rps: 1,
                proxy_url: '',
                proxy_enabled: false,
                created_at: new Date().toISOString(),
            },
        ],
        total: 1,
    } as never)

    renderPanel()

    await waitFor(() => expect(screen.getByText('VJudge Bot Accounts')).toBeInTheDocument())
    expect(api.admin.botAccounts.list).toHaveBeenCalled()
    expect(api.admin.listUsers).toHaveBeenCalledWith(0, 100)
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
})

test('shows the empty state when no bots are configured', async () => {
    vi.mocked(api.admin.botAccounts.list).mockResolvedValue({ data: [], total: 0 } as never)

    renderPanel()

    await waitFor(() =>
        expect(screen.getByText('No bot accounts configured. Click "Add Bot Account" to add one.')).toBeInTheDocument()
    )
})

test('opening the add form reveals the platform selector and inputs', async () => {
    vi.mocked(api.admin.botAccounts.list).mockResolvedValue({ data: [], total: 0 } as never)

    renderPanel()
    await waitFor(() => expect(screen.getByText('VJudge Bot Accounts')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /Add Bot Account/ }))

    expect(screen.getByText('Add New Bot Account')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Codeforces' })).toBeInTheDocument()
})
