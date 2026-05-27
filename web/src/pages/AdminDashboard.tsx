import { useEffect, useState } from 'react'
import { api } from '../lib/api'

export default function AdminDashboard() {
    const [users, setUsers] = useState<any[]>([])
    const [apps, setApps] = useState<any[]>([])

    const loadData = () => {
        api.admin.listUsers().then(d => setUsers(d.data || [])).catch(console.error)
        api.admin.listApps().then(d => setApps(d.data || [])).catch(console.error)
    }

    useEffect(() => { loadData() }, [])

    const handleRoleChange = async (userId: string, newRole: string) => {
        if (!confirm(`Change role to ${newRole}?`)) return
        try {
            await api.admin.updateRole(userId, newRole)
            loadData()
        } catch (e: any) {
            alert(e.message)
        }
    }

    const handleAppReview = async (userId: string, status: string) => {
        if (!confirm(`Mark application as ${status}?`)) return
        try {
            await api.admin.reviewApp(userId, status)
            loadData()
        } catch (e: any) {
            alert(e.message)
        }
    }

    return (
        <div>
            <h1 className="text-2xl font-bold mb-6">Admin Dashboard</h1>

            {/* Setter Applications */}
            <section className="mb-8">
                <h2 className="text-lg font-semibold mb-3">Setter Applications</h2>
                <div className="border border-gray-200 rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Applicant ID</th>
                                <th className="px-4 py-3 text-left">Reason</th>
                                <th className="px-4 py-3 text-left">Status</th>
                                <th className="px-4 py-3 text-left">Applied</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                            {apps.map(a => (
                                <tr key={a.user_id}>
                                    <td className="px-4 py-3 font-mono text-xs">{a.user_id.substring(0, 8)}...</td>
                                    <td className="px-4 py-3 max-w-xs truncate" title={a.reason}>{a.reason}</td>
                                    <td className="px-4 py-3">
                                        <span className={`px-2 py-1 rounded text-xs font-medium ${
                                            a.status === 'pending' ? 'bg-yellow-100 text-yellow-800' :
                                            a.status === 'approved' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
                                        }`}>
                                            {a.status}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3 text-gray-500">{new Date(a.created_at).toLocaleString()}</td>
                                    <td className="px-4 py-3 text-right">
                                        {a.status === 'pending' && (
                                            <div className="flex justify-end gap-2">
                                                <button onClick={() => handleAppReview(a.user_id, 'approved')} className="text-green-600 hover:underline">Approve</button>
                                                <button onClick={() => handleAppReview(a.user_id, 'rejected')} className="text-red-600 hover:underline">Reject</button>
                                            </div>
                                        )}
                                    </td>
                                </tr>
                            ))}
                            {apps.length === 0 && (
                                <tr>
                                    <td colSpan={5} className="px-4 py-8 text-center text-gray-400">
                                        No pending applications.
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>
            </section>

            {/* Users Table */}
            <section className="mb-8">
                <h2 className="text-lg font-semibold mb-3">Users</h2>
                <div className="border border-gray-200 rounded-lg overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                            <tr>
                                <th className="px-4 py-3 text-left">Username</th>
                                <th className="px-4 py-3 text-left">Email</th>
                                <th className="px-4 py-3 text-left">Role</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100">
                            {users.map(u => (
                                <tr key={u.id}>
                                    <td className="px-4 py-3 font-medium">{u.username}</td>
                                    <td className="px-4 py-3 text-gray-500">{u.email}</td>
                                    <td className="px-4 py-3">
                                        <select 
                                            value={u.role} 
                                            onChange={e => handleRoleChange(u.id, e.target.value)}
                                            className="border rounded px-2 py-1 text-sm bg-gray-50"
                                            disabled={u.username === 'admin'}
                                        >
                                            <option value="user">User</option>
                                            <option value="teacher">Teacher (Setter)</option>
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
                                    <td colSpan={4} className="px-4 py-8 text-center text-gray-400">
                                        No users found.
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>
            </section>
        </div>
    )
}
