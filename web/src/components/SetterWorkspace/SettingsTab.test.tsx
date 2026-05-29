import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import SettingsTab from './SettingsTab'

test('renders visibility toggle and danger zone', () => {
  render(
    <SettingsTab
      visible={true}
      saving={false}
      onUpdateVisibility={vi.fn()}
      onToggleVisibility={vi.fn()}
      onDelete={vi.fn()}
    />
  )
  expect(screen.getByText('Workspace Settings')).toBeInTheDocument()
  expect(screen.getByText('Problem Visibility (Visible to Solvers)')).toBeInTheDocument()
  expect(screen.getByText('Danger Zone')).toBeInTheDocument()
  expect(screen.getByText('Delete Problem')).toBeInTheDocument()
})

test('calls onUpdateVisibility when Update Visibility clicked', () => {
  const onUpdateVisibility = vi.fn()
  render(
    <SettingsTab
      visible={true}
      saving={false}
      onUpdateVisibility={onUpdateVisibility}
      onToggleVisibility={vi.fn()}
      onDelete={vi.fn()}
    />
  )
  fireEvent.click(screen.getByText('Update Visibility'))
  expect(onUpdateVisibility).toHaveBeenCalledTimes(1)
})

test('calls onToggleVisibility when checkbox clicked', () => {
  const onToggleVisibility = vi.fn()
  render(
    <SettingsTab
      visible={true}
      saving={false}
      onUpdateVisibility={vi.fn()}
      onToggleVisibility={onToggleVisibility}
      onDelete={vi.fn()}
    />
  )
  fireEvent.click(screen.getByRole('checkbox'))
  expect(onToggleVisibility).toHaveBeenCalledWith(false)
})

test('calls onDelete when Delete Problem clicked', () => {
  const onDelete = vi.fn()
  render(
    <SettingsTab
      visible={true}
      saving={false}
      onUpdateVisibility={vi.fn()}
      onToggleVisibility={vi.fn()}
      onDelete={onDelete}
    />
  )
  fireEvent.click(screen.getByText('Delete Problem'))
  expect(onDelete).toHaveBeenCalledTimes(1)
})
