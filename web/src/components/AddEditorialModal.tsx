import EditorialForm from './EditorialForm'

interface AddEditorialModalProps {
  problemId: string
  isUserAdmin: boolean
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

export default function AddEditorialModal({ problemId, isUserAdmin, isOpen, onClose, onSuccess }: AddEditorialModalProps) {
  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 overflow-y-auto">
      <div className="relative w-full max-w-2xl bg-white dark:bg-gray-800 rounded-lg shadow-lg max-h-[90vh] overflow-y-auto p-6">
        <div className="flex justify-between items-center border-b pb-3 mb-4">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 font-medium">Add Editorial</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">
            ✕
          </button>
        </div>
        <EditorialForm
          problemId={problemId}
          isUserAdmin={isUserAdmin}
          onSuccess={() => {
            onSuccess()
            onClose()
          }}
          onCancel={onClose}
        />
      </div>
    </div>
  )
}
