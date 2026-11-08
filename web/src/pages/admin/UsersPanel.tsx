import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
    Ban, CheckCircle2, ChevronLeft, ChevronRight, RefreshCw, RotateCcw, Search,
    ShieldCheck, ShieldOff, UserCog, X,
} from 'lucide-react'
import { api } from '../../lib/api'
import type {
    AdminBulkAction, AdminUserDetail, AdminUserQuery, AdminUserRow, AdminUserSort, AdminUserStatus,
} from '../../types/api'
import { Badge, type BadgeTone } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Field, Input, Select } from '../../components/ui/Field'
import { Table, TableShell, Td, Th, THead, Tr } from '../../components/ui/Table'
import { useConfirm } from '../../components/ConfirmDialog'
import { useToast } from '../../components/Toast'

const ROLES = ['admin', 'setter', 'user', 'contestant', 'bot'] as const

const ROLE_TONE: Record<string, BadgeTone> = {
    admin: 'brand',
    setter: 'accent',
    user: 'neutral',
    contestant: 'neutral',
    bot: 'warn',
}

const STATUS_OPTIONS: { value: AdminUserStatus | ''; label: string }[] = [
    { value: '', label: 'All accounts' },
    { value: 'active', label: 'Active' },
    { value: 'banned', label: 'Banned' },
    { value: 'unverified', label: 'Unverified email' },
    { value: 'must_change_password', label: 'Must change password' },
]

const SORT_OPTIONS: { value: AdminUserSort | ''; label: string }[] = [
    { value: 'created', label: 'Newest first' },
    { value: 'last_login', label: 'Last sign-in' },
    { value: 'username', label: 'Username' },
    { value: 'rating', label: 'Rating' },
    { value: 'submissions', label: 'Submissions' },
]

const PAGE_SIZES = [25, 50, 100]

function fmtDate(iso?: string): string {
    if (!iso) return '—'
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return '—'
    return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

function fmtRelative(iso?: string): string {
    if (!iso) return 'never'
    const d = new Date(iso).getTime()
    if (Number.isNaN(d)) return 'never'
    const secs = Math.round((Date.now() - d) / 1000)
    if (secs < 60) return 'just now'
    const mins = Math.round(secs / 60)
    if (mins < 60) return `${mins}m ago`
    const hours = Math.round(mins / 60)
    if (hours < 24) return `${hours}h ago`
    const days = Math.round(hours / 24)
    if (days < 30) return `${days}d ago`
    return fmtDate(iso)
}

/** Minimal modal shell: overlay, Escape to close, labelled by its heading. */
function Modal({
    title, description, onClose, children, footer,
}: {
    title: string
    description?: string
    onClose: () => void
    children: React.ReactNode
    footer: React.ReactNode
}) {
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [onClose])

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="presentation">
            <div className="absolute inset-0 bg-black/50" onClick={onClose} role="presentation" />
            <div
                role="dialog"
                aria-modal="true"
                aria-label={title}
                className="relative w-full max-w-md rounded-xl border border-hairline bg-surface p-5 shadow-[var(--shadow-card)]"
            >
                <div className="mb-4 flex items-start justify-between gap-4">
                    <div>
                        <h3 className="text-base font-semibold text-ink">{title}</h3>
                        {description && <p className="mt-1 text-sm text-muted">{description}</p>}
                    </div>
                    <button onClick={onClose} className="rounded p-1 text-muted hover:bg-surface-2" aria-label="Close">
                        <X className="h-4 w-4" />
                    </button>
                </div>
                {children}
                <div className="mt-5 flex justify-end gap-2">{footer}</div>
            </div>
        </div>
    )
}

