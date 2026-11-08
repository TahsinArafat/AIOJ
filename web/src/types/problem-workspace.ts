export interface TestCase {
  input_name: string
  output_name: string
  score: number
}

export interface Collaborator {
  problem_id: string
  user_id: string
  username?: string
  access_level: string
}

export interface ProblemFormState {
  // Statement
  title: string
  description: string
  inputFormat: string
  outputFormat: string
  /** LaTeX bounds block; renders as its own Constraints section. */
  constraints: string
  hint: string
  timeLimit: number
  memoryLimit: number
  difficulty: string
  /** Codeforces-scale rating; 0 means unrated. */
  rating: number
  tags: string
  sampleCases: { input: string; output: string; explanation?: string | null }[]

  // Test Cases
  testcases: TestCase[]

  // Checker
  checkerType: string
  floatEpsilon: number
  spj: boolean
  spjLanguage: string
  spjSourceCode: string

  // Interactive
  interactive: boolean
  interactorLanguage: string
  interactorSourceCode: string

  // Settings
  visible: boolean
}

export function createDefaultFormState(): ProblemFormState {
  return {
    title: '',
    description: '',
    inputFormat: '',
    outputFormat: '',
    constraints: '',
    hint: '',
    timeLimit: 1000,
    memoryLimit: 262144,
    difficulty: 'easy',
    rating: 0,
    tags: '',
    sampleCases: [],
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
}
