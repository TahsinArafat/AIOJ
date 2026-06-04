import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../lib/api'
import CommentSection from '../components/CommentSection'

export default function EditorialDetail() {
    const { id } = useParams<{ id: string }>()
    const [editorial, setEditorial] = useState<any>(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        if (!id) return
        api.editorials.get(id).then(setEditorial).catch(console.error).finally(() => setLoading(false))
    }, [id])

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading...</div>
    if (!editorial) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Editorial not found</div>

    return (
        <div className="max-w-3xl mx-auto">
            <article className="mb-8">
                <h1 className="text-3xl font-bold mb-2">{editorial.title}</h1>
                <div className="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400 mb-6">
                    <span className="font-semibold text-gray-700 dark:text-gray-300">{editorial.username}</span>
                    <span>{new Date(editorial.created_at).toLocaleDateString()}</span>
                    {editorial.is_official && (
                        <span className="text-xs bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 px-2 py-0.5 rounded font-medium">Official</span>
                    )}
                </div>

                {editorial.approach && (
                    <div className="mb-4 p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
                        <span className="text-xs font-semibold text-blue-700 dark:text-blue-300 uppercase tracking-wide">Approach:</span>
                        <p className="mt-1 text-sm text-blue-800">{editorial.approach}</p>
                    </div>
                )}

                <div className="prose max-w-none text-gray-800 dark:text-gray-200 mb-8">{editorial.content}</div>

                {editorial.solution_code && (
                    <div className="mb-8">
                        <h3 className="font-semibold text-gray-700 dark:text-gray-300 mb-2">
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
                            <div className="bg-gray-50 dark:bg-gray-800 px-4 py-2 rounded-lg">
                                <span className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide">Time:</span>
                                <span className="ml-2 font-mono font-semibold text-gray-700 dark:text-gray-300">{editorial.time_complexity}</span>
                            </div>
                        )}
                        {editorial.space_complexity && (
                            <div className="bg-gray-50 dark:bg-gray-800 px-4 py-2 rounded-lg">
                                <span className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide">Space:</span>
                                <span className="ml-2 font-mono font-semibold text-gray-700 dark:text-gray-300">{editorial.space_complexity}</span>
                            </div>
                        )}
                    </div>
                )}
            </article>

            <CommentSection parentType="editorial" parentId={id!} />
        </div>
    )
}
