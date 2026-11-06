import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import EditorialForm from '../EditorialForm'

interface EditorialItem { id: string; title: string; username?: string; is_official?: boolean; upvotes?: number }

interface EditorialTabProps {
  problemId: string
  isUserAdmin: boolean
}

export default function EditorialTab({ problemId, isUserAdmin }: EditorialTabProps) {
  const [editorials, setEditorials] = useState<EditorialItem[]>([])
  const [loading, setLoading] = useState(true)
  const [isAdding, setIsAdding] = useState(false)

  const loadEditorials = async () => {
    try {
      const res = await api.editorials.getByProblem(problemId)
      setEditorials(res.data || [])
    } catch (err) {
      console.error('Failed to load editorials', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    // deferred a microtask so no setState is reachable synchronously from the effect
    queueMicrotask(loadEditorials)
  }, [problemId]) // eslint-disable-line react-hooks/exhaustive-deps -- refetch only when the problem changes

  if (loading) {
    return <div className="text-center py-10 text-muted">Loading editorials...</div>
  }

  return (
    <div className="space-y-6 text-ink">
      <div className="flex justify-between items-center border-b pb-4 border-hairline">
        <div>
          <h2 className="text-lg font-semibold text-ink">Editorials</h2>
          <p className="text-xs text-muted">Create or view explanation of solutions for this problem.</p>
        </div>
        {!isAdding && (
          <button
            onClick={() => setIsAdding(true)}
            className="bg-brand hover:bg-brand-hover text-on-brand px-3 py-1.5 rounded text-sm font-medium"
          >
            Add Editorial
          </button>
        )}
      </div>

      {isAdding ? (
        <div className="border rounded-lg p-6 bg-surface-2/50 border-hairline">
          <h3 className="text-md font-semibold mb-4 text-ink">New Editorial</h3>
          <EditorialForm
            problemId={problemId}
            isUserAdmin={isUserAdmin}
            onSuccess={() => {
              setIsAdding(false)
              loadEditorials()
            }}
            onCancel={() => setIsAdding(false)}
          />
        </div>
      ) : editorials.length === 0 ? (
        <div className="text-center py-20 border-2 border-dashed rounded-lg text-muted-soft border-hairline">
          No editorials created yet.
        </div>
      ) : (
        <div className="grid gap-4">
          {editorials.map(e => (
            <div key={e.id} className="border rounded-lg p-4 bg-surface border-hairline flex justify-between items-center">
              <div>
                <div className="flex items-center gap-2">
                  {e.is_official && (
                    <span className="text-[10px] font-bold uppercase bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300 px-1.5 py-0.5 rounded">
                      Official
                    </span>
                  )}
                  <h4 className="font-semibold text-ink">{e.title}</h4>
                </div>
                <div className="text-xs text-muted-soft mt-1">
                  By {e.username} • upvotes: {e.upvotes}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
