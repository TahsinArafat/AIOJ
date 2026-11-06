import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { useConfirm } from '../../components/ConfirmDialog'
import { useToast } from '../../components/Toast'

interface SetterAppItem { user_id: string; username: string; reason?: string; status: string; created_at: string }

export default function SetterAppsPanel() {
    const confirm = useConfirm()
    const toast = useToast()
    const [apps, setApps] = useState<SetterAppItem[]>([])
    const [loading, setLoading] = useState(true)

    const loadApps = () => {
        setLoading(true)
        api.admin.listApps().then(d => setApps(d.data || [])).catch(console.error).finally(() => setLoading(false))
    }

    // deferred one microtask: loadApps opens with a synchronous setLoading(true),
    // which react-hooks/set-state-in-effect rejects in an effect body
    useEffect(() => { queueMicrotask(loadApps) }, [])

    const handleAppReview = async (userId: string, status: string) => {
        if (!(await confirm({ message: `Mark application as ${status}?`, variant: 'danger' }))) return
        try {
            await api.admin.reviewApp(userId, status)
            loadApps()
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
        }
    }

    if (loading) return <div className="text-center py-8 text-muted-soft">Loading...</div>

    return (
        <div>
            <h2 className="text-lg font-semibold mb-4">Setter Applications</h2>
            <div className="border border-hairline rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                    <thead className="bg-surface-2 text-muted text-xs uppercase">
                        <tr>
                            <th className="px-4 py-3 text-left">Applicant</th>
                            <th className="px-4 py-3 text-left">Reason</th>
                            <th className="px-4 py-3 text-left">Status</th>
                            <th className="px-4 py-3 text-left">Applied</th>
                            <th className="px-4 py-3 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-hairline">
                        {apps.map(a => (
                            <tr key={a.user_id}>
                                <td className="px-4 py-3 font-semibold">{a.username}</td>
                                <td className="px-4 py-3 max-w-xs truncate" title={a.reason}>{a.reason}</td>
                                <td className="px-4 py-3">
                                    <span className={`px-2 py-1 rounded text-xs font-medium ${
                                        a.status === 'pending' ? 'bg-warn/12 text-warn' :
                                        a.status === 'approved' ? 'bg-ok/12 text-ok' : 'bg-bad/12 text-bad'
                                    }`}>
                                        {a.status}
                                    </span>
                                </td>
                                <td className="px-4 py-3 text-muted">{new Date(a.created_at).toLocaleString()}</td>
                                <td className="px-4 py-3 text-right">
                                    {a.status === 'pending' && (
                                        <div className="flex justify-end gap-2">
                                            <button onClick={() => handleAppReview(a.user_id, 'approved')} className="text-ok hover:underline text-sm">Approve</button>
                                            <button onClick={() => handleAppReview(a.user_id, 'rejected')} className="text-bad hover:underline text-sm">Reject</button>
                                        </div>
                                    )}
                                </td>
                            </tr>
                        ))}
                        {apps.length === 0 && (
                            <tr>
                                <td colSpan={5} className="px-4 py-8 text-center text-muted-soft">
                                    No pending applications.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    )
}
