import { useEffect, useState } from 'react'
import { useSearchParams, Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function GroupJoin() {
    const [searchParams] = useSearchParams()
    const code = searchParams.get('code') || ''
    const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading')
    const [message, setMessage] = useState('')
    const [groupName, setGroupName] = useState('')
    const [confirmed, setConfirmed] = useState(false)

    useEffect(() => {
        if (!code) {
            setStatus('error')
            setMessage('No invite code provided.')
            return
        }
        // Try to join immediately
        api.groups.joinByCode(code).then(d => {
            setStatus('success')
            setGroupName(d.group_name || 'the group')
            setConfirmed(true)
        }).catch((e: any) => {
            setStatus('error')
            setMessage(e.message || 'Failed to join group.')
        })
    }, [code])

    const handleJoin = async () => {
        if (!code) return
        setStatus('loading')
        try {
            const d = await api.groups.joinByCode(code)
            setStatus('success')
            setGroupName(d.group_name || 'the group')
            setConfirmed(true)
        } catch (e: any) {
            setStatus('error')
            setMessage(e.message || 'Failed to join group.')
        }
    }

    return (
        <div className="max-w-lg mx-auto text-center py-20">
            {status === 'loading' && (
                <div>
                    <h1 className="text-2xl font-bold mb-4">Joining group...</h1>
                    <p className="text-gray-400 dark:text-gray-500">Please wait while we process your request.</p>
                </div>
            )}

            {status === 'success' && (
                <div>
                    <h1 className="text-2xl font-bold mb-4">Joined Successfully!</h1>
                    <p className="text-gray-600 dark:text-gray-400 mb-6">
                        {confirmed ? 'You are now a member of ' : ''}<span className="font-semibold">{groupName}</span>.
                    </p>
                    <div className="flex justify-center gap-4">
                        <Link to="/groups" className="px-6 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700 font-medium">
                            Go to Groups
                        </Link>
                    </div>
                </div>
            )}

            {status === 'error' && !confirmed && (
                <div>
                    <h1 className="text-2xl font-bold mb-4">Join Group</h1>
                    {code ? (
                        <div>
                            <p className="text-gray-600 dark:text-gray-400 mb-2">
                                You were invited to join a group using code: <code className="font-mono bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded">{code}</code>
                            </p>
                            {message && <p className="text-red-600 dark:text-red-400 text-sm mb-4">{message}</p>}
                            <button onClick={handleJoin}
                                className="px-6 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700 font-medium cursor-pointer">
                                Try Again
                            </button>
                        </div>
                    ) : (
                        <div>
                            <p className="text-red-600 dark:text-red-400 mb-4">{message}</p>
                            <Link to="/groups" className="px-6 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700 font-medium">
                                Browse Groups
                            </Link>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}
