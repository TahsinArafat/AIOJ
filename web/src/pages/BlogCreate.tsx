import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

export default function BlogCreate() {
    const navigate = useNavigate()
    const [title, setTitle] = useState('')
    const [content, setContent] = useState('')
    const [tags, setTags] = useState('')
    const [submitting, setSubmitting] = useState(false)

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!title.trim() || !content.trim()) return
        setSubmitting(true)
        try {
            const post = await api.blog.create({
                title,
                content,
                tags: tags.split(',').map(t => t.trim()).filter(Boolean)
            })
            navigate(`/blog/${post.id}`)
        } catch (e: any) {
            alert('Failed: ' + e.message)
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="max-w-2xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">Write Blog Post</h1>
            <form onSubmit={handleSubmit} className="space-y-4">
                <div>
                    <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-1">Title</label>
                    <input type="text" value={title} onChange={e => setTitle(e.target.value)} required
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="Post title" />
                </div>
                <div>
                    <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-1">Content</label>
                    <textarea value={content} onChange={e => setContent(e.target.value)} required rows={10}
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="Write your post content..." />
                </div>
                <div>
                    <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-1">Tags (comma-separated)</label>
                    <input type="text" value={tags} onChange={e => setTags(e.target.value)}
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="e.g. tutorial, dp, graph" />
                </div>
                <button type="submit" disabled={submitting || !title.trim() || !content.trim()}
                    className="w-full bg-blue-600 text-white py-2 rounded text-sm hover:bg-blue-700 font-medium disabled:opacity-50">
                    {submitting ? 'Publishing...' : 'Publish Post'}
                </button>
            </form>
        </div>
    )
}
