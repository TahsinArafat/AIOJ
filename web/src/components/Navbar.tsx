import { useState, useRef, useEffect } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import { getAccessToken, signOut } from '../lib/api'
import { useTheme } from '../context/ThemeContext'
import NotificationBell from './NotificationBell'
import LanguageSwitcher from './LanguageSwitcher'
import GlobalSearch from './GlobalSearch'
import type { LucideIcon } from 'lucide-react'
import {
    Code2, Trophy, Dumbbell, BookOpen, Users, UserCheck,
    MessageSquare, BarChart3, Settings, LogOut, User, FileCode,
    Bell, Key, ChevronDown, Menu, X, Building2, GraduationCap, Sun, Moon, Terminal
} from 'lucide-react'

function decodeRole(): string | null {
    const token = localStorage.getItem('access_token')
    if (!token) return null
    try {
        const payload = JSON.parse(atob(token.split('.')[1]))
        return payload.role ?? null
    } catch {
        return null
    }
}

const TRIGGER =
    'flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm text-muted transition-colors ' +
    'hover:bg-surface-2 hover:text-ink'

const PANEL =
    'absolute mt-2 w-52 overflow-hidden rounded-xl border border-hairline bg-surface py-1 shadow-lg'

const ITEM =
    'flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm text-ink-soft transition-colors hover:bg-surface-2 hover:text-ink'

const ICON_BUTTON =
    'flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors hover:bg-surface-2 hover:text-ink'

function NavDropdown({ label, icon: Icon, align = 'left', ariaLabel, children }: { label: string; icon: LucideIcon; align?: 'left' | 'right'; ariaLabel?: string; children: React.ReactNode }) {
    const [open, setOpen] = useState(false)
    const ref = useRef<HTMLDivElement>(null)

    useEffect(() => {
        const handler = (e: MouseEvent) => {
            if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
        }
        document.addEventListener('mousedown', handler)
        return () => document.removeEventListener('mousedown', handler)
    }, [])

    return (
        <div ref={ref} className="relative">
            <button
                type="button"
                onClick={() => setOpen(!open)}
                aria-label={ariaLabel || label || undefined}
                aria-expanded={open}
                className={TRIGGER}
            >
                <Icon className="h-4 w-4" />
                {label && <span>{label}</span>}
                <ChevronDown className={`h-3.5 w-3.5 transition-transform ${open ? 'rotate-180' : ''}`} />
            </button>
            {open && (
                <div className={`${PANEL} ${align === 'right' ? 'right-0' : 'left-0'} z-50`}>
                    {children}
                </div>
            )}
        </div>
    )
}

function NavLink({ to, icon: Icon, children, onClick }: { to?: string; icon?: LucideIcon; children: React.ReactNode; onClick?: () => void }) {
    const inner = (
        <span className="flex items-center gap-2.5">
            {Icon && <Icon className="h-4 w-4 shrink-0 text-muted" />}
            {children}
        </span>
    )
    return to
        ? <Link to={to} onClick={onClick} className={`${ITEM} block`}>{inner}</Link>
        : <button type="button" onClick={onClick} className={ITEM}>{inner}</button>
}

