import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { AIBrief, AIGenerationJob, AIModel, AIProposal } from '../../types/api'

type Props = { problemId: string; problemUpdatedAt?: string; onApplied?: () => void }
const artifacts = [['full_problem','Full package'],['statement','Statement & samples'],['tests','Tests & generator'],['solution','Reference solution'],['editorial','Editorial']] as const

function previewValue(v: unknown) { return typeof v === 'string' ? v : JSON.stringify(v, null, 2) }

export default function AIStudioTab({ problemId, problemUpdatedAt, onApplied }: Props) {
 const [connections,setConnections]=useState<AIModel[]>([])
 const [artifact,setArtifact]=useState('full_problem')
 const [brief,setBrief]=useState<AIBrief>({tags:['dp'],rating:1400,rating_min:1300,rating_max:1500,problem_type:'standard',testcase_count:10,has_subtasks:false,solver_mode:'solo',event_context:'practice',format_profile:'general',expected_solve_minutes:60})
 const [job,setJob]=useState<AIGenerationJob|null>(null)
 const [selected,setSelected]=useState<string[]>(['statement','solution','editorial','tests'])
 const [error,setError]=useState('')
 useEffect(()=>{api.ai.connections().then(r=>setConnections(r.data||[])).catch(e=>setError(String(e)))},[])
 useEffect(()=>{if(!job||!['queued','running'].includes(job.status))return;const id=window.setInterval(()=>api.ai.getJob(job.id).then(setJob).catch(()=>{}),1500);return()=>window.clearInterval(id)},[job])
 const start=async()=>{setError('');try{setJob(await api.ai.createJob({problem_id:problemId,ai_model_id:connections[0]?.id,artifact_type:artifact,brief}))}catch(e){setError(e instanceof Error?e.message:String(e))}}
 const apply=async()=>{if(!job)return;try{await api.ai.applyJob(job.id,selected,problemUpdatedAt);setJob(await api.ai.getJob(job.id));onApplied?.()}catch(e){setError(e instanceof Error?e.message:String(e))}}
 const proposal:AIProposal|undefined=job?.proposal
 return <div className="space-y-5">
  <div><h2 className="text-lg font-semibold">AI Studio</h2><p className="text-sm text-gray-500">Generate a reviewable proposal. Nothing is overwritten until you apply selected sections.</p></div>
  {error&&<div className="border border-red-300 bg-red-50 dark:bg-red-900/20 p-3 rounded text-sm text-red-700 dark:text-red-300">{error}</div>}
  {!job&&<div className="space-y-4">
   <div className="flex flex-wrap gap-2">{artifacts.map(([id,label])=><button key={id} onClick={()=>setArtifact(id)} className={`min-h-11 px-3 rounded border text-sm ${artifact===id?'bg-blue-600 text-white border-blue-600':'border-gray-300 dark:border-gray-600'}`}>{label}</button>)}</div>
   <div className="grid sm:grid-cols-3 gap-3">
    <label className="text-sm">Connection<select className="mt-1 w-full border rounded px-3 py-2 bg-white dark:bg-gray-800">{connections.map(c=><option key={c.id}>{c.name} · {c.model_name}</option>)}</select></label>
    <label className="text-sm">Target rating<input type="number" value={brief.rating} onChange={e=>setBrief({...brief,rating:Number(e.target.value)})} className="mt-1 w-full border rounded px-3 py-2 bg-white dark:bg-gray-800" /></label>
    <label className="text-sm">Profile<select value={brief.format_profile} onChange={e=>setBrief({...brief,format_profile:e.target.value as AIBrief['format_profile']})} className="mt-1 w-full border rounded px-3 py-2 bg-white dark:bg-gray-800"><option value="general">General</option><option value="codeforces">Codeforces</option><option value="icpc">ICPC onsite</option><option value="oi">OI batch</option></select></label>
   </div>
   <label className="block text-sm">Topics<input value={brief.tags.join(', ')} onChange={e=>setBrief({...brief,tags:e.target.value.split(',').map(x=>x.trim()).filter(Boolean)})} className="mt-1 w-full border rounded px-3 py-2 bg-white dark:bg-gray-800" /></label>
   <button disabled={!connections.length} onClick={start} className="min-h-11 px-5 bg-blue-600 text-white rounded disabled:opacity-50">Generate proposal</button>
  </div>}
  {job&&<div className="space-y-4">
   <div className="border rounded p-4"><div className="flex justify-between text-sm"><span className="font-medium capitalize">{job.stage.replaceAll('_',' ')}</span><span>{job.progress}%</span></div><div className="mt-2 h-2 bg-gray-200 dark:bg-gray-700 rounded"><div className="h-2 bg-blue-600 rounded" style={{width:`${job.progress}%`}} /></div><p className="mt-2 text-xs text-gray-500">{job.status} · {job.prompt_tokens+job.completion_tokens} tokens · {job.latency_ms} ms</p></div>
   {job.status==='running'||job.status==='queued'?<button onClick={()=>api.ai.cancelJob(job.id).then(()=>api.ai.getJob(job.id).then(setJob))} className="min-h-11 px-4 border rounded">Cancel</button>:null}
   {job.validation_report&&<div className="border rounded p-4"><h3 className="font-medium mb-2">Validation</h3>{job.validation_report.checks.map(c=><p key={c.name} className={`text-sm ${c.passed?'text-green-700':'text-red-700'}`}>{c.passed?'✓':'✗'} {c.name}: {c.message}</p>)}</div>}
   {proposal&&job.status==='ready'&&<div className="space-y-3"><h3 className="font-medium">Review proposed artifacts</h3>{selected.map(section=><label key={section} className="block border rounded p-3"><span className="flex items-center gap-2 font-medium"><input type="checkbox" checked={selected.includes(section)} onChange={e=>setSelected(e.target.checked?[...selected,section]:selected.filter(x=>x!==section))}/>{section}</span><pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap text-xs text-gray-600 dark:text-gray-300">{previewValue(section==='statement'?{title:proposal.title,description:proposal.description}:section==='solution'?proposal.solution_code:section==='editorial'?proposal.editorial_content:{generator:proposal.generator_code,cases:proposal.curated_cases})}</pre></label>)}<div className="flex gap-2"><button onClick={apply} className="min-h-11 px-4 bg-blue-600 text-white rounded">Apply selected</button><button onClick={()=>api.ai.discardJob(job.id).then(()=>setJob(null))} className="min-h-11 px-4 border rounded">Discard</button></div></div>}
   {['failed','validation_failed','cancelled'].includes(job.status)&&<div><p className="text-sm text-red-600">{job.error_message||`Job ${job.status}`}</p><div className="mt-2 flex gap-2"><button onClick={()=>api.ai.retryJob(job.id).then(setJob).catch(e=>setError(String(e)))} className="min-h-11 px-4 bg-blue-600 text-white rounded">Retry</button><button onClick={()=>setJob(null)} className="min-h-11 px-4 border rounded">Start again</button></div></div>}
  </div>}
 </div>
}
