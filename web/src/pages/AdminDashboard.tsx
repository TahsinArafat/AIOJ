import { useState } from 'react'
import { Users, FileText, Bot, Settings, Code } from 'lucide-react'
import UsersPanel from './admin/UsersPanel'
import SetterAppsPanel from './admin/SetterAppsPanel'
import BotAccountsPanel from './admin/BotAccountsPanel'
import SystemSettingsPanel from './admin/SystemSettingsPanel'
import LanguagesPanel from './admin/LanguagesPanel'

type AdminTab = 'users' | 'applications' | 'bots' | 'settings' | 'languages'

const tabs: { key: AdminTab; label: string; icon: typeof Users }[] = [
    { key: 'users', label: 'Users', icon: Users },
    { key: 'applications', label: 'Applications', icon: FileText },
    { key: 'bots', label: 'Bot Accounts', icon: Bot },
    { key: 'languages', label: 'Languages', icon: Code },
    { key: 'settings', label: 'Settings', icon: Settings },
]

export default function AdminDashboard() {
    const [activeTab, setActiveTab] = useState<AdminTab>('users')

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
                                        ? 'bg-blue-50 text-blue-700 font-medium'
                                        : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
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
                    {activeTab === 'settings' && <SystemSettingsPanel />}
                </div>
            </div>
        </div>
    )
}
