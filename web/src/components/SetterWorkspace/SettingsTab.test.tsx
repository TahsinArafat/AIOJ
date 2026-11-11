import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import SettingsTab from './SettingsTab'

/**
 * Publishing became an admin decision, so this tab no longer offers a visibility
 * switch: it reports the review state and offers the author's action (submit or
 * resubmit). The old checkbox sent `visible`, which the API now ignores for
 * non-admins — it looked like it worked and did nothing.
 */

function renderTab(overrides: Partial<React.ComponentProps<typeof SettingsTab>> = {}) {
  return render(
    <SettingsTab
      status="draft"
      visible={false}
      saving={false}
      onSubmitForReview={vi.fn()}
      onDelete={vi.fn()}
      {...overrides}
    />,
  )
}

test('shows the review state instead of a visibility switch', () => {
  renderTab()
  expect(screen.getByText('Workspace Settings')).toBeInTheDocument()
  expect(screen.getByText('Draft')).toBeInTheDocument()
  // The control that no longer does anything must be gone.
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  expect(screen.queryByText(/Problem Visibility/i)).not.toBeInTheDocument()
})

test('offers Submit for review on a draft', () => {
  const onSubmitForReview = vi.fn()
  renderTab({ status: 'draft', onSubmitForReview })

  fireEvent.click(screen.getByText('Submit for review'))

  expect(onSubmitForReview).toHaveBeenCalledTimes(1)
})

test('offers Resubmit and shows the reviewer note on a rejection', () => {
  const onSubmitForReview = vi.fn()
  renderTab({ status: 'rejected', reviewNote: 'Test 3 violates the constraints', onSubmitForReview })

  expect(screen.getByText('Rejected')).toBeInTheDocument()
  expect(screen.getByText('Test 3 violates the constraints')).toBeInTheDocument()
  fireEvent.click(screen.getByText('Resubmit for review'))
  expect(onSubmitForReview).toHaveBeenCalledTimes(1)
})

test('a pending problem cannot be submitted again', () => {
  renderTab({ status: 'pending' })
  expect(screen.getByText('Awaiting review')).toBeInTheDocument()
  expect(screen.queryByText(/Submit for review/)).not.toBeInTheDocument()
})

test('a published problem offers no submit action', () => {
  renderTab({ status: 'published', visible: true })
  expect(screen.getByText('Published')).toBeInTheDocument()
  expect(screen.getByText('in the archive')).toBeInTheDocument()
  expect(screen.queryByText(/Submit for review/)).not.toBeInTheDocument()
})

test('calls onDelete when Delete Problem clicked', () => {
  const onDelete = vi.fn()
  renderTab({ onDelete })

  fireEvent.click(screen.getByText('Delete Problem'))

  expect(onDelete).toHaveBeenCalledTimes(1)
})
