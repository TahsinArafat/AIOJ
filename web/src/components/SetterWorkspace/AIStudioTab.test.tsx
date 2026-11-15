import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { expect, test, vi, beforeEach } from 'vitest'
import AIStudioTab from './AIStudioTab'
import { api } from '../../lib/api'
import type { AIArtifactSpec } from '../../types/api'

vi.mock('../../lib/api', () => ({ api: { ai: { connections: vi.fn(), artifacts: vi.fn(), problemJobs: vi.fn(), listJobs: vi.fn(), createJob: vi.fn(), getJob: vi.fn(), cancelJob: vi.fn(), discardJob: vi.fn(), applyJob: vi.fn() } } }))

/**
 * The artifact catalog as the server serves it (GET /api/ai/artifacts).
 *
 * Copied from internal/model/ai_artifact.go rather than invented: the frontend
 * gates its form on these field lists, so a fixture that disagreed with the
 * server would let a form that offers the wrong controls pass.
 */
/** The brief a queued job was created from, for the tests that do not inspect it. */
const BLANK_BRIEF = {tags:[] as string[], rating:0, problem_type:'standard', testcase_count:0, has_subtasks:false}

const ARTIFACTS: AIArtifactSpec[] = [
  { id: 'full_problem', label: 'Full package', description: 'Statement, tests, generator & reference solution', offered_for_existing: false,
    fields: ['topics','rating','rating_range','format_profile','solver_mode','event_context','insight_emphasis','expected_solve_minutes','primary_topics','secondary_topics','required_techniques','forbidden_techniques','constraint_guidance','audience_level','story_tone','testcase_count','has_subtasks','additional_prompt'] },
  { id: 'statement', label: 'Statement & samples', description: 'Rewrites the problem statement only', offered_for_existing: true,
    fields: ['secondary_topics','required_techniques','forbidden_techniques','constraint_guidance','audience_level','story_tone','additional_prompt'] },
  { id: 'tests', label: 'Tests & generator', description: 'Generates a new test suite and generator', offered_for_existing: true,
    fields: ['testcase_count','has_subtasks','secondary_topics','required_techniques','forbidden_techniques','constraint_guidance','additional_prompt'] },
  { id: 'solution', label: 'Reference solution', description: 'Writes or replaces the reference solution', offered_for_existing: true,
    fields: ['secondary_topics','required_techniques','forbidden_techniques','constraint_guidance','additional_prompt'] },
  { id: 'editorial', label: 'Editorial', description: 'Authors an editorial for this problem', offered_for_existing: true,
    fields: ['audience_level','story_tone','additional_prompt'] },
]

beforeEach(() => {
 vi.mocked(api.ai.connections).mockReset().mockResolvedValue({data:[{id:'m1',name:'Local',endpoint:'http://localhost',model_name:'model',enabled:true,priority:1,description:'',created_at:'',updated_at:''}]})
 vi.mocked(api.ai.artifacts).mockReset().mockResolvedValue({data: ARTIFACTS})
 vi.mocked(api.ai.createJob).mockReset()
 vi.mocked(api.ai.problemJobs).mockReset().mockResolvedValue({data:[]})
 vi.mocked(api.ai.listJobs).mockReset().mockResolvedValue({data:[], total:0})
})

test('renders artifact choices and creates a scoped job', async () => {
 vi.mocked(api.ai.createJob).mockResolvedValue({id:'j1',actor_id:'u1',artifact_type:'tests',status:'queued',stage:'queued',progress:0,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},prompt_tokens:0,completion_tokens:0,latency_ms:0,created_at:''})
 render(<AIStudioTab problemId="p1" />)
 await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())
 // The artifact picker is now radio inputs — select "Tests & generator"
 fireEvent.click(screen.getByRole('radio', {name:/Tests/}))
 fireEvent.click(screen.getByRole('button',{name:/Generate Tests/}))
 await waitFor(()=>expect(api.ai.createJob).toHaveBeenCalledWith(expect.objectContaining({problem_id:'p1',artifact_type:'tests'})))
 expect(screen.getAllByText(/queued/).length).toBeGreaterThan(0)
})

