import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { expect, test, vi, beforeEach } from 'vitest'
import UsersPanel from './UsersPanel'
import { ToastProvider } from '../../components/Toast'
import { ConfirmProvider } from '../../components/ConfirmDialog'
import { api } from '../../lib/api'
import type { AdminUserRow } from '../../types/api'

vi.mock('../../lib/api', async (importOriginal) => {
    const actual = await importOriginal<typeof import('../../lib/api')>()
    return {
        ...actual,
        api: {
            ...actual.api,
            admin: {
                ...actual.api.admin,
                users: {
                    list: vi.fn(),
                    get: vi.fn(),
                    setRole: vi.fn(),
                    resetPassword: vi.fn(),
                    ban: vi.fn(),
                    unban: vi.fn(),
                    verifyEmail: vi.fn(),
                    unverifyEmail: vi.fn(),
                    bulk: vi.fn(),
                },
            },
        },
    }
})

function row(overrides: Partial<AdminUserRow> = {}): AdminUserRow {
    return {
        id: 'u1',
        username: 'alice',
        email: 'alice@example.com',
        role: 'user',
        is_bot: false,
        email_verified: true,
        must_change_password: false,
        rating: 1500,
        problems_solved: 12,
        submissions: 40,
        created_at: new Date('2026-01-02T00:00:00Z').toISOString(),
        ...overrides,
    }
}

function renderPanel() {
    return render(
        <ToastProvider>
            <ConfirmProvider>
                <UsersPanel />
            </ConfirmProvider>
        </ToastProvider>,
    )
}

const usersApi = () => api.admin.users

beforeEach(() => {
    vi.mocked(usersApi().list).mockReset().mockResolvedValue({ data: [row()], total: 1, offset: 0, limit: 25 })
    vi.mocked(usersApi().bulk).mockReset()
    vi.mocked(usersApi().ban).mockReset().mockResolvedValue(undefined as never)
    vi.mocked(usersApi().unban).mockReset().mockResolvedValue(undefined as never)
    vi.mocked(usersApi().verifyEmail).mockReset().mockResolvedValue(undefined as never)
    vi.mocked(usersApi().unverifyEmail).mockReset().mockResolvedValue(undefined as never)
    vi.mocked(usersApi().resetPassword).mockReset().mockResolvedValue(undefined as never)
    vi.mocked(usersApi().setRole).mockReset().mockResolvedValue(undefined as never)
})

test('renders the account list with identity and status', async () => {
    vi.mocked(usersApi().list).mockResolvedValue({
        data: [
            row(),
            row({
                id: 'u2',
                username: 'spammer',
                email: 'spammer@example.com',
                banned_at: new Date('2026-02-03T00:00:00Z').toISOString(),
                ban_reason: 'vote manipulation',
                email_verified: false,
            }),
        ],
        total: 2,
        offset: 0,
        limit: 25,
    })

    renderPanel()

    expect(await screen.findByText('alice')).toBeTruthy()
    // Scoped to the table: "Banned" is also a status-filter <option>, so a
    // page-wide query is ambiguous.
    const table = within(screen.getByRole('table'))
    expect(table.getByText('spammer')).toBeTruthy()
    expect(table.getByText('Banned')).toBeTruthy()
    expect(table.getByText('Unverified')).toBeTruthy()
    expect(screen.getByText('2 accounts')).toBeTruthy()
})

test('status filter is sent to the API', async () => {
    renderPanel()
    await screen.findByText('alice')

    fireEvent.change(screen.getByLabelText('Filter by account status'), { target: { value: 'banned' } })

    // Offset must reset to 0 on a filter change, or a filtered view opens on an
    // empty page.
    await waitFor(() => {
        expect(usersApi().list).toHaveBeenCalledWith(expect.objectContaining({ status: 'banned', offset: 0 }))
    })
})

test('search is debounced into one request with the query', async () => {
    renderPanel()
    await screen.findByText('alice')
    vi.mocked(usersApi().list).mockClear()

    fireEvent.change(screen.getByLabelText('Search users'), { target: { value: 'spam' } })

    await waitFor(() => {
        expect(usersApi().list).toHaveBeenCalledWith(expect.objectContaining({ search: 'spam' }))
    }, { timeout: 2000 })
})

test('bulk ban sends the selected ids', async () => {
    vi.mocked(usersApi().list).mockResolvedValue({
        data: [row(), row({ id: 'u2', username: 'bob', email: 'bob@example.com' })],
        total: 2,
        offset: 0,
        limit: 25,
    })
    vi.mocked(usersApi().bulk).mockResolvedValue({ results: [], succeeded: 2, failed: 0 })

    renderPanel()
    await screen.findByText('alice')

    fireEvent.click(screen.getByLabelText('Select alice'))
    fireEvent.click(screen.getByLabelText('Select bob'))

    expect(screen.getByText('2 selected')).toBeTruthy()

    // The confirm dialog is shown; confirming it must carry both ids.
    fireEvent.click(screen.getByRole('button', { name: /^Ban$/ }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: /Confirm|Yes|Ban/i }))

    await waitFor(() => {
        expect(usersApi().bulk).toHaveBeenCalledWith(
            expect.objectContaining({ action: 'ban', ids: ['u1', 'u2'] }),
        )
    })
})

test('a skipped account is reported instead of a bare success', async () => {
    vi.mocked(usersApi().list).mockResolvedValue({
        data: [row()],
        total: 1,
        offset: 0,
        limit: 25,
    })
    // Partial success is normal: the last administrator, or your own account,
    // is refused while the rest of the batch succeeds.
    vi.mocked(usersApi().bulk).mockResolvedValue({
        results: [{ id: 'u1', ok: false, error: 'cannot remove the last active administrator' }],
        succeeded: 0,
        failed: 1,
    })

    renderPanel()
    await screen.findByText('alice')

    fireEvent.click(screen.getByLabelText('Select alice'))
    fireEvent.click(screen.getByRole('button', { name: /^Ban$/ }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: /Confirm|Yes|Ban/i }))

    expect(await screen.findByText('0 succeeded, 1 skipped')).toBeTruthy()
})

test('the ban dialog requires nothing but sends the reason', async () => {
    renderPanel()
    await screen.findByText('alice')

    fireEvent.click(screen.getByLabelText('Suspend alice'))
    const dialog = await screen.findByRole('dialog')

    fireEvent.change(within(dialog).getByLabelText('Reason'), { target: { value: 'spam' } })
    fireEvent.click(within(dialog).getByRole('button', { name: /Suspend account/i }))

    await waitFor(() => {
        expect(usersApi().ban).toHaveBeenCalledWith('u1', 'spam')
    })
})
