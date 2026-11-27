import { render, screen, waitFor, fireEvent, within } from '@testing-library/react'
import { expect, test, vi, beforeEach } from 'vitest'
import EditorialTab from './EditorialTab'
import { ConfirmProvider } from '../ConfirmDialog'
import { api } from '../../lib/api'

vi.mock('../../lib/api', () => ({
  api: {
    editorials: {
      getByProblem: vi.fn(),
      get: vi.fn(),
      delete: vi.fn(),
      update: vi.fn(),
      create: vi.fn(),
    },
  },
  getAccessToken: vi.fn(() => 'mock-token'),
}))

const mockApi = vi.mocked(api, true)

function renderTab() {
  return render(
    <ConfirmProvider>
      <EditorialTab problemId="p1" isUserAdmin={false} />
    </ConfirmProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockApi.editorials.getByProblem.mockResolvedValue({
    data: [
      { id: 'e1', title: 'Greedy approach', username: 'alice', is_official: true, upvotes: 3 },
      { id: 'e2', title: 'DP approach', username: 'bob', is_official: false, upvotes: 0 },
    ],
  } as never)
})

// The tab was add-only: an editorial could be created and never touched again,
// so a typo in a published editorial was permanent.
test('offers edit and delete on every row', async () => {
  renderTab()

  await waitFor(() => expect(screen.getByText('Greedy approach')).toBeInTheDocument())
  expect(screen.getAllByRole('button', { name: 'Edit' })).toHaveLength(2)
  expect(screen.getAllByRole('button', { name: 'Delete' })).toHaveLength(2)
})

test('editing loads the full editorial so the body can be prefilled', async () => {
  mockApi.editorials.get.mockResolvedValue({
    id: 'e1',
    problem_id: 'p1',
    user_id: 'alice',
    title: 'Greedy approach',
    content: 'Sort, then take the largest.',
    is_official: true,
    upvotes: 3,
  } as never)

  renderTab()
  await waitFor(() => expect(screen.getByText('Greedy approach')).toBeInTheDocument())

  fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[0])

  await waitFor(() => expect(screen.getByText('Edit Editorial')).toBeInTheDocument())
  // The list response carries only the title, so the body has to be fetched.
  expect(mockApi.editorials.get).toHaveBeenCalledWith('e1')
  expect(await screen.findByDisplayValue('Sort, then take the largest.')).toBeInTheDocument()
})

test('deleting asks for confirmation and removes the row', async () => {
  mockApi.editorials.delete.mockResolvedValue(undefined as never)

  renderTab()
  await waitFor(() => expect(screen.getByText('Greedy approach')).toBeInTheDocument())

  fireEvent.click(screen.getAllByRole('button', { name: 'Delete' })[0])

  // Destructive and irreversible, so it goes through the dialog. The dialog's
  // own Delete button shares the name with the row buttons, so scope to it.
  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByText(/cannot be undone/i)).toBeInTheDocument()
  fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

  await waitFor(() => expect(mockApi.editorials.delete).toHaveBeenCalledWith('e1'))
  await waitFor(() => expect(screen.queryByText('Greedy approach')).not.toBeInTheDocument())
  expect(screen.getByText('DP approach')).toBeInTheDocument()
})

test('cancelling the confirmation leaves the editorial alone', async () => {
  renderTab()
  await waitFor(() => expect(screen.getByText('Greedy approach')).toBeInTheDocument())

  fireEvent.click(screen.getAllByRole('button', { name: 'Delete' })[0])
  const dialog = await screen.findByRole('dialog')
  fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

  await waitFor(() => expect(screen.getByText('Greedy approach')).toBeInTheDocument())
  expect(mockApi.editorials.delete).not.toHaveBeenCalled()
})

test('a failed delete reports it instead of dropping the row', async () => {
  mockApi.editorials.delete.mockRejectedValue(new Error('nope'))

  renderTab()
  await waitFor(() => expect(screen.getByText('Greedy approach')).toBeInTheDocument())

  fireEvent.click(screen.getAllByRole('button', { name: 'Delete' })[0])
  const dialog = await screen.findByRole('dialog')
  fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

  await waitFor(() => expect(screen.getByText('Could not delete that editorial.')).toBeInTheDocument())
  expect(screen.getByText('Greedy approach')).toBeInTheDocument()
})
