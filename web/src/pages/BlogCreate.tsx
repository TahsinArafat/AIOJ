import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'
import MarkdownContent from '../components/MarkdownContent'

export default function BlogCreate() {
    const toast = useToast()
    const navigate = useNavigate()
    const [title, setTitle] = useState('')
    const [content, setContent] = useState('')
    const [tags, setTags] = useState('')
    const [submitting, setSubmitting] = useState(false)
    const [showPreview, setShowPreview] = useState(false)

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
                    <div className="flex items-center justify-between mb-1">
                        <label htmlFor="blog-content" className="block text-sm font-semibold text-ink-soft">Content (Markdown + LaTeX)</label>
                        <button type="button" onClick={() => setShowPreview(v => !v)} className="text-xs text-brand hover:underline">
                            {showPreview ? 'Write' : 'Preview'}
                        </button>
                    </div>
                    {showPreview ? (
                        <div aria-label="Blog content preview" className="prose max-w-none text-ink border rounded px-3 py-2 min-h-48">
                            {content.trim()
                                ? <MarkdownContent content={content} />
                                : <p className="text-muted-soft italic">Nothing to preview yet.</p>}
                        </div>
                    ) : (
                        <textarea id="blog-content" value={content} onChange={e => setContent(e.target.value)} required rows={10}
                            className="w-full border rounded px-3 py-2 text-sm"
                            placeholder="Write your post content... Use $...$ for inline math and $$...$$ for display math." />
                    )}
                    <p className="mt-1 text-xs text-muted">Markdown and LaTeX math are rendered when the post is published.</p>
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
