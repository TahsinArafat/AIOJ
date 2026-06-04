import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import EditorialForm from '../EditorialForm'

interface EditorialTabProps {
  problemId: string
  isUserAdmin: boolean
}

export default function EditorialTab({ problemId, isUserAdmin }: EditorialTabProps) {
  const [editorials, setEditorials] = useState<any[]>([])
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
    loadEditorials()
  }, [problemId])

  if (loading) {
    return <div className="text-center py-10 text-gray-500">Loading editorials...</div>
  }

  return (
    <div className="space-y-6 text-gray-900 dark:text-gray-100">
      <div className="flex justify-between items-center border-b pb-4 border-gray-200 dark:border-gray-700">
        <div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Editorials</h2>
          <p className="text-xs text-gray-500">Create or view explanation of solutions for this problem.</p>
        </div>
        {!isAdding && (
          <button
            onClick={() => setIsAdding(true)}
            className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1.5 rounded text-sm font-medium"
          >
            Add Editorial
          </button>
        )}
      </div>

      {isAdding ? (
        <div className="border rounded-lg p-6 bg-gray-50 dark:bg-gray-900/50 border-gray-200 dark:border-gray-700">
          <h3 className="text-md font-semibold mb-4 text-gray-900 dark:text-gray-100">New Editorial</h3>
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
        <div className="text-center py-20 border-2 border-dashed rounded-lg text-gray-400 dark:text-gray-500 border-gray-200 dark:border-gray-700">
          No editorials created yet.
        </div>
      ) : (
        <div className="grid gap-4">
          {editorials.map(e => (
            <div key={e.id} className="border rounded-lg p-4 bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700 flex justify-between items-center">
              <div>
                <div className="flex items-center gap-2">
                  {e.is_official && (
                    <span className="text-[10px] font-bold uppercase bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300 px-1.5 py-0.5 rounded">
                      Official
                    </span>
                  )}
                  <h4 className="font-semibold text-gray-900 dark:text-gray-100">{e.title}</h4>
                </div>
                <div className="text-xs text-gray-400 mt-1">
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
