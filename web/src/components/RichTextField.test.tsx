import { render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import RichTextField from './RichTextField'

const SEEDED = `# Approach

Sort the array, then take the two largest.

Inline math is $O(n \\log n)$ and a bound like $1 \\le n \\le 2\\cdot10^5$.
`

/**
 * The statement tab mounts five of these at once. Mounting several used to throw
 * inside the editor's effect ("Cannot read properties of null (reading
 * 'commands')") and take the whole form down, so the count is the assertion.
 */
test('several fields mount together without throwing', async () => {
  render(
    <div>
      <RichTextField ariaLabel="one" value={SEEDED} onChange={vi.fn()} />
      <RichTextField ariaLabel="two" value="Two integers $n$ and $q$." onChange={vi.fn()} />
      <RichTextField ariaLabel="three" value="" onChange={vi.fn()} />
    </div>,
  )

  await waitFor(() => expect(document.querySelectorAll('.tiptap')).toHaveLength(3))
})

test('renders the stored markdown as formatted content, not raw text', async () => {
  render(<RichTextField ariaLabel="statement" value={SEEDED} onChange={vi.fn()} />)

  await waitFor(() => expect(document.querySelector('.tiptap h1')).toBeTruthy())
  expect(document.querySelector('.tiptap h1')?.textContent).toBe('Approach')
  expect(document.querySelector('.tiptap')?.textContent).not.toContain('$')
})

test('honours minHeight so short fields stay short', async () => {
  render(<RichTextField ariaLabel="explanation" value="note" onChange={vi.fn()} minHeight="80px" />)

  await waitFor(() => expect(document.querySelector('.tiptap')).toBeTruthy())
  const surface = document.querySelector('.tiptap')?.parentElement as HTMLElement
  expect(surface.style.minHeight).toBe('80px')
})

test('labels the field when a label is given', () => {
  render(<RichTextField ariaLabel="Input Format" label="Input Format" value="" onChange={vi.fn()} />)
  expect(screen.getByText('Input Format')).toBeInTheDocument()
})
