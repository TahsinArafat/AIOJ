import { useState, useEffect } from 'react'
import { Users, FileText, Bot, Settings, Code, Globe, Send, Database, Cpu } from 'lucide-react'
import UsersPanel from './admin/UsersPanel'
import SetterAppsPanel from './admin/SetterAppsPanel'
import BotAccountsPanel from './admin/BotAccountsPanel'
import SystemSettingsPanel from './admin/SystemSettingsPanel'
import LanguagesPanel from './admin/LanguagesPanel'
import RemoteLanguagesPanel from './admin/RemoteLanguagesPanel'
import SubmissionsPanel from './admin/SubmissionsPanel'
import BackupsPanel from './admin/BackupsPanel'
import AIModelsPanel from './admin/AIModelsPanel'

type AdminTab = 'users' | 'applications' | 'bots' | 'languages' | 'remote-languages' | 'submissions' | 'settings' | 'backups' | 'ai-models'

const tabs: { key: AdminTab; label: string; icon: typeof Users }[] = [
    { key: 'users', label: 'Users', icon: Users },
    { key: 'applications', label: 'Applications', icon: FileText },
    { key: 'bots', label: 'Bot Accounts', icon: Bot },
    { key: 'languages', label: 'Languages', icon: Code },
    { key: 'remote-languages', label: 'Remote Languages', icon: Globe },
    { key: 'submissions', label: 'Remote Subs', icon: Send },
    { key: 'backups', label: 'Backup & Restore', icon: Database },
    { key: 'ai-models', label: 'AI Models', icon: Cpu },
    { key: 'settings', label: 'Settings', icon: Settings },
]

export default function AdminDashboard() {
    const [activeTab, setActiveTab] = useState<AdminTab>(() => {
        const hash = window.location.hash.replace('#', '') as AdminTab
        const validTabs: AdminTab[] = ['users', 'applications', 'bots', 'languages', 'remote-languages', 'submissions', 'backups', 'ai-models', 'settings']
        return validTabs.includes(hash) ? hash : 'users'
    })

    // Keep the URL and the selected tab in sync in both directions. The hash is
    // only read in the useState initializer, so without this listener a
    // deep link or a Back/Forward step that changes the hash alone would leave
    // the panel showing a different tab than the URL claims.
    useEffect(() => {
        if (window.location.hash.replace('#', '') !== activeTab) {
            window.history.replaceState(null, '', `#${activeTab}`)
        }
        const onHashChange = () => {
            const next = window.location.hash.replace('#', '') as AdminTab
            const valid: AdminTab[] = ['users', 'applications', 'bots', 'languages', 'remote-languages', 'submissions', 'backups', 'ai-models', 'settings']
            if (valid.includes(next)) setActiveTab(next)
        }
        window.addEventListener('hashchange', onHashChange)
        return () => window.removeEventListener('hashchange', onHashChange)
    }, [activeTab])

    return (
        <div>
            <h1 className="text-2xl font-bold mb-6">Admin Dashboard</h1>
            {/* Stack under lg: a 12rem sidebar next to content left ~150px for the
                panel on a 390px phone. On mobile the tabs become a horizontal
                scroller instead so every tab stays reachable. */}
            <div className="flex flex-col lg:flex-row gap-6">
                {/* Sidebar */}
                <nav className="w-full lg:w-48 lg:flex-shrink-0">
                    <div className="flex lg:block gap-1 overflow-x-auto lg:overflow-visible lg:space-y-1 pb-1 lg:pb-0 -mx-1 px-1 lg:mx-0 lg:px-0">
                        {tabs.map(t => (
                            <button
                                key={t.key}
                                onClick={() => setActiveTab(t.key)}
                                className={`w-auto lg:w-full shrink-0 flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors text-left whitespace-nowrap ${activeTab === t.key
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
                    {activeTab === 'backups' && <BackupsPanel />}
                    {activeTab === 'ai-models' && <AIModelsPanel />}
                    {activeTab === 'settings' && <SystemSettingsPanel />}
                </div>
            </div>
        </div>
    )
}
