import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { expect, test, vi, beforeEach } from 'vitest'
import AIStudioTab from './AIStudioTab'
import { api } from '../../lib/api'

vi.mock('../../lib/api', () => ({ api: { ai: { connections: vi.fn(), problemJobs: vi.fn(), createJob: vi.fn(), getJob: vi.fn(), cancelJob: vi.fn(), discardJob: vi.fn(), applyJob: vi.fn() } } }))

beforeEach(() => {
 vi.mocked(api.ai.connections).mockReset().mockResolvedValue({data:[{id:'m1',name:'Local',endpoint:'http://localhost',model_name:'model',enabled:true,priority:1,description:'',created_at:'',updated_at:''}]})
 vi.mocked(api.ai.createJob).mockReset()
 vi.mocked(api.ai.problemJobs).mockReset().mockResolvedValue({data:[]})
})

test('renders artifact choices and creates a scoped job', async () => {
 vi.mocked(api.ai.createJob).mockResolvedValue({id:'j1',actor_id:'u1',artifact_type:'tests',status:'queued',stage:'queued',progress:0,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},prompt_tokens:0,completion_tokens:0,latency_ms:0,created_at:''})
 render(<AIStudioTab problemId="p1" />)
 await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())
 fireEvent.click(screen.getByRole('button',{name:'Tests & generator'}))
 fireEvent.click(screen.getByRole('button',{name:'Generate proposal'}))
 await waitFor(()=>expect(api.ai.createJob).toHaveBeenCalledWith(expect.objectContaining({problem_id:'p1',artifact_type:'tests'})))
 expect(screen.getAllByText(/queued/).length).toBeGreaterThan(0)
})

test('ready proposal exposes selective apply controls', async () => {
 const ready={id:'j1',actor_id:'u1',artifact_type:'full_problem',status:'ready' as const,stage:'ready',progress:100,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},proposal:{title:'Generated',description:'Statement'},validation_report:{valid:true,checks:[{name:'statement',passed:true,message:'ok'}]},prompt_tokens:10,completion_tokens:20,latency_ms:100,created_at:''}
 vi.mocked(api.ai.createJob).mockResolvedValue(ready)
 render(<AIStudioTab problemId="p1" />)
 await waitFor(()=>screen.getByText(/Local/))
 fireEvent.click(screen.getByRole('button',{name:'Generate proposal'}))
 await waitFor(()=>expect(screen.getByRole('button',{name:'Apply selected'})).toBeInTheDocument())
 expect(screen.getByText(/statement: ok/)).toBeInTheDocument()
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
 expect(screen.queryByRole('button',{name:'Tests & generator'})).not.toBeInTheDocument()

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
 render(<AIStudioTab problemId="p1" />)
 await waitFor(()=>expect(screen.getByText(/Local/)).toBeInTheDocument())

 // Topic chip, rating quick-pick, and a free-form field.
 fireEvent.click(screen.getByRole('button',{name:'greedy'}))
 fireEvent.click(screen.getByRole('button',{name:'1900'}))
 fireEvent.change(screen.getByLabelText('Forbidden techniques'), {target:{value:'flows, geometry'}})

 fireEvent.click(screen.getByRole('button',{name:'Generate proposal'}))
 await waitFor(()=>expect(api.ai.createJob).toHaveBeenCalledTimes(1))
 const sent = vi.mocked(api.ai.createJob).mock.calls[0][0]
 expect(sent.brief.tags).toEqual(expect.arrayContaining(['dp','greedy']))
 expect(sent.brief.rating).toBe(1900)
 // The acceptable range must still contain the rating or the backend rejects it.
 expect(sent.brief.rating_min).toBeLessThanOrEqual(1900)
 expect(sent.brief.rating_max).toBeGreaterThanOrEqual(1900)
 expect(sent.brief.forbidden_techniques).toEqual(['flows','geometry'])
})

test('resumes the newest job for the problem on mount', async () => {
 const ready={id:'j9',actor_id:'u1',artifact_type:'full_problem',status:'ready' as const,stage:'ready',progress:100,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},proposal:{title:'Resumed',description:'Statement'},validation_report:{valid:true,checks:[{name:'coverage',passed:true,message:'31 cases'}]},prompt_tokens:10,completion_tokens:20,latency_ms:100,created_at:''}
 vi.mocked(api.ai.problemJobs).mockResolvedValue({data:[ready]})
 render(<AIStudioTab problemId="p1" />)
 // A finished proposal must survive a reload instead of resetting to a blank
 // form, otherwise it can never be reviewed or applied.
 await waitFor(()=>expect(screen.getByRole('button',{name:'Apply selected'})).toBeInTheDocument())
 expect(api.ai.problemJobs).toHaveBeenCalledWith('p1')
})
