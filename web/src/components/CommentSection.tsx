import { useEffect, useState } from 'react'
import { api, getAccessToken } from '../lib/api'
import { useToast } from './Toast.tsx'

interface Props {
    parentType: string
    parentId: string
}

interface CommentItem { id: string; username?: string; created_at: string; content?: string; upvotes?: number }

export default function CommentSection({ parentType, parentId }: Props) {
    const toast = useToast()
    const [comments, setComments] = useState<CommentItem[]>([])
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
    }, [parentType, parentId]) // eslint-disable-line react-hooks/exhaustive-deps -- refetch only when the comment target changes

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
        } catch (e) { toast.error('Failed: ' + (e instanceof Error ? e.message : String(e))) }
    }

    const handleCommentVote = async (commentId: string, value: number) => {
        try {
            await api.blog.vote({ target_type: 'comment', target_id: commentId, value })
            fetchComments()
        } catch (e) { toast.error('Vote failed: ' + (e instanceof Error ? e.message : String(e))) }
    }

    return (
        <div className="space-y-4">
            <h3 className="font-semibold text-ink-soft">Comments ({comments.length})</h3>

            {getAccessToken() && (
                <div className="flex gap-2">
                    <textarea value={newComment} onChange={e => setNewComment(e.target.value)}
                        placeholder="Write a comment..." rows={2}
                        className="flex-1 border rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-transparent text-ink border-hairline-strong" />
                    <button onClick={handlePost} disabled={!newComment.trim()}
                        className="bg-brand text-on-brand px-4 py-2 rounded text-sm hover:bg-brand-hover font-medium disabled:opacity-50 self-end cursor-pointer">
                        Post
                    </button>
                </div>
            )}

            {loading ? (
                <p className="text-sm text-muted-soft">Loading comments...</p>
            ) : comments.length === 0 ? (
                <p className="text-sm text-muted-soft">No comments yet. Be the first!</p>
            ) : (
                <div className="space-y-3">
                    {comments.map(c => (
                        <div key={c.id} className="border rounded p-3 border-hairline bg-surface shadow-sm">
                            <div className="flex justify-between items-start mb-1">
                                <span className="font-medium text-sm text-ink">{c.username}</span>
                                <span className="text-xs text-muted-soft">{new Date(c.created_at).toLocaleDateString()}</span>
                            </div>
                            <p className="text-sm text-ink-soft whitespace-pre-wrap">{c.content}</p>
                            {getAccessToken() && (
                                <div className="flex items-center gap-2 mt-2 pt-2 border-t border-hairline text-xs">
                                    <button onClick={() => handleCommentVote(c.id, 1)} className="text-muted-soft hover:text-green-600 cursor-pointer">▲</button>
                                    <span className="font-semibold text-muted">{c.upvotes || 0}</span>
                                    <button onClick={() => handleCommentVote(c.id, -1)} className="text-muted-soft hover:text-red-600 cursor-pointer">▼</button>
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
