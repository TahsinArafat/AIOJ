import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { setTokens } from '../lib/api'

/** Landing page for OAuth callbacks: tokens arrive in the URL fragment. */
export default function OAuthComplete() {
    const nav = useNavigate()

    useEffect(() => {
        const hash = window.location.hash.replace(/^#/, '')
        const params = new URLSearchParams(hash)
        const access = params.get('access_token')
        const refresh = params.get('refresh_token')
        if (access && refresh) {
            setTokens(access, refresh)
            window.history.replaceState(null, '', window.location.pathname)
            nav('/')
            return
        }
        nav('/login')
    }, [nav])

    return (
        <div className="max-w-sm mx-auto mt-20 text-center text-sm text-gray-600 dark:text-gray-300">
            Completing sign-in…
        </div>
    )
}
