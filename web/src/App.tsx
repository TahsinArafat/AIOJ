import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Link } from 'react-router-dom'
import { api, contestSlug } from './lib/api'
import { ThemeProvider } from './context/ThemeContext'
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
import GroupJoin from './pages/GroupJoin'
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
import ContestProblem from './pages/ContestProblem'
import ContestEdit from './pages/ContestEdit'
import ContestManage from './pages/ContestManage'
import './global.css'

function Home() {
    const [contests, setContests] = useState<any[]>([])
    const [posts, setPosts] = useState<any[]>([])
    const [stats, setStats] = useState({ problems: 0, users: 0, submissions: 0 })
    const [rankings, setRankings] = useState<any[]>([])
    const [loading, setLoading] = useState(true)
    const token = localStorage.getItem('access_token')
    const username = token ? JSON.parse(atob(token.split('.')[1])).uname : null

    useEffect(() => {
        Promise.all([
            api.contests.list(0, 5),
            api.blog.list(0, 5),
            api.stats.getPlatform(),
            api.rankings.list(0, 10),
        ]).then(([contestData, blogData, statsData, rankData]) => {
            setContests(contestData.data || [])
            setPosts(blogData.data || [])
            setStats(statsData)
            setRankings(rankData.data || [])
        }).catch(() => {}).finally(() => setLoading(false))
    }, [])

    const contestStatus = (c: any) => {
        const now = Date.now()
        const start = new Date(c.start_time).getTime()
        const end = new Date(c.end_time).getTime()
        if (now < start) return { text: 'Upcoming', cls: 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20' }
        if (now < end) return { text: 'Running', cls: 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20' }
        return { text: 'Ended', cls: 'text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700' }
    }

    return (
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
            {/* Main Content (Left, spans 3 columns) */}
            <div className="lg:col-span-3 space-y-6">
                {/* Hero */}
                <section className="bg-gradient-to-r from-blue-500 to-blue-700 text-white rounded-lg px-8 py-12">
                    <h1 className="text-3xl font-bold mb-2">Welcome to AIOJ</h1>
                    <p className="text-blue-100 mb-6 max-w-lg">A lightweight online judge for competitive programming. Practice problems, join contests, and improve your skills.</p>
                    <div className="flex gap-3">
                        <Link to="/problems" className="bg-white dark:bg-gray-800 text-blue-700 dark:text-blue-300 px-5 py-2 rounded font-medium hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors">Browse Problems</Link>
                        <Link to="/contests" className="border border-white/40 px-5 py-2 rounded font-medium hover:bg-white dark:hover:bg-gray-700/10 transition-colors">View Contests</Link>
                    </div>
                </section>

                {/* Stats */}
                <section className="grid grid-cols-3 gap-4">
                    {[
                        { label: 'Problems', value: stats.problems },
                        { label: 'Users', value: stats.users },
                        { label: 'Submissions', value: stats.submissions },
                    ].map(s => (
                        <div key={s.label} className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 text-center bg-white dark:bg-gray-800">
                            <div className="text-2xl font-bold text-gray-900 dark:text-gray-100">{s.value.toLocaleString()}</div>
                            <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">{s.label}</div>
                        </div>
                    ))}
                </section>

                {/* Recent Blog Posts */}
                <section className="space-y-4">
                    <div className="flex justify-between items-center">
                        <h2 className="text-xl font-bold">Latest Posts</h2>
                        <Link to="/blog" className="text-sm text-blue-600 dark:text-blue-400 hover:underline">View all</Link>
                    </div>
                    {loading ? (
                        <div className="text-center py-8 text-gray-400 dark:text-gray-500">Loading...</div>
                    ) : posts.length === 0 ? (
                        <div className="text-center py-8 text-gray-400 dark:text-gray-500">No posts yet.</div>
                    ) : (
                        <div className="space-y-3">
                            {posts.map(p => (
                                <Link key={p.id} to={`/blog/${p.id}`}
                                    className="block border border-gray-200 dark:border-gray-700 rounded-lg p-5 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors bg-white dark:bg-gray-800 shadow-sm">
                                    <h3 className="text-lg font-semibold text-blue-600 dark:text-blue-400 hover:underline mb-2">{p.title}</h3>
                                    <div className="text-xs text-gray-500 dark:text-gray-400 flex gap-4 items-center">
                                        <span>By <span className="font-semibold text-gray-700 dark:text-gray-300">{p.username}</span></span>
                                        <span>•</span>
                                        <span>{p.upvotes} upvotes</span>
                                        <span>•</span>
                                        <span>{new Date(p.created_at).toLocaleDateString()}</span>
                                    </div>
                                </Link>
                            ))}
                        </div>
                    )}
                </section>
            </div>

            {/* Sidebar (Right, spans 1 column) */}
            <div className="space-y-6">
                {/* User Stats / Login box */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 shadow-sm">
                    {username ? (
                        <div className="text-center space-y-3">
                            <div className="w-16 h-16 bg-blue-100 dark:bg-blue-900/50 rounded-full flex items-center justify-center mx-auto">
                                <span className="text-xl font-bold text-blue-600 dark:text-blue-300 uppercase">{username[0]}</span>
                            </div>
                            <div>
                                <h3 className="font-bold text-gray-950 dark:text-gray-50">{username}</h3>
                                <p className="text-xs text-gray-500">Logged in</p>
                            </div>
                            <Link to="/profile" className="block text-xs bg-blue-600 text-white py-1.5 px-3 rounded hover:bg-blue-700 transition-colors font-medium">
                                View Profile
                            </Link>
                        </div>
                    ) : (
                        <div className="space-y-3 text-center">
                            <h3 className="font-bold text-gray-800 dark:text-gray-200 text-sm">Join the Community</h3>
                            <p className="text-xs text-gray-500">Sign in to solve problems, compete in contests, and read posts.</p>
                            <div className="flex gap-2 justify-center">
                                <Link to="/login" className="text-xs bg-blue-600 text-white py-1.5 px-4 rounded hover:bg-blue-700 transition-colors font-medium">
                                    Login
                                </Link>
                                <Link to="/register" className="text-xs bg-gray-100 hover:bg-gray-200 text-gray-700 py-1.5 px-4 rounded transition-colors font-medium">
                                    Register
                                </Link>
                            </div>
                        </div>
                    )}
                </div>

                {/* Contests Block */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 shadow-sm">
                    <h3 className="font-bold text-sm mb-3 border-b pb-2 border-gray-100 dark:border-gray-700">Recent & Upcoming Contests</h3>
                    {loading ? (
                        <div className="text-xs text-gray-400 py-2">Loading...</div>
                    ) : contests.length === 0 ? (
                        <div className="text-xs text-gray-400 py-2">No contests</div>
                    ) : (
                        <div className="space-y-2">
                            {contests.map(c => {
                                const status = contestStatus(c)
                                return (
                                    <Link key={c.id} to={`/contests/${contestSlug(c)}`} className="block group">
                                        <div className="text-xs font-semibold text-gray-850 dark:text-gray-205 group-hover:text-blue-600 transition-colors line-clamp-1">
                                            {c.title}
                                        </div>
                                        <div className="flex items-center justify-between mt-1 text-[10px] text-gray-500">
                                            <span>{new Date(c.start_time).toLocaleDateString()}</span>
                                            <span className={`px-1.5 rounded-[3px] font-medium scale-90 origin-right ${status.cls}`}>{status.text}</span>
                                        </div>
                                    </Link>
                                )
                            })}
                            <Link to="/contests" className="block text-center text-xs text-blue-600 hover:underline mt-2">
                                View all contests
                            </Link>
                        </div>
                    )}
                </div>

                {/* Top Rated Users rankings widget */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 shadow-sm">
                    <h3 className="font-bold text-sm mb-3 border-b pb-2 border-gray-100 dark:border-gray-700">Top Rated Users</h3>
                    {loading ? (
                        <div className="text-xs text-gray-400 py-2">Loading...</div>
                    ) : rankings.length === 0 ? (
                        <div className="text-xs text-gray-400 py-2">No rankings</div>
                    ) : (
                        <div className="space-y-2">
                            {rankings.map((user, i) => (
                                <div key={user.user_id} className="flex justify-between items-center text-xs">
                                    <div className="flex gap-2 items-center">
                                        <span className="font-mono text-gray-400 w-4">{i + 1}</span>
                                        <Link to={`/user/${user.username}`} className="font-semibold text-blue-600 dark:text-blue-400 hover:underline">
                                            {user.username}
                                        </Link>
                                    </div>
                                    <span className="font-mono font-bold text-gray-700 dark:text-gray-350">{user.rating}</span>
                                </div>
                            ))}
                            <Link to="/rankings" className="block text-center text-xs text-blue-600 hover:underline mt-2">
                                View full standings
                            </Link>
                        </div>
                    )}
                </div>

                {/* Quick Links */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 shadow-sm">
                    <h3 className="font-bold text-sm mb-3 border-b pb-2 border-gray-100 dark:border-gray-700">Quick Links</h3>
                    <div className="grid grid-cols-2 gap-2 text-center">
                        <Link to="/problems" className="bg-gray-50 hover:bg-gray-100 dark:bg-gray-900/50 dark:hover:bg-gray-900 border border-gray-100 dark:border-gray-750 text-xs py-2 rounded text-gray-700 dark:text-gray-300 font-medium transition-colors">Problems</Link>
                        <Link to="/practice" className="bg-gray-50 hover:bg-gray-100 dark:bg-gray-900/50 dark:hover:bg-gray-900 border border-gray-100 dark:border-gray-750 text-xs py-2 rounded text-gray-700 dark:text-gray-300 font-medium transition-colors">Practice</Link>
                        <Link to="/blog" className="bg-gray-50 hover:bg-gray-100 dark:bg-gray-900/50 dark:hover:bg-gray-900 border border-gray-100 dark:border-gray-750 text-xs py-2 rounded text-gray-700 dark:text-gray-300 font-medium transition-colors">Blogs</Link>
                        <Link to="/rankings" className="bg-gray-50 hover:bg-gray-100 dark:bg-gray-900/50 dark:hover:bg-gray-900 border border-gray-100 dark:border-gray-750 text-xs py-2 rounded text-gray-700 dark:text-gray-300 font-medium transition-colors">Rankings</Link>
                    </div>
                </div>
            </div>
        </div>
    )
}

export default function App() {
    return (
        <BrowserRouter>
            <ThemeProvider>
                <div className="min-h-screen bg-white dark:bg-gray-800">
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
                        <Route path="/contests/:contestId/problem/:index" element={<ContestProblem />} />
                        <Route path="/setter/contest/:id/edit" element={<ContestEdit />} />
                        <Route path="/setter/contest/:id/manage" element={<ContestManage />} />
                        <Route path="/gym" element={<GymList />} />
                        <Route path="/gym/:id" element={<GymDetail />} />
                        <Route path="/hack/:contestId/:problemId" element={<HackPanel />} />
                        <Route path="/groups" element={<GroupList />} />
                        <Route path="/groups/create" element={<GroupCreate />} />
                        <Route path="/groups/join" element={<GroupJoin />} />
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
                        <Route path="*" element={<div className="text-center py-20 text-gray-400 dark:text-gray-500">404 Not Found</div>} />
                    </Routes>
                </main>
            </div>
            </ThemeProvider>
        </BrowserRouter>
    )
}