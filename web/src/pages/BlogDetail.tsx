import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import CommentSection from '../components/CommentSection'
import { getAccessToken } from '../lib/api'

export default function BlogDetail() {
    const { id } = useParams<{ id: string }>()
    const [post, setPost] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [vote, setVote] = useState(0)

    useEffect(() => {
        if (!id) return
        api.blog.get(id).then(setPost).catch(console.error).finally(() => setLoading(false))
    }, [id])

    const handleVote = async (value: number) => {
        if (!id) return
        try {
            await api.blog.vote({ target_type: 'blog', target_id: id, value })
            setVote(value)
        } catch (e: any) { alert('Vote failed: ' + e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400">Loading...</div>
    if (!post) return <div className="text-center py-20 text-gray-400">Post not found</div>

    return (
        <div className="max-w-3xl mx-auto">
            <article className="mb-8">
                <h1 className="text-3xl font-bold mb-4">{post.title}</h1>
                <div className="flex items-center gap-4 text-sm text-gray-500 mb-6">
                    <span className="font-semibold text-gray-700">{post.username}</span>
                    <span>{new Date(post.created_at).toLocaleDateString()}</span>
                    {post.tags?.length > 0 && (
                        <div className="flex gap-2 ml-2">
                            {post.tags.map((tag: string) => (
                                <span key={tag} className="text-xs bg-blue-50 text-blue-700 px-2 py-0.5 rounded font-medium">{tag}</span>
                            ))}
                        </div>
                    )}
                </div>
                <div className="prose max-w-none text-gray-800">{post.content}</div>

                {getAccessToken() && (
                    <div className="flex items-center gap-4 mt-8 pt-4 border-t">
                        <button onClick={() => handleVote(1)} className={`text-lg ${vote === 1 ? 'text-green-600' : 'text-gray-400 hover:text-green-600'}`}>▲</button>
                        <span className="font-semibold text-gray-700">{post.upvotes}</span>
                        <button onClick={() => handleVote(-1)} className={`text-lg ${vote === -1 ? 'text-red-600' : 'text-gray-400 hover:text-red-600'}`}>▼</button>
                    </div>
                )}
            </article>

            <CommentSection parentType="blog" parentId={id!} />
        </div>
    )
}
