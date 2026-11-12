import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import type { ComponentProps } from 'react'
import { expect, test, vi } from 'vitest'
import TestCasesTab from './TestCasesTab'
import type { TestCase } from '../../types/problem-workspace'
import type { TestcaseFile } from '../../types/api'

const testcases: TestCase[] = [
  { input_name: '01.in', output_name: '01.out', score: 10 },
  { input_name: '02.in', output_name: '02.out', score: 20 },
]

const files: TestcaseFile[] = [
  { name: '01.in', size: 12, role: 'input', pair: '01.out' },
  { name: '01.out', size: 8, role: 'output', pair: '01.in' },
  { name: '03.in', size: 4, role: 'input' },
  { name: 'notes.md', size: 2048, role: 'other' },
]

function renderTab(overrides: Partial<ComponentProps<typeof TestCasesTab>> = {}) {
  const props: ComponentProps<typeof TestCasesTab> = {
    testcases: [],
    files: [],
    filesLoading: false,
    newTestCase: { input_name: '', output_name: '', score: 10 },
    batchScore: 10,
    batchApplying: false,
    onAdd: vi.fn(),
    onRemove: vi.fn(),
    onScoreChange: vi.fn(),
    onSubtaskChange: vi.fn(),
    onBatchSet: vi.fn(),
    onUpload: vi.fn(),
    onNewTestCaseChange: vi.fn(),
    onBatchScoreChange: vi.fn(),
    onReloadFiles: vi.fn(),
    onLoadFile: vi.fn().mockResolvedValue({ name: '01.in', size: 12, content: '1 2\n', binary: false }),
    onSaveFile: vi.fn().mockResolvedValue(true),
    onCreatePair: vi.fn().mockResolvedValue(true),
    onDeleteFile: vi.fn(),
    onDownloadFile: vi.fn(),
    onDownloadAll: vi.fn(),
    ...overrides,
  }
  render(<TestCasesTab {...props} />)
  return props
}

test('shows empty states when nothing is uploaded', () => {
  renderTab()
  expect(screen.getByText(/No test data files yet/)).toBeInTheDocument()
  expect(screen.getByText(/No testcase scores registered/)).toBeInTheDocument()
})

test('lists test data files with role, size and pair state', () => {
  renderTab({ files, testcases })
  expect(screen.getByText('notes.md')).toBeInTheDocument()
  expect(screen.getAllByText('input').length).toBe(2)
  expect(screen.getAllByText('output').length).toBe(1)
  expect(screen.getByText('2.0 KB')).toBeInTheDocument()
  expect(screen.getByText('unpaired')).toBeInTheDocument()
})

test('renders testcase table with scores and totals', () => {
  renderTab({ testcases })
  expect(screen.getByLabelText('Score for 01.in')).toHaveValue(10)
  expect(screen.getByLabelText('Score for 02.in')).toHaveValue(20)
  expect(screen.getByText(/Total Testcases: 2/)).toBeInTheDocument()
  expect(screen.getByText(/Total Points: 30/)).toBeInTheDocument()
})

test('viewing a file loads it into a read-only dialog', async () => {
  const onLoadFile = vi.fn().mockResolvedValue({ name: '01.in', size: 12, content: '1 2\n', binary: false })
  renderTab({ files, onLoadFile })

  fireEvent.click(screen.getAllByText('View')[0])

  await waitFor(() => expect(onLoadFile).toHaveBeenCalledWith('01.in'))
  const textarea = await screen.findByLabelText('Test case content')
  expect(textarea).toHaveValue('1 2\n')
  expect(textarea).toHaveAttribute('readonly')
})

test('editing a file saves the changed content', async () => {
  const onLoadFile = vi.fn().mockResolvedValue({ name: '01.in', size: 12, content: '1 2\n', binary: false })
  const onSaveFile = vi.fn().mockResolvedValue(true)
  renderTab({ files, onLoadFile, onSaveFile })

  fireEvent.click(screen.getAllByText('Edit')[0])
  const textarea = await screen.findByLabelText('Test case content')
  fireEvent.change(textarea, { target: { value: '3 4\n' } })
  fireEvent.click(screen.getByText('Save file'))

  await waitFor(() => expect(onSaveFile).toHaveBeenCalledWith('01.in', '3 4\n'))
})

