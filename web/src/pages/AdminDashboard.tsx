import { useState, useEffect } from 'react'
import { Users, FileText, Bot, Settings, Code, Globe, Send } from 'lucide-react'
import UsersPanel from './admin/UsersPanel'
import SetterAppsPanel from './admin/SetterAppsPanel'
import BotAccountsPanel from './admin/BotAccountsPanel'
import SystemSettingsPanel from './admin/SystemSettingsPanel'
import LanguagesPanel from './admin/LanguagesPanel'
import RemoteLanguagesPanel from './admin/RemoteLanguagesPanel'
import SubmissionsPanel from './admin/SubmissionsPanel'

type AdminTab = 'users' | 'applications' | 'bots' | 'languages' | 'remote-languages' | 'submissions' | 'settings'

const tabs: { key: AdminTab; label: string; icon: typeof Users }[] = [
    { key: 'users', label: 'Users', icon: Users },
    { key: 'applications', label: 'Applications', icon: FileText },
    { key: 'bots', label: 'Bot Accounts', icon: Bot },
    { key: 'languages', label: 'Languages', icon: Code },
    { key: 'remote-languages', label: 'Remote Languages', icon: Globe },
    { key: 'submissions', label: 'Remote Subs', icon: Send },
    { key: 'settings', label: 'Settings', icon: Settings },
]

export default function AdminDashboard() {
    const [activeTab, setActiveTab] = useState<AdminTab>(() => {
        const hash = window.location.hash.replace('#', '') as AdminTab
        const validTabs: AdminTab[] = ['users', 'applications', 'bots', 'languages', 'remote-languages', 'submissions', 'settings']
        return validTabs.includes(hash) ? hash : 'users'
    })

    useEffect(() => {
        window.location.hash = activeTab
    }, [activeTab])

    return (
        <div>
            <h1 className="text-2xl font-bold mb-6">Admin Dashboard</h1>
            <div className="flex gap-6">
                {/* Sidebar */}
                <nav className="w-48 flex-shrink-0">
                    <div className="space-y-1">
                        {tabs.map(t => (
                            <button
                                key={t.key}
                                onClick={() => setActiveTab(t.key)}
                                className={`w-full flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors text-left ${
                                    activeTab === t.key
                                        ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 font-medium'
                                        : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-gray-100'
                                }`}
                            >
                                <t.icon className="w-4 h-4" />
                                {t.label}
                            </button>
                        ))}
                    </div>
                </nav>

                {/* Content */}
                <div className="flex-1 min-w-0">
                    {activeTab === 'users' && <UsersPanel />}
                    {activeTab === 'applications' && <SetterAppsPanel />}
                    {activeTab === 'bots' && <BotAccountsPanel />}
                    {activeTab === 'languages' && <LanguagesPanel />}
                    {activeTab === 'remote-languages' && <RemoteLanguagesPanel />}
                    {activeTab === 'submissions' && <SubmissionsPanel />}
                    {activeTab === 'settings' && <SystemSettingsPanel />}
                </div>
            </div>
        </div>
    )
}
