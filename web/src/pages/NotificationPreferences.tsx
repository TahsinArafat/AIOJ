import { useEffect, useState } from 'react'
import { api } from '../lib/api'

interface NotificationPrefs {
    email_contest_reminders: boolean
    email_rating_changes: boolean
    email_problem_updates: boolean
    email_blog_comments: boolean
    push_contest_reminders: boolean
    push_rating_changes: boolean
    push_problem_updates: boolean
    push_blog_comments: boolean
}

const DEFAULT_PREFS: NotificationPrefs = {
    email_contest_reminders: true,
    email_rating_changes: true,
    email_problem_updates: true,
    email_blog_comments: false,
    push_contest_reminders: true,
    push_rating_changes: true,
    push_problem_updates: true,
    push_blog_comments: false,
}

const PREF_LABELS: Record<keyof NotificationPrefs, string> = {
    email_contest_reminders: 'Contest Reminders',
    email_rating_changes: 'Rating Changes',
    email_problem_updates: 'Problem Updates',
    email_blog_comments: 'Blog Comments',
    push_contest_reminders: 'Contest Reminders',
    push_rating_changes: 'Rating Changes',
    push_problem_updates: 'Problem Updates',
    push_blog_comments: 'Blog Comments',
}

export default function NotificationPreferences() {
    const [prefs, setPrefs] = useState<NotificationPrefs>(DEFAULT_PREFS)
    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [saved, setSaved] = useState(false)

    useEffect(() => {
        api.notifications.getPreferences()
            .then(d => setPrefs({ ...DEFAULT_PREFS, ...d }))
            .catch(() => {})
            .finally(() => setLoading(false))
    }, [])

    const handleToggle = (key: keyof NotificationPrefs) => {
        setPrefs(prev => ({ ...prev, [key]: !prev[key] }))
        setSaved(false)
    }

    const handleSave = async () => {
        setSaving(true)
        try {
            await api.notifications.updatePreferences(prefs)
            setSaved(true)
            setTimeout(() => setSaved(false), 3000)
        } catch (e: any) {
            alert('Failed to save: ' + e.message)
        } finally {
            setSaving(false)
        }
    }

    if (loading) return <div className="text-center py-20 text-gray-400 dark:text-gray-500">Loading preferences...</div>

    const emailKeys = Object.keys(prefs).filter(k => k.startsWith('email_')) as (keyof NotificationPrefs)[]
    const pushKeys = Object.keys(prefs).filter(k => k.startsWith('push_')) as (keyof NotificationPrefs)[]

    return (
        <div className="max-w-2xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">Notification Preferences</h1>

            {/* Email Notifications */}
            <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 mb-6">
                <h2 className="text-lg font-semibold mb-4">Email Notifications</h2>
                <div className="space-y-4">
                    {emailKeys.map(key => (
                        <label key={key} className="flex items-center justify-between cursor-pointer">
                            <span className="text-sm text-gray-700 dark:text-gray-300">{PREF_LABELS[key]}</span>
                            <button
                                type="button"
                                role="switch"
                                aria-checked={prefs[key]}
                                onClick={() => handleToggle(key)}
                                className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${prefs[key] ? 'bg-blue-600' : 'bg-gray-200 dark:bg-gray-700'}`}
                            >
                                <span className={`inline-block h-4 w-4 transform rounded-full bg-white dark:bg-gray-800 transition-transform ${prefs[key] ? 'translate-x-6' : 'translate-x-1'}`} />
                            </button>
                        </label>
                    ))}
                </div>
            </div>

            {/* Push Notifications */}
            <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 mb-6">
                <h2 className="text-lg font-semibold mb-4">Push Notifications</h2>
                <div className="space-y-4">
                    {pushKeys.map(key => (
                        <label key={key} className="flex items-center justify-between cursor-pointer">
                            <span className="text-sm text-gray-700 dark:text-gray-300">{PREF_LABELS[key]}</span>
                            <button
                                type="button"
                                role="switch"
                                aria-checked={prefs[key]}
                                onClick={() => handleToggle(key)}
                                className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${prefs[key] ? 'bg-blue-600' : 'bg-gray-200 dark:bg-gray-700'}`}
                            >
                                <span className={`inline-block h-4 w-4 transform rounded-full bg-white dark:bg-gray-800 transition-transform ${prefs[key] ? 'translate-x-6' : 'translate-x-1'}`} />
                            </button>
                        </label>
                    ))}
                </div>
            </div>

            {/* Save Button */}
            <div className="flex items-center gap-4">
                <button
                    onClick={handleSave}
                    disabled={saving}
                    className="bg-blue-600 text-white px-6 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
                >
                    {saving ? 'Saving...' : 'Save Preferences'}
                </button>
                {saved && (
                    <span className="text-sm text-green-600 dark:text-green-400 font-medium">Saved successfully!</span>
                )}
            </div>
        </div>
    )
}
