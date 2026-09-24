import { useState, useEffect, Suspense, lazy } from 'react'
import { BrowserRouter, Routes, Route, Link, useLocation } from 'react-router-dom'
import { api, contestSlug } from './lib/api'
import { ThemeProvider } from './context/ThemeContext'
import { ConfirmProvider } from './components/ConfirmDialog'
import { ToastProvider } from './components/Toast'
import { useTranslation } from 'react-i18next'
const Login = lazy(() => import('./pages/Login'))
const Register = lazy(() => import('./pages/Register'))
const ForgotPassword = lazy(() => import('./pages/ForgotPassword'))
const ResetPassword = lazy(() => import('./pages/ResetPassword'))
const VerifyEmail = lazy(() => import('./pages/VerifyEmail'))
const OAuthComplete = lazy(() => import('./pages/OAuthComplete'))
const ProblemList = lazy(() => import('./pages/ProblemList'))
const ProblemDetail = lazy(() => import('./pages/ProblemDetail'))
const ContestList = lazy(() => import('./pages/ContestList'))
const ContestDetail = lazy(() => import('./pages/ContestDetail'))
const ContestScoreboard = lazy(() => import('./pages/ContestScoreboard'))
const AdminDashboard = lazy(() => import('./pages/AdminDashboard'))
const SetterPanel = lazy(() => import('./pages/SetterPanel'))
const SetterProblemWorkspace = lazy(() => import('./pages/SetterProblemWorkspace'))

const Profile = lazy(() => import('./pages/Profile'))
const ProblemCreate = lazy(() => import('./pages/ProblemCreate'))
import Navbar from './components/Navbar'
import ActivityFeed from './components/ActivityFeed'
const Practice = lazy(() => import('./pages/Practice'))
const VirtualContest = lazy(() => import('./pages/VirtualContest'))
const RatingHistory = lazy(() => import('./pages/RatingHistory'))
const Submissions = lazy(() => import('./pages/Submissions'))
const SubmissionDetail = lazy(() => import('./pages/SubmissionDetail'))
const GymList = lazy(() => import('./pages/GymList'))
const GymDetail = lazy(() => import('./pages/GymDetail'))
const HackPanel = lazy(() => import('./pages/HackPanel'))
const GroupList = lazy(() => import('./pages/GroupList'))
const GroupCreate = lazy(() => import('./pages/GroupCreate'))
const GroupDetail = lazy(() => import('./pages/GroupDetail'))
const GroupJoin = lazy(() => import('./pages/GroupJoin'))
const TeamList = lazy(() => import('./pages/TeamList'))
const TeamCreate = lazy(() => import('./pages/TeamCreate'))
const TeamDetail = lazy(() => import('./pages/TeamDetail'))
const BlogList = lazy(() => import('./pages/BlogList'))
const BlogCreate = lazy(() => import('./pages/BlogCreate'))
const BlogDetail = lazy(() => import('./pages/BlogDetail'))
const EditorialList = lazy(() => import('./pages/EditorialList'))
const EditorialDetail = lazy(() => import('./pages/EditorialDetail'))
const APISettings = lazy(() => import('./pages/APISettings'))
const Rankings = lazy(() => import('./pages/Rankings'))
const UserPublicProfile = lazy(() => import('./pages/UserPublicProfile'))
const ContestCreate = lazy(() => import('./pages/ContestCreate'))
const NotificationPreferences = lazy(() => import('./pages/NotificationPreferences'))
const Notifications = lazy(() => import('./pages/Notifications'))
const OrganizationList = lazy(() => import('./pages/OrganizationList'))
const OrganizationCreate = lazy(() => import('./pages/OrganizationCreate'))
const OrganizationDetail = lazy(() => import('./pages/OrganizationDetail'))
const ClassDetail = lazy(() => import('./pages/ClassDetail'))
const TrainingPlanList = lazy(() => import('./pages/TrainingPlanList'))
const TrainingPlanCreate = lazy(() => import('./pages/TrainingPlanCreate'))
const TrainingPlanDetail = lazy(() => import('./pages/TrainingPlanDetail'))
const ContestPlagiarism = lazy(() => import('./pages/ContestPlagiarism'))
const ContestProblem = lazy(() => import('./pages/ContestProblem'))
const ContestEdit = lazy(() => import('./pages/ContestEdit'))
const ContestManage = lazy(() => import('./pages/ContestManage'))
const IDE = lazy(() => import('./pages/IDE'))
const GenerateProblem = lazy(() => import('./pages/GenerateProblem'))
const TermsOfService = lazy(() => import('./pages/legal/TermsOfService'))
const PrivacyPolicy = lazy(() => import('./pages/legal/PrivacyPolicy'))
const DMCA = lazy(() => import('./pages/legal/DMCA'))
import CookieConsent from './components/CookieConsent'
const TwoFactorSetup = lazy(() => import('./pages/auth/TwoFactorSetup'))
const TwoFactorVerify = lazy(() => import('./pages/auth/TwoFactorVerify'))
const Settings = lazy(() => import('./pages/Settings'))
import './global.css'

