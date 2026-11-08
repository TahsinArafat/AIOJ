import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import { useDocumentTitle } from '../lib/documentTitle'
import CommentSection from '../components/CommentSection'
import MarkdownContent from '../components/MarkdownContent'

interface EditorialDetailData { title: string; username?: string; created_at: string; is_official?: boolean; approach?: string; content?: string; solution_code?: string; solution_language?: string; time_complexity?: string; space_complexity?: string }

export default function EditorialDetail() {
    const { id } = useParams<{ id: string }>()
    const [editorial, setEditorial] = useState<EditorialDetailData | null>(null)
    useDocumentTitle(editorial?.title)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        if (!id) return
        api.editorials.get(id).then(setEditorial).catch(console.error).finally(() => setLoading(false))
    }, [id])

    if (loading) return <div className="text-center py-20 text-muted-soft">Loading...</div>
    if (!editorial) return <div className="text-center py-20 text-muted-soft">Editorial not found</div>

    return (
        <div className="max-w-3xl mx-auto">
            <article className="mb-8">
                <h1 className="text-3xl font-bold mb-2">{editorial.title}</h1>
                <div className="flex items-center gap-4 text-sm text-muted mb-6">
                    <span className="font-semibold text-ink-soft">{editorial.username}</span>
                    <span>{new Date(editorial.created_at).toLocaleDateString()}</span>
                    {editorial.is_official && (
                        <span className="text-xs bg-ok/12 text-ok px-2 py-0.5 rounded font-medium">Official</span>
                    )}
                </div>

                {editorial.approach && (
                    <div className="mb-4 p-3 bg-brand-soft rounded-lg">
                        <span className="text-xs font-semibold text-brand uppercase tracking-wide">Approach:</span>
                        <MarkdownContent inline content={editorial.approach} className="mt-1 text-sm text-blue-800" />
                    </div>
                )}

                <MarkdownContent content={editorial.content} className="prose max-w-none text-ink mb-8" />

                {editorial.solution_code && (
                    <div className="mb-8">
                        <h3 className="font-semibold text-ink-soft mb-2">
                            Solution Code {editorial.solution_language && `(${editorial.solution_language})`}
                        </h3>
                        <pre className="bg-gray-900 text-green-400 p-4 rounded-lg overflow-x-auto text-sm font-mono">
                            {editorial.solution_code}
                        </pre>
                    </div>
                )}

                {(editorial.time_complexity || editorial.space_complexity) && (
                    <div className="flex gap-6 mb-8">
                        {editorial.time_complexity && (
                            <div className="bg-surface-2 px-4 py-2 rounded-lg">
                                <span className="text-xs text-muted uppercase tracking-wide">Time:</span>
                                <MarkdownContent inline content={editorial.time_complexity} className="ml-2 font-mono font-semibold text-ink-soft" />
                            </div>
                        )}
                        {editorial.space_complexity && (
                            <div className="bg-surface-2 px-4 py-2 rounded-lg">
                                <span className="text-xs text-muted uppercase tracking-wide">Space:</span>
                                <MarkdownContent inline content={editorial.space_complexity} className="ml-2 font-mono font-semibold text-ink-soft" />
                            </div>
                        )}
                    </div>
                )}
            </article>

            <CommentSection parentType="editorial" parentId={id!} />
        </div>
    )
}