test('ready proposal exposes selective apply controls', async () => {
 const ready={id:'j1',actor_id:'u1',artifact_type:'statement',status:'ready' as const,stage:'ready',progress:100,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},proposal:{title:'Generated',description:'Statement'},validation_report:{valid:true,checks:[{name:'statement',passed:true,message:'ok'}]},prompt_tokens:10,completion_tokens:20,latency_ms:100,created_at:''}
 vi.mocked(api.ai.createJob).mockResolvedValue(ready)
 render(<AIStudioTab problemId="p1" />)
 await waitFor(()=>screen.getByText(/Local/))
 fireEvent.click(screen.getByRole('button',{name:/Generate Statement/}))
 await waitFor(()=>expect(screen.getByRole('button',{name:'Apply selected'})).toBeInTheDocument())
 // The gate name and its message are separate nodes now that AIValidationList
 // renders them (the old inline markup emitted "statement: ok" as one string, which
 // the shared component deliberately does not — a long message has to wrap on its
 // own rather than as part of the name).
 expect(screen.getByText('statement')).toBeInTheDocument()
 expect(screen.getByText(/— ok/)).toBeInTheDocument()
})

// Without a problemId the tab authors a brand-new problem: it must not look up
// per-problem jobs, must send no problem_id, and must always ask for the whole
// package (the other artifact scopes only patch an existing problem).
test('creates a new problem when no problemId is given', async () => {
 vi.mocked(api.ai.createJob).mockResolvedValue({id:'j2',actor_id:'u1',artifact_type:'full_problem',status:'queued',stage:'queued',progress:0,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},prompt_tokens:0,completion_tokens:0,latency_ms:0,created_at:''})
 render(<AIStudioTab />)
 await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())
 // No existing problem, so no resume lookup and no artifact picker.
 expect(api.ai.problemJobs).not.toHaveBeenCalled()
 // No radio artifact picker when isNew
 expect(screen.queryByRole('radio')).not.toBeInTheDocument()

 fireEvent.click(screen.getByRole('button',{name:'Generate problem'}))
 await waitFor(()=>expect(api.ai.createJob).toHaveBeenCalledTimes(1))
 const sent = vi.mocked(api.ai.createJob).mock.calls[0][0]
 expect(sent.problem_id).toBeUndefined()
 expect(sent.artifact_type).toBe('full_problem')
})

// The brief controls are the whole point of the page: changing a topic and the
// rating must reach the job request, not just the default brief.
test('carries the operator brief customisation into the job', async () => {
 vi.mocked(api.ai.createJob).mockResolvedValue({id:'j3',actor_id:'u1',artifact_type:'full_problem',status:'queued',stage:'queued',progress:0,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},prompt_tokens:0,completion_tokens:0,latency_ms:0,created_at:''})
 // No problemId: the full-package scope, which is the one that takes topics and
 // a rating at all.
 render(<AIStudioTab />)
 await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())

 // Topic chip, rating quick-pick, and a free-form field.
 fireEvent.click(screen.getByRole('button',{name:'greedy'}))
 fireEvent.click(screen.getByRole('button',{name:'1900'}))
 fireEvent.change(screen.getByLabelText('Forbidden techniques'), {target:{value:'flows, geometry'}})

 fireEvent.click(screen.getByRole('button',{name:/Generate problem/}))
 await waitFor(()=>expect(api.ai.createJob).toHaveBeenCalledTimes(1))
 const sent = vi.mocked(api.ai.createJob).mock.calls[0][0]
 expect(sent.brief.tags).toEqual(expect.arrayContaining(['dp','greedy']))
 expect(sent.brief.rating).toBe(1900)
 // The acceptable range must still contain the rating or the backend rejects it.
 expect(sent.brief.rating_min).toBeLessThanOrEqual(1900)
 expect(sent.brief.rating_max).toBeGreaterThanOrEqual(1900)
 expect(sent.brief.forbidden_techniques).toEqual(['flows','geometry'])
})

