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

test('resumes the newest job for the problem on mount', async () => {
 const ready={id:'j9',actor_id:'u1',artifact_type:'full_problem',status:'ready' as const,stage:'ready',progress:100,submitted_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},normalized_brief:{tags:['dp'],rating:1400,problem_type:'standard',testcase_count:10,has_subtasks:false},proposal:{title:'Resumed',description:'Statement'},validation_report:{valid:true,checks:[{name:'coverage',passed:true,message:'31 cases'}]},prompt_tokens:10,completion_tokens:20,latency_ms:100,created_at:''}
 vi.mocked(api.ai.problemJobs).mockResolvedValue({data:[ready]})
 render(<AIStudioTab problemId="p1" />)
 // A finished proposal must survive a reload instead of resetting to a blank
 // form, otherwise it can never be reviewed or applied.
 await waitFor(()=>expect(screen.getByRole('button',{name:'Apply selected'})).toBeInTheDocument())
 expect(api.ai.problemJobs).toHaveBeenCalledWith('p1')
})