export default function UsersPanel() {
    const confirm = useConfirm()
    const toast = useToast()

    const [search, setSearch] = useState('')
    // `query` is what was actually requested; the search box is debounced into it
    // so typing does not fire a request per keystroke.
    const [query, setQuery] = useState<AdminUserQuery>({ sort: 'created', order: 'desc', limit: 25, offset: 0 })
    const [users, setUsers] = useState<AdminUserRow[]>([])
    const [total, setTotal] = useState(0)
    const [loading, setLoading] = useState(true)
    const [selected, setSelected] = useState<Set<string>>(new Set())
    const [busy, setBusy] = useState(false)

    const [detail, setDetail] = useState<AdminUserDetail | null>(null)
    const [detailLoading, setDetailLoading] = useState(false)
    const [banTarget, setBanTarget] = useState<AdminUserRow | null>(null)
    const [banReason, setBanReason] = useState('')
    const [pwTarget, setPwTarget] = useState<AdminUserRow | null>(null)
    const [newPassword, setNewPassword] = useState('')
    const [pwError, setPwError] = useState('')

    const requestSeq = useRef(0)

    const load = useCallback((q: AdminUserQuery) => {
        // Out-of-order responses: a slow request for an older query must not
        // overwrite a newer one, so only the latest sequence may set state.
        const seq = ++requestSeq.current
        setLoading(true)
        api.admin.users.list(q)
            .then(d => {
                if (seq !== requestSeq.current) return
                setUsers(d.data || [])
                setTotal(d.total ?? 0)
            })
            .catch(e => { if (seq === requestSeq.current) toast.error(e instanceof Error ? e.message : String(e)) })
            .finally(() => { if (seq === requestSeq.current) setLoading(false) })
    }, [toast])

    useEffect(() => { queueMicrotask(() => load(query)) }, [load, query])

    useEffect(() => {
        const t = setTimeout(() => {
            setQuery(prev => ({ ...prev, search: search.trim(), offset: 0 }))
        }, 300)
        return () => clearTimeout(t)
    }, [search])

    const refresh = () => load(query)
    const patchQuery = (patch: Partial<AdminUserQuery>) =>
        setQuery(prev => ({ ...prev, ...patch, offset: patch.offset ?? 0 }))

    const page = Math.floor((query.offset ?? 0) / (query.limit ?? 25)) + 1
    const pageCount = Math.max(1, Math.ceil(total / (query.limit ?? 25)))

    const allVisibleSelected = users.length > 0 && users.every(u => selected.has(u.id))
    const toggleAll = () => {
        setSelected(prev => {
            const next = new Set(prev)
            if (allVisibleSelected) users.forEach(u => next.delete(u.id))
            else users.forEach(u => next.add(u.id))
            return next
        })
    }
    const toggleOne = (id: string) => {
        setSelected(prev => {
            const next = new Set(prev)
            if (next.has(id)) next.delete(id)
            else next.add(id)
            return next
        })
    }

    const runBulk = async (action: AdminBulkAction, extra: { role?: string; reason?: string } = {}) => {
        const ids = [...selected]
        if (ids.length === 0) return
        const label = action === 'set_role' ? `Change role to ${extra.role}` : action.replace('_', ' ')
        if (!(await confirm({ message: `${label} for ${ids.length} account${ids.length > 1 ? 's' : ''}?`, variant: 'danger' }))) return

        setBusy(true)
        try {
            const res = await api.admin.users.bulk({ action, ids, ...extra })
            // Partial success is the normal case (the last admin, or your own
            // account, is skipped), so report both halves instead of a bare "done".
            if (res.failed > 0) {
                toast.error(`${res.succeeded} succeeded, ${res.failed} skipped`)
            } else {
                toast.success(`${res.succeeded} account${res.succeeded > 1 ? 's' : ''} updated`)
            }
            setSelected(new Set())
            refresh()
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
        } finally {
            setBusy(false)
        }
    }

    const setRole = async (u: AdminUserRow, role: string) => {
        if (!(await confirm({ message: `Change ${u.username}'s role to ${role}?`, variant: 'danger' }))) return
        try {
            await api.admin.users.setRole(u.id, role)
            toast.success('Role updated')
            refresh()
            if (detail?.id === u.id) openDetail(u.id)
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
        }
    }

    const unban = async (u: AdminUserRow) => {
        if (!(await confirm({ message: `Lift the ban on ${u.username}?` }))) return
        try {
            await api.admin.users.unban(u.id)
            toast.success(`${u.username} can sign in again`)
            refresh()
            if (detail?.id === u.id) openDetail(u.id)
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
        }
    }

    const submitBan = async () => {
        if (!banTarget) return
        setBusy(true)
        try {
            await api.admin.users.ban(banTarget.id, banReason.trim())
            toast.success(`${banTarget.username} suspended`)
            setBanTarget(null)
            setBanReason('')
            refresh()
            if (detail?.id === banTarget.id) openDetail(banTarget.id)
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
        } finally {
            setBusy(false)
        }
    }

    const toggleVerified = async (u: AdminUserRow) => {
        try {
            if (u.email_verified) await api.admin.users.unverifyEmail(u.id)
            else await api.admin.users.verifyEmail(u.id)
            toast.success(u.email_verified ? 'Email marked unverified' : 'Email marked verified')
            refresh()
            if (detail?.id === u.id) openDetail(u.id)
        } catch (e) {
            toast.error(e instanceof Error ? e.message : String(e))
        }
    }

    const submitPassword = async () => {
        if (!pwTarget) return
        setPwError('')
        if (newPassword.length < 12) {
            setPwError('At least 12 characters.')
            return
        }
        setBusy(true)
        try {
            await api.admin.users.resetPassword(pwTarget.id, newPassword, true)
            toast.success(`Password reset for ${pwTarget.username}`)
            setPwTarget(null)
            setNewPassword('')
        } catch (e) {
            // The API enforces the same policy as registration and returns the
            // reason (too weak / too common); surface it in the field.
            setPwError(e instanceof Error ? e.message : String(e))
        } finally {
            setBusy(false)
        }
    }

    const openDetail = (id: string) => {
        setDetailLoading(true)
        api.admin.users.get(id)
            .then(setDetail)
            .catch(e => toast.error(e instanceof Error ? e.message : String(e)))
            .finally(() => setDetailLoading(false))
    }

    const selectedCount = selected.size
    const filtersActive = Boolean(query.search || query.role || query.status)

    const rows = useMemo(() => users, [users])

    return (
        <div>
            <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
                <div>
                    <h2 className="text-lg font-semibold">Users</h2>
                    <p className="mt-1 text-sm text-muted">
                        {total.toLocaleString()} account{total === 1 ? '' : 's'}
                        {filtersActive ? ' matching the current filters' : ''}
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    <Button variant="secondary" size="sm" onClick={refresh} disabled={loading}>
                        <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} /> Refresh
                    </Button>
                </div>
            </div>

            {/* Filters */}
            <div className="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <div className="relative sm:col-span-2">
                    <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-soft" />
                    <Input
                        value={search}
                        onChange={e => setSearch(e.target.value)}
                        placeholder="Search username, email, or user id"
                        aria-label="Search users"
                        className="pl-9"
                    />
                </div>
                <Select
                    value={query.role ?? ''}
                    onChange={e => patchQuery({ role: e.target.value })}
                    aria-label="Filter by role"
                >
                    <option value="">Any role</option>
                    {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
                </Select>
                <Select
                    value={query.status ?? ''}
                    onChange={e => patchQuery({ status: e.target.value as AdminUserStatus | '' })}
                    aria-label="Filter by account status"
                >
                    {STATUS_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                </Select>
            </div>

            <div className="mb-4 flex flex-wrap items-center gap-2">
                <Select
                    value={query.sort ?? 'created'}
                    onChange={e => patchQuery({ sort: e.target.value as AdminUserSort })}
                    aria-label="Sort by"
                    className="w-auto"
                >
                    {SORT_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                </Select>
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => patchQuery({ order: query.order === 'asc' ? 'desc' : 'asc' })}
                    title="Toggle sort direction"
                >
                    {query.order === 'asc' ? 'Ascending' : 'Descending'}
                </Button>
                <Select
                    value={String(query.limit ?? 25)}
                    onChange={e => patchQuery({ limit: Number(e.target.value) })}
                    aria-label="Rows per page"
                    className="w-auto"
                >
                    {PAGE_SIZES.map(n => <option key={n} value={n}>{n} / page</option>)}
                </Select>
                {filtersActive && (
                    <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => { setSearch(''); setQuery({ sort: 'created', order: 'desc', limit: query.limit ?? 25, offset: 0 }) }}
                    >
                        Clear filters
                    </Button>
                )}
            </div>

            {/* Bulk action bar */}
            {selectedCount > 0 && (
                <div className="mb-3 flex flex-wrap items-center gap-2 rounded-lg border border-hairline bg-surface-2 px-3 py-2">
                    <span className="text-sm text-ink-soft">{selectedCount} selected</span>
                    <div className="ml-auto flex flex-wrap items-center gap-2">
                        <Button size="sm" variant="secondary" disabled={busy} onClick={() => runBulk('verify_email')}>
                            <ShieldCheck className="h-4 w-4" /> Verify email
                        </Button>
                        <Button size="sm" variant="secondary" disabled={busy} onClick={() => runBulk('unverify_email')}>
                            <ShieldOff className="h-4 w-4" /> Unverify
                        </Button>
                        <Select
                            value=""
                            aria-label="Set role for selected"
                            className="w-auto"
                            disabled={busy}
                            onChange={e => { if (e.target.value) runBulk('set_role', { role: e.target.value }) }}
                        >
                            <option value="">Set role…</option>
                            {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
                        </Select>
                        <Button size="sm" variant="danger" disabled={busy} onClick={() => runBulk('unban')}>
                            Unban
                        </Button>
                        <Button size="sm" variant="danger" disabled={busy} onClick={() => runBulk('ban')}>
                            <Ban className="h-4 w-4" /> Ban
                        </Button>
                        <Button size="sm" variant="ghost" onClick={() => setSelected(new Set())}>Cancel</Button>
                    </div>
                </div>
            )}

            <TableShell>
                <Table>
                    <THead>
                        <tr>
                            <Th className="w-10">
                                <input
                                    type="checkbox"
                                    checked={allVisibleSelected}
                                    onChange={toggleAll}
                                    aria-label="Select all users on this page"
                                />
                            </Th>
                            <Th>User</Th>
                            <Th>Role</Th>
                            <Th>Status</Th>
                            <Th className="text-right">Rating</Th>
                            <Th className="text-right">Solved</Th>
                            <Th>Last sign-in</Th>
                            <Th className="text-right">Actions</Th>
                        </tr>
                    </THead>
                    <tbody>
                        {loading && users.length === 0 && (
                            <Tr><Td className="py-8 text-center text-muted-soft" colSpan={8}>Loading…</Td></Tr>
                        )}
                        {!loading && rows.length === 0 && (
                            <Tr>
                                <Td className="py-8 text-center text-muted-soft" colSpan={8}>
                                    No users match these filters.
                                </Td>
                            </Tr>
                        )}
                        {rows.map(u => {
                            const banned = Boolean(u.banned_at)
                            return (
                                <Tr key={u.id} className={banned ? 'opacity-70' : ''}>
                                    <Td>
                                        <input
                                            type="checkbox"
                                            checked={selected.has(u.id)}
                                            onChange={() => toggleOne(u.id)}
                                            aria-label={`Select ${u.username}`}
                                        />
                                    </Td>
                                    <Td>
                                        <button
                                            onClick={() => openDetail(u.id)}
                                            className="text-left font-medium text-ink hover:text-brand hover:underline"
                                        >
                                            {u.username}
                                        </button>
                                        <div className="text-xs text-muted">{u.email || '—'}</div>
                                    </Td>
                                    <Td>
                                        <Select
                                            value={u.role}
                                            onChange={e => setRole(u, e.target.value)}
                                            aria-label={`Role for ${u.username}`}
                                            className="h-8 w-auto text-xs"
                                        >
                                            {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
                                        </Select>
                                    </Td>
                                    <Td>
                                        <div className="flex flex-wrap gap-1">
                                            {banned && <Badge tone="bad" dot>Banned</Badge>}
                                            {!u.email_verified && <Badge tone="warn">Unverified</Badge>}
                                            {u.must_change_password && <Badge tone="warn">Must change pw</Badge>}
                                            {u.is_bot && <Badge tone="neutral">Bot</Badge>}
                                            {!banned && u.email_verified && !u.must_change_password && !u.is_bot && (
                                                <Badge tone="ok">Active</Badge>
                                            )}
                                        </div>
                                    </Td>
                                    <Td className="tnum text-right">{u.rating}</Td>
                                    <Td className="tnum text-right">{u.problems_solved}</Td>
                                    <Td className="text-xs text-muted">{fmtRelative(u.last_login_at)}</Td>
                                    <Td>
                                        <div className="flex items-center justify-end gap-1">
                                            <Button
                                                size="sm"
                                                variant="ghost"
                                                onClick={() => openDetail(u.id)}
                                                title="View details"
                                                aria-label={`View ${u.username}`}
                                            >
                                                <UserCog className="h-4 w-4" />
                                            </Button>
                                            <Button
                                                size="sm"
                                                variant="ghost"
                                                onClick={() => toggleVerified(u)}
                                                title={u.email_verified ? 'Mark email unverified' : 'Mark email verified'}
                                                aria-label={u.email_verified ? 'Mark email unverified' : 'Mark email verified'}
                                            >
                                                {u.email_verified
                                                    ? <ShieldOff className="h-4 w-4" />
                                                    : <ShieldCheck className="h-4 w-4" />}
                                            </Button>
                                            <Button
                                                size="sm"
                                                variant="ghost"
                                                onClick={() => { setPwTarget(u); setNewPassword(''); setPwError('') }}
                                                title="Reset password"
                                                aria-label={`Reset password for ${u.username}`}
                                            >
                                                <RotateCcw className="h-4 w-4" />
                                            </Button>
                                            {banned ? (
                                                <Button size="sm" variant="ghost" onClick={() => unban(u)} title="Lift ban">
                                                    <CheckCircle2 className="h-4 w-4" />
                                                </Button>
                                            ) : (
                                                <Button
                                                    size="sm"
                                                    variant="ghost"
                                                    onClick={() => { setBanTarget(u); setBanReason('') }}
                                                    title="Suspend account"
                                                    aria-label={`Suspend ${u.username}`}
                                                >
                                                    <Ban className="h-4 w-4" />
                                                </Button>
                                            )}
                                        </div>
                                    </Td>
                                </Tr>
                            )
                        })}
                    </tbody>
                </Table>
            </TableShell>

            {/* Pagination */}
            <div className="mt-3 flex items-center justify-between text-sm text-muted">
                <span>Page {page} of {pageCount}</span>
                <div className="flex items-center gap-1">
                    <Button
                        size="sm"
                        variant="secondary"
                        disabled={page <= 1 || loading}
                        onClick={() => setQuery(prev => ({ ...prev, offset: Math.max(0, (prev.offset ?? 0) - (prev.limit ?? 25)) }))}
                    >
                        <ChevronLeft className="h-4 w-4" /> Previous
                    </Button>
                    <Button
                        size="sm"
                        variant="secondary"
                        disabled={page >= pageCount || loading}
                        onClick={() => setQuery(prev => ({ ...prev, offset: (prev.offset ?? 0) + (prev.limit ?? 25) }))}
                    >
                        Next <ChevronRight className="h-4 w-4" />
                    </Button>
                </div>
            </div>

            {/* Detail drawer */}
            {(detail || detailLoading) && (
                <div className="fixed inset-0 z-40 flex justify-end" role="presentation">
                    <div className="absolute inset-0 bg-black/40" onClick={() => setDetail(null)} role="presentation" />
                    <aside
                        role="dialog"
                        aria-modal="true"
                        aria-label="User details"
                        className="relative h-full w-full max-w-md overflow-y-auto border-l border-hairline bg-surface p-5 shadow-[var(--shadow-card)]"
                    >
                        <div className="mb-4 flex items-start justify-between">
                            <h3 className="text-base font-semibold text-ink">
                                {detailLoading ? 'Loading…' : detail?.username}
                            </h3>
                            <button onClick={() => setDetail(null)} className="rounded p-1 text-muted hover:bg-surface-2" aria-label="Close details">
                                <X className="h-4 w-4" />
                            </button>
                        </div>
                        {detail && (
                            <div className="space-y-4 text-sm">
                                <div className="flex flex-wrap gap-1">
                                    <Badge tone={ROLE_TONE[detail.role] ?? 'neutral'}>{detail.role}</Badge>
                                    {detail.banned_at && <Badge tone="bad" dot>Banned</Badge>}
                                    {!detail.email_verified && <Badge tone="warn">Unverified</Badge>}
                                    {detail.totp_enabled && <Badge tone="ok">2FA on</Badge>}
                                    {detail.is_bot && <Badge tone="neutral">Bot</Badge>}
                                </div>
                                {detail.banned_at && (
                                    <div className="rounded-lg border border-hairline bg-surface-2 p-3">
                                        <p className="text-xs font-medium text-muted">Suspended {fmtDate(detail.banned_at)}</p>
                                        <p className="mt-1 text-ink-soft">{detail.ban_reason || 'No reason recorded.'}</p>
                                    </div>
                                )}
                                <dl className="grid grid-cols-2 gap-3">
                                    {[
                                        ['Email', detail.email || '—'],
                                        ['Rating', `${detail.rating} (max ${detail.max_rating})`],
                                        ['Contests', String(detail.contest_count)],
                                        ['Problems solved', String(detail.problems_solved)],
                                        ['Submissions', String(detail.submissions)],
                                        ['Active sessions', String(detail.active_sessions)],
                                        ['Joined', fmtDate(detail.created_at)],
                                        ['Last sign-in', fmtRelative(detail.last_login_at)],
                                        ['Country', detail.country || '—'],
                                        ['Organisation', detail.organization || '—'],
                                    ].map(([k, v]) => (
                                        <div key={k}>
                                            <dt className="text-xs text-muted">{k}</dt>
                                            <dd className="text-ink-soft break-words">{v}</dd>
                                        </div>
                                    ))}
                                </dl>
                                {detail.github_url && (
                                    <a href={detail.github_url} target="_blank" rel="noopener noreferrer" className="text-brand hover:underline">
                                        {detail.github_url}
                                    </a>
                                )}
                                {detail.bio && <p className="text-ink-soft">{detail.bio}</p>}
                                <div className="flex flex-wrap gap-2 border-t border-hairline pt-4">
                                    <Button size="sm" variant="secondary" onClick={() => { setPwTarget(detail); setNewPassword(''); setPwError('') }}>
                                        <RotateCcw className="h-4 w-4" /> Reset password
                                    </Button>
                                    <Button size="sm" variant="secondary" onClick={() => toggleVerified(detail)}>
                                        {detail.email_verified
                                            ? <><ShieldOff className="h-4 w-4" /> Unverify email</>
                                            : <><ShieldCheck className="h-4 w-4" /> Verify email</>}
                                    </Button>
                                    {detail.banned_at ? (
                                        <Button size="sm" variant="secondary" onClick={() => unban(detail)}>
                                            <CheckCircle2 className="h-4 w-4" /> Lift ban
                                        </Button>
                                    ) : (
                                        <Button size="sm" variant="danger" onClick={() => { setBanTarget(detail); setBanReason('') }}>
                                            <Ban className="h-4 w-4" /> Suspend
                                        </Button>
                                    )}
                                </div>
                            </div>
                        )}
                    </aside>
                </div>
            )}

            {/* Ban dialog */}
            {banTarget && (
                <Modal
                    title={`Suspend ${banTarget.username}`}
                    description="The account cannot sign in and its sessions are revoked. Lift the ban from this panel."
                    onClose={() => setBanTarget(null)}
                    footer={
                        <>
                            <Button variant="secondary" onClick={() => setBanTarget(null)}>Cancel</Button>
                            <Button variant="danger" onClick={submitBan} disabled={busy}>
                                {busy ? 'Suspending…' : 'Suspend account'}
                            </Button>
                        </>
                    }
                >
                    <Field label="Reason" htmlFor="ban-reason" hint="Shown to administrators in the user drawer.">
                        <Input
                            id="ban-reason"
                            value={banReason}
                            onChange={e => setBanReason(e.target.value)}
                            placeholder="e.g. vote manipulation in contest #42"
                            autoFocus
                        />
                    </Field>
                </Modal>
            )}

            {/* Password reset dialog */}
            {pwTarget && (
                <Modal
                    title={`Reset password for ${pwTarget.username}`}
                    description="Every session is signed out. The user must choose a new password at next sign-in."
                    onClose={() => setPwTarget(null)}
                    footer={
                        <>
                            <Button variant="secondary" onClick={() => setPwTarget(null)}>Cancel</Button>
                            <Button onClick={submitPassword} disabled={busy}>
                                {busy ? 'Saving…' : 'Set password'}
                            </Button>
                        </>
                    }
                >
                    <Field
                        label="New password"
                        htmlFor="new-password"
                        error={pwError || undefined}
                        hint="At least 12 characters with upper case, lower case, a digit, and a symbol."
                    >
                        <Input
                            id="new-password"
                            type="password"
                            value={newPassword}
                            onChange={e => setNewPassword(e.target.value)}
                            autoComplete="new-password"
                            autoFocus
                        />
                    </Field>
                </Modal>
            )}
        </div>
    )
}