// The reported issue: "the options problem/editorial needs different inputs, not
// every type of input like rating/topic etc". An editorial takes no topics, no
// rating and no test count — it explains a problem that already exists.
test('an editorial asks for no topics, rating or test count', async () => {
  render(<AIStudioTab problemId="p1" />)
  await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())

  fireEvent.click(screen.getByRole('radio', {name:/Editorial/}))

  // The prose controls an editorial does use are present…
  expect(screen.getByText('Audience level')).toBeInTheDocument()
  expect(screen.getByText('Story tone')).toBeInTheDocument()
  // …and the problem-definition inputs are not offered at all.
  expect(screen.queryByLabelText('Add a topic tag')).not.toBeInTheDocument()
  expect(screen.queryByText(/Difficulty rating/)).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Test case count')).not.toBeInTheDocument()
  expect(screen.queryByText('Technique constraints')).not.toBeInTheDocument()
})

// The scope drives the gate, not a hardcoded topic rule: an editorial has no
// topics to pick, so it must be startable without them.
test('an editorial is not blocked for having no topics', async () => {
  vi.mocked(api.ai.createJob).mockResolvedValue({id:'j20',actor_id:'u1',artifact_type:'editorial',status:'queued',stage:'queued',progress:0,submitted_brief:BLANK_BRIEF,normalized_brief:BLANK_BRIEF,prompt_tokens:0,completion_tokens:0,latency_ms:0,created_at:''})
  render(<AIStudioTab problemId="p1" />)
  await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())

  fireEvent.click(screen.getByRole('radio', {name:/Editorial/}))

  const generate = screen.getByRole('button', {name:/Generate Editorial/})
  expect(generate).toBeEnabled()
  fireEvent.click(generate)

  await waitFor(()=>expect(api.ai.createJob).toHaveBeenCalledTimes(1))
  const sent = vi.mocked(api.ai.createJob).mock.calls[0][0]
  expect(sent.artifact_type).toBe('editorial')
  // The brief is pruned to the scope, so a stale default topic and rating from
  // the full-package brief are not sent with an editorial.
  expect(sent.brief.tags).toBeUndefined()
  expect(sent.brief.rating).toBeUndefined()
  expect(sent.brief.testcase_count).toBeUndefined()
})

// Switching scope must not carry the previous scope's answers into the job.
test('switching from statement to tests drops the fields the new scope ignores', async () => {
  vi.mocked(api.ai.createJob).mockResolvedValue({id:'j21',actor_id:'u1',artifact_type:'tests',status:'queued',stage:'queued',progress:0,submitted_brief:BLANK_BRIEF,normalized_brief:BLANK_BRIEF,prompt_tokens:0,completion_tokens:0,latency_ms:0,created_at:''})
  render(<AIStudioTab problemId="p1" />)
  await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())

  // A statement takes story tone but no test count.
  fireEvent.change(screen.getByLabelText(/Story tone/), {target:{value:'playful'}})
  fireEvent.click(screen.getByRole('radio', {name:/Tests/}))

  const generate = screen.getByRole('button', {name:/Generate Tests/})
  fireEvent.click(generate)

  await waitFor(()=>expect(api.ai.createJob).toHaveBeenCalledTimes(1))
  const sent = vi.mocked(api.ai.createJob).mock.calls[0][0]
  expect(sent.brief.testcase_count).toBe(10)
  // story_tone belonged to the statement scope; the tests scope has no use for it.
  expect(sent.brief.story_tone).toBeUndefined()
})

