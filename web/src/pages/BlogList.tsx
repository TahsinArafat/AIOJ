import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function BlogList() {
    const [posts, setPosts] = useState<any[]>([])
    const [total, setTotal] = useState(0)
    const [tag, setTag] = useState('')
    const [offset, setOffset] = useState(0)
    const LIMIT = 20

    useEffect(() => {
        api.blog.list(offset, LIMIT, tag || undefined).then(d => {
            setPosts(d.data || [])
            setTotal(d.total || 0)
        }).catch(console.error)
    }, [offset, tag])

    const handleTagChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setTag(e.target.value)
        setOffset(0)
    }

    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-2xl font-bold">Blog</h1>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Read and share CP insights.</p>
                </div>
                <div className="flex items-center gap-4">
                    <input
                        type="text"
                        placeholder="Filter by Tag..."
                        value={tag}
                        onChange={handleTagChange}
                        className="border border-gray-300 dark:border-gray-700 bg-transparent rounded px-3 py-1.5 text-sm"
                    />
                    <Link to="/blog/create" className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 font-medium whitespace-nowrap">
                        Write Post
                    </Link>
                </div>
            </div>

            <div className="space-y-3">
                {posts.map(p => (
                    <Link key={p.id} to={`/blog/${p.id}`} className="block border rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
                        <div>
                            <h3 className="font-semibold text-lg">{p.title}</h3>
                            <div className="flex gap-4 mt-2 text-xs text-gray-400 dark:text-gray-500">
                                <span className="font-medium text-gray-600 dark:text-gray-400">{p.username}</span>
                                <span>{p.upvotes} votes</span>
                                <span>{p.comment_count} comments</span>
                                <span>{new Date(p.created_at).toLocaleDateString()}</span>
                            </div>
                            {p.tags?.length > 0 && (
                                <div className="flex gap-2 mt-3">
                                    {p.tags.map((tag: string) => (
                                        <span key={tag} className="text-xs bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 px-2 py-0.5 rounded font-medium">{tag}</span>
                                    ))}
                                </div>
                            )}
                        </div>
                    </Link>
                ))}
                {posts.length === 0 && (
                    <div className="text-center py-16 text-gray-400 dark:text-gray-500">No blog posts yet.</div>
                )}
            </div>

            {total > 0 && <p className="text-sm text-gray-400 dark:text-gray-500 mt-4">{total} posts</p>}

            {total > LIMIT && (
                <div className="flex gap-2 justify-center mt-6">
                    <button
                        disabled={offset === 0}
                        onClick={() => setOffset(Math.max(0, offset - LIMIT))}
                        className="px-4 py-2 border rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-800 cursor-pointer"
                    >
                        Previous
                    </button>
                    <button
                        disabled={offset + LIMIT >= total}
                        onClick={() => setOffset(offset + LIMIT)}
                        className="px-4 py-2 border rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-800 cursor-pointer"
                    >
                        Next
                    </button>
                </div>
            )}
        </div>
    )
}
