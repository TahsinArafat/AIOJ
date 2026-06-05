import { useEffect, useState } from 'react'
import { api } from '../../lib/api'

export default function UsersPanel() {
    const [users, setUsers] = useState<any[]>([])
    const [loading, setLoading] = useState(true)

    const loadUsers = () => {
        setLoading(true)
        api.admin.listUsers().then(d => setUsers(d.data || [])).catch(console.error).finally(() => setLoading(false))
    }

    useEffect(() => { loadUsers() }, [])

    const handleRoleChange = async (userId: string, newRole: string) => {
        if (!confirm(`Change role to ${newRole}?`)) return
        try {
            await api.admin.updateRole(userId, newRole)
            loadUsers()
        } catch (e: any) {
            alert(e.message)
        }
    }

    if (loading) return <div className="text-center py-8 text-gray-400 dark:text-gray-500">Loading...</div>

    return (
        <div>
            <h2 className="text-lg font-semibold mb-4">Users</h2>
            <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                        <tr>
                            <th className="px-4 py-3 text-left">Username</th>
                            <th className="px-4 py-3 text-left">Email</th>
                            <th className="px-4 py-3 text-left">Role</th>
                            <th className="px-4 py-3 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                        {users.map(u => (
                            <tr key={u.id}>
                                <td className="px-4 py-3 font-medium">{u.username}</td>
                                <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{u.email}</td>
                                <td className="px-4 py-3">
                                    <select
                                        value={u.role}
                                        onChange={e => handleRoleChange(u.id, e.target.value)}
                                        className="border rounded px-2 py-1 text-sm bg-gray-50 dark:bg-gray-800"
                                        disabled={u.username === 'admin'}
                                    >
                                        <option value="user">User</option>
                                        <option value="setter">Problem Setter</option>
                                        <option value="admin">Admin</option>
                                        <option value="bot">Bot</option>
                                    </select>
                                </td>
                                <td className="px-4 py-3 text-right">
                                    {/* Actions */}
                                </td>
                            </tr>
                        ))}
                        {users.length === 0 && (
                            <tr>
                                <td colSpan={4} className="px-4 py-8 text-center text-gray-400 dark:text-gray-500">
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
