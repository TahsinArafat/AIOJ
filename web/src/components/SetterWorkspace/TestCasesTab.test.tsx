import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import TestCasesTab from './TestCasesTab'
import type { TestCase } from '../../types/problem-workspace'

const testcases: TestCase[] = [
  { input_name: '01.in', output_name: '01.out', score: 10 },
  { input_name: '02.in', output_name: '02.out', score: 20 },
]

test('shows empty state when no testcases', () => {
  render(
    <TestCasesTab
      testcases={[]}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  expect(screen.getByText(/No testcase scores registered/)).toBeInTheDocument()
})

test('renders testcase table with scores', () => {
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  expect(screen.getByText('01.in')).toBeInTheDocument()
  expect(screen.getByText('02.out')).toBeInTheDocument()
  expect(screen.getByText('20 pts')).toBeInTheDocument()
})

test('shows total testcase count and points', () => {
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  expect(screen.getByText(/Total Testcases: 2/)).toBeInTheDocument()
  expect(screen.getByText(/Total Points: 30/)).toBeInTheDocument()
})

test('calls onBatchSet when Apply to All clicked', () => {
  const onBatchSet = vi.fn()
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onBatchSet={onBatchSet}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  fireEvent.click(screen.getByText('Apply to All'))
  expect(onBatchSet).toHaveBeenCalledTimes(1)
})

test('calls onAdd when Add File Match clicked', () => {
  const onAdd = vi.fn()
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '03.in', output_name: '03.out', score: 30 }}
      batchScore={10}
      batchApplying={false}
      onAdd={onAdd}
      onRemove={vi.fn()}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  fireEvent.click(screen.getByText('Add File Match'))
  expect(onAdd).toHaveBeenCalledTimes(1)
})

test('calls onRemove when Remove button clicked', () => {
  const onRemove = vi.fn()
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={onRemove}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  const removeButtons = screen.getAllByText('Remove')
  fireEvent.click(removeButtons[0])
  expect(onRemove).toHaveBeenCalledWith(0)
})
