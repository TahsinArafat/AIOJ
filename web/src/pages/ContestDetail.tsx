import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import DivisionBadge from '../components/DivisionBadge'

export default function ContestDetail() {
    const { id } = useParams<{ id: string }>()
    const [data, setData] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [registered, setRegistered] = useState(false)
    const [registrationCount, setRegistrationCount] = useState(0)

    useEffect(() => {
        if (!id) return
        api.contests.get(id).then(setData).catch(() => {}).finally(() => setLoading(false))
        if (getAccessToken()) {
            api.contests.checkRegistration(id).then(d => setRegistered(d.registered)).catch(() => {})
        }
        api.contests.listRegistrations(id).then(d => setRegistrationCount(d.count)).catch(() => {})
    }, [id])

    const handleRegister = async () => {
        if (!id) return
        try {
            await api.contests.register(id)
            setRegistered(true)
            setRegistrationCount(c => c + 1)
        } catch (e: any) { alert('Registration failed: ' + e.message) }
    }

    const handleUnregister = async () => {
        if (!id) return
        try {
            await api.contests.unregister(id)
            setRegistered(false)
            setRegistrationCount(c => Math.max(0, c - 1))
        } catch (e: any) { alert('Unregister failed: ' + e.message) }
    }

    if (loading) return <div className="text-center py-20 text-gray-400">Loading...</div>
    if (!data) return <div className="text-center py-20 text-gray-400">Contest not found.</div>

    const { contest, problems } = data
    const now = Date.now()
    const start = new Date(contest.start_time).getTime()
    const end = new Date(contest.end_time).getTime()
    const isRunning = now >= start && now <= end
    const isUpcoming = now < start
    const isEnded = now > end

    return (
        <div className="space-y-6">
            <div className="flex items-start justify-between">
                <div>
                    <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold">{contest.title}</h1>
                        <DivisionBadge division={contest.division ?? 0} />
                        {contest.type === 'educational' && (
                            <span className="bg-green-100 text-green-800 text-xs px-2 py-0.5 rounded font-medium">Educational</span>
                        )}
                    </div>
                    {contest.description && <p className="text-gray-600 mt-1">{contest.description}</p>}
                    <div className="mt-3 text-sm text-gray-500 space-y-1">
                        <div>Start: <span className="text-gray-700">{new Date(contest.start_time).toLocaleString()}</span></div>
                        <div>End: <span className="text-gray-700">{new Date(contest.end_time).toLocaleString()}</span></div>
                        {contest.freeze_time && (
                            <div>Freeze: <span className="text-gray-700">{new Date(contest.freeze_time).toLocaleString()}</span></div>
                        )}
                        <div>Type: <span className="text-gray-700 uppercase font-medium">{contest.type}</span></div>
                    </div>
                </div>
                <div>
                    {isRunning && <span className="text-green-600 bg-green-50 px-3 py-1 rounded font-medium text-sm">Running</span>}
                    {isUpcoming && <span className="text-blue-600 bg-blue-50 px-3 py-1 rounded font-medium text-sm">Upcoming</span>}
                    {isEnded && <span className="text-gray-500 bg-gray-100 px-3 py-1 rounded font-medium text-sm">Ended</span>}
                </div>
            </div>

            {contest.registration_required && (
                <div className="border border-gray-200 rounded-lg p-4">
                    <div className="flex items-center justify-between mb-2">
                        <span className="text-sm text-gray-600">
                            {registrationCount} / {contest.max_participants || '∞'} registered
                        </span>
                        {registered && <span className="text-green-600 text-sm">✓ Registered</span>}
                    </div>
                    {isUpcoming && getAccessToken() && (
                        <button
                            onClick={() => registered ? handleUnregister() : handleRegister()}
                            className={`w-full py-2 rounded text-sm font-medium ${
                                registered
                                    ? 'bg-red-50 text-red-600 hover:bg-red-100'
                                    : 'bg-blue-600 text-white hover:bg-blue-700'
                            }`}
                        >
                            {registered ? 'Unregister' : 'Register'}
                        </button>
                    )}
                </div>
            )}

            {contest.type === 'educational' && (
                <div className="bg-green-50 border border-green-200 rounded-lg p-4">
                    <h3 className="font-semibold text-green-800 mb-1">Educational Contest</h3>
                    <p className="text-sm text-green-700">
                        This is an educational round. After the contest ends, there will be a {contest.educational_config?.hack_phase_hours || 24}-hour open hacking phase where you can review others' code, submit counter-test cases, and contribute to the problem tests!
                    </p>
                </div>
            )}

            <div>
                <h2 className="text-lg font-semibold mb-3">Problems</h2>
                {problems?.length > 0 ? (
                    <div className="border border-gray-200 rounded-lg overflow-hidden">
                        <table className="w-full text-sm">
                            <thead className="bg-gray-50">
                                <tr>
                                    <th className="px-4 py-2 text-left text-gray-500 text-xs uppercase">#</th>
                                    <th className="px-4 py-2 text-left text-gray-500 text-xs uppercase">Problem</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100">
                                {problems.map((p: any) => (
                                    <tr key={p.problem_id} className="hover:bg-gray-50">
                                        <td className="px-4 py-2.5 font-bold text-blue-600">{p.index}</td>
                                        <td className="px-4 py-2.5">
                                            <Link to={`/problems/${p.problem_id}`} className="hover:underline text-blue-600">
                                                {p.problem_id}
                                            </Link>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                ) : <p className="text-gray-400">No problems added yet.</p>}
            </div>

            {(isRunning || isEnded) && (
                <div className="flex gap-4 items-center">
                    <Link to={`/contests/${id}/scoreboard`}
                        className="inline-flex items-center gap-1 text-blue-600 hover:underline text-sm">
                        View Scoreboard →
                    </Link>
                </div>
            )}

            {isEnded && getAccessToken() && (
                <div className="border border-gray-200 rounded-lg p-4">
                    <h3 className="font-semibold mb-2">Virtual Contest</h3>
                    <p className="text-sm text-gray-600 mb-3">Simulate this contest as if it were live.</p>
                    <button onClick={async () => {
                        if (!id) return
                        try { await api.virtual.start(id); alert('Virtual contest started!') }
                        catch (e: any) { alert('Failed: ' + e.message) }
                    }} className="bg-purple-600 text-white px-4 py-2 rounded text-sm hover:bg-purple-700">
                        Start Virtual Contest
                    </button>
                </div>
            )}
        </div>
    )
}
