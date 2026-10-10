import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import ContestList from './ContestList'
import { ThemeProvider } from '../context/ThemeContext'
import { api } from '../lib/api'

vi.mock('../lib/api', async (importOriginal) => {
    const actual = await importOriginal<typeof import('../lib/api')>()
    return {
        ...actual,
        api: {
            contests: {
                list: vi.fn(),
            },
        },
    }
})

function renderPage() {
    return render(
        <MemoryRouter>
            <ThemeProvider>
                <ContestList />
            </ThemeProvider>
        </MemoryRouter>
    )
}

const futureStart = new Date(Date.now() + 3600_000).toISOString()
const futureEnd = new Date(Date.now() + 7200_000).toISOString()
const pastStart = new Date(Date.now() - 7200_000).toISOString()
const pastEnd = new Date(Date.now() - 3600_000).toISOString()

beforeEach(() => {
    vi.mocked(api.contests.list).mockReset()
})

/** The page only reads a subset of Contest; the API type is wider. */
function contest(overrides: Record<string, unknown>) {
    return { type: 'acm', division: 0, ...overrides }
}

test('renders upcoming and past contests split by status', async () => {
    vi.mocked(api.contests.list).mockResolvedValue({
        data: [
            contest({ id: 'c1', title: 'Upcoming Cup', type: 'acm', start_time: futureStart, end_time: futureEnd, division: 2 }),
            contest({ id: 'c2', title: 'Old Round', type: 'oi', start_time: pastStart, end_time: pastEnd, division: 0 }),
        ],
        total: 2,
    } as never)

    renderPage()

    await waitFor(() => expect(screen.getByText('Upcoming Cup')).toBeInTheDocument())
    expect(screen.getByText('Old Round')).toBeInTheDocument()
    expect(screen.getByText('Current or Upcoming Contests')).toBeInTheDocument()
    expect(screen.getByText('Past Contests')).toBeInTheDocument()
    expect(screen.getByText('2 contests available')).toBeInTheDocument()
})

test('clicking a division button refetches with that division', async () => {
    vi.mocked(api.contests.list).mockResolvedValue({ data: [], total: 0 } as never)

    renderPage()
    await waitFor(() => expect(api.contests.list).toHaveBeenCalledWith(0, 30, undefined))

    fireEvent.click(screen.getByRole('button', { name: 'Div. 2' }))

    await waitFor(() => expect(api.contests.list).toHaveBeenCalledWith(0, 30, 2))
})

test('search input debounces and filters loaded contests client-side', async () => {
    vi.useFakeTimers()
    vi.mocked(api.contests.list).mockResolvedValue({
        data: [
            contest({ id: 'c1', title: 'Alpha Cup', type: 'acm', start_time: futureStart, end_time: futureEnd }),
            contest({ id: 'c2', title: 'Beta Round', type: 'oi', start_time: futureStart, end_time: futureEnd }),
        ],
        total: 2,
    } as never)

    renderPage()

    await vi.waitFor(() => expect(screen.getByText('Alpha Cup')).toBeInTheDocument())

    fireEvent.change(screen.getByPlaceholderText('Search contests...'), { target: { value: 'alpha' } })
    await vi.advanceTimersByTimeAsync(300)

    await vi.waitFor(() => {
        expect(screen.getByText('Alpha Cup')).toBeInTheDocument()
        expect(screen.queryByText('Beta Round')).not.toBeInTheDocument()
    })

    vi.useRealTimers()
})

test('shows empty states when no contests exist', async () => {
    vi.mocked(api.contests.list).mockResolvedValue({ data: [], total: 0 } as never)

    renderPage()

    await waitFor(() => expect(screen.getByText('No upcoming contests at the moment.')).toBeInTheDocument())
    expect(screen.getByText('No past contests found.')).toBeInTheDocument()
})
