import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import ActivityFeed from './ActivityFeed'
import { api } from '../lib/api'

vi.mock('../lib/api', () => ({
  api: {
    feed: {
      activity: vi.fn(),
    },
  },
}))

const sample = {
  data: [
    {
      id: '1',
      type: 'ac',
      username: 'alice',
      title: 'solved',
      summary: 'Two Sum',
      created_at: new Date().toISOString(),
    },
    {
      id: '2',
      type: 'rating',
      username: 'bob',
      title: 'rating +42 → 1602',
      created_at: new Date().toISOString(),
    },
  ],
}

beforeEach(() => {
  vi.clearAllMocks()
})

function renderFeed() {
  return render(
    <MemoryRouter>
      <ActivityFeed />
    </MemoryRouter>,
  )
}

describe('ActivityFeed', () => {
  it('renders activity items from API', async () => {
    ;(api.feed.activity as Mock).mockResolvedValue(sample)
    renderFeed()
    expect(await screen.findByText('alice')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
    expect(screen.getByText(/rating \+42/)).toBeInTheDocument()
  })

  it('renders empty state on API failure', async () => {
    ;(api.feed.activity as Mock).mockRejectedValue(new Error('down'))
    renderFeed()
    await waitFor(() => {
      expect(screen.getByText(/No activity yet/)).toBeInTheDocument()
    })
  })
})
