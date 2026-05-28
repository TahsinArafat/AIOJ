import { BrowserRouter, Routes, Route, Link, useNavigate, useLocation } from 'react-router-dom'
import { getAccessToken, clearTokens } from './lib/api'
import Login from './pages/Login'
import Register from './pages/Register'
import ProblemList from './pages/ProblemList'
import ProblemDetail from './pages/ProblemDetail'
import ContestList from './pages/ContestList'
import ContestDetail from './pages/ContestDetail'
import ContestScoreboard from './pages/ContestScoreboard'
import AdminDashboard from './pages/AdminDashboard'
import SetterPanel from './pages/SetterPanel'
import Profile from './pages/Profile'
import ProblemCreate from './pages/ProblemCreate'
import Submissions from './pages/Submissions'
import GymList from './pages/GymList'
import GymDetail from './pages/GymDetail'
import HackPanel from './pages/HackPanel'
import './global.css'

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

function Navbar() {
    const navigate = useNavigate()
    useLocation() // force re-render on route change so auth state updates
    const loggedIn = !!getAccessToken()
    const role = decodeRole()
    const isAdmin = role === 'admin'
    const isSetter = role === 'teacher' || isAdmin

    const handleLogout = () => {
        clearTokens()
        navigate('/login')
    }

    return (
        <nav className="border-b border-gray-200 px-6 py-3 flex items-center justify-between bg-white sticky top-0 z-10">
            <div className="flex gap-6 items-center">
                <Link to="/" className="font-bold text-blue-600 text-lg">AIOJ</Link>
                <Link to="/problems" className="text-sm text-gray-600 hover:text-black transition-colors">Problems</Link>
                <Link to="/contests" className="text-sm text-gray-600 hover:text-black transition-colors">Contests</Link>
                <Link to="/gym" className="text-sm text-gray-600 hover:text-black transition-colors">Gym</Link>
                {isAdmin && <Link to="/admin" className="text-sm font-medium text-purple-600 hover:text-purple-800 transition-colors">Admin</Link>}
                {isSetter && <Link to="/setter" className="text-sm font-medium text-orange-600 hover:text-orange-800 transition-colors">Setter Workspace</Link>}
            </div>
            <div className="flex gap-3 items-center">
                {loggedIn ? (
                    <>
                        <Link to="/profile" className="text-sm text-gray-600 hover:text-black">Profile</Link>
                        <Link to="/submissions" className="text-sm text-gray-600 hover:text-black">My Submissions</Link>
                        <button onClick={handleLogout} className="text-sm text-gray-500 hover:text-black">Logout</button>
                    </>
                ) : (
                    <>
                        <Link to="/login" className="text-sm text-gray-600 hover:text-black">Login</Link>
                        <Link to="/register" className="text-sm bg-blue-600 text-white px-3 py-1.5 rounded hover:bg-blue-700 transition-colors">Register</Link>
                    </>
                )}
            </div>
        </nav>
    )
}

function Home() {
    return (
        <div className="text-center py-24">
            <h1 className="text-4xl font-bold mb-3">AIOJ</h1>
            <p className="text-gray-500">Lightweight Online Judge for Competitive Programming</p>
            <div className="mt-8 flex justify-center gap-4">
                <Link to="/problems" className="bg-blue-600 text-white px-6 py-2.5 rounded hover:bg-blue-700 transition-colors">Browse Problems</Link>
                <Link to="/contests" className="border border-gray-300 px-6 py-2.5 rounded hover:bg-gray-50 transition-colors">View Contests</Link>
            </div>
        </div>
    )
}

export default function App() {
    return (
        <BrowserRouter>
            <div className="min-h-screen bg-white">
                <Navbar />
                <main className="max-w-5xl mx-auto px-4 py-6">
                    <Routes>
                        <Route path="/" element={<Home />} />
                        <Route path="/problems" element={<ProblemList />} />
                        <Route path="/problems/:slug" element={<ProblemDetail />} />
                        <Route path="/login" element={<Login />} />
                        <Route path="/register" element={<Register />} />
                        <Route path="/contests" element={<ContestList />} />
                        <Route path="/contests/:id" element={<ContestDetail />} />
                        <Route path="/contests/:id/scoreboard" element={<ContestScoreboard />} />
                        <Route path="/gym" element={<GymList />} />
                        <Route path="/gym/:id" element={<GymDetail />} />
                        <Route path="/hack/:contestId/:problemId" element={<HackPanel />} />
                        <Route path="/submissions" element={<Submissions />} />
                        <Route path="/admin" element={<AdminDashboard />} />
                        <Route path="/setter" element={<SetterPanel />} />
                        <Route path="/setter/create" element={<ProblemCreate />} />
                        <Route path="/profile" element={<Profile />} />
                        <Route path="*" element={<div className="text-center py-20 text-gray-400">404 Not Found</div>} />
                    </Routes>
                </main>
            </div>
        </BrowserRouter>
    )
}