import { useState, useRef, useEffect } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import { getAccessToken, clearTokens } from '../lib/api'
import NotificationBell from './NotificationBell'
import LanguageSwitcher from './LanguageSwitcher'
import GlobalSearch from './GlobalSearch'
import {
    Code2, Trophy, Dumbbell, BookOpen, Users, UserCheck,
    MessageSquare, BarChart3, Settings, LogOut, User, FileCode,
    Bell, Key, ChevronDown, Menu, X, Building2, GraduationCap
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

function NavDropdown({ label, icon: Icon, children }: { label: string; icon: any; children: React.ReactNode }) {
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
                onClick={() => setOpen(!open)}
                className="flex items-center gap-1 text-sm text-gray-600 hover:text-black transition-colors cursor-pointer"
            >
                <Icon className="w-4 h-4" />
                <span>{label}</span>
                <ChevronDown className={`w-3 h-3 transition-transform ${open ? 'rotate-180' : ''}`} />
            </button>
            {open && (
                <div className="absolute left-0 mt-2 w-48 bg-white border border-gray-200 rounded-lg shadow-lg py-1 z-50">
                    {children}
                </div>
            )}
        </div>
    )
}

function NavLink({ to, icon: Icon, children, onClick }: { to?: string; icon?: any; children: React.ReactNode; onClick?: () => void }) {
    const inner = (
        <span className="flex items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50 rounded-lg transition-colors">
            {Icon && <Icon className="w-4 h-4" />}
            {children}
        </span>
    )
    return to ? <Link to={to} onClick={onClick} className="block">{inner}</Link> : <button onClick={onClick} className="w-full text-left block">{inner}</button>
}

