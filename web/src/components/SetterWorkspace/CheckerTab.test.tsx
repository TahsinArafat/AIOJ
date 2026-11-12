import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import CheckerTab from './CheckerTab'
import type { ProblemFormState } from '../../types/problem-workspace'

const baseState: ProblemFormState = {
  title: '', description: '', inputFormat: '', outputFormat: '', constraints: '', hint: '',
  timeLimit: 1000, memoryLimit: 262144, difficulty: 'easy', rating: 1500, tags: '',
  sampleCases: [], testcases: [],
  checkerType: 'exact',
  floatEpsilon: 1e-6,
  spj: false,
  spjLanguage: 'cpp-gpp-64',
  spjSourceCode: '',
  interactive: false,
  interactorLanguage: 'cpp-gpp-64',
  interactorSourceCode: '',
  problemType: 'standard',
  scoringMode: 'complete',
  subtaskAggregation: 'min',
}

test('renders checker type selector with all options', () => {
  render(<CheckerTab formState={baseState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Checker & Special Judge Configuration')).toBeInTheDocument()
  // The tab now also carries the problem-type and scoring selects, so address
  // the checker selector by its label rather than by "the one combobox".
  expect(screen.getByLabelText('Checker Method')).toHaveValue('exact')
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

// The backend has always supported output_only and subtask scoring, but nothing
// in the UI could set them, so those paths were unreachable.
test('exposes problem type and scoring mode', () => {
  render(<CheckerTab formState={baseState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)

  expect(screen.getByLabelText('Problem Type')).toHaveValue('standard')
  expect(screen.getByLabelText('Scoring Mode')).toHaveValue('complete')
  // Aggregation is only meaningful for partial scoring.
  expect(screen.queryByLabelText('Subtask Aggregation')).not.toBeInTheDocument()
})

test('shows aggregation only for partial scoring', () => {
  render(<CheckerTab formState={{ ...baseState, scoringMode: 'partial' }} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByLabelText('Subtask Aggregation')).toHaveValue('min')
})

// Picking "interactive" is the same declaration as ticking the box, and the API
// rejects a row where the enum and the boolean disagree.
test('choosing the interactive type also ticks the boolean', () => {
  const onUpdate = vi.fn()
  render(<CheckerTab formState={baseState} onUpdate={onUpdate} onSave={vi.fn()} saving={false} />)

  fireEvent.change(screen.getByLabelText('Problem Type'), { target: { value: 'interactive' } })

  expect(onUpdate).toHaveBeenCalledWith('problemType', 'interactive')
  expect(onUpdate).toHaveBeenCalledWith('interactive', true)
})
