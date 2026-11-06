import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { EmptyState } from '../components/EmptyState'

interface BlogCard { id: string; title: string; username?: string; created_at: string; upvotes?: number; comment_count?: number; tags?: string[] }

export default function BlogList() {
    const [posts, setPosts] = useState<BlogCard[]>([])
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
                    <p className="text-sm text-muted mt-1">Read and share CP insights.</p>
                </div>
                <div className="flex items-center gap-4">
                    <input
                        type="text"
                        placeholder="Filter by Tag..."
                        value={tag}
                        onChange={handleTagChange}
                        className="border border-hairline-strong bg-transparent rounded px-3 py-1.5 text-sm"
                    />
                    <Link to="/blog/create" className="bg-brand text-on-brand px-4 py-2 rounded text-sm hover:bg-brand-hover font-medium whitespace-nowrap">
                        Write Post
                    </Link>
                </div>
            </div>

            <div className="space-y-3">
                {posts.map(p => (
                    <Link key={p.id} to={`/blog/${p.id}`} className="block border rounded-lg p-4 hover:bg-surface-2 transition-colors">
                        <div>
                            <h3 className="font-semibold text-lg">{p.title}</h3>
                            <div className="flex gap-4 mt-2 text-xs text-muted-soft">
                                <span className="font-medium text-muted">{p.username}</span>
                                <span>{p.upvotes} votes</span>
                                <span>{p.comment_count} comments</span>
                                <span>{new Date(p.created_at).toLocaleDateString()}</span>
                            </div>
                            {p.tags && p.tags.length > 0 && (
                                <div className="flex gap-2 mt-3">
                                    {p.tags.map((tag: string) => (
                                        <span key={tag} className="text-xs bg-brand-soft text-brand px-2 py-0.5 rounded font-medium">{tag}</span>
                                    ))}
                                </div>
                            )}
                        </div>
                    </Link>
                ))}
                {posts.length === 0 && (
                    <EmptyState text="No blog posts yet" description="Posts from the community will show up here." />
                )}
            </div>

            {total > 0 && <p className="text-sm text-muted-soft mt-4">{total} posts</p>}

            {total > LIMIT && (
                <div className="flex gap-2 justify-center mt-6">
                    <button
                        disabled={offset === 0}
                        onClick={() => setOffset(Math.max(0, offset - LIMIT))}
                        className="px-4 py-2 border rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-surface-2 cursor-pointer"
                    >
                        Previous
                    </button>
                    <button
                        disabled={offset + LIMIT >= total}
                        onClick={() => setOffset(offset + LIMIT)}
                        className="px-4 py-2 border rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-surface-2 cursor-pointer"
                    >
                        Next
                    </button>
                </div>
            )}
        </div>
    )
}