// "problem rewrite thing getting old problem statement or not, this also should
// be checked." Every scope but the full package reads the existing problem, so
// the tab states what the run is based on — and which version of it.
test('says what the run is based on, and which version', async () => {
  render(<AIStudioTab problemId="p1" problemUpdatedAt="2026-11-14T09:30:00Z" />)
  await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())

  // The whole note, not just the label span, so the assertions can read it.
  const note = screen.getByText(/Based on/).closest('p')!
  // The timestamp is rendered, so a setter can tell the run is built on the
  // version on screen rather than a stale one.
  expect(note.textContent).toMatch(/this problem as of/)
  // And it says the existing problem is what the model is given.
  expect(note.textContent).toMatch(/statement, constraints, limits and samples/)
})

// A new problem has nothing to be based on.
test('a new problem does not claim to be based on an existing one', async () => {
  render(<AIStudioTab />)
  await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())
  expect(screen.queryByText(/Based on/)).not.toBeInTheDocument()
})

// The apply guard is unreachable unless the tab forwards the timestamp it
// loaded, which is what makes "the problem changed since generation" possible.
test('apply sends the loaded problem timestamp so a stale write is refused', async () => {
  const ready={id:'j9',actor_id:'u1',artifact_type:'statement',status:'ready' as const,stage:'ready',progress:100,submitted_brief:BLANK_BRIEF,normalized_brief:BLANK_BRIEF,proposal:{title:'Generated',description:'Statement'},validation_report:{valid:true,checks:[]},prompt_tokens:10,completion_tokens:20,latency_ms:100,created_at:''}
  vi.mocked(api.ai.problemJobs).mockResolvedValue({data:[ready]})
  vi.mocked(api.ai.applyJob).mockReset().mockResolvedValue({problem_id:'p1',problem_slug:'p1'})
  vi.mocked(api.ai.getJob).mockReset().mockResolvedValue(ready)
  render(<AIStudioTab problemId="p1" problemUpdatedAt="2026-11-14T09:30:00Z" />)

  const applyButton = await screen.findByRole('button',{name:'Apply selected'})
  // The proposal review seeds its section selection in a microtask, and the
  // button is disabled while nothing is selected. A real browser flushes that
  // before paint; here the wait has to be explicit or the click is swallowed.
  await waitFor(()=>expect(applyButton).toBeEnabled())
  fireEvent.click(applyButton)

  await waitFor(()=>expect(api.ai.applyJob).toHaveBeenCalled())
  expect(vi.mocked(api.ai.applyJob).mock.calls[0][2]).toBe('2026-11-14T09:30:00Z')
})

// of letting the click fail with a limit error.
// A running job on another problem is invisible to this tab: the per-problem
// lookup cannot see it. The button must still be blocked, and say why, instead
// of letting the click fail with a limit error.
test('blocks starting a job while another problem has one running', async () => {
    const elsewhere = {id:'j4',actor_id:'u1',problem_id:'other',artifact_type:'full_problem',status:'running' as const,stage:'drafting',progress:10,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},prompt_tokens:0,completion_tokens:0,latency_ms:0,created_at:''}
    vi.mocked(api.ai.listJobs).mockResolvedValue({data:[elsewhere], total:1})
    render(<AIStudioTab />)

    const button = await screen.findByRole('button', {name:'Generate problem'})
    await waitFor(()=>expect(button).toBeDisabled())
    expect(screen.getByText(/already running \(drafting\)/)).toBeInTheDocument()
    // The budget is visible so the limit is not a surprise.
    expect(screen.getByText(/1 running, 0 queued · limit 1 running, 3 queued/)).toBeInTheDocument()
    expect(api.ai.createJob).not.toHaveBeenCalled()
})

