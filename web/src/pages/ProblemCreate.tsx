import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

export default function ProblemCreate() {
    const nav = useNavigate()
    const [form, setForm] = useState({
        slug: '', title: '', description: '', difficulty: 'easy',
        time_limit: 1000, memory_limit: 262144, tags: '', input_format: '', output_format: '',
    })
    const [error, setError] = useState('')
    const [submitting, setSubmitting] = useState(false)

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')
        setSubmitting(true)
        try {
            await api.problems.create({
                slug: form.slug, title: form.title, description: form.description,
                difficulty: form.difficulty, time_limit: form.time_limit, memory_limit: form.memory_limit,
                tags: form.tags.split(',').map((t: string) => t.trim()).filter(Boolean),
                input_format: form.input_format, output_format: form.output_format,
            })
            nav('/setter')
        } catch (err: any) {
            setError(err.message || 'Failed to create problem')
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="max-w-2xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">Create Problem</h1>
            {error && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">{error}</div>}
            <form onSubmit={handleSubmit} className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Slug</label>
                        <input required value={form.slug} onChange={e => setForm(p => ({...p, slug: e.target.value}))}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
                        <input required value={form.title} onChange={e => setForm(p => ({...p, title: e.target.value}))}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
                    <textarea required rows={6} value={form.description} onChange={e => setForm(p => ({...p, description: e.target.value}))}
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                </div>
                <div className="grid grid-cols-3 gap-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Difficulty</label>
                        <select value={form.difficulty} onChange={e => setForm(p => ({...p, difficulty: e.target.value}))}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
                            <option value="easy">Easy</option><option value="medium">Medium</option><option value="hard">Hard</option>
                        </select>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Time Limit (ms)</label>
                        <input type="number" value={form.time_limit} onChange={e => setForm(p => ({...p, time_limit: +e.target.value}))}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Memory Limit (KB)</label>
                        <input type="number" value={form.memory_limit} onChange={e => setForm(p => ({...p, memory_limit: +e.target.value}))}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    </div>
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Tags (comma-separated)</label>
                    <input value={form.tags} onChange={e => setForm(p => ({...p, tags: e.target.value}))}
                        placeholder="e.g. dp, graph, math" className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Input Format</label>
                    <textarea rows={2} value={form.input_format} onChange={e => setForm(p => ({...p, input_format: e.target.value}))}
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Output Format</label>
                    <textarea rows={2} value={form.output_format} onChange={e => setForm(p => ({...p, output_format: e.target.value}))}
                        className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                </div>
                <button type="submit" disabled={submitting}
                    className="bg-blue-600 text-white px-6 py-2 rounded hover:bg-blue-700 disabled:opacity-50 transition-colors">
                    {submitting ? 'Creating...' : 'Create Problem'}
                </button>
            </form>
        </div>
    )
}
