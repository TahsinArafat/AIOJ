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

    const fetchComments = () => {
        api.blog.getComments(parentType, parentId)
            .then(d => setComments(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }

    useEffect(() => {
        fetchComments()
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

    const handleCommentVote = async (commentId: string, value: number) => {
        try {
            await api.blog.vote({ target_type: 'comment', target_id: commentId, value })
            fetchComments()
        } catch (e: any) { alert('Vote failed: ' + e.message) }
    }

    return (
        <div className="space-y-4">
            <h3 className="font-semibold text-gray-700 dark:text-gray-300">Comments ({comments.length})</h3>

            {getAccessToken() && (
                <div className="flex gap-2">
                    <textarea value={newComment} onChange={e => setNewComment(e.target.value)}
                        placeholder="Write a comment..." rows={2}
                        className="flex-1 border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 bg-transparent text-gray-800 dark:text-gray-200 border-gray-300 dark:border-gray-700" />
                    <button onClick={handlePost} disabled={!newComment.trim()}
                        className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 font-medium disabled:opacity-50 self-end cursor-pointer">
                        Post
                    </button>
                </div>
            )}

            {loading ? (
                <p className="text-sm text-gray-400 dark:text-gray-500">Loading comments...</p>
            ) : comments.length === 0 ? (
                <p className="text-sm text-gray-400 dark:text-gray-500">No comments yet. Be the first!</p>
            ) : (
                <div className="space-y-3">
                    {comments.map(c => (
                        <div key={c.id} className="border rounded p-3 border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 shadow-sm">
                            <div className="flex justify-between items-start mb-1">
                                <span className="font-medium text-sm text-gray-800 dark:text-gray-200">{c.username}</span>
                                <span className="text-xs text-gray-400 dark:text-gray-500">{new Date(c.created_at).toLocaleDateString()}</span>
                            </div>
                            <p className="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap">{c.content}</p>
                            {getAccessToken() && (
                                <div className="flex items-center gap-2 mt-2 pt-2 border-t border-gray-100 dark:border-gray-800 text-xs">
                                    <button onClick={() => handleCommentVote(c.id, 1)} className="text-gray-400 hover:text-green-600 cursor-pointer">▲</button>
                                    <span className="font-semibold text-gray-600 dark:text-gray-400">{c.upvotes || 0}</span>
                                    <button onClick={() => handleCommentVote(c.id, -1)} className="text-gray-400 hover:text-red-600 cursor-pointer">▼</button>
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
