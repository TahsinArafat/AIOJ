import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import CheckerTab from './CheckerTab'
import type { ProblemFormState } from '../../types/problem-workspace'

const baseState: ProblemFormState = {
  title: '', description: '', inputFormat: '', outputFormat: '', hint: '',
  timeLimit: 1000, memoryLimit: 262144, difficulty: 'easy', tags: '',
  sampleCases: [], testcases: [],
  checkerType: 'exact',
  floatEpsilon: 1e-6,
  spj: false,
  spjLanguage: 'cpp-gpp-64',
  spjSourceCode: '',
  interactive: false,
  interactorLanguage: 'cpp-gpp-64',
  interactorSourceCode: '',
  visible: true,
}

test('renders checker type selector with all options', () => {
  render(<CheckerTab formState={baseState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Checker & Special Judge Configuration')).toBeInTheDocument()
  expect(screen.getByRole('combobox')).toHaveValue('exact')
})

test('shows epsilon input for float checker types', () => {
  const floatState = { ...baseState, checkerType: 'float_absolute' }
  const { rerender } = render(<CheckerTab formState={floatState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Float Epsilon (Precision Tolerance)')).toBeInTheDocument()

  rerender(<CheckerTab formState={{ ...baseState, checkerType: 'float_relative' }} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Float Epsilon (Precision Tolerance)')).toBeInTheDocument()

  rerender(<CheckerTab formState={{ ...baseState, checkerType: 'exact' }} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.queryByText('Float Epsilon (Precision Tolerance)')).not.toBeInTheDocument()
})

test('shows SPJ editor when checker type is custom', () => {
  const customState = { ...baseState, checkerType: 'custom' }
  render(<CheckerTab formState={customState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Special Judge (SPJ) Sandbox Environment Protocol')).toBeInTheDocument()
  expect(screen.getByText('Advanced SPJ Presets')).toBeInTheDocument()
})

test('shows interactive section when toggled', () => {
  const interactiveState = { ...baseState, interactive: true }
  render(<CheckerTab formState={interactiveState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Interactive Problem (Judge communicates via stdin/stdout)')).toBeInTheDocument()
  expect(screen.getByText('Interactor Language')).toBeInTheDocument()
  expect(screen.getByText('Interactor Source Code')).toBeInTheDocument()
})

test('calls onSave when save button clicked', () => {
  const onSave = vi.fn()
  render(<CheckerTab formState={baseState} onUpdate={vi.fn()} onSave={onSave} saving={false} />)
  fireEvent.click(screen.getByText('Save Checker Configuration'))
  expect(onSave).toHaveBeenCalledTimes(1)
})
