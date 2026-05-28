import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function BlogList() {
    const [posts, setPosts] = useState<any[]>([])
    const [total, setTotal] = useState(0)

    useEffect(() => {
        api.blog.list().then(d => {
            setPosts(d.data || [])
            setTotal(d.total || 0)
        }).catch(console.error)
    }, [])

    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-2xl font-bold">Blog</h1>
                    <p className="text-sm text-gray-500 mt-1">Read and share CP insights.</p>
                </div>
                <Link to="/blog/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 font-medium">
                    Write Post
                </Link>
            </div>

            <div className="space-y-3">
                {posts.map(p => (
                    <Link key={p.id} to={`/blog/${p.id}`} className="block border rounded-lg p-4 hover:bg-gray-50 transition-colors">
                        <div>
                            <h3 className="font-semibold text-lg">{p.title}</h3>
                            <div className="flex gap-4 mt-2 text-xs text-gray-400">
                                <span className="font-medium text-gray-600">{p.username}</span>
                                <span>{p.upvotes} votes</span>
                                <span>{p.comment_count} comments</span>
                                <span>{new Date(p.created_at).toLocaleDateString()}</span>
                            </div>
                            {p.tags?.length > 0 && (
                                <div className="flex gap-2 mt-3">
                                    {p.tags.map((tag: string) => (
                                        <span key={tag} className="text-xs bg-blue-50 text-blue-700 px-2 py-0.5 rounded font-medium">{tag}</span>
                                    ))}
                                </div>
                            )}
                        </div>
                    </Link>
                ))}
                {posts.length === 0 && (
                    <div className="text-center py-16 text-gray-400">No blog posts yet.</div>
                )}
            </div>
            {total > 0 && <p className="text-sm text-gray-400 mt-4">{total} posts</p>}
        </div>
    )
}
