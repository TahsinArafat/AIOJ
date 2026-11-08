import { render, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import VisualEditor from './index'

/** Two constraints bounds, as stored: one per line, single newlines. */
const twoLines = '$1 \\le n \\le 2\\cdot10^5$\n$1 \\le q \\le 2\\cdot10^5$'

test('keeps stored line-per-line markdown on separate lines', () => {
  render(<VisualEditor content={twoLines} onChange={vi.fn()} />)
  const paragraphs = document.querySelectorAll('.tiptap p')

  expect(paragraphs).toHaveLength(1)
  expect(paragraphs[0].querySelectorAll('br:not(.ProseMirror-trailingBreak)')).toHaveLength(1)
})

test('keeps blank-line-separated markdown as separate paragraphs', () => {
  render(<VisualEditor content={'first\n\nsecond'} onChange={vi.fn()} />)
  const paragraphs = document.querySelectorAll('.tiptap p')

  expect(paragraphs).toHaveLength(2)
  expect(paragraphs[0].textContent).toBe('first')
  expect(paragraphs[1].textContent).toBe('second')
})

// The editor used to seed once and ignore `content` afterwards, so state written
// from outside (an inserted image) never appeared and the next keystroke
// overwrote it.
test('re-seeds when content is replaced from outside', () => {
  const { rerender } = render(<VisualEditor content={'old'} onChange={vi.fn()} />)
  rerender(<VisualEditor content={'new\n\nbody'} onChange={vi.fn()} />)

  const paragraphs = document.querySelectorAll('.tiptap p')
  expect(paragraphs).toHaveLength(2)
  expect(paragraphs[0].textContent).toBe('new')
})

// Stored math has to arrive as a math node, not as `$...$` text: text is escaped
// on the way back out, which turned `\le` into `\\le` and rendered as "l e".
test('renders stored latex as math', async () => {
  render(<VisualEditor content={twoLines} onChange={vi.fn()} />)

  await waitFor(() => expect(document.querySelectorAll('.katex')).toHaveLength(2))
  expect(document.querySelector('.tiptap')?.textContent).not.toContain('$')
})
