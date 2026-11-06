import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { useConfirm } from '../../components/ConfirmDialog'
import { useToast } from '../../components/Toast'

interface AdminUser { id: string; username: string; email?: string; role: string }

export default function UsersPanel() {
    const confirm = useConfirm()
    const toast = useToast()
    const [users, setUsers] = useState<AdminUser[]>([])
    const [loading, setLoading] = useState(true)

    const loadUsers = () => {
        setLoading(true)
        api.admin.listUsers().then(d => setUsers(d.data || [])).catch(console.error).finally(() => setLoading(false))
    }

    // deferred one microtask: loadUsers opens with a synchronous setLoading(true),
    // which react-hooks/set-state-in-effect rejects in an effect body
    useEffect(() => { queueMicrotask(loadUsers) }, [])

    const handleRoleChange = async (userId: string, newRole: string) => {
        if (!(await confirm({ message: `Change role to ${newRole}?`, variant: 'danger' }))) return
        try {
            await api.admin.updateRole(userId, newRole)
            loadUsers()
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
        }
    }

    if (loading) return <div className="text-center py-8 text-muted-soft">Loading...</div>

    return (
        <div>
            <h2 className="text-lg font-semibold mb-4">Users</h2>
            <div className="border border-hairline rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-surface-2 text-muted text-xs uppercase">
                        <tr>
                            <th className="px-4 py-3 text-left">Username</th>
                            <th className="px-4 py-3 text-left">Email</th>
                            <th className="px-4 py-3 text-left">Role</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-hairline">
                        {users.map(u => (
                            <tr key={u.id}>
                                <td className="px-4 py-3 font-medium">{u.username}</td>
                                <td className="px-4 py-3 text-muted">{u.email}</td>
                                <td className="px-4 py-3">
                                    <select
                                        value={u.role}
                                        onChange={e => handleRoleChange(u.id, e.target.value)}
                                        className="border rounded px-2 py-1 text-sm bg-surface-2"
                                        disabled={u.username === 'admin'}
                                    >
                                        <option value="user">User</option>
                                        <option value="setter">Problem Setter</option>
                                        <option value="admin">Admin</option>
                                        <option value="bot">Bot</option>
                                    </select>
                                </td>
                            </tr>
                        ))}
                        {users.length === 0 && (
                            <tr>
                                <td colSpan={3} className="px-4 py-8 text-center text-muted-soft">
                                    No users found.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    )
}
