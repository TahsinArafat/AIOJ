import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'

export default function BlogCreate() {
    const toast = useToast()
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
        } catch (e) {
            toast.error('Failed: ' + (e instanceof Error ? e.message : String(e)))
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="max-w-2xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">Write Blog Post</h1>
            <form onSubmit={handleSubmit} className="space-y-4">
                <div>
                    <label htmlFor="blog-title" className="block text-sm font-semibold text-ink-soft mb-1">Title</label>
                    <input id="blog-title" type="text" value={title} onChange={e => setTitle(e.target.value)} required
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="Post title" />
                </div>
                <div>
                    <label htmlFor="blog-content" className="block text-sm font-semibold text-ink-soft mb-1">Content</label>
                    <textarea id="blog-content" value={content} onChange={e => setContent(e.target.value)} required rows={10}
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="Write your post content..." />
                </div>
                <div>
                    <label htmlFor="blog-tags" className="block text-sm font-semibold text-ink-soft mb-1">Tags (comma-separated)</label>
                    <input id="blog-tags" type="text" value={tags} onChange={e => setTags(e.target.value)}
                        className="w-full border rounded px-3 py-2 text-sm" placeholder="e.g. tutorial, dp, graph" />
                </div>
                <button type="submit" disabled={submitting || !title.trim() || !content.trim()}
                    className="w-full bg-brand text-on-brand py-2 rounded text-sm hover:bg-brand-hover font-medium disabled:opacity-50">
                    {submitting ? 'Publishing...' : 'Publish Post'}
                </button>
            </form>
        </div>
    )
}
