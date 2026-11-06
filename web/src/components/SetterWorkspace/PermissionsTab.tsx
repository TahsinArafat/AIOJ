import type { Collaborator } from '../../types/problem-workspace'
import { Badge, type BadgeTone } from '../ui/Badge'

/** Owner is the strongest level, so it gets the only "accent" pill. */
const ACCESS_TONE: Record<string, BadgeTone> = {
  owner: 'accent',
  co_author: 'brand',
  tester: 'neutral',
}

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
      <h2 className="text-lg font-bold text-ink border-b pb-2 mb-4">Problem Collaborators</h2>

      <div className="border border-hairline rounded-lg overflow-hidden mb-4">
        <table className="w-full text-sm text-left bg-surface">
          <thead className="bg-surface-2 text-muted text-xs uppercase font-semibold">
            <tr>
              <th className="px-4 py-2.5">User</th>
              <th className="px-4 py-2.5">Access Level</th>
              <th className="px-4 py-2.5 text-right">Action</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-hairline">
            {collaborators.map((c, idx) => (
              <tr key={idx} className="hover:bg-surface-2">
                <td className="px-4 py-2.5 font-medium text-ink">{c.username}</td>
                <td className="px-4 py-2.5">
                  {/* Both the owner and co-author pills used a fixed light
                      palette with no dark variant, so on the dark canvas the
                      co-author pill was dark blue text on a dark blue fill. */}
                  <Badge tone={ACCESS_TONE[c.access_level] || 'neutral'} className="uppercase tracking-wider">
                    {c.access_level}
                  </Badge>
                </td>
                <td className="px-4 py-2.5 text-right">
                  {c.access_level !== 'owner' ? (
                    <button
                      onClick={() => onRemove(c.user_id)}
                      className="text-bad hover:underline cursor-pointer"
                    >
                      Remove
                    </button>
                  ) : (
                    <span className="text-muted-soft text-xs font-medium">Primary Owner</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="bg-surface-2 border border-hairline rounded-lg p-4 space-y-4">
        <h4 className="font-semibold text-xs text-muted uppercase tracking-wider">Add Collaborator</h4>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-muted mb-1">Username</label>
            <input
              type="text"
              placeholder="Enter username"
              value={newUsername}
              onChange={e => onNewUsernameChange(e.target.value)}
              className="w-full border border-hairline-strong rounded px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)] bg-surface text-ink"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-muted mb-1">Access Level</label>
            <select
              value={newAccessLevel}
              onChange={e => onNewAccessLevelChange(e.target.value)}
              className="w-full border border-hairline-strong rounded px-3 py-1.5 text-xs bg-surface text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
            >
              <option value="co_author">Co-Author (Full Edit Permissions)</option>
              <option value="tester">Tester (Read & Submit Private Problem)</option>
            </select>
          </div>
        </div>
        <button
          onClick={onAdd}
          className="bg-brand text-on-brand px-3 py-1.5 rounded text-xs hover:bg-brand-hover transition-colors cursor-pointer"
        >
          Share Permissions
        </button>
      </div>
    </div>
  )
}
