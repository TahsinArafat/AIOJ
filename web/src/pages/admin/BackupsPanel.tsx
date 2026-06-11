import { useEffect, useState, useRef } from 'react'
import { api } from '../../lib/api'
import { Database, HardDrive, Archive, Upload, Download, Trash2, RefreshCw, AlertTriangle, X, FolderArchive, ShieldAlert } from 'lucide-react'

interface BackupFile {
    filename: string
    size: number
    created_at: string
    type: string
}

export default function BackupsPanel() {
    const [backups, setBackups] = useState<BackupFile[]>([])
    const [loading, setLoading] = useState(true)
    const [creating, setCreating] = useState(false)
    const [createType, setCreateType] = useState<string>('full')
    const [uploading, setUploading] = useState(false)
    const [restoreModal, setRestoreModal] = useState<{ backup: BackupFile; type: string } | null>(null)
    const [password, setPassword] = useState('')
    const [restoring, setRestoring] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const fileInputRef = useRef<HTMLInputElement>(null)

    const loadBackups = () => {
        setLoading(true)
        api.admin.backups.list()
            .then(d => setBackups(d.data || []))
            .catch(err => setError(err.message))
            .finally(() => setLoading(false))
    }

    useEffect(() => { loadBackups() }, [])

    const formatSize = (bytes: number) => {
        if (bytes === 0) return '0 B'
        const k = 1024
        const sizes = ['B', 'KB', 'MB', 'GB']
        const i = Math.floor(Math.log(bytes) / Math.log(k))
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
    }

    const formatDate = (dateStr: string) => {
        return new Date(dateStr).toLocaleString('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
        })
    }

    const getTypeBadge = (type: string) => {
        const badges: Record<string, { color: string; icon: React.ReactNode }> = {
            database: {
                color: 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300',
                icon: <Database className="w-3 h-3" />,
            },
            files: {
                color: 'bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-300',
                icon: <HardDrive className="w-3 h-3" />,
            },
            full: {
                color: 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-800 dark:text-emerald-300',
                icon: <Archive className="w-3 h-3" />,
            },
        }
        const badge = badges[type] || badges.full
        return (
            <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${badge.color}`}>
                {badge.icon}
                {type}
            </span>
        )
    }

    const handleCreateBackup = async () => {
        setCreating(true)
        setError(null)
        try {
            await api.admin.backups.create(createType)
            loadBackups()
        } catch (err: any) {
            setError(err.message)
        } finally {
            setCreating(false)
        }
    }

    const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (!file) return

        setUploading(true)
        setError(null)
        try {
            await api.admin.backups.upload(file)
            loadBackups()
        } catch (err: any) {
            setError(err.message)
        } finally {
            setUploading(false)
            if (fileInputRef.current) fileInputRef.current.value = ''
        }
    }

    const handleDelete = async (filename: string) => {
        if (!confirm(`Delete backup "${filename}"?`)) return
        setError(null)
        try {
            await api.admin.backups.delete(filename)
            loadBackups()
        } catch (err: any) {
            setError(err.message)
        }
    }

    const handleDownload = (backup: BackupFile) => {
        const token = localStorage.getItem('access_token')
        const link = document.createElement('a')
        link.href = `/api/admin/backups/${encodeURIComponent(backup.filename)}`
        if (token) {
            link.href += `?token=${token}`
        }
        link.download = backup.filename
        link.click()
    }

    const handleRestore = async () => {
        if (!restoreModal || !password) return
        setRestoring(true)
        setError(null)
        try {
            await api.admin.backups.restore(restoreModal.backup.filename, password, restoreModal.type)
            setRestoreModal(null)
            setPassword('')
            alert('Restore completed successfully')
        } catch (err: any) {
            setError(err.message)
        } finally {
            setRestoring(false)
        }
    }

    const openRestoreModal = (backup: BackupFile, type: string) => {
        setRestoreModal({ backup, type })
        setPassword('')
    }

    if (loading) {
        return (
            <div className="flex items-center justify-center py-12">
                <RefreshCw className="w-5 h-5 animate-spin text-gray-400" />
                <span className="ml-2 text-gray-500 dark:text-gray-400">Loading backups...</span>
            </div>
        )
    }

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <div>
                    <h2 className="text-lg font-semibold">Backups</h2>
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                        Create, manage, and restore system backups
                    </p>
                </div>
                <button
                    onClick={loadBackups}
                    className="flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                >
                    <RefreshCw className="w-4 h-4" /> Refresh
                </button>
            </div>

            {error && (
                <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-700 dark:text-red-300 flex items-center gap-2">
                    <AlertTriangle className="w-4 h-4 flex-shrink-0" />
                    {error}
                    <button onClick={() => setError(null)} className="ml-auto">
                        <X className="w-4 h-4" />
                    </button>
                </div>
            )}

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mb-6">
                <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                    <div className="flex items-center gap-3 mb-3">
                        <div className="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
                            <Database className="w-5 h-5 text-blue-600 dark:text-blue-400" />
                        </div>
                        <div>
                            <h3 className="font-medium text-sm">Database Backup</h3>
                            <p className="text-xs text-gray-500 dark:text-gray-400">PostgreSQL dump</p>
                        </div>
                    </div>
                    <button
                        onClick={() => { setCreateType('database'); handleCreateBackup() }}
                        disabled={creating}
                        className="w-full px-3 py-2 bg-blue-600 text-white text-sm rounded hover:bg-blue-700 disabled:opacity-50 transition-colors"
                    >
                        {creating && createType === 'database' ? 'Creating...' : 'Create Database Backup'}
                    </button>
                </div>

                <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                    <div className="flex items-center gap-3 mb-3">
                        <div className="p-2 bg-amber-100 dark:bg-amber-900/30 rounded-lg">
                            <HardDrive className="w-5 h-5 text-amber-600 dark:text-amber-400" />
                        </div>
                        <div>
                            <h3 className="font-medium text-sm">Files Backup</h3>
                            <p className="text-xs text-gray-500 dark:text-gray-400">Uploaded files & testcases</p>
                        </div>
                    </div>
                    <button
                        onClick={() => { setCreateType('files'); handleCreateBackup() }}
                        disabled={creating}
                        className="w-full px-3 py-2 bg-amber-600 text-white text-sm rounded hover:bg-amber-700 disabled:opacity-50 transition-colors"
                    >
                        {creating && createType === 'files' ? 'Creating...' : 'Create Files Backup'}
                    </button>
                </div>

                <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                    <div className="flex items-center gap-3 mb-3">
                        <div className="p-2 bg-emerald-100 dark:bg-emerald-900/30 rounded-lg">
                            <Archive className="w-5 h-5 text-emerald-600 dark:text-emerald-400" />
                        </div>
                        <div>
                            <h3 className="font-medium text-sm">Full Backup</h3>
                            <p className="text-xs text-gray-500 dark:text-gray-400">Database + Files combined</p>
                        </div>
                    </div>
                    <button
                        onClick={() => { setCreateType('full'); handleCreateBackup() }}
                        disabled={creating}
                        className="w-full px-3 py-2 bg-emerald-600 text-white text-sm rounded hover:bg-emerald-700 disabled:opacity-50 transition-colors"
                    >
                        {creating && createType === 'full' ? 'Creating...' : 'Create Full Backup'}
                    </button>
                </div>
            </div>

            <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 mb-6">
                <div className="flex items-center gap-3">
                    <div className="p-2 bg-purple-100 dark:bg-purple-900/30 rounded-lg">
                        <FolderArchive className="w-5 h-5 text-purple-600 dark:text-purple-400" />
                    </div>
                    <div className="flex-1">
                        <h3 className="font-medium text-sm">Upload Backup Archive</h3>
                        <p className="text-xs text-gray-500 dark:text-gray-400">Upload a .tar.gz backup file to restore</p>
                    </div>
                    <label className="cursor-pointer">
                        <input
                            ref={fileInputRef}
                            type="file"
                            accept=".tar.gz"
                            onChange={handleUpload}
                            className="hidden"
                        />
                        <span className="inline-flex items-center gap-1.5 px-3 py-2 bg-purple-600 text-white text-sm rounded hover:bg-purple-700 transition-colors">
                            <Upload className="w-4 h-4" />
                            {uploading ? 'Uploading...' : 'Choose File'}
                        </span>
                    </label>
                </div>
            </div>

            <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                <div className="bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 py-3">
                    <h3 className="font-semibold text-sm text-gray-700 dark:text-gray-300">Saved Backups</h3>
                </div>
                <table className="w-full text-sm">
                    <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400 text-xs uppercase">
                        <tr>
                            <th className="px-4 py-3 text-left">Filename</th>
                            <th className="px-4 py-3 text-left">Type</th>
                            <th className="px-4 py-3 text-left">Size</th>
                            <th className="px-4 py-3 text-left">Created</th>
                            <th className="px-4 py-3 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                        {backups.map(b => (
                            <tr key={b.filename} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                                <td className="px-4 py-3 font-mono text-xs">{b.filename}</td>
                                <td className="px-4 py-3">{getTypeBadge(b.type)}</td>
                                <td className="px-4 py-3 text-gray-500 dark:text-gray-400 text-xs">{formatSize(b.size)}</td>
                                <td className="px-4 py-3 text-gray-500 dark:text-gray-400 text-xs">{formatDate(b.created_at)}</td>
                                <td className="px-4 py-3 text-right">
                                    <div className="flex justify-end gap-2">
                                        <button
                                            onClick={() => handleDownload(b)}
                                            className="p-1.5 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded transition-colors"
                                            title="Download"
                                        >
                                            <Download className="w-4 h-4" />
                                        </button>
                                        <button
                                            onClick={() => openRestoreModal(b, b.type)}
                                            className="p-1.5 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/20 rounded transition-colors"
                                            title="Restore"
                                        >
                                            <ShieldAlert className="w-4 h-4" />
                                        </button>
                                        <button
                                            onClick={() => handleDelete(b.filename)}
                                            className="p-1.5 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
                                            title="Delete"
                                        >
                                            <Trash2 className="w-4 h-4" />
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        ))}
                        {backups.length === 0 && (
                            <tr>
                                <td colSpan={5} className="px-4 py-12 text-center text-gray-400 dark:text-gray-500">
                                    <Archive className="w-8 h-8 mx-auto mb-2 opacity-50" />
                                    <p>No backups found. Create your first backup above.</p>
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>

            {restoreModal && (
                <div className="fixed inset-0 z-50 flex items-center justify-center">
                    <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => !restoring && setRestoreModal(null)} />
                    <div className="relative bg-white dark:bg-gray-800 rounded-xl shadow-2xl border border-gray-200 dark:border-gray-700 w-full max-w-md mx-4 overflow-hidden">
                        <div className="bg-red-600 px-6 py-4 flex items-center gap-3">
                            <div className="p-2 bg-white/20 rounded-lg">
                                <AlertTriangle className="w-6 h-6 text-white" />
                            </div>
                            <div>
                                <h3 className="font-bold text-white text-lg">Restore Backup</h3>
                                <p className="text-red-100 text-xs">This action is irreversible</p>
                            </div>
                        </div>

                        <div className="p-6">
                            <div className="bg-red-50 dark:bg-red-900/20 border-2 border-red-200 dark:border-red-800 rounded-lg p-4 mb-4">
                                <div className="flex items-start gap-3">
                                    <ShieldAlert className="w-5 h-5 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
                                    <div>
                                        <p className="text-sm font-bold text-red-800 dark:text-red-300 uppercase tracking-wide mb-1">
                                            Destructive Operation
                                        </p>
                                        <p className="text-sm text-red-700 dark:text-red-400">
                                            Restoring will <strong>permanently overwrite</strong> your current {restoreModal.type} data.
                                            All changes made after this backup was created will be <strong>lost</strong>.
                                        </p>
                                    </div>
                                </div>
                            </div>

                            <div className="mb-4">
                                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                                    Backup File
                                </label>
                                <div className="px-3 py-2 bg-gray-50 dark:bg-gray-900 rounded border border-gray-200 dark:border-gray-700 font-mono text-xs text-gray-700 dark:text-gray-300">
                                    {restoreModal.backup.filename}
                                </div>
                            </div>

                            <div className="mb-6">
                                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                                    Admin Password
                                </label>
                                <input
                                    type="password"
                                    value={password}
                                    onChange={e => setPassword(e.target.value)}
                                    placeholder="Enter your password to confirm"
                                    className="w-full border border-gray-300 dark:border-gray-600 rounded px-3 py-2 text-sm bg-white dark:bg-gray-900 focus:ring-2 focus:ring-red-500 focus:border-red-500 outline-none"
                                    autoFocus
                                />
                                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                    Password required to authorize this destructive operation
                                </p>
                            </div>

                            <div className="flex justify-end gap-3">
                                <button
                                    onClick={() => setRestoreModal(null)}
                                    disabled={restoring}
                                    className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 transition-colors"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={handleRestore}
                                    disabled={!password || restoring}
                                    className="px-4 py-2 bg-red-600 text-white text-sm font-medium rounded hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center gap-2"
                                >
                                    {restoring ? (
                                        <>
                                            <RefreshCw className="w-4 h-4 animate-spin" />
                                            Restoring...
                                        </>
                                    ) : (
                                        <>
                                            <ShieldAlert className="w-4 h-4" />
                                            Restore Backup
                                        </>
                                    )}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