test('binary files explain why they cannot be edited', async () => {
  const onLoadFile = vi.fn().mockResolvedValue({ name: '01.out', size: 8, content: '', binary: true })
  renderTab({ files, onLoadFile })

  fireEvent.click(screen.getAllByText('View')[1])
  expect(await screen.findByText(/not valid UTF-8/)).toBeInTheDocument()
  expect(screen.queryByLabelText('Test case content')).not.toBeInTheDocument()
})

test('download and delete act on the right file', () => {
  const onDownloadFile = vi.fn()
  const onDeleteFile = vi.fn()
  renderTab({ files, onDownloadFile, onDeleteFile })

  fireEvent.click(screen.getAllByText('Download')[0])
  expect(onDownloadFile).toHaveBeenCalledWith('01.in')

  fireEvent.click(screen.getAllByText('Delete')[3])
  expect(onDeleteFile).toHaveBeenCalledWith('notes.md')
})

test('write-a-test-case form creates a pair with the typed content', async () => {
  const onCreatePair = vi.fn().mockResolvedValue(true)
  renderTab({ onCreatePair })

  const createButton = screen.getByText('Create test case')
  expect(createButton).toBeDisabled()

  fireEvent.change(screen.getByLabelText('Test case name'), { target: { value: '03' } })
  fireEvent.change(screen.getByLabelText('Input'), { target: { value: '5 6' } })
  fireEvent.change(screen.getByLabelText('Expected output'), { target: { value: '11' } })
  fireEvent.click(createButton)

  await waitFor(() =>
    expect(onCreatePair).toHaveBeenCalledWith({ stem: '03', input: '5 6', output: '11', score: 10 }),
  )
})

test('editing an inline score commits on blur', () => {
  const onScoreChange = vi.fn()
  renderTab({ testcases, onScoreChange })

  const input = screen.getByLabelText('Score for 01.in')
  fireEvent.change(input, { target: { value: '25' } })
  fireEvent.blur(input)

  expect(onScoreChange).toHaveBeenCalledWith(0, 25)
})

test('calls onBatchSet when Apply to All clicked', () => {
  const onBatchSet = vi.fn()
  renderTab({ testcases, onBatchSet })
  fireEvent.click(screen.getByText('Apply to All'))
  expect(onBatchSet).toHaveBeenCalledTimes(1)
})

test('calls onAdd when Add file match clicked', () => {
  const onAdd = vi.fn()
  renderTab({ testcases, onAdd })
  fireEvent.click(screen.getByText('Add file match'))
  expect(onAdd).toHaveBeenCalledTimes(1)
})

test('calls onRemove with the row index', () => {
  const onRemove = vi.fn()
  renderTab({ testcases, onRemove })
  fireEvent.click(screen.getAllByText('Remove')[0])
  expect(onRemove).toHaveBeenCalledWith(0)
})

// Subtask grouping is what makes "partial" scoring mean anything: without it
// every problem falls back to a single all-or-nothing bucket.
test('shows a subtask field per case', () => {
  renderTab({ testcases: [{ input_name: '01.in', output_name: '01.out', score: 10, subtask_id: 2 }] })

  const field = screen.getByLabelText('Subtask for 01.in')
  expect(field).toHaveValue(2)
})

test('commits a subtask change on blur', () => {
  const onSubtaskChange = vi.fn()
  renderTab({
    testcases: [{ input_name: '01.in', output_name: '01.out', score: 10 }],
    onSubtaskChange,
  })

  const field = screen.getByLabelText('Subtask for 01.in')
  fireEvent.change(field, { target: { value: '3' } })
  fireEvent.blur(field)

  expect(onSubtaskChange).toHaveBeenCalledWith(0, 3)
})

// Clearing the field detaches the case from every subtask; it is still judged.
test('clearing the subtask field sends 0', () => {
  const onSubtaskChange = vi.fn()
  renderTab({
    testcases: [{ input_name: '01.in', output_name: '01.out', score: 10, subtask_id: 5 }],
    onSubtaskChange,
  })

  const field = screen.getByLabelText('Subtask for 01.in')
  fireEvent.change(field, { target: { value: '' } })
  fireEvent.blur(field)

  expect(onSubtaskChange).toHaveBeenCalledWith(0, 0)
})

test('does not save when the subtask is unchanged', () => {
  const onSubtaskChange = vi.fn()
  renderTab({
    testcases: [{ input_name: '01.in', output_name: '01.out', score: 10, subtask_id: 2 }],
    onSubtaskChange,
  })

  const field = screen.getByLabelText('Subtask for 01.in')
  fireEvent.blur(field)

  expect(onSubtaskChange).not.toHaveBeenCalled()
})