export default function Navbar() {
    const navigate = useNavigate()
    useLocation()
    const loggedIn = !!getAccessToken()
    const role = decodeRole()
    const isAdmin = role === 'admin'
    const isSetter = role === 'teacher' || isAdmin
    const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

    const handleLogout = () => {
        clearTokens()
        setMobileMenuOpen(false)
        navigate('/login')
    }

    return (
        <nav className="border-b border-gray-200 bg-white sticky top-0 z-50">
            <div className="px-6 py-3 flex items-center justify-between">
                {/* Left: Logo + Main nav */}
                <div className="flex gap-6 items-center">
                    <Link to="/" onClick={() => setMobileMenuOpen(false)} className="font-bold text-blue-600 text-lg">AIOJ</Link>
                    <div className="hidden md:flex gap-4 items-center">
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
                        <Link to="/problems" className="flex items-center gap-1.5 text-sm text-gray-600 hover:text-black transition-colors">
                            <Code2 className="w-4 h-4" />
                            <span>Problems</span>
                        </Link>
                        {isAdmin && (
                            <Link to="/admin" className="flex items-center gap-1.5 text-sm font-medium text-purple-600 hover:text-purple-800 transition-colors">
                                <Settings className="w-4 h-4" />
                                <span>Admin</span>
                            </Link>
                        )}
                        {isSetter && (
                            <Link to="/setter" className="flex items-center gap-1.5 text-sm font-medium text-orange-600 hover:text-orange-800 transition-colors">
                                <FileCode className="w-4 h-4" />
                                <span>Setter</span>
                            </Link>
                        )}
                    </div>
                </div>

                {/* Right: Search + User */}
                <div className="hidden md:flex gap-3 items-center">
                    <GlobalSearch />
                    <LanguageSwitcher />
                    {loggedIn ? (
                        <>
                            <NotificationBell />
                            <NavDropdown label="" icon={User}>
                                <NavLink to="/profile" icon={User}>Profile</NavLink>
                                <NavLink to="/notifications" icon={Bell}>Notifications</NavLink>
                                <NavLink to="/submissions" icon={FileCode}>My Submissions</NavLink>
                                <NavLink to="/settings/api" icon={Key}>API Keys</NavLink>
                                <NavLink to="/settings/notifications" icon={Bell}>Notification Settings</NavLink>
                                <hr className="border-gray-100 my-1" />
                                <NavLink icon={LogOut} onClick={handleLogout}>Logout</NavLink>
                            </NavDropdown>
                        </>
                    ) : (
                        <>
                            <Link to="/login" className="text-sm text-gray-600 hover:text-black">Login</Link>
                            <Link to="/register" className="text-sm bg-blue-600 text-white px-3 py-1.5 rounded hover:bg-blue-700 transition-colors">Register</Link>
                        </>
                    )}
                </div>

                {/* Mobile Menu Button */}
                <div className="flex md:hidden items-center gap-3">
                    <GlobalSearch />
                    <LanguageSwitcher />
                    {loggedIn && <NotificationBell />}
                    <button
                        onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                        className="text-gray-600 hover:text-black focus:outline-none p-1.5 rounded border border-gray-200"
                        aria-label="Toggle menu"
                    >
                        {mobileMenuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
                    </button>
                </div>
            </div>

            {/* Mobile Drawer */}
            {mobileMenuOpen && (
                <div className="md:hidden border-t border-gray-200 bg-gray-50 py-4 px-6 space-y-1 flex flex-col transition-all duration-200">
                    <div className="px-3 py-1 text-xs font-semibold text-gray-400 uppercase tracking-wider">Compete</div>
                    <Link to="/contests" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                        <Trophy className="w-4 h-4" /> Contests
                    </Link>
                    <Link to="/gym" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                        <Dumbbell className="w-4 h-4" /> Gym
                    </Link>
                    <Link to="/practice" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                        <BookOpen className="w-4 h-4" /> Practice
                    </Link>
                    <Link to="/problems" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                        <Code2 className="w-4 h-4" /> Problems
                    </Link>

                    <div className="px-3 py-1 mt-2 text-xs font-semibold text-gray-400 uppercase tracking-wider">Community</div>
                    <Link to="/groups" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                        <Users className="w-4 h-4" /> Groups
                    </Link>
                    <Link to="/teams" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                        <UserCheck className="w-4 h-4" /> Teams
                    </Link>
                    <Link to="/blog" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                        <MessageSquare className="w-4 h-4" /> Blog
                    </Link>
                    <Link to="/rankings" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                        <BarChart3 className="w-4 h-4" /> Rankings
                    </Link>

                    {isAdmin && (
                        <>
                            <div className="px-3 py-1 mt-2 text-xs font-semibold text-gray-400 uppercase tracking-wider">Admin</div>
                            <Link to="/admin" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm font-semibold text-purple-600 hover:text-purple-800 py-2 px-3 rounded-lg hover:bg-purple-50">
                                <Settings className="w-4 h-4" /> Admin Dashboard
                            </Link>
                        </>
                    )}
                    {isSetter && (
                        <Link to="/setter" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm font-semibold text-orange-600 hover:text-orange-800 py-2 px-3 rounded-lg hover:bg-orange-50">
                            <FileCode className="w-4 h-4" /> Setter Workspace
                        </Link>
                    )}

                    <hr className="border-gray-200 my-2" />
                    {loggedIn ? (
                        <>
                            <Link to="/profile" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                                <User className="w-4 h-4" /> Profile
                            </Link>
                            <Link to="/notifications" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                                <Bell className="w-4 h-4" /> Notifications
                            </Link>
                            <Link to="/settings/api" onClick={() => setMobileMenuOpen(false)} className="flex items-center gap-2 text-sm text-gray-600 hover:text-black font-medium py-2 px-3 rounded-lg hover:bg-gray-100">
                                <Key className="w-4 h-4" /> API Keys
                            </Link>
                            <button onClick={handleLogout} className="flex items-center gap-2 text-left text-sm text-red-600 hover:text-red-800 font-medium py-2 px-3 rounded-lg hover:bg-red-50 cursor-pointer">
                                <LogOut className="w-4 h-4" /> Logout
                            </button>
                        </>
                    ) : (
                        <div className="flex flex-col gap-2 pt-2">
                            <Link to="/login" onClick={() => setMobileMenuOpen(false)} className="flex items-center justify-center gap-2 text-sm text-gray-600 hover:text-black font-medium border border-gray-300 rounded-lg py-2.5 hover:bg-gray-100">
                                Login
                            </Link>
                            <Link to="/register" onClick={() => setMobileMenuOpen(false)} className="flex items-center justify-center gap-2 text-sm bg-blue-600 text-white font-medium rounded-lg py-2.5 hover:bg-blue-700">
                                Register
                            </Link>
                        </div>
                    )}
                </div>
            )}
        </nav>
    )
}