// Cancelling frees the setter's slot on the server at once, so the tab must
// leave the running state immediately rather than waiting for the worker to
// observe the request.
test('leaves the running state as soon as the job is cancelled', async () => {
    const running = {id:'j5',actor_id:'u1',artifact_type:'statement',status:'running' as const,stage:'drafting',progress:10,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},prompt_tokens:0,completion_tokens:0,latency_ms:0,created_at:''}
    vi.mocked(api.ai.problemJobs).mockResolvedValue({data:[running]})
    vi.mocked(api.ai.listJobs).mockResolvedValue({data:[running], total:1})
    vi.mocked(api.ai.cancelJob).mockReset().mockResolvedValue({cancelled:true})
    render(<AIStudioTab problemId="p1" />)

    fireEvent.click(await screen.findByRole('button', {name:'Cancel'}))
    await waitFor(()=>expect(api.ai.cancelJob).toHaveBeenCalledWith('j5'))
    // 'Start a new run' only renders for a settled job, so its appearance is the
    // signal that the tab stopped treating this run as active.
    await waitFor(()=>expect(screen.getByRole('button',{name:'Start a new run'})).toBeInTheDocument())
    expect(screen.queryByRole('button',{name:'Cancel'})).not.toBeInTheDocument()
})

test('resumes the newest job for the problem on mount', async () => {
 const ready={id:'j9',actor_id:'u1',artifact_type:'statement',status:'ready' as const,stage:'ready',progress:100,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},proposal:{title:'Resumed',description:'Statement'},validation_report:{valid:true,checks:[{name:'coverage',passed:true,message:'31 cases'}]},prompt_tokens:10,completion_tokens:20,latency_ms:100,created_at:''}
 vi.mocked(api.ai.problemJobs).mockResolvedValue({data:[ready]})
 render(<AIStudioTab problemId="p1" />)
 // A finished proposal must survive a reload instead of resetting to a blank
 // form, otherwise it can never be reviewed or applied.
 await waitFor(()=>expect(screen.getByRole('button',{name:'Apply selected'})).toBeInTheDocument())
 expect(api.ai.problemJobs).toHaveBeenCalledWith('p1')
})

// The reported bug: "only may be used once for a problem, no new run option,
// only shows old run history". The mount effect matched every status except
// discarded and cancelled, so an APPLIED job was resumed on every visit — and
// since a saved proposal's form is hidden and an applied job has neither Discard
// nor Retry, the tab could never start another run on that problem again.
test('an applied job does not trap the tab — a new run is still startable', async () => {
  const applied={id:'j10',actor_id:'u1',artifact_type:'statement',status:'applied' as const,stage:'applied',progress:100,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},prompt_tokens:10,completion_tokens:20,latency_ms:100,created_at:''}
  vi.mocked(api.ai.problemJobs).mockResolvedValue({data:[applied]})
  render(<AIStudioTab problemId="p1" />)

  // The form must be reachable rather than the old job being force-loaded.
  const generate = await screen.findByRole('button', {name: /Generate/})
  expect(generate).toBeEnabled()
  expect(api.ai.createJob).not.toHaveBeenCalled()
})

// A settled job still reached through Job history must offer a way back to the
// form, because that view hides the form entirely.
test('a settled job offers a way back to a new run', async () => {
  const ready={id:'j11',actor_id:'u1',artifact_type:'statement',status:'ready' as const,stage:'ready',progress:100,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},proposal:{title:'Resumed',description:'Statement'},validation_report:{valid:true,checks:[]},prompt_tokens:10,completion_tokens:20,latency_ms:100,created_at:''}
  vi.mocked(api.ai.problemJobs).mockResolvedValue({data:[ready]})
  render(<AIStudioTab problemId="p1" />)

  // The resumed proposal hides the form; the button is the way back to it.
  const back = await screen.findByRole('button', {name:'Start a new run'})
  expect(screen.queryByRole('button', {name:/Generate/})).not.toBeInTheDocument()

  fireEvent.click(back)

  // Form is back, and the proposal is gone from view (still in history).
  expect(await screen.findByRole('button', {name:/Generate/})).toBeInTheDocument()
  expect(screen.queryByRole('button', {name:'Apply selected'})).not.toBeInTheDocument()
  // Leaving the view must not delete the job.
  expect(api.ai.discardJob).not.toHaveBeenCalled()
})
