import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, getAccessToken } from '../lib/api'
import DivisionBadge from '../components/DivisionBadge'
import { resolveProblemSlug, resolveProblemTitle } from '../lib/problemSlugResolver'

export default function ContestDetail() {
    const { id } = useParams<{ id: string }>()
    const [data, setData] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [registered, setRegistered] = useState(false)
    const [registrationCount, setRegistrationCount] = useState(0)
    const [problemSlugs, setProblemSlugs] = useState<Map<string, string>>(new Map())
    const [problemTitles, setProblemTitles] = useState<Map<string, string>>(new Map())

    useEffect(() => {
        if (!id) return
        api.contests.get(id).then(d => {
            setData(d)
            // Resolve UUID → slug/title for each problem
            if (d.problems?.length) {
                Promise.all(d.problems.map(async (p: any) => {
                    const [slug, title] = await Promise.all([
                        resolveProblemSlug(p.problem_id),
                        resolveProblemTitle(p.problem_id),
                    ])
                    return { id: p.problem_id, slug, title }
                })).then(results => {
                    const slugMap = new Map<string, string>()
                    const titleMap = new Map<string, string>()
                    for (const r of results) {
                        if (r.slug) slugMap.set(r.id, r.slug)
                        if (r.title) titleMap.set(r.id, r.title)
                    }
                    setProblemSlugs(slugMap)
                    setProblemTitles(titleMap)
                })
            }
        }).catch(() => {}).finally(() => setLoading(false))
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
                                {problems.map((p: any) => {
                                    const slug = problemSlugs.get(p.problem_id)
                                    const title = problemTitles.get(p.problem_id)
                                    return (
                                    <tr key={p.problem_id} className="hover:bg-gray-50">
                                        <td className="px-4 py-2.5 font-bold text-blue-600">{p.index}</td>
                                        <td className="px-4 py-2.5 flex items-center justify-between">
                                            <Link to={slug ? `/problems/${slug}` : '#'} className="hover:underline text-blue-600 font-medium">
                                                {title || p.problem_id}
                                            </Link>
                                            <div className="flex gap-2 items-center">
                                                {contest.hack_phase_enabled && slug && (
                                                    <Link to={`/hack/${id}/${slug}`} className="text-xs text-red-600 bg-red-50 hover:bg-red-100 px-2.5 py-1 rounded font-medium">
                                                        Hack
                                                    </Link>
                                                )}
                                                {isEnded && (
                                                    <span className="text-xs text-gray-500 bg-gray-100 px-2 py-0.5 rounded">Upsolving Mode</span>
                                                )}
                                            </div>
                                        </td>
                                    </tr>
                                )
                            })}
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

            {getAccessToken() && (
                <VirtualContestSection contestId={id!} isEnded={isEnded} />
            )}
        </div>
    )
}

function VirtualContestSection({ contestId, isEnded }: { contestId: string; isEnded: boolean }) {
    const [activeVirtual, setActiveVirtual] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [duration, setDuration] = useState(120)

    useEffect(() => {
        api.virtual.status().then(d => {
            if (d.is_active) setActiveVirtual(d)
        }).catch(() => {}).finally(() => setLoading(false))
    }, [])

    if (loading) return null

    return (
        <div className="border border-gray-200 rounded-lg p-4">
            <h3 className="font-semibold mb-2">Virtual Contest</h3>
            <p className="text-sm text-gray-600 mb-3">Simulate this contest as if it were live.</p>
            
            {activeVirtual ? (
                <Link to="/virtual" className="inline-block bg-purple-600 text-white px-4 py-2 rounded text-sm hover:bg-purple-700">
                    Continue Virtual Contest ({activeVirtual.remaining_minutes}min remaining)
                </Link>
            ) : isEnded ? (
                <div className="space-y-3">
                    <div className="flex items-center gap-3">
                        <label className="text-sm text-gray-600">Duration:</label>
                        <select value={duration} onChange={e => setDuration(Number(e.target.value))} className="border rounded px-2 py-1 text-sm">
                            <option value={60}>60 min</option>
                            <option value={120}>120 min</option>
                            <option value={180}>180 min</option>
                            <option value={240}>240 min</option>
                        </select>
                    </div>
                    <button onClick={async () => {
                        try { await api.virtual.start(contestId, duration); window.location.href = '/virtual' }
                        catch (e: any) { alert('Failed: ' + e.message) }
                    }} className="bg-purple-600 text-white px-4 py-2 rounded text-sm hover:bg-purple-700">
                        Start Virtual Contest
                    </button>
                </div>
            ) : (
                <p className="text-sm text-gray-400">Virtual contests are only available after the contest ends.</p>
            )}
        </div>
    )
}
