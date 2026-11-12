export interface TestCase {
  input_name: string
  output_name: string
  score: number
  /** 0 means "not in a subtask". Cases sharing an id are graded together. */
  subtask_id?: number
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

  /** "standard" | "interactive" | "output_only".
   *
   * The server keeps this in step with `interactive`, but the workspace has to
   * load and send it back: an output-only problem is not interactive, so a save
   * that omitted the field would leave the two disagreeing. */
  problemType: string
  /** "complete" | "partial". Partial is what enables subtask scoring; the judge
   * silently treats everything as complete until this is set. */
  scoringMode: string
  /** "min" | "sum" — how the cases inside a subtask combine. */
  subtaskAggregation: string

  // Settings. There is deliberately no `visible` field: publishing is an admin
  // decision derived from the review status, so the author has no visibility
  // switch to send.
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
    problemType: 'standard',
    scoringMode: 'complete',
    subtaskAggregation: 'min',
  }
}
