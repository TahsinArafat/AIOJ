import type { Collaborator } from '../../types/problem-workspace'

interface PermissionsTabProps {
  collaborators: Collaborator[]
  newUsername: string
  newAccessLevel: string
  onAdd: () => void
  onRemove: (userId: string) => void
  onNewUsernameChange: (username: string) => void
  onNewAccessLevelChange: (level: string) => void
}

export default function PermissionsTab({
  collaborators,
  newUsername,
  newAccessLevel,
  onAdd,
  onRemove,
  onNewUsernameChange,
  onNewAccessLevelChange,
}: PermissionsTabProps) {
  return (
    <div className="space-y-6">
      <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Problem Collaborators</h2>

      <div className="border border-gray-200 rounded-lg overflow-hidden mb-4">
        <table className="w-full text-sm text-left bg-white">
          <thead className="bg-gray-50 text-gray-500 text-xs uppercase font-semibold">
            <tr>
              <th className="px-4 py-2.5">User</th>
              <th className="px-4 py-2.5">Access Level</th>
              <th className="px-4 py-2.5 text-right">Action</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {collaborators.map((c, idx) => (
              <tr key={idx} className="hover:bg-gray-50">
                <td className="px-4 py-2.5 font-medium text-gray-800">{c.username}</td>
                <td className="px-4 py-2.5">
                  <span className={`px-2 py-0.5 rounded text-xs font-semibold uppercase tracking-wider ${
                    c.access_level === 'owner' ? 'bg-purple-100 text-purple-800' :
                    c.access_level === 'co_author' ? 'bg-blue-100 text-blue-800' : 'bg-gray-100 text-gray-800'
                  }`}>
                    {c.access_level}
                  </span>
                </td>
                <td className="px-4 py-2.5 text-right">
                  {c.access_level !== 'owner' ? (
                    <button
                      onClick={() => onRemove(c.user_id)}
                      className="text-red-600 hover:underline cursor-pointer"
                    >
                      Remove
                    </button>
                  ) : (
                    <span className="text-gray-400 text-xs font-medium">Primary Owner</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-4">
        <h4 className="font-semibold text-xs text-gray-500 uppercase tracking-wider">Add Collaborator</h4>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Username</label>
            <input
              type="text"
              placeholder="Enter username"
              value={newUsername}
              onChange={e => onNewUsernameChange(e.target.value)}
              className="w-full border border-gray-300 rounded px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white text-gray-800"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Access Level</label>
            <select
              value={newAccessLevel}
              onChange={e => onNewAccessLevelChange(e.target.value)}
              className="w-full border border-gray-300 rounded px-3 py-1.5 text-xs bg-white text-gray-800 focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="co_author">Co-Author (Full Edit Permissions)</option>
              <option value="tester">Tester (Read & Submit Private Problem)</option>
            </select>
          </div>
        </div>
        <button
          onClick={onAdd}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-xs hover:bg-blue-700 transition-colors cursor-pointer"
        >
          Share Permissions
        </button>
      </div>
    </div>
  )
}
