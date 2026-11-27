import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import EditorialForm from '../EditorialForm'
import { useConfirm } from '../ConfirmDialog'
import type { Editorial } from '../../types/api'

interface EditorialItem { id: string; title: string; username?: string; is_official?: boolean; upvotes?: number }

interface EditorialTabProps {
  problemId: string
  isUserAdmin: boolean
}

export default function EditorialTab({ problemId, isUserAdmin }: EditorialTabProps) {
  const confirm = useConfirm()
  const [editorials, setEditorials] = useState<EditorialItem[]>([])
  const [loading, setLoading] = useState(true)
  const [isAdding, setIsAdding] = useState(false)
  // The editorial being edited, loaded in full: the list response carries only
  // the title, so the body has to be fetched before the form can prefill it.
  const [editing, setEditing] = useState<Editorial | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [error, setError] = useState('')

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

  const startEditing = async (id: string) => {
    setBusyId(id)
    setError('')
    try {
      const full = await api.editorials.get(id)
      setEditing(full)
      setIsAdding(false)
    } catch {
      setError('Could not load that editorial.')
    } finally {
      setBusyId(null)
    }
  }

  const handleDelete = async (item: EditorialItem) => {
    const ok = await confirm({
      title: 'Delete editorial',
      message: `Delete "${item.title}"? This cannot be undone.`,
      confirmLabel: 'Delete',
      variant: 'danger',
    })
    if (!ok) return
    setBusyId(item.id)
    setError('')
    try {
      await api.editorials.delete(item.id)
      setEditorials(prev => prev.filter(e => e.id !== item.id))
    } catch {
      setError('Could not delete that editorial.')
    } finally {
      setBusyId(null)
    }
  }

  if (loading) {
    return <div className="text-center py-10 text-muted">Loading editorials...</div>
  }

  const showForm = isAdding || editing !== null

  return (
    <div className="space-y-6 text-ink">
      <div className="flex justify-between items-center border-b pb-4 border-hairline">
        <div>
          <h2 className="text-lg font-semibold text-ink">Editorials</h2>
          <p className="text-xs text-muted">Create or view explanation of solutions for this problem.</p>
        </div>
        {!showForm && (
          <button
            onClick={() => { setIsAdding(true); setEditing(null) }}
            className="bg-brand hover:bg-brand-hover text-on-brand px-3 py-1.5 rounded text-sm font-medium"
          >
            Add Editorial
          </button>
        )}
      </div>

      {error && <div className="p-3 bg-bad/10 text-bad rounded border border-bad/30 text-sm">{error}</div>}

      {showForm ? (
        <div className="border rounded-lg p-6 bg-surface-2/50 border-hairline">
          <h3 className="text-md font-semibold mb-4 text-ink">
            {editing ? 'Edit Editorial' : 'New Editorial'}
          </h3>
          <EditorialForm
            problemId={problemId}
            isUserAdmin={isUserAdmin}
            editorial={editing ?? undefined}
            onSuccess={() => {
              setIsAdding(false)
              setEditing(null)
              loadEditorials()
            }}
            onCancel={() => { setIsAdding(false); setEditing(null) }}
          />
        </div>
      ) : editorials.length === 0 ? (
        <div className="text-center py-20 border-2 border-dashed rounded-lg text-muted-soft border-hairline">
          No editorials created yet.
        </div>
      ) : (
        <div className="grid gap-4">
          {editorials.map(e => (
            <div key={e.id} className="border rounded-lg p-4 bg-surface border-hairline flex justify-between items-center gap-4">
              <div>
                <div className="flex items-center gap-2">
                  {e.is_official && (
                    <span className="text-[10px] font-bold uppercase bg-ok/12 text-ok px-1.5 py-0.5 rounded">
                      Official
                    </span>
                  )}
                  <h4 className="font-semibold text-ink">{e.title}</h4>
                </div>
                <div className="text-xs text-muted-soft mt-1">
                  By {e.username} • upvotes: {e.upvotes}
                </div>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <button
                  type="button"
                  onClick={() => startEditing(e.id)}
                  disabled={busyId === e.id}
                  className="border border-hairline-strong px-2.5 py-1 rounded text-xs text-ink-soft hover:bg-surface-2 disabled:opacity-50"
                >
                  Edit
                </button>
                <button
                  type="button"
                  onClick={() => handleDelete(e)}
                  disabled={busyId === e.id}
                  className="border border-bad/40 px-2.5 py-1 rounded text-xs text-bad hover:bg-bad/10 disabled:opacity-50"
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