export default function Navbar() {
    const navigate = useNavigate()
    const location = useLocation()
    const loggedIn = !!getAccessToken()
    const role = decodeRole()
    const isAdmin = role === 'admin'
    const isContestant = role === 'contestant'
    const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
    const { theme, toggleTheme } = useTheme()

    const handleLogout = () => {
        // Revokes the refresh token server-side; local state clears immediately.
        void signOut()
        setMobileMenuOpen(false)
        navigate('/login')
    }

    const isActive = (to: string) => location.pathname === to || location.pathname.startsWith(to + '/')

    return (
        <nav className="sticky top-0 z-50 border-b border-hairline bg-surface/85 backdrop-blur-md">
            {/* The full desktop nav (logo + Compete + Community + Problems + IDE +
                Admin + search + language + theme + account) needs ~964px. It used
                to switch on at `md` (768px), so on 768–964px tablets the right
                cluster was clipped off-screen and unreachable. `lg` (1024px) is
                the first width that actually fits it. */}
            <div className="mx-auto flex h-14 w-full max-w-[1400px] items-center justify-between gap-3 px-4 sm:px-6">
                {/* Left: Logo + Main nav */}
                <div className="flex min-w-0 items-center gap-5">
                    <Link
                        to="/"
                        className="flex h-10 shrink-0 items-center gap-2 rounded-lg text-sm font-semibold tracking-tight text-ink"
                    >
                        <span className="flex h-7 w-7 items-center justify-center rounded-md bg-brand text-on-brand">
                            <Code2 className="h-4 w-4" />
                        </span>
                        AIOJ
                    </Link>
                    <div className="hidden items-center gap-0.5 lg:flex">
                        {!isContestant && (
                            <>
                                <NavDropdown label="Compete" icon={Trophy}>
                                    <NavLink to="/contests" icon={Trophy}>Contests</NavLink>
                                    <NavLink to="/gym" icon={Dumbbell}>Gym</NavLink>
                                    <NavLink to="/practice" icon={BookOpen}>Practice</NavLink>
                                </NavDropdown>
                                <NavDropdown label="Community" icon={Users}>
                                    <NavLink to="/organizations" icon={Building2}>Organizations</NavLink>
                                    <NavLink to="/groups" icon={Users}>Groups</NavLink>
                                    <NavLink to="/teams" icon={UserCheck}>Teams</NavLink>
                                    <NavLink to="/blog" icon={MessageSquare}>Blog</NavLink>
                                    <NavLink to="/training" icon={GraduationCap}>Training Plans</NavLink>
                                    <NavLink to="/rankings" icon={BarChart3}>Rankings</NavLink>
                                </NavDropdown>
                                <Link
                                    to="/problems"
                                    aria-current={isActive('/problems') ? 'page' : undefined}
                                    className={`${TRIGGER} ${isActive('/problems') ? 'bg-surface-2 text-ink' : ''}`}
                                >
                                    <Code2 className="h-4 w-4" />
                                    <span>Problems</span>
                                </Link>
                                <Link
                                    to="/ide"
                                    aria-current={isActive('/ide') ? 'page' : undefined}
                                    className={`${TRIGGER} ${isActive('/ide') ? 'bg-surface-2 text-ink' : ''}`}
                                >
                                    <Terminal className="h-4 w-4" />
                                    <span>IDE</span>
                                </Link>
                            </>
                        )}
                        {isAdmin && (
                            <Link
                                to="/admin"
                                className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-violet-600 transition-colors hover:bg-violet-500/10 dark:text-violet-400"
                            >
                                <Settings className="h-4 w-4" />
                                <span>Admin</span>
                            </Link>
                        )}
                    </div>
                </div>

                {/* Right: Search + User */}
                <div className="hidden shrink-0 items-center gap-1.5 lg:flex">
                    <GlobalSearch />
                    <LanguageSwitcher />
                    <button
                        type="button"
                        onClick={toggleTheme}
                        className={ICON_BUTTON}
                        aria-label="Toggle theme"
                    >
                        {theme === 'dark' ? <Sun className="h-4.5 w-4.5" /> : <Moon className="h-4.5 w-4.5" />}
                    </button>
                    {loggedIn ? (
                        <NavDropdown label="" icon={User} align="right" ariaLabel="Account menu">
                            <NavLink to="/profile" icon={User}>Profile</NavLink>
                            <NavLink to="/notifications" icon={Bell}>Notifications</NavLink>
                            <NavLink to="/submissions" icon={FileCode}>My Submissions</NavLink>
                            <NavLink to="/setter" icon={FileCode}>Setter Workspace</NavLink>
                            {!isContestant && (
                                <>
                                    <NavLink to="/settings/api" icon={Key}>API Keys</NavLink>
                                    <NavLink to="/settings/notifications" icon={Bell}>Notification Settings</NavLink>
                                </>
                            )}
                            <hr className="my-1 border-hairline" />
                            <NavLink icon={LogOut} onClick={handleLogout}>Logout</NavLink>
                        </NavDropdown>
                    ) : (
                        <div className="flex items-center gap-1.5 pl-1">
                            <Link
                                to="/login"
                                className="rounded-lg px-2.5 py-1.5 text-sm text-muted transition-colors hover:bg-surface-2 hover:text-ink"
                            >
                                Login
                            </Link>
                            <Link
                                to="/register"
                                className="rounded-lg bg-brand px-3 py-1.5 text-sm font-medium text-on-brand transition-colors hover:bg-brand-hover"
                            >
                                Register
                            </Link>
                        </div>
                    )}
                </div>

                {/* Mobile / tablet controls */}
                <div className="flex items-center gap-1 lg:hidden">
                    <GlobalSearch />
                    <LanguageSwitcher />
                    <button
                        type="button"
                        onClick={toggleTheme}
                        className={`${ICON_BUTTON} h-11 w-11`}
                        aria-label="Toggle theme"
                    >
                        {theme === 'dark' ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
                    </button>
                    {loggedIn && <NotificationBell />}
                    <button
                        type="button"
                        onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                        className="flex h-11 w-11 items-center justify-center rounded-lg border border-hairline-strong text-muted transition-colors hover:bg-surface-2 hover:text-ink"
                        aria-label="Toggle menu"
                        aria-expanded={mobileMenuOpen}
                        aria-controls="mobile-nav"
                    >
                        {mobileMenuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
                    </button>
                </div>
            </div>

            {/* Mobile Drawer */}
            {mobileMenuOpen && (
                <div
                    id="mobile-nav"
                    // Any link tap closes the drawer, so navigating never leaves
                    // the overlay covering the page it just opened.
                    onClick={e => {
                        if ((e.target as HTMLElement).closest('a')) setMobileMenuOpen(false)
                    }}
                    className="max-h-[calc(100dvh-3.5rem)] overflow-y-auto border-t border-hairline bg-surface px-4 pb-6 pt-3 lg:hidden"
                >
                    {!isContestant && (
                        <>
                            <p className="px-2 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-soft">Compete</p>
                            <MobileLink to="/contests" icon={Trophy}>Contests</MobileLink>
                            <MobileLink to="/gym" icon={Dumbbell}>Gym</MobileLink>
                            <MobileLink to="/practice" icon={BookOpen}>Practice</MobileLink>
                            <MobileLink to="/problems" icon={Code2}>Problems</MobileLink>
                            <MobileLink to="/ide" icon={Terminal}>IDE</MobileLink>

                            <p className="px-2 pb-1 pt-4 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-soft">Community</p>
                            <MobileLink to="/organizations" icon={Building2}>Organizations</MobileLink>
                            <MobileLink to="/groups" icon={Users}>Groups</MobileLink>
                            <MobileLink to="/teams" icon={UserCheck}>Teams</MobileLink>
                            <MobileLink to="/blog" icon={MessageSquare}>Blog</MobileLink>
                            <MobileLink to="/training" icon={GraduationCap}>Training Plans</MobileLink>
                            <MobileLink to="/rankings" icon={BarChart3}>Rankings</MobileLink>
                        </>
                    )}

                    {isAdmin && (
                        <>
                            <p className="px-2 pb-1 pt-4 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-soft">Admin</p>
                            <MobileLink to="/admin" icon={Settings}>Admin Dashboard</MobileLink>
                        </>
                    )}
                    {loggedIn && !isContestant && (
                        <MobileLink to="/setter" icon={FileCode}>Setter Workspace</MobileLink>
                    )}

                    <hr className="my-3 border-hairline" />
                    {loggedIn ? (
                        <>
                            <MobileLink to="/profile" icon={User}>Profile</MobileLink>
                            <MobileLink to="/notifications" icon={Bell}>Notifications</MobileLink>
                            <MobileLink to="/submissions" icon={FileCode}>My Submissions</MobileLink>
                            {!isContestant && <MobileLink to="/settings/api" icon={Key}>API Keys</MobileLink>}
                            <button
                                type="button"
                                onClick={handleLogout}
                                className="mt-1 flex w-full items-center gap-2.5 rounded-lg px-3 py-3 text-left text-sm font-medium text-bad transition-colors hover:bg-bad/10"
                            >
                                <LogOut className="h-4 w-4" /> Logout
                            </button>
                        </>
                    ) : (
                        <div className="flex flex-col gap-2 pt-1">
                            <Link
                                to="/login"
                                className="flex items-center justify-center rounded-lg border border-hairline-strong py-3 text-sm font-medium text-ink transition-colors hover:bg-surface-2"
                            >
                                Login
                            </Link>
                            <Link
                                to="/register"
                                className="flex items-center justify-center rounded-lg bg-brand py-3 text-sm font-medium text-on-brand transition-colors hover:bg-brand-hover"
                            >
                                Register
                            </Link>
                        </div>
                    )}
                </div>
            )}
        </nav>
    )
}

function MobileLink({ to, icon: Icon, children }: { to: string; icon: LucideIcon; children: React.ReactNode }) {
    return (
        <Link
            to={to}
            className="flex items-center gap-2.5 rounded-lg px-3 py-3 text-sm font-medium text-ink-soft transition-colors hover:bg-surface-2 hover:text-ink"
        >
            <Icon className="h-4 w-4 shrink-0 text-muted" />
            {children}
        </Link>
    )
}
