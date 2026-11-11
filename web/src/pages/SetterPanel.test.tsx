import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, expect, test, vi } from 'vitest'
import { ToastProvider } from '../components/Toast'
import SetterPanel from './SetterPanel'

/**
 * "My Contests" used to show two buttons that went to the same place:
 * "Edit settings" -> /setter/contest/{id}/manage#settings and "Manage" ->
 * /setter/contest/{id}/manage. One row, two links, one destination. These pin
 * that each row offers a single manage action, so a duplicate cannot creep back.
 */

vi.mock('../lib/api', () => ({
  api: {
    problems: {
      listMy: vi.fn(() => Promise.resolve({ data: [], total: 0 })),
      list: vi.fn(() => Promise.resolve({ data: [], total: 0 })),
    },
    contests: {
      listMy: vi.fn(() =>
        Promise.resolve({
          data: [
            { id: 'c1', title: 'Round One', start_time: '2026-03-01T10:00:00Z', format: 'acm', visible: true },
            { id: 'c2', title: 'Round Two', start_time: '2026-04-01T10:00:00Z', format: 'acm', visible: false },
          ],
          total: 2,
        }),
      ),
    },
  },
}))

// The page reads the role straight out of the JWT in localStorage.
function signIn(role: string) {
  const payload = btoa(JSON.stringify({ role }))
  localStorage.setItem('access_token', `h.${payload}.s`)
}

function renderPanel() {
  return render(
    <MemoryRouter>
      <ToastProvider>
        <SetterPanel />
      </ToastProvider>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  localStorage.clear()
  signIn('setter')
})

test('a contest row offers exactly one link to its manage page', async () => {
  renderPanel()

  fireEvent.click(screen.getByText('My Contests'))
  await screen.findByText('Round One')

  const links = screen.getAllByRole('link', { name: /manage|edit settings/i })
  // Two contests => two manage links, one each. Four would mean the duplicate.
  expect(links).toHaveLength(2)

  // And nothing points at the #settings anchor variant any more.
  const settingsLinks = links.filter(l => (l.getAttribute('href') ?? '').includes('#settings'))
  expect(settingsLinks).toHaveLength(0)
})

test('the redundant "Edit settings" label is gone', async () => {
  renderPanel()

  fireEvent.click(screen.getByText('My Contests'))
  await screen.findByText('Round One')

  expect(screen.queryByText('Edit settings')).not.toBeInTheDocument()
  expect(screen.getAllByText('Manage')).toHaveLength(2)
})

test('View and Manage remain distinct destinations', async () => {
  renderPanel()

  fireEvent.click(screen.getByText('My Contests'))
  await screen.findByText('Round One')

  const view = screen.getAllByRole('link', { name: 'View' })[0]
  const manage = screen.getAllByRole('link', { name: 'Manage' })[0]

  expect(view.getAttribute('href')).toBe('/contests/c1')
  // Manage opens the workspace root; the settings tab is reachable inside it.
  expect(manage.getAttribute('href')).toBe('/setter/contest/c1/manage')
})
