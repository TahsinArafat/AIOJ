import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Link } from 'react-router-dom'
import { api } from './lib/api'
import Login from './pages/Login'
import Register from './pages/Register'
import ForgotPassword from './pages/ForgotPassword'
import ResetPassword from './pages/ResetPassword'
import ProblemList from './pages/ProblemList'
import ProblemDetail from './pages/ProblemDetail'
import ContestList from './pages/ContestList'
import ContestDetail from './pages/ContestDetail'
import ContestScoreboard from './pages/ContestScoreboard'
import AdminDashboard from './pages/AdminDashboard'
import SetterPanel from './pages/SetterPanel'
import SetterProblemWorkspace from './pages/SetterProblemWorkspace'
import SetterTestPage from './pages/SetterTestPage'
import Profile from './pages/Profile'
import ProblemCreate from './pages/ProblemCreate'
import Navbar from './components/Navbar'
import Practice from './pages/Practice'
import VirtualContest from './pages/VirtualContest'
import RatingHistory from './pages/RatingHistory'
import Submissions from './pages/Submissions'
import SubmissionDetail from './pages/SubmissionDetail'
import GymList from './pages/GymList'
import GymDetail from './pages/GymDetail'
import HackPanel from './pages/HackPanel'
import GroupList from './pages/GroupList'
import GroupCreate from './pages/GroupCreate'
import GroupDetail from './pages/GroupDetail'
import TeamList from './pages/TeamList'
import TeamCreate from './pages/TeamCreate'
import TeamDetail from './pages/TeamDetail'
import BlogList from './pages/BlogList'
import BlogCreate from './pages/BlogCreate'
import BlogDetail from './pages/BlogDetail'
import EditorialList from './pages/EditorialList'
import EditorialDetail from './pages/EditorialDetail'
import APISettings from './pages/APISettings'
import Rankings from './pages/Rankings'
import UserPublicProfile from './pages/UserPublicProfile'
import ContestCreate from './pages/ContestCreate'
import NotificationPreferences from './pages/NotificationPreferences'
import Notifications from './pages/Notifications'
import OrganizationList from './pages/OrganizationList'
import OrganizationCreate from './pages/OrganizationCreate'
import OrganizationDetail from './pages/OrganizationDetail'
import ClassDetail from './pages/ClassDetail'
import TrainingPlanList from './pages/TrainingPlanList'
import TrainingPlanCreate from './pages/TrainingPlanCreate'
import TrainingPlanDetail from './pages/TrainingPlanDetail'
import ContestPlagiarism from './pages/ContestPlagiarism'
import './global.css'

function Home() {
    const [contests, setContests] = useState<any[]>([])
    const [posts, setPosts] = useState<any[]>([])
    const [stats, setStats] = useState({ problems: 0, users: 0, submissions: 0 })
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        Promise.all([
            api.contests.list(0, 5),
            api.blog.list(0, 3),
            api.stats.getPlatform(),
        ]).then(([contestData, blogData, statsData]) => {
            setContests(contestData.data || [])
            setPosts(blogData.data || [])
            setStats(statsData)
        }).catch(() => {}).finally(() => setLoading(false))
    }, [])

    const contestStatus = (c: any) => {
        const now = Date.now()
        const start = new Date(c.start_time).getTime()
        const end = new Date(c.end_time).getTime()
        if (now < start) return { text: 'Upcoming', cls: 'text-blue-600 bg-blue-50' }
        if (now < end) return { text: 'Running', cls: 'text-green-600 bg-green-50' }
        return { text: 'Ended', cls: 'text-gray-500 bg-gray-100' }
    }

    return (
        <div className="space-y-10">
            {/* Hero */}
            <section className="bg-gradient-to-r from-blue-500 to-blue-700 text-white rounded-lg px-8 py-12">
                <h1 className="text-3xl font-bold mb-2">Welcome to AIOJ</h1>
                <p className="text-blue-100 mb-6 max-w-lg">A lightweight online judge for competitive programming. Practice problems, join contests, and improve your skills.</p>
                <div className="flex gap-3">
                    <Link to="/problems" className="bg-white text-blue-700 px-5 py-2 rounded font-medium hover:bg-blue-50 transition-colors">Browse Problems</Link>
                    <Link to="/contests" className="border border-white/40 px-5 py-2 rounded font-medium hover:bg-white/10 transition-colors">View Contests</Link>
                </div>
            </section>

            {/* Stats */}
            <section className="grid grid-cols-3 gap-4">
                {[
                    { label: 'Problems', value: stats.problems },
                    { label: 'Users', value: stats.users },
                    { label: 'Submissions', value: stats.submissions },
                ].map(s => (
                    <div key={s.label} className="border border-gray-200 rounded-lg p-5 text-center bg-white">
                        <div className="text-2xl font-bold text-gray-900">{s.value.toLocaleString()}</div>
                        <div className="text-sm text-gray-500 mt-1">{s.label}</div>
                    </div>
                ))}
            </section>

            {/* Recent Contests */}
            <section>
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-bold">Recent Contests</h2>
                    <Link to="/contests" className="text-sm text-blue-600 hover:underline">View all</Link>
                </div>
                {loading ? (
                    <div className="text-center py-8 text-gray-400">Loading...</div>
                ) : contests.length === 0 ? (
                    <div className="text-center py-8 text-gray-400">No contests yet.</div>
                ) : (
                    <div className="space-y-2">
                        {contests.map(c => {
                            const status = contestStatus(c)
                            return (
                                <Link key={c.id} to={`/contests/${c.id}`}
                                    className="block border border-gray-200 rounded-lg p-4 hover:bg-gray-50 transition-colors bg-white">
                                    <div className="flex items-center justify-between">
                                        <span className="font-medium">{c.title}</span>
                                        <span className={`text-xs px-2 py-0.5 rounded font-medium ${status.cls}`}>{status.text}</span>
                                    </div>
                                    <div className="text-xs text-gray-400 mt-1">
                                        {new Date(c.start_time).toLocaleDateString()} — {new Date(c.end_time).toLocaleDateString()}
                                    </div>
                                </Link>
                            )
                        })}
                    </div>
                )}
            </section>

            {/* Recent Blog Posts */}
            <section>
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-bold">Latest Posts</h2>
                    <Link to="/blog" className="text-sm text-blue-600 hover:underline">View all</Link>
                </div>
                {loading ? (
                    <div className="text-center py-8 text-gray-400">Loading...</div>
                ) : posts.length === 0 ? (
                    <div className="text-center py-8 text-gray-400">No posts yet.</div>
                ) : (
                    <div className="space-y-2">
                        {posts.map(p => (
                            <Link key={p.id} to={`/blog/${p.id}`}
                                className="block border border-gray-200 rounded-lg p-4 hover:bg-gray-50 transition-colors bg-white">
                                <h3 className="font-medium">{p.title}</h3>
                                <div className="text-xs text-gray-400 mt-1 flex gap-3">
                                    <span className="text-gray-600">{p.username}</span>
                                    <span>{p.upvotes} votes</span>
                                    <span>{new Date(p.created_at).toLocaleDateString()}</span>
                                </div>
                            </Link>
                        ))}
                    </div>
                )}
            </section>

            {/* Quick Links */}
            <section>
                <h2 className="text-xl font-bold mb-4">Quick Links</h2>
                <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
                    {[
                        { to: '/problems', label: 'Browse Problems' },
                        { to: '/contests', label: 'View Contests' },
                        { to: '/practice', label: 'Practice' },
                        { to: '/blog', label: 'Blog' },
                        { to: '/rankings', label: 'Rankings' },
                    ].map(link => (
                        <Link key={link.to} to={link.to}
                            className="border border-gray-200 rounded-lg p-4 text-center text-sm font-medium hover:bg-gray-50 transition-colors bg-white">
                            {link.label}
                        </Link>
                    ))}
                </div>
            </section>
        </div>
    )
}

