import { useState, useEffect, Suspense, lazy } from 'react'
import { BrowserRouter, Routes, Route, Link, useLocation } from 'react-router-dom'
import { api, contestSlug } from './lib/api'
import { ThemeProvider } from './context/ThemeContext'
import { FeaturesProvider } from './context/FeaturesContext'
import { FeatureGate } from './components/FeatureGate'
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
import { DocumentTitle } from './components/DocumentTitle'
import ActivityFeed from './components/ActivityFeed'
import { buttonClass } from './components/ui/buttonStyles'
import { Badge } from './components/ui/Badge'
import { Card, CardBody, CardHeader } from './components/ui/Card'
import { Skeleton } from './components/ui/PageHeader'
const Practice = lazy(() => import('./pages/Practice'))
const VirtualContest = lazy(() => import('./pages/VirtualContest'))
const RatingHistory = lazy(() => import('./pages/RatingHistory'))
const Submissions = lazy(() => import('./pages/Submissions'))
const AllSubmissions = lazy(() => import('./pages/AllSubmissions'))
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
const ContestManage = lazy(() => import('./pages/ContestManage'))
const IDE = lazy(() => import('./pages/IDE'))
const AIProblemCreate = lazy(() => import('./pages/AIProblemCreate'))
const AIJobsPage = lazy(() => import('./pages/AIJobsPage'))
const AIJobDetailPage = lazy(() => import('./pages/AIJobDetailPage'))
const TermsOfService = lazy(() => import('./pages/legal/TermsOfService'))
const PrivacyPolicy = lazy(() => import('./pages/legal/PrivacyPolicy'))
const DMCA = lazy(() => import('./pages/legal/DMCA'))
import CookieConsent from './components/CookieConsent'
const TwoFactorSetup = lazy(() => import('./pages/auth/TwoFactorSetup'))
const TwoFactorVerify = lazy(() => import('./pages/auth/TwoFactorVerify'))
const Settings = lazy(() => import('./pages/Settings'))
import './global.css'

/**
 * Decode the display name from the access token without throwing.
 *
 * The old inline `JSON.parse(atob(...))` had no guard: a corrupted or
 * half-written token in localStorage threw during render and blanked the
 * whole home page instead of just showing the signed-out state.
 */
function decodeUsername(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return typeof payload?.uname === 'string' ? payload.uname : null
    } catch {
        return null
    }
}

function RouteFallback() {
    const { t } = useTranslation()
    return (
        <div role="status" aria-live="polite" className="mx-auto w-full max-w-5xl space-y-4 py-6">
            <Skeleton className="h-7 w-48" />
            <Skeleton className="h-4 w-72" />
            <Skeleton className="h-72 w-full" />
            <span className="sr-only">{t('common.loading')}</span>
        </div>
    )
}

function NotFound() {
    return (
        <div className="flex flex-col items-center justify-center py-24 text-center">
            <p aria-hidden="true" className="tnum text-6xl font-semibold tracking-tight text-surface-3">404</p>
            <h1 className="mt-4 text-xl font-semibold tracking-tight text-ink">This page does not exist</h1>
            <p className="mt-1 max-w-sm text-sm text-muted">
                The link may be stale, or the problem or contest it pointed at was removed.
            </p>
            <div className="mt-6 flex flex-wrap justify-center gap-2">
                <Link to="/" className={buttonClass({ variant: 'secondary' })}>Back to home</Link>
                <Link to="/problems" className={buttonClass()}>Browse problems</Link>
            </div>
        </div>
    )
}

