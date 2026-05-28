import { useEffect, useState } from 'react'
import { api, getAccessToken } from '../lib/api'

interface Props {
    parentType: string
    parentId: string
}

export default function CommentSection({ parentType, parentId }: Props) {
    const [comments, setComments] = useState<any[]>([])
    const [newComment, setNewComment] = useState('')
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.blog.getComments(parentType, parentId)
            .then(d => setComments(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }, [parentType, parentId])

    const handlePost = async () => {
        if (!newComment.trim()) return
        try {
            const c = await api.blog.createComment({
                parent_type: parentType,
                parent_id: parentId,
                content: newComment.trim()
            })
            setComments(prev => [...prev, c])
            setNewComment('')
        } catch (e: any) { alert('Failed: ' + e.message) }
    }

    return (
        <div className="space-y-4">
            <h3 className="font-semibold text-gray-700">Comments ({comments.length})</h3>

            {getAccessToken() && (
                <div className="flex gap-2">
                    <textarea value={newComment} onChange={e => setNewComment(e.target.value)}
                        placeholder="Write a comment..." rows={2}
                        className="flex-1 border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
                    <button onClick={handlePost} disabled={!newComment.trim()}
                        className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 font-medium disabled:opacity-50 self-end">
                        Post
                    </button>
                </div>
            )}

            {loading ? (
                <p className="text-sm text-gray-400">Loading comments...</p>
            ) : comments.length === 0 ? (
                <p className="text-sm text-gray-400">No comments yet. Be the first!</p>
            ) : (
                <div className="space-y-3">
                    {comments.map(c => (
                        <div key={c.id} className="border rounded p-3">
                            <div className="flex justify-between items-start mb-1">
                                <span className="font-medium text-sm text-gray-800">{c.username}</span>
                                <span className="text-xs text-gray-400">{new Date(c.created_at).toLocaleDateString()}</span>
                            </div>
                            <p className="text-sm text-gray-700 whitespace-pre-wrap">{c.content}</p>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
