import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import StatementTab from './StatementTab'
import type { ProblemFormState } from '../../types/problem-workspace'

const baseState: ProblemFormState = {
  title: 'Test Problem',
  description: 'Description here',
  inputFormat: 'Two ints',
  outputFormat: 'Sum',
  hint: 'Think',
  timeLimit: 1000,
  memoryLimit: 262144,
  difficulty: 'easy',
  tags: 'math, ad-hoc',
  sampleCases: [{ input: '1 2', output: '3', explanation: '' }],
  testcases: [],
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

test('renders all statement form fields', () => {
  const onUpdate = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={onUpdate} onSave={vi.fn()} saving={false} />)

  expect(screen.getByDisplayValue('Test Problem')).toBeInTheDocument()
  expect(screen.getByDisplayValue('Description here')).toBeInTheDocument()
  expect(screen.getByDisplayValue('Two ints')).toBeInTheDocument()
  expect(screen.getByDisplayValue('Sum')).toBeInTheDocument()
  expect(screen.getByDisplayValue('1000')).toBeInTheDocument()
  expect(screen.getByDisplayValue('262144')).toBeInTheDocument()
  expect(screen.getByDisplayValue('math, ad-hoc')).toBeInTheDocument()
})

test('calls onUpdate when title changes', () => {
  const onUpdate = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={onUpdate} onSave={vi.fn()} saving={false} />)

  const titleInput = screen.getByDisplayValue('Test Problem')
  fireEvent.change(titleInput, { target: { value: 'New Title' } })

  expect(onUpdate).toHaveBeenCalledWith('title', 'New Title')
})

test('calls onSave when save button clicked', () => {
  const onSave = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={vi.fn()} onSave={onSave} saving={false} />)

  fireEvent.click(screen.getByText('Save Statement'))
  expect(onSave).toHaveBeenCalledTimes(1)
})

test('adds a new sample case', () => {
  const onUpdate = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={onUpdate} onSave={vi.fn()} saving={false} />)

  fireEvent.click(screen.getByText('+ Add Sample'))
  expect(onUpdate).toHaveBeenCalledWith(
    'sampleCases',
    [...baseState.sampleCases, { input: '', output: '', explanation: '' }]
  )
})

test('removes a sample case', () => {
  const onUpdate = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={onUpdate} onSave={vi.fn()} saving={false} />)

  const removeButton = screen.getByText('Remove')
  fireEvent.click(removeButton)

  expect(onUpdate).toHaveBeenCalledWith('sampleCases', [])
})
