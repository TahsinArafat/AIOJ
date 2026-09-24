import { act, render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import AdminDashboard from './AdminDashboard'
import { api } from '../lib/api'
import type { Mock } from 'vitest'

/**
 * Regression coverage for the tabbed-workspace hash bug.
 *
 * AdminDashboard, ContestManage and SetterProblemWorkspace all read
 * location.hash only in their useState initializer and had no hashchange
 * listener, so a deep link such as /admin#settings — or a Back/Forward step
 * after mount — changed the URL but left the panel on the default tab. The old
 * effect also wrote window.location.hash itself, fighting hash changes.
 *
 * AdminDashboard is the cheapest full-stack representative of the pattern, so
 * it is exercised end-to-end here; ContestManage and SetterProblemWorkspace
 * received the same change.
 */

// vi.mock factories are hoisted above module init, so the shared mock has to
// be created inside the factory and reached via vi.hoisted().
const mocks = vi.hoisted(() => ({
    listUsers: vi.fn().mockResolvedValue({ data: [] }),
    listApps: vi.fn().mockResolvedValue({ data: [] }),
}))

vi.mock('../lib/api', () => ({
    api: {
        admin: {
            listUsers: mocks.listUsers,
            listApps: mocks.listApps,
            updateRole: vi.fn(),
            reviewApp: vi.fn(),
            settings: { list: vi.fn().mockResolvedValue({ data: [] }), update: vi.fn() },
            botAccounts: { list: vi.fn().mockResolvedValue({ data: [] }), create: vi.fn(), update: vi.fn(), delete: vi.fn(), testLogin: vi.fn() },
            languages: { list: vi.fn().mockResolvedValue({ data: [] }), templates: vi.fn().mockResolvedValue({ data: [] }) },
            remoteLanguages: { list: vi.fn().mockResolvedValue({ data: [] }) },
            submissions: { pendingRemote: vi.fn().mockResolvedValue({ data: [] }), refresh: vi.fn(), rejudge: vi.fn() },
            backups: { list: vi.fn().mockResolvedValue([]) },
            aiModels: { list: vi.fn().mockResolvedValue({ data: [] }) },
        },
    },
    getAccessToken: vi.fn().mockReturnValue('mock-token'),
}))

beforeEach(() => {
    vi.clearAllMocks()
    mocks.listUsers.mockResolvedValue({ data: [] })
    mocks.listApps.mockResolvedValue({ data: [] })
    window.location.hash = ''
})

function renderAdmin() {
    return render(
        <MemoryRouter initialEntries={['/admin']}>
            <Routes>
                <Route path="/admin" element={<AdminDashboard />} />
            </Routes>
        </MemoryRouter>
    )
}

function setHash(hash: string) {
    // Dispatching hashchange is what the browser does on a Back/Forward step;
    // act() keeps the resulting state update inside React's act scope.
    act(() => {
        window.location.hash = hash
        window.dispatchEvent(new HashChangeEvent('hashchange'))
    })
}

function activeTabLabel(): string | undefined {
    return document.querySelector('main nav button.bg-blue-50, nav button.bg-blue-50')?.textContent?.trim()
}

test('admin panel starts on the tab named in the URL hash', async () => {
    window.location.hash = 'bots'
    renderAdmin()
    await waitFor(() => {
        expect(activeTabLabel()).toBe('Bot Accounts')
    })
})

test('changing the hash after mount switches the visible panel', async () => {
    renderAdmin()
    await waitFor(() => {
        expect(activeTabLabel()).toBe('Users')
    })

    // What a Back/Forward step or a hand-edited hash produces.
    setHash('settings')

    await waitFor(() => {
        expect(activeTabLabel()).toBe('Settings')
    })
})

test('an unknown hash segment is ignored instead of blanking the panel', async () => {
    renderAdmin()
    await waitFor(() => {
        expect(activeTabLabel()).toBe('Users')
    })

    setHash('not-a-tab')

    await waitFor(() => {
        expect(activeTabLabel()).toBe('Users')
    })
})

test('clicking a tab updates the hash without pushing history entries', async () => {
    renderAdmin()
    await waitFor(() => {
        expect(activeTabLabel()).toBe('Users')
    })

    const replaceState = vi.spyOn(window.history, 'replaceState')
    const pushState = vi.spyOn(window.history, 'pushState')

    fireEvent.click(screen.getByRole('button', { name: 'Applications' }))

    await waitFor(() => {
        expect(activeTabLabel()).toBe('Applications')
    })
    expect(window.location.hash).toBe('#applications')
    expect(replaceState).toHaveBeenCalled()
    expect(pushState).not.toHaveBeenCalled()
})