export default function App() {
    return (
        <BrowserRouter>
            <div className="min-h-screen bg-white">
                <Navbar />
                <main className="max-w-[1400px] mx-auto px-6 py-6">
                    <Routes>
                        <Route path="/" element={<Home />} />
                        <Route path="/problems" element={<ProblemList />} />
                        <Route path="/problems/:slug" element={<ProblemDetail />} />
                        <Route path="/login" element={<Login />} />
                        <Route path="/register" element={<Register />} />
                        <Route path="/forgot-password" element={<ForgotPassword />} />
                        <Route path="/reset-password" element={<ResetPassword />} />
                        <Route path="/contests" element={<ContestList />} />
                        <Route path="/contests/:id" element={<ContestDetail />} />
                        <Route path="/contests/:id/scoreboard" element={<ContestScoreboard />} />
                        <Route path="/contests/:id/plagiarism" element={<ContestPlagiarism />} />
                        <Route path="/gym" element={<GymList />} />
                        <Route path="/gym/:id" element={<GymDetail />} />
                        <Route path="/hack/:contestId/:problemId" element={<HackPanel />} />
                        <Route path="/groups" element={<GroupList />} />
                        <Route path="/groups/create" element={<GroupCreate />} />
                        <Route path="/groups/:id" element={<GroupDetail />} />
                        <Route path="/teams" element={<TeamList />} />
                        <Route path="/teams/create" element={<TeamCreate />} />
                        <Route path="/teams/:id" element={<TeamDetail />} />
                        <Route path="/blog" element={<BlogList />} />
                        <Route path="/blog/create" element={<BlogCreate />} />
                        <Route path="/blog/:id" element={<BlogDetail />} />
                        <Route path="/editorials" element={<EditorialList />} />
                        <Route path="/editorials/:id" element={<EditorialDetail />} />
                        <Route path="/settings/api" element={<APISettings />} />
                        <Route path="/settings/notifications" element={<NotificationPreferences />} />
                        <Route path="/notifications" element={<Notifications />} />
                        <Route path="/submissions" element={<Submissions />} />
                        <Route path="/submissions/:id" element={<SubmissionDetail />} />
                        <Route path="/admin" element={<AdminDashboard />} />
                        <Route path="/setter" element={<SetterPanel />} />
                        <Route path="/setter/create" element={<ProblemCreate />} />
                        <Route path="/setter/:slug" element={<SetterProblemWorkspace />} />
                        <Route path="/setter/contest/create" element={<ContestCreate />} />
                        <Route path="/setter/test1" element={<SetterTestPage />} />
                        <Route path="/practice" element={<Practice />} />
                        <Route path="/organizations" element={<OrganizationList />} />
                        <Route path="/organizations/create" element={<OrganizationCreate />} />
                        <Route path="/organizations/:id" element={<OrganizationDetail />} />
                        <Route path="/classes/:id" element={<ClassDetail />} />
                        <Route path="/training" element={<TrainingPlanList />} />
                        <Route path="/training/create" element={<TrainingPlanCreate />} />
                        <Route path="/training/:id" element={<TrainingPlanDetail />} />
                        <Route path="/profile" element={<Profile />} />
                        <Route path="/virtual" element={<VirtualContest />} />
                        <Route path="/rating-history" element={<RatingHistory />} />
                        <Route path="/rankings" element={<Rankings />} />
                        <Route path="/user/:username" element={<UserPublicProfile />} />
                        <Route path="*" element={<div className="text-center py-20 text-gray-400">404 Not Found</div>} />
                    </Routes>
                </main>
            </div>
        </BrowserRouter>
    )
}