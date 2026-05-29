import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import SetterProblemWorkspace from './SetterProblemWorkspace'
import { api } from '../lib/api'
import type { Mock } from 'vitest'

vi.mock('../lib/api', () => ({
  api: {
    problems: {
      get: vi.fn(),
      update: vi.fn().mockResolvedValue({}),
      getPermissions: vi.fn().mockResolvedValue({ data: [] }),
      delete: vi.fn().mockResolvedValue({}),
    },
  },
  getAccessToken: vi.fn().mockReturnValue('mock-token'),
}))

const mockProblem = {
  id: 'prob-1',
  slug: 'test-problem',
  title: 'Test Problem',
  description: '# Description',
  input_format: 'Two integers',
  output_format: 'Sum',
  hint: 'Think harder',
  time_limit: 1000,
  memory_limit: 262144,
  difficulty: 'easy',
  tags: ['math', 'ad-hoc'],
  sample_cases: [{ input: '1 2', output: '3', explanation: '' }],
  testcase_score: [],
  checker_type: 'exact',
  float_epsilon: 1e-6,
  spj: false,
  spj_language: 'cpp-gpp-64',
  spj_source_code: '',
  interactive: false,
  interactor_language: 'cpp-gpp-64',
  interactor_source_code: '',
  visible: true,
}

beforeEach(() => {
  vi.clearAllMocks()
  ;(api.problems.get as Mock).mockResolvedValue(mockProblem)
})

function renderWorkspace() {
  return render(
    <MemoryRouter initialEntries={['/setter/problem/test-problem']}>
      <Routes>
        <Route path="/setter/problem/:slug" element={<SetterProblemWorkspace />} />
      </Routes>
    </MemoryRouter>
  )
}

test('loads and displays problem title in header', async () => {
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Test Problem')).toBeInTheDocument()
  })
})

test('renders all 5 tab buttons', async () => {
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Statement & Details')).toBeInTheDocument()
    expect(screen.getByText('Test Cases / Data')).toBeInTheDocument()
    expect(screen.getByText('Checker / Special Judge')).toBeInTheDocument()
    expect(screen.getByText('Collaborators')).toBeInTheDocument()
    expect(screen.getByText('Workspace Settings')).toBeInTheDocument()
  })
})

test('switches tabs when clicked', async () => {
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Test Problem')).toBeInTheDocument()
  })

  // Click test cases tab
  fireEvent.click(screen.getByText('Test Cases / Data'))
  await waitFor(() => {
    expect(screen.getByText('Upload Testcase Package (ZIP)')).toBeInTheDocument()
  })

  // Click checker tab
  fireEvent.click(screen.getByText('Checker / Special Judge'))
  await waitFor(() => {
    expect(screen.getByText('Checker & Special Judge Configuration')).toBeInTheDocument()
  })

  // Click collaborators tab
  fireEvent.click(screen.getByText('Collaborators'))
  await waitFor(() => {
    expect(screen.getByText('Problem Collaborators')).toBeInTheDocument()
  })

  // Click settings tab
  fireEvent.click(screen.getByText('Workspace Settings'))
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: 'Workspace Settings' })).toBeInTheDocument()
  })
})

test('save statement calls api.problems.update with correct payload', async () => {
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Test Problem')).toBeInTheDocument()
  })

  const saveBtn = screen.getByText('Save Statement')
  fireEvent.click(saveBtn)

  await waitFor(() => {
    expect(api.problems.update).toHaveBeenCalledTimes(1)
    expect(api.problems.update).toHaveBeenCalledWith('test-problem', expect.objectContaining({
      title: 'Test Problem',
      difficulty: 'easy',
      time_limit: 1000,
    }))
  })
})

test('shows error banner on API failure', async () => {
  ;(api.problems.getPermissions as Mock).mockRejectedValue(new Error('Network error'))
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Network error')).toBeInTheDocument()
  })
})