function Home() {
    const { t } = useTranslation()
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
        }).catch(() => { }).finally(() => setLoading(false))
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
                    <h1 className="text-3xl font-bold mb-2">{t('home.welcome')}</h1>
                    <p className="text-blue-100 mb-6 max-w-lg">{t('home.subtitle')}</p>
                    <div className="flex gap-3">
                        <Link to="/problems" className="bg-white dark:bg-gray-800 text-blue-700 dark:text-blue-300 px-5 py-2 rounded font-medium hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors">{t('home.browseProblems')}</Link>
                        <Link to="/contests" className="border border-white/40 px-5 py-2 rounded font-medium hover:bg-white dark:hover:bg-gray-700/10 transition-colors">{t('home.viewContests')}</Link>
                    </div>
                </section>

                {/* Stats */}
                <section className="grid grid-cols-3 gap-4">
                    {[
                        { label: t('home.problems'), value: stats.problems },
                        { label: t('home.users'), value: stats.users },
                        { label: t('home.submissions'), value: stats.submissions },
                    ].map(s => (
                        <div key={s.label} className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 text-center bg-white dark:bg-gray-800">
                            <div className="text-2xl font-bold text-gray-900 dark:text-gray-100">{s.value.toLocaleString()}</div>
                            <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">{s.label}</div>
                        </div>
                    ))}
                </section>

                <ActivityFeed limit={15} />

                {/* Recent Blog Posts */}
                <section className="space-y-4">
                    <div className="flex justify-between items-center">
                        <h2 className="text-xl font-bold">{t('home.latestPosts')}</h2>
                        <Link to="/blog" className="text-sm text-blue-600 dark:text-blue-400 hover:underline">{t('home.viewAll')}</Link>
                    </div>
                    {loading ? (
                        <div className="text-center py-8 text-gray-400 dark:text-gray-500">{t('common.loading')}</div>
                    ) : posts.length === 0 ? (
                        <div className="text-center py-8 text-gray-400 dark:text-gray-500">{t('home.noPostsYet')}</div>
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
                                <p className="text-xs text-gray-500">{t('common.loggedIn')}</p>
                            </div>
                            <Link to="/profile" className="block text-xs bg-blue-600 text-white py-1.5 px-3 rounded hover:bg-blue-700 transition-colors font-medium">
                                {t('common.viewProfile')}
                            </Link>
                        </div>
                    ) : (
                        <div className="space-y-3 text-center">
                            <h3 className="font-bold text-gray-800 dark:text-gray-200 text-sm">{t('home.joinCommunity')}</h3>
                            <p className="text-xs text-gray-500">{t('home.signInPrompt')}</p>
                            <div className="flex gap-2 justify-center">
                                <Link to="/login" className="text-xs bg-blue-600 text-white py-1.5 px-4 rounded hover:bg-blue-700 transition-colors font-medium">
                                    {t('nav.login')}
                                </Link>
                                <Link to="/register" className="text-xs bg-gray-100 hover:bg-gray-200 text-gray-700 py-1.5 px-4 rounded transition-colors font-medium">
                                    {t('nav.register')}
                                </Link>
                            </div>
                        </div>
                    )}
                </div>

                {/* Contests Block */}
                <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-5 bg-white dark:bg-gray-800 shadow-sm">
                    <h3 className="font-bold text-sm mb-3 border-b pb-2 border-gray-100 dark:border-gray-700">{t('home.recentContests')}</h3>
                    {loading ? (
                        <div className="text-xs text-gray-400 py-2">{t('common.loading')}</div>
                    ) : contests.length === 0 ? (
                        <div className="text-xs text-gray-400 py-2">{t('home.noContests')}</div>
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
                    <h3 className="font-bold text-sm mb-3 border-b pb-2 border-gray-100 dark:border-gray-700">{t('home.topRated')}</h3>
                    {loading ? (
                        <div className="text-xs text-gray-400 py-2">{t('common.loading')}</div>
                    ) : rankings.length === 0 ? (
                        <div className="text-xs text-gray-400 py-2">{t('home.noRankings')}</div>
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
                    <h3 className="font-bold text-sm mb-3 border-b pb-2 border-gray-100 dark:border-gray-700">{t('home.quickLinks')}</h3>
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

function AppShell() {
    const location = useLocation()
    const { t } = useTranslation()
    // Full-height workbench routes (no page chrome/footer so flex height can fill the viewport).
    const isFullscreenRoute = location.pathname === '/ide'

    return (
        <div
            className={
                isFullscreenRoute
                    ? 'h-[100dvh] overflow-hidden flex flex-col bg-white dark:bg-gray-800'
                    : 'min-h-screen flex flex-col bg-white dark:bg-gray-800'
            }
        >
            <Navbar />
            {/* `w-full min-w-0` plus the same on the routed child is load-bearing for
                mobile width, not cosmetic. Every page root sits in this column flex
                container and most carry `mx-auto`; auto cross-axis margins cancel the
                `align-items: stretch` default, so the child fell back to fit-content
                sizing and floored at the min-content width of its widest `whitespace-nowrap`
                table (609px on /contests) — which pushed the document to 691px on a
                390px phone. `min-w-0` alone does NOT fix it: fit-content is computed
                from max/min-content independently of `min-width`. Only an explicit
                width does, hence `w-full` on the child. Tables still scroll inside their
                own `overflow-x-auto` wrappers. */}
            <main
                className={
                    isFullscreenRoute
                        ? 'flex-1 min-h-0 flex flex-col max-w-[1400px] mx-auto w-full min-w-0 [&>*]:w-full [&>*]:min-w-0'
                        : 'max-w-[1400px] mx-auto w-full min-w-0 px-6 py-6 flex-1 min-h-0 flex flex-col [&>*]:w-full [&>*]:min-w-0'
                }
            >
                <Suspense fallback={<div className="text-center py-8 text-gray-400 dark:text-gray-500">{t('common.loading')}</div>}>
                <Routes>
                    <Route path="/" element={<Home />} />
                    <Route path="/problems" element={<ProblemList />} />
                    <Route path="/problems/:slug" element={<ProblemDetail />} />
                    <Route path="/login" element={<Login />} />
                    <Route path="/register" element={<Register />} />
                    <Route path="/forgot-password" element={<ForgotPassword />} />
                    <Route path="/reset-password" element={<ResetPassword />} />
                    <Route path="/verify-email" element={<VerifyEmail />} />
                    <Route path="/oauth/complete" element={<OAuthComplete />} />
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
                    <Route path="/settings" element={<Settings />} />
                    <Route path="/settings/api" element={<APISettings />} />
                    <Route path="/settings/notifications" element={<NotificationPreferences />} />
                    <Route path="/auth/2fa/setup" element={<TwoFactorSetup />} />
                    <Route path="/auth/2fa/verify" element={<TwoFactorVerify />} />
                    <Route path="/notifications" element={<Notifications />} />
                    <Route path="/submissions" element={<Submissions />} />
                    <Route path="/submissions/:id" element={<SubmissionDetail />} />
                    <Route path="/admin" element={<AdminDashboard />} />
                    <Route path="/setter" element={<SetterPanel />} />
                    <Route path="/setter/create" element={<ProblemCreate />} />
                    <Route path="/setter/:slug" element={<SetterProblemWorkspace />} />
                    <Route path="/setter/contest/create" element={<ContestCreate />} />
                    <Route path="/generate/problem" element={<GenerateProblem />} />

                    <Route path="/practice" element={<Practice />} />
                    <Route path="/organizations" element={<OrganizationList />} />
                    <Route path="/organizations/create" element={<OrganizationCreate />} />
                    <Route path="/organizations/:id" element={<OrganizationDetail />} />
                    <Route path="/classes/:id" element={<ClassDetail />} />
                    <Route path="/training" element={<TrainingPlanList />} />
                    <Route path="/training/create" element={<TrainingPlanCreate />} />
                    <Route path="/training/:id" element={<TrainingPlanDetail />} />
                    <Route path="/ide" element={<IDE />} />
                    <Route path="/profile" element={<Profile />} />
                    <Route path="/virtual" element={<VirtualContest />} />
                    <Route path="/rating-history" element={<RatingHistory />} />
                    <Route path="/rankings" element={<Rankings />} />
                    <Route path="/user/:username" element={<UserPublicProfile />} />
                    <Route path="/legal/terms" element={<TermsOfService />} />
                    <Route path="/legal/privacy" element={<PrivacyPolicy />} />
                    <Route path="/legal/dmca" element={<DMCA />} />
                    <Route path="*" element={<div className="text-center py-20 text-gray-400 dark:text-gray-500">404 Not Found</div>} />
                </Routes>
                </Suspense>
            </main>
            {!isFullscreenRoute && (
                <footer className="max-w-[1400px] mx-auto w-full px-6 py-6 border-t border-gray-200 dark:border-gray-700 text-sm text-gray-500 dark:text-gray-400 flex flex-wrap gap-4">
                    <Link to="/legal/terms" className="hover:underline">Terms of Service</Link>
                    <Link to="/legal/privacy" className="hover:underline">Privacy Policy</Link>
                    <Link to="/legal/dmca" className="hover:underline">DMCA</Link>
                </footer>
            )}
            <CookieConsent />
        </div>
    )
}

export default function App() {
    return (
        <BrowserRouter>
            <ThemeProvider>
                <ToastProvider>
                    <ConfirmProvider>
                        <AppShell />
                    </ConfirmProvider>
                </ToastProvider>
            </ThemeProvider>
        </BrowserRouter>
    )
}