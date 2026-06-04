interface SettingsTabProps {
  visible: boolean
  saving: boolean
  onUpdateVisibility: () => void
  onToggleVisibility: (visible: boolean) => void
  onDelete: () => void
}

export default function SettingsTab({
  visible,
  saving,
  onUpdateVisibility,
  onToggleVisibility,
  onDelete,
}: SettingsTabProps) {
  return (
    <div className="space-y-6">
      <h2 className="text-lg font-bold text-gray-900 dark:text-gray-100 border-b pb-2 mb-4">Workspace Settings</h2>

      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <input
            type="checkbox"
            id="visible"
            checked={visible}
            onChange={e => onToggleVisibility(e.target.checked)}
            className="h-4 w-4 border-gray-300 dark:border-gray-600 rounded text-blue-600 dark:text-blue-400 focus:ring-blue-500 dark:focus:ring-blue-400"
          />
          <label htmlFor="visible" className="text-sm font-medium text-gray-700 dark:text-gray-300">
            Problem Visibility (Visible to Solvers)
          </label>
        </div>
        <p className="text-xs text-gray-400 dark:text-gray-500">
          If unchecked, the problem statement, statistics, and editorials will only be visible to
          authors and testers. Useful for preparing problems before contests.
        </p>

        <button
          onClick={onUpdateVisibility}
          disabled={saving}
          className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-blue-700 transition-colors cursor-pointer"
        >
          Update Visibility
        </button>
      </div>

      <hr className="border-gray-200 dark:border-gray-700" />

      <div className="space-y-2">
        <h3 className="text-sm font-bold text-red-600 dark:text-red-400 uppercase tracking-wider">Danger Zone</h3>
        <p className="text-xs text-gray-400 dark:text-gray-500">
          Deleting the problem will discard all statements, submissions history, and testcase files
          from the server sandbox. This operation cannot be undone.
        </p>
        <button
          onClick={onDelete}
          className="bg-red-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-red-700 transition-colors cursor-pointer"
        >
          Delete Problem
        </button>
      </div>
    </div>
  )
}
