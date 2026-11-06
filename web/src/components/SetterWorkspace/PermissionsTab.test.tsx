import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import PermissionsTab from './PermissionsTab'
import type { Collaborator } from '../../types/problem-workspace'

const collaborators: Collaborator[] = [
  { problem_id: 'p1', user_id: 'u1', username: 'owneruser', access_level: 'owner' },
  { problem_id: 'p1', user_id: 'u2', username: 'coauthor1', access_level: 'co_author' },
  { problem_id: 'p1', user_id: 'u3', username: 'tester1', access_level: 'tester' },
]

test('renders collaborators table', () => {
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  expect(screen.getByText('owneruser')).toBeInTheDocument()
  expect(screen.getByText('coauthor1')).toBeInTheDocument()
  expect(screen.getByText('tester1')).toBeInTheDocument()
})

test('shows Primary Owner badge for owner', () => {
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  expect(screen.getByText('Primary Owner')).toBeInTheDocument()
})

test('shows correct access level badges', () => {
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  // The requirement is that the three levels are visually distinguishable and
  // labelled. Pinning exact palette class names made this test fail on every
  // theme change while telling us nothing about what a user sees.
  const badgeFor = (level: string) => screen.getByText(level).closest('span')
  const classes = ['owner', 'co_author', 'tester'].map(l => badgeFor(l)!.className)
  expect(new Set(classes).size, 'each access level needs its own styling').toBe(3)
  for (const level of ['owner', 'co_author', 'tester']) {
    expect(badgeFor(level)).toHaveTextContent(level)
  }
})

test('does not show Remove button for owner', () => {
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  // Only 2 Remove buttons (for non-owners)
  const removeButtons = screen.getAllByText('Remove')
  expect(removeButtons).toHaveLength(2)
})

test('calls onRemove when Remove clicked', () => {
  const onRemove = vi.fn()
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={onRemove}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  const removeButtons = screen.getAllByText('Remove')
  fireEvent.click(removeButtons[0])
  expect(onRemove).toHaveBeenCalledWith('u2')
})

test('calls onAdd when Share Permissions clicked', () => {
  const onAdd = vi.fn()
  render(
    <PermissionsTab
      collaborators={[]}
      newUsername="newuser"
      newAccessLevel="tester"
      onAdd={onAdd}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  fireEvent.click(screen.getByText('Share Permissions'))
  expect(onAdd).toHaveBeenCalledTimes(1)
})
