import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import CommentSection from '../components/CommentSection'
import ReactMarkdown from 'react-markdown'
import remarkMath from 'remark-math'
import rehypeKatex from 'rehype-katex'
import 'katex/dist/katex.min.css'

function getUserId(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.uid ?? null
    } catch {
        return null
    }
}

function getRole(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.role ?? null
    } catch {
        return null
    }
}

export default function BlogDetail() {
    const { id } = useParams<{ id: string }>()
    const navigate = useNavigate()
    const [post, setPost] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [vote, setVote] = useState(0)

    const [editing, setEditing] = useState(false)
    const [editTitle, setEditTitle] = useState('')
    const [editContent, setEditContent] = useState('')
    const [editTags, setEditTags] = useState('')

    useEffect(() => {
        if (!id) return
        api.blog.get(id).then(setPost).catch(console.error).finally(() => setLoading(false))
        if (getAccessToken()) {
            api.blog.getVote(id).then(res => setVote(res.value)).catch(console.error)
        }
    }, [id])

    const handleVote = async (value: number) => {
        if (!id) return
        try {
            await api.blog.vote({ target_type: 'blog', target_id: id, value })
            setVote(value)
            const updated = await api.blog.get(id)
            setPost(updated)
        } catch (e: any) { alert('Vote failed: ' + e.message) }
    }

    const handleStartEdit = () => {
        setEditTitle(post.title)
        setEditContent(post.content)
        setEditTags(post.tags?.join(', ') || '')
        setEditing(false) // toggle off/on
        setTimeout(() => setEditing(true), 0)
    }

    const handleSaveEdit = async () => {
        if (!id) return
        try {
            const parsedTags = editTags.split(',').map(t => t.trim()).filter(Boolean)
            const updated = await api.blog.update(id, { title: editTitle, content: editContent, tags: parsedTags })
            setPost(updated)
            setEditing(false)
        } catch (e: any) { alert('Update failed: ' + e.message) }
    }

    const handleDelete = async () => {
        if (!id) return
        if (!confirm('Are you sure you want to delete this post?')) return
        try {
            await api.blog.delete(id)
            navigate('/blog')
        } catch (e: any) { alert('Delete failed: ' + e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading...</div>
    if (!post) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Post not found</div>

    const userId = getUserId()
    const role = getRole()
    const canManage = post.user_id === userId || role === 'admin'

    return (
        <div className="max-w-3xl mx-auto">
            {editing ? (
                <div className="space-y-4 bg-white dark:bg-gray-900 p-6 rounded-lg border border-gray-200 dark:border-gray-800">
                    <h2 className="text-xl font-bold">Edit Blog Post</h2>
                    <div>
                        <label className="block text-sm font-medium mb-1">Title</label>
                        <input
                            type="text"
                            value={editTitle}
                            onChange={e => setEditTitle(e.target.value)}
                            className="w-full border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1">Tags (comma separated)</label>
                        <input
                            type="text"
                            value={editTags}
                            onChange={e => setEditTags(e.target.value)}
                            className="w-full border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1">Content (Markdown supported)</label>
                        <textarea
                            value={editContent}
                            onChange={e => setEditContent(e.target.value)}
                            rows={12}
                            className="w-full border border-gray-300 dark:border-gray-700 bg-transparent rounded p-2 text-sm font-mono"
                        />
                    </div>
                    <div className="flex gap-2 justify-end">
                        <button onClick={() => setEditing(false)} className="px-4 py-2 border border-gray-300 dark:border-gray-700 rounded text-sm hover:bg-gray-100 dark:hover:bg-gray-800">Cancel</button>
                        <button onClick={handleSaveEdit} className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">Save Changes</button>
                    </div>
                </div>
            ) : (
                <article className="mb-8">
                    <div className="flex justify-between items-start mb-4">
                        <h1 className="text-3xl font-bold">{post.title}</h1>
                        {canManage && (
                            <div className="flex gap-2">
                                <button onClick={handleStartEdit} className="px-3 py-1 border border-gray-300 dark:border-gray-700 text-xs rounded hover:bg-gray-100 dark:hover:bg-gray-800">Edit</button>
                                <button onClick={handleDelete} className="px-3 py-1 bg-red-600 hover:bg-red-700 text-white text-xs rounded">Delete</button>
                            </div>
                        )}
                    </div>
                    <div className="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400 mb-6">
                        <span className="font-semibold text-gray-700 dark:text-gray-300">{post.username}</span>
                        <span>{new Date(post.created_at).toLocaleDateString()}</span>
                        {post.tags?.length > 0 && (
                            <div className="flex gap-2 ml-2">
                                {post.tags.map((tag: string) => (
                                    <span key={tag} className="text-xs bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 px-2 py-0.5 rounded font-medium">{tag}</span>
                                ))}
                            </div>
                        )}
                    </div>
                    <div className="prose max-w-none text-gray-800 dark:text-gray-200">
                        <ReactMarkdown remarkPlugins={[remarkMath]} rehypePlugins={[rehypeKatex]}>
                            {post.content}
                        </ReactMarkdown>
                    </div>

                    {getAccessToken() && (
                        <div className="flex items-center gap-4 mt-8 pt-4 border-t border-gray-200 dark:border-gray-800">
                            <button onClick={() => handleVote(1)} className={`text-lg cursor-pointer ${vote === 1 ? 'text-green-600 dark:text-green-400' : 'text-gray-400 dark:text-gray-500 hover:text-green-600'}`}>▲</button>
                            <span className="font-semibold text-gray-700 dark:text-gray-300">+{post.upvotes} / -{post.downvotes}</span>
                            <button onClick={() => handleVote(-1)} className={`text-lg cursor-pointer ${vote === -1 ? 'text-red-600 dark:text-red-400' : 'text-gray-400 dark:text-gray-500 hover:text-red-600'}`}>▼</button>
                        </div>
                    )}
                </article>
            )}

            <CommentSection parentType="blog" parentId={id!} />
        </div>
    )
}