function Home() {
    const { t } = useTranslation()
    const [contests, setContests] = useState<ContestLite[]>([])
    const [posts, setPosts] = useState<FeedPost[]>([])
    const [stats, setStats] = useState({ problems: 0, users: 0, submissions: 0 })
    const [rankings, setRankings] = useState<RankingRow[]>([])
    const [loading, setLoading] = useState(true)
    const username = decodeUsername()

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

    const contestStatus = (c: ContestLite) => {
        const now = Date.now() // eslint-disable-line react-hooks/purity -- status intentionally compared against wall clock at render
        const start = new Date(c.start_time).getTime()
        const end = new Date(c.end_time).getTime()
        if (now < start) return { text: 'Upcoming', tone: 'brand' as const }
        if (now < end) return { text: 'Running', tone: 'ok' as const }
        return { text: 'Ended', tone: 'neutral' as const }
    }

    return (
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
            {/* Main Content (Left, spans 3 columns) */}
            <div className="lg:col-span-3 space-y-6">
                {/* Hero */}
                <section className="relative overflow-hidden rounded-2xl border border-hairline bg-surface p-6 shadow-[var(--shadow-card)] sm:p-8">
                    <div
                        aria-hidden="true"
                        className="pointer-events-none absolute -right-16 -top-24 h-64 w-64 rounded-full bg-brand/10 blur-3xl"
                    />
                    <div className="relative grid gap-8 lg:grid-cols-[1.4fr_1fr] lg:items-center">
                        <div>
                            <p className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted">
                                <span className="h-1.5 w-1.5 rounded-full bg-brand" aria-hidden="true" />
                                {t('home.title')}
                            </p>
                            <h1 className="mt-2 text-3xl font-semibold leading-tight tracking-tight text-ink sm:text-4xl">
                                {t('home.welcome')}
                            </h1>
                            <p className="mt-3 max-w-xl text-sm leading-relaxed text-muted">{t('home.subtitle')}</p>
                            <div className="mt-6 flex flex-wrap gap-2">
                                <Link to="/problems" className={buttonClass({ size: 'lg' })}>{t('home.browseProblems')}</Link>
                                <Link to="/contests" className={buttonClass({ variant: 'secondary', size: 'lg' })}>{t('home.viewContests')}</Link>
                            </div>
                        </div>
                        <dl className="grid grid-cols-3 divide-x divide-hairline rounded-xl border border-hairline bg-surface-2">
                            {[
                                { label: t('home.problems'), value: stats.problems },
                                { label: t('home.users'), value: stats.users },
                                { label: t('home.submissions'), value: stats.submissions },
                            ].map(s => (
                                <div key={s.label} className="px-2 py-4 text-center">
                                    <dd className="tnum text-xl font-semibold tracking-tight text-ink sm:text-2xl">{s.value.toLocaleString()}</dd>
                                    <dt className="mt-1 text-[10px] font-medium uppercase tracking-wide text-muted sm:text-[11px]">{s.label}</dt>
                                </div>
                            ))}
                        </dl>
                    </div>
                </section>

                <ActivityFeed limit={15} />

                {/* Recent Blog Posts */}
                <Card>
                    <CardHeader
                        title={t('home.latestPosts')}
                        action={<Link to="/blog" className="text-xs font-medium text-brand hover:underline">{t('home.viewAll')}</Link>}
                    />
                    {loading ? (
                        <div className="space-y-3 px-5 py-4">
                            <Skeleton className="h-4 w-2/3" />
                            <Skeleton className="h-3 w-1/3" />
                        </div>
                    ) : posts.length === 0 ? (
                        <p className="px-5 py-8 text-center text-sm text-muted">{t('home.noPostsYet')}</p>
                    ) : (
                        <ul className="divide-y divide-hairline">
                            {posts.map(p => (
                                <li key={p.id}>
                                    <Link to={`/blog/${p.id}`} className="group block px-5 py-4 transition-colors hover:bg-surface-2">
                                        <h3 className="text-sm font-semibold text-ink transition-colors group-hover:text-brand">{p.title}</h3>
                                        <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted">
                                            <span>{p.username}</span>
                                            <span aria-hidden="true">·</span>
                                            <span className="tnum">{p.upvotes} upvotes</span>
                                            <span aria-hidden="true">·</span>
                                            <time dateTime={p.created_at}>{new Date(p.created_at).toLocaleDateString()}</time>
                                        </div>
                                    </Link>
                                </li>
                            ))}
                        </ul>
                    )}
                </Card>
            </div>

            {/* Sidebar (Right, spans 1 column) */}
            <div className="space-y-6">
                {/* User Stats / Login box */}
                <Card>
                    <CardBody>
                        {username ? (
                            <div className="flex items-center gap-3">
                                <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-brand-soft text-base font-semibold uppercase text-brand">
                                    {username[0]}
                                </div>
                                <div className="min-w-0 flex-1">
                                    <p className="truncate text-sm font-semibold text-ink">{username}</p>
                                    <p className="text-xs text-muted">{t('common.loggedIn')}</p>
                                </div>
                                <Link to="/profile" className={buttonClass({ variant: 'secondary', size: 'sm' })}>
                                    {t('common.viewProfile')}
                                </Link>
                            </div>
                        ) : (
                            <div className="space-y-3">
                                <div>
                                    <h3 className="text-sm font-semibold text-ink">{t('home.joinCommunity')}</h3>
                                    <p className="mt-1 text-xs leading-relaxed text-muted">{t('home.signInPrompt')}</p>
                                </div>
                                <div className="flex gap-2">
                                    <Link to="/login" className={buttonClass({ size: 'sm', className: 'flex-1' })}>{t('nav.login')}</Link>
                                    <Link to="/register" className={buttonClass({ variant: 'secondary', size: 'sm', className: 'flex-1' })}>{t('nav.register')}</Link>
                                </div>
                            </div>
                        )}
                    </CardBody>
                </Card>

                {/* Contests Block */}
                <Card>
                    <CardHeader
                        title={t('home.recentContests')}
                        action={<Link to="/contests" className="text-xs font-medium text-brand hover:underline">{t('home.viewAll')}</Link>}
                    />
                    {loading ? (
                        <div className="space-y-2 px-5 py-4">
                            <Skeleton className="h-3 w-3/4" />
                            <Skeleton className="h-3 w-1/2" />
                        </div>
                    ) : contests.length === 0 ? (
                        <p className="px-5 py-6 text-center text-xs text-muted">{t('home.noContests')}</p>
                    ) : (
                        <ul className="divide-y divide-hairline">
                            {contests.map(c => {
                                const status = contestStatus(c)
                                return (
                                    <li key={c.id}>
                                        <Link to={`/contests/${contestSlug(c)}`} className="group block px-5 py-3 transition-colors hover:bg-surface-2">
                                            <p className="line-clamp-2 text-xs font-medium leading-snug text-ink transition-colors group-hover:text-brand">
                                                {c.title}
                                            </p>
                                            <div className="mt-1.5 flex items-center justify-between gap-2">
                                                <span className="text-[11px] text-muted">
                                                    {new Date(c.start_time).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
                                                </span>
                                                <Badge tone={status.tone} dot={status.tone === 'ok'}>{status.text}</Badge>
                                            </div>
                                        </Link>
                                    </li>
                                )
                            })}
                        </ul>
                    )}
                </Card>

                {/* Top Rated Users rankings widget */}
                <Card>
                    <CardHeader
                        title={t('home.topRated')}
                        action={<Link to="/rankings" className="text-xs font-medium text-brand hover:underline">{t('home.viewAll')}</Link>}
                    />
                    {loading ? (
                        <div className="space-y-2 px-5 py-4">
                            <Skeleton className="h-3 w-2/3" />
                            <Skeleton className="h-3 w-1/2" />
                        </div>
                    ) : rankings.length === 0 ? (
                        <p className="px-5 py-6 text-center text-xs text-muted">{t('home.noRankings')}</p>
                    ) : (
                        <ol className="divide-y divide-hairline">
                            {rankings.map((user, i) => (
                                <li key={user.id} className="flex items-center justify-between gap-3 px-5 py-2">
                                    <div className="flex min-w-0 items-center gap-2.5">
                                        <span className="tnum w-4 shrink-0 text-right text-[11px] text-muted-soft">{i + 1}</span>
                                        <Link to={`/user/${user.username}`} className="truncate text-xs font-medium text-ink hover:text-brand">
                                            {user.username}
                                        </Link>
                                    </div>
                                    <span className="tnum shrink-0 text-xs font-semibold text-ink">{user.rating}</span>
                                </li>
                            ))}
                        </ol>
                    )}
                </Card>

                {/* Quick Links */}
                <Card>
                    <CardHeader title={t('home.quickLinks')} />
                    <div className="grid grid-cols-2 gap-2 p-4">
                        <Link to="/problems" className={buttonClass({ variant: 'secondary', size: 'sm' })}>Problems</Link>
                        <Link to="/practice" className={buttonClass({ variant: 'secondary', size: 'sm' })}>Practice</Link>
                        <Link to="/blog" className={buttonClass({ variant: 'secondary', size: 'sm' })}>Blogs</Link>
                        <Link to="/rankings" className={buttonClass({ variant: 'secondary', size: 'sm' })}>Rankings</Link>
                    </div>
                </Card>
            </div>
        </div>
    )
}

function AppShell() {
    const location = useLocation()
    // Full-height workbench routes (no page chrome/footer so flex height can fill the viewport).
    const isFullscreenRoute = location.pathname === '/ide'

    return (
        <div
            className={
                isFullscreenRoute
                    ? 'h-[100dvh] overflow-hidden flex flex-col bg-canvas'
                    : 'min-h-screen flex flex-col bg-canvas'
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
                        : 'max-w-[1400px] mx-auto w-full min-w-0 px-4 sm:px-6 py-6 sm:py-8 flex-1 min-h-0 flex flex-col [&>*]:w-full [&>*]:min-w-0'
                }
            >
                <Suspense fallback={<RouteFallback />}>
                {/* Rendered before <Routes> on purpose: sibling effects run in
                    source order, so the route-level title is applied first and a
                    page's own useDocumentTitle can refine it (e.g. the problem
                    name) in the same commit. */}
                <DocumentTitle />
                <Routes>
                    <Route path="/" element={<Home />} />
                    <Route path="/problems" element={<FeatureGate feature="problems"><ProblemList /></FeatureGate>} />
                    <Route path="/problems/:slug" element={<FeatureGate feature="problems"><ProblemDetail /></FeatureGate>} />
                    <Route path="/login" element={<Login />} />
                    <Route path="/register" element={<Register />} />
                    <Route path="/forgot-password" element={<ForgotPassword />} />
                    <Route path="/reset-password" element={<ResetPassword />} />
                    <Route path="/verify-email" element={<VerifyEmail />} />
                    <Route path="/oauth/complete" element={<OAuthComplete />} />
                    <Route path="/contests" element={<FeatureGate feature="contests"><ContestList /></FeatureGate>} />
                    <Route path="/contests/:id" element={<FeatureGate feature="contests"><ContestDetail /></FeatureGate>} />
                    <Route path="/contests/:id/scoreboard" element={<FeatureGate feature="contests"><ContestScoreboard /></FeatureGate>} />
                    <Route path="/contests/:id/plagiarism" element={<FeatureGate feature="plagiarism"><ContestPlagiarism /></FeatureGate>} />
                    <Route path="/contests/:contestId/problem/:index" element={<FeatureGate feature="contests"><ContestProblem /></FeatureGate>} />
                    <Route path="/setter/contest/:id/manage" element={<ContestManage />} />
                    <Route path="/gym" element={<FeatureGate feature="gym"><GymList /></FeatureGate>} />
                    <Route path="/gym/:id" element={<FeatureGate feature="gym"><GymDetail /></FeatureGate>} />
                    <Route path="/hack/:contestId/:problemId" element={<FeatureGate feature="hacks"><HackPanel /></FeatureGate>} />
                    <Route path="/groups" element={<FeatureGate feature="groups"><GroupList /></FeatureGate>} />
                    <Route path="/groups/create" element={<FeatureGate feature="groups"><GroupCreate /></FeatureGate>} />
                    <Route path="/groups/join" element={<FeatureGate feature="groups"><GroupJoin /></FeatureGate>} />
                    <Route path="/groups/:id" element={<FeatureGate feature="groups"><GroupDetail /></FeatureGate>} />
                    <Route path="/teams" element={<FeatureGate feature="teams"><TeamList /></FeatureGate>} />
                    <Route path="/teams/create" element={<FeatureGate feature="teams"><TeamCreate /></FeatureGate>} />
                    <Route path="/teams/:id" element={<FeatureGate feature="teams"><TeamDetail /></FeatureGate>} />
                    <Route path="/blog" element={<FeatureGate feature="blog"><BlogList /></FeatureGate>} />
                    <Route path="/blog/create" element={<FeatureGate feature="blog"><BlogCreate /></FeatureGate>} />
                    <Route path="/blog/:id" element={<FeatureGate feature="blog"><BlogDetail /></FeatureGate>} />
                    <Route path="/editorials" element={<FeatureGate feature="editorials"><EditorialList /></FeatureGate>} />
                    <Route path="/editorials/:id" element={<FeatureGate feature="editorials"><EditorialDetail /></FeatureGate>} />
                    <Route path="/settings" element={<Settings />} />
                    <Route path="/settings/api" element={<FeatureGate feature="api_keys"><APISettings /></FeatureGate>} />
                    <Route path="/settings/notifications" element={<FeatureGate feature="notifications"><NotificationPreferences /></FeatureGate>} />
                    <Route path="/auth/2fa/setup" element={<TwoFactorSetup />} />
                    <Route path="/auth/2fa/verify" element={<TwoFactorVerify />} />
                    <Route path="/notifications" element={<FeatureGate feature="notifications"><Notifications /></FeatureGate>} />
                    <Route path="/submissions" element={<FeatureGate feature="submissions"><Submissions /></FeatureGate>} />
                    <Route path="/status" element={<FeatureGate feature="submissions"><AllSubmissions /></FeatureGate>} />
                    <Route path="/submissions/:id" element={<FeatureGate feature="submissions"><SubmissionDetail /></FeatureGate>} />
                    <Route path="/admin" element={<AdminDashboard />} />
                    <Route path="/setter" element={<FeatureGate feature="setter_workspace"><SetterPanel /></FeatureGate>} />
                    <Route path="/setter/create" element={<FeatureGate feature="setter_workspace"><ProblemCreate /></FeatureGate>} />
                    {/* Declared before /setter/:slug so the static segment wins on
                        its own merits rather than relying on route ranking. */}
                    <Route path="/setter/ai" element={<FeatureGate feature="setter_workspace"><FeatureGate feature="ai_generation"><AIProblemCreate /></FeatureGate></FeatureGate>} />
                    <Route path="/setter/ai/jobs" element={<FeatureGate feature="setter_workspace"><FeatureGate feature="ai_generation"><AIJobsPage /></FeatureGate></FeatureGate>} />
                    <Route path="/setter/ai/jobs/:id" element={<FeatureGate feature="setter_workspace"><FeatureGate feature="ai_generation"><AIJobDetailPage /></FeatureGate></FeatureGate>} />
                    <Route path="/setter/contest/create" element={<FeatureGate feature="setter_workspace"><ContestCreate /></FeatureGate>} />
                    <Route path="/setter/:slug" element={<FeatureGate feature="setter_workspace"><SetterProblemWorkspace /></FeatureGate>} />

                    <Route path="/practice" element={<FeatureGate feature="practice"><Practice /></FeatureGate>} />
                    <Route path="/organizations" element={<FeatureGate feature="organizations"><OrganizationList /></FeatureGate>} />
                    <Route path="/organizations/create" element={<FeatureGate feature="organizations"><OrganizationCreate /></FeatureGate>} />
                    <Route path="/organizations/:id" element={<FeatureGate feature="organizations"><OrganizationDetail /></FeatureGate>} />
                    <Route path="/classes/:id" element={<FeatureGate feature="classes"><ClassDetail /></FeatureGate>} />
                    <Route path="/training" element={<FeatureGate feature="training"><TrainingPlanList /></FeatureGate>} />
                    <Route path="/training/create" element={<FeatureGate feature="training"><TrainingPlanCreate /></FeatureGate>} />
                    <Route path="/training/:id" element={<FeatureGate feature="training"><TrainingPlanDetail /></FeatureGate>} />
                    <Route path="/ide" element={<FeatureGate feature="ide"><IDE /></FeatureGate>} />
                    <Route path="/profile" element={<Profile />} />
                    <Route path="/virtual" element={<FeatureGate feature="virtual_contests"><VirtualContest /></FeatureGate>} />
                    <Route path="/rating-history" element={<RatingHistory />} />
                    <Route path="/rankings" element={<FeatureGate feature="rankings"><Rankings /></FeatureGate>} />
                    <Route path="/user/:username" element={<UserPublicProfile />} />
                    <Route path="/legal/terms" element={<TermsOfService />} />
                    <Route path="/legal/privacy" element={<PrivacyPolicy />} />
                    <Route path="/legal/dmca" element={<DMCA />} />
                    <Route path="*" element={<NotFound />} />
                </Routes>
                </Suspense>
            </main>
            {!isFullscreenRoute && (
                <footer className="border-t border-hairline bg-surface">
                    <div className="mx-auto flex w-full max-w-[1400px] flex-col gap-3 px-4 py-5 text-xs text-muted sm:flex-row sm:items-center sm:justify-between sm:px-6">
                        <div className="flex flex-wrap items-center gap-2">
                            <span className="font-semibold tracking-tight text-ink">AIOJ</span>
                            <span aria-hidden="true" className="text-hairline-strong">/</span>
                            <span>Self-hostable competitive programming</span>
                        </div>
                        <nav aria-label="Legal" className="flex flex-wrap items-center gap-4">
                            <Link to="/legal/terms" className="transition-colors hover:text-ink">Terms of Service</Link>
                            <Link to="/legal/privacy" className="transition-colors hover:text-ink">Privacy Policy</Link>
                            <Link to="/legal/dmca" className="transition-colors hover:text-ink">DMCA</Link>
                        </nav>
                    </div>
                </footer>
            )}
            <CookieConsent />
        </div>
    )
}

interface ContestLite { id: string; title: string; start_time: string; end_time: string }
interface FeedPost { id: string; title: string; username?: string; created_at: string; upvotes?: number }
interface RankingRow { id: string; username: string; rating?: number }

export default function App() {
    return (
        <BrowserRouter>
            <ThemeProvider>
                <FeaturesProvider>
                    <ToastProvider>
                        <ConfirmProvider>
                            <AppShell />
                        </ConfirmProvider>
                    </ToastProvider>
                </FeaturesProvider>
            </ThemeProvider>
        </BrowserRouter>
    )
}