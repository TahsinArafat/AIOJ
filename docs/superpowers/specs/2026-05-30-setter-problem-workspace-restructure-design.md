# Setter Problem Workspace Restructure — Design Spec

## Goal

Restructure `web/src/pages/SetterProblemWorkspace.tsx` from a 1101-line monolithic component into a clean, maintainable architecture: one orchestrator component + focused tab sub-components, with shared types and a single `buildPayload()` utility to eliminate 5x payload duplication.

## Constraints

1. **Zero compilation errors** after restructure.
2. **Strictly preserve existing styling** — blue-600 active states, white backgrounds, border-gray-200, gray-50 tables, gray-800 buttons, red-600 danger zone. No dark themes or gradients added.
3. **No new features** — the checker options (`float_absolute`, `float_relative`) and batch score set already exist and are functional.
4. **React 19 + TypeScript + Tailwind** — no new dependencies.
5. **Respect existing component API** — `CodeEditor` from `../components/CodeEditor` remains unchanged.
6. **Only restructure this file** — do not modify `api.ts` or any other file unless strictly necessary.

## Current Architecture Problems

### A. 5x Payload Duplication
Every handler rebuilds the same massive object:
```typescript
const payload = {
    title, description, input_format: inputFormat, output_format: outputFormat,
    hint, time_limit: Number(timeLimit), memory_limit: Number(memoryLimit),
    difficulty, tags: tagsArray, sample_cases: sampleCases,
    testcase_score: testcases, spj, spj_language: spjLanguage,
    spj_source_code: spjSourceCode, checker_type: checkerType,
    float_epsilon: floatEpsilon, interactive, interactor_language: interactorLanguage,
    interactor_source_code: interactorSourceCode, visible
}
```

### B. State Explosion (25+ `useState` calls)
No grouping into logical form state objects. All state is flat at the component root.

### C. Missing Type Safety
`useState<any>` for the problem object. The `ProblemDetail` interface exists in `api.ts` but is unused.

### D. Mixed Concerns
- `loadProblem()` fetches problem data AND permission data
- Handlers for 5 different tab sections coexist in the same scope
- Error/success/toast management is manual and repeated

## Proposed Architecture

```
SetterProblemWorkspace.tsx  (orchestrator, ~150 lines)
├── types/problem-workspace.ts    (shared interfaces)
├── components/SetterWorkspace/
│   ├── StatementTab.tsx           (~200 lines)
│   ├── TestCasesTab.tsx           (~250 lines)
│   ├── CheckerTab.tsx             (~300 lines)
│   ├── PermissionsTab.tsx         (~120 lines)
│   └── SettingsTab.tsx            (~100 lines)
```

### Data Flow

```
SetterProblemWorkspace (orchestrator)
  │
  │  owns: formState, activeTab, saving, error, success, problem
  │  provides: buildPayload(), handleSaveAll()
  │
  ├── StatementTab
  │     props: formState.statement, onChange, onSave
  │
  ├── TestCasesTab
  │     props: testcases, batchScore, onAdd/Remove/Batch, onUpload
  │
  ├── CheckerTab
  │     props: formState.checker, onChange, onSave
  │
  ├── PermissionsTab
  │     props: collaborators, onAdd/Remove
  │
  └── SettingsTab
        props: visible, onChange, onDelete
```

All save operations go through `buildPayload()` + `api.problems.update()` in the orchestrator, never in the sub-components.

## Type Contract

```typescript
// Shared form state interface
export interface ProblemFormState {
  // Statement
  title: string
  description: string
  inputFormat: string
  outputFormat: string
  hint: string
  timeLimit: number
  memoryLimit: number
  difficulty: string
  tags: string
  sampleCases: { input: string; output: string; explanation: string }[]
  
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

export interface TestCase {
  input_name: string
  output_name: string
  score: number
}

export interface Collaborator {
  problem_id: string
  user_id: string
  username: string
  access_level: string
}
```

## Component Responsibilities

### SetterProblemWorkspace (orchestrator)
- Owns all state (formState, meta-state, collaborators)
- `loadProblem()` — fetches and populates
- `buildPayload()` — single source of truth for update payload
- Generic `handleSave()` — calls `buildPayload()` + `api.problems.update()`
- Renders header, tabs sidebar, error/success banners, and active tab component
- Handles delete problem

### StatementTab
- Receives statement portion of formState + change handlers
- Renders title, tags, difficulty, time/memory limits, description, input/output format, hint, sample cases
- Calls onSave with tab-specific validation

### TestCasesTab  
- Receives testcases array, batch score, file upload ref
- Handles add/remove/batch-set operations locally (optimistic update + parent save)
- Renders upload section, scores table, batch set UI, register new match form

### CheckerTab
- Receives checker/interactive portion of formState + change handlers
- Renders checker type selector, epsilon input (conditional), SPJ code editor with presets, interactive toggle
- Calls onSave with checker-specific payload

### PermissionsTab
- Receives collaborators array + add/remove handlers
- Renders collaborators table and add-collaborator form

### SettingsTab
- Receives visibility + change handler + delete handler
- Renders visibility toggle and danger zone delete button

## Implementation Waves

### Wave 1: Foundation
1. Create `types/problem-workspace.ts` with `ProblemFormState`, `TestCase`, `Collaborator` interfaces
2. Add `buildPayload()` function inside SetterProblemWorkspace that builds the complete update payload from formState
3. Replace all 5 inline payload constructions with `buildPayload()` calls
4. Add proper `ProblemFormState` type to consolidate state

### Wave 2: Sub-Components
5. Create `SetterWorkspace/StatementTab.tsx`
6. Create `SetterWorkspace/TestCasesTab.tsx`
7. Create `SetterWorkspace/CheckerTab.tsx`
8. Create `SetterWorkspace/PermissionsTab.tsx`
9. Create `SetterWorkspace/SettingsTab.tsx`

### Wave 3: Integration
10. Refactor `SetterProblemWorkspace.tsx` to use sub-components
11. Type-safe all state references
12. Verify zero compilation errors

## Design Decisions

**Decision 1: Sub-components in `components/SetterWorkspace/` rather than file-level sections**
- Rationale: Each tab is an independent visual unit with its own form fields. Separating into files enforces boundaries, enables isolated testing, and makes future feature additions non-conflicting.
- Trade-off: More files to navigate, but each file is focused (~100-300 lines vs 1100).

**Decision 2: `buildPayload()` in the orchestrator, not shared utility**
- Rationale: The payload shape is specific to this component's form state. Exporting it as a generic utility would couple it to a single consumer.
- Trade-off: Minor duplication if another component needs the same shape (currently no other component does).

**Decision 3: Optimistic local state + parent save pattern for testcases**
- Rationale: Test case score changes are frequent (add/remove/batch-set). Optimistic update makes the UI feel responsive. The parent orchestrator still owns persistence.
- Trade-off: Slightly more complex state management, but avoids "loading spinner on every score change."

**Decision 4: Props interface per tab, not a single mega-interface**
- Rationale: Each tab has different needs. A single props interface with optional fields would be unclear.
- Trade-off: More interface definitions, but clearer contracts.

## Self-Review

- [x] No placeholders or TODOs in spec
- [x] Architecture matches feature descriptions
- [x] Scope is focused on restructuring this single file
- [x] No ambiguous requirements — each component's responsibility is clear
- [x] Styling constraint explicitly documented
