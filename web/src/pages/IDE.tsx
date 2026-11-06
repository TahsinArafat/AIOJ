import { useState, useEffect, useRef } from 'react'
import { Link } from 'react-router-dom'
import CodeEditor from '../components/CodeEditor'
import { api, getAccessToken } from '../lib/api'
import { DEFAULT_LANGUAGE, LANGUAGE_OPTIONS, languageTemplate } from '../lib/languages'
import { VerdictBadge } from '../components/ui/VerdictBadge'
import { Play, Clock, Cpu, AlertTriangle, Maximize2, Minimize2 } from 'lucide-react'

/**
 * The verdict to show for a run.
 *
 * `passed` is authoritative when the request supplied expected output; without
 * it, fall back to the raw status. Anything that ran cleanly with no expected
 * output is reported as "Executed" rather than a pass/fail verdict.
 */
function runVerdict(o: Record<string, unknown>): string {
  if (o.passed === false) return 'wa'
  if (o.passed === true) return 'ac'
  const status = String(o.status ?? '')
  return status || 'executed'
}

const IDE_TIME_LIMIT_MS = 5000
const IDE_MEMORY_LIMIT_KB = 524288
const COOLDOWN_MS = 5000
const MAX_OUTPUT_LENGTH = 100000

export default function IDE() {
    const [language, setLanguage] = useState(() => localStorage.getItem('aioj_ide_lang') || DEFAULT_LANGUAGE)
    const [code, setCode] = useState('')
    const [customInput, setCustomInput] = useState('')
    const [output, setOutput] = useState<Record<string, unknown> | null>(null)
    const [running, setRunning] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [lastRunTime, setLastRunTime] = useState(0)
    const [cooldownRemaining, setCooldownRemaining] = useState(0)
    const [isFullscreen, setIsFullscreen] = useState(false)
    const [consoleOpen, setConsoleOpen] = useState(false)
    const [consoleTab, setConsoleTab] = useState<'custom' | 'result'>('result')
    const cooldownRef = useRef<ReturnType<typeof setInterval> | null>(null)

    const isLoggedIn = !!getAccessToken()

    useEffect(() => {
        return () => { if (cooldownRef.current) clearInterval(cooldownRef.current) }
    }, [])

    useEffect(() => {
        const handleKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape' && isFullscreen) setIsFullscreen(false)
        }
        window.addEventListener('keydown', handleKey)
        return () => window.removeEventListener('keydown', handleKey)
    }, [isFullscreen])

    const truncate = (s: string): string => {
        if (!s) return ''
        if (s.length <= MAX_OUTPUT_LENGTH) return s
        return s.slice(0, MAX_OUTPUT_LENGTH) + `\n\n... [truncated ${s.length - MAX_OUTPUT_LENGTH} more chars]`
    }

    useEffect(() => {
        const saved = localStorage.getItem(`aioj_ide_code_${language}`)
        // deferred a microtask: draft lands pre-paint (react-hooks/set-state-in-effect)
        queueMicrotask(() => setCode(saved || languageTemplate(language)))
    }, [language])

    const handleCodeChange = (newCode: string) => {
        setCode(newCode)
        localStorage.setItem(`aioj_ide_code_${language}`, newCode)
    }

    const handleLanguageChange = (lang: string) => {
        localStorage.setItem('aioj_ide_lang', lang)
        setLanguage(lang)
    }

    const openCustomTest = () => {
        setConsoleOpen(true)
        setConsoleTab('custom')
    }

    const handleRun = async () => {
        if (!code.trim()) return
        const now = Date.now()
        const elapsed = now - lastRunTime
        if (elapsed < COOLDOWN_MS) {
            const remaining = Math.ceil((COOLDOWN_MS - elapsed) / 1000)
            setError(`Please wait ${remaining}s before running again`)
            return
        }
        setRunning(true)
        setError(null)
        setOutput(null)
        setLastRunTime(now)
        setConsoleOpen(true)
        setConsoleTab('result')
        if (cooldownRef.current) clearInterval(cooldownRef.current)
        setCooldownRemaining(COOLDOWN_MS)
        cooldownRef.current = setInterval(() => {
            setCooldownRemaining(prev => {
                if (prev <= 100) { if (cooldownRef.current) clearInterval(cooldownRef.current); return 0 }
                return prev - 100
            })
        }, 100)
        try {
            const res = await api.submissions.run({
                source_code: code,
                language,
                input: customInput,
                time_limit_ms: IDE_TIME_LIMIT_MS,
                memory_limit_kb: IDE_MEMORY_LIMIT_KB,
            })
            setOutput({
                ...res,
                stdout: truncate(res.stdout as string),
                stderr: truncate(res.stderr as string),
                compile_output: truncate(res.compile_output as string),
            })
        } catch (e: unknown) {
            setError(e instanceof Error ? e.message : 'Run failed')
        } finally {
            setRunning(false)
        }
    }

    if (!isLoggedIn) {
        return (
            <div className="flex items-center justify-center h-full">
                <div className="text-center space-y-4 max-w-md">
                    <AlertTriangle className="w-12 h-12 mx-auto text-yellow-500" />
                    <h2 className="text-xl font-bold text-ink">Login Required</h2>
                    <p className="text-muted">You must be logged in to use the IDE.</p>
                    <Link to="/login" className="inline-block bg-brand text-on-brand px-6 py-2 rounded-lg font-medium hover:bg-brand-hover transition-colors">
                        Log In
                    </Link>
                </div>
            </div>
        )
    }

    const content = (
        <>
            {/* Header */}
            <div className="flex items-center justify-between px-4 md:px-6 py-3 md:py-4 border-b border-hairline bg-surface flex-shrink-0">
                <div className="flex items-center gap-2 md:gap-4 min-w-0">
                    <h1 className="text-lg md:text-xl font-bold text-ink whitespace-nowrap">IDE</h1>
                    <span className="text-xs text-muted-soft hidden sm:inline">|</span>
                    <span className="text-xs text-muted-soft hidden sm:inline whitespace-nowrap">
                        Limits: {IDE_TIME_LIMIT_MS / 1000}s / {IDE_MEMORY_LIMIT_KB / 1024}MB
                    </span>
                    {cooldownRemaining > 0 && (
                        <span className="text-xs text-warn whitespace-nowrap">
                            Cooldown: {Math.ceil(cooldownRemaining / 1000)}s
                        </span>
                    )}
                </div>
                <button
                    onClick={() => setIsFullscreen(!isFullscreen)}
                    className="flex h-10 w-10 items-center justify-center rounded-lg text-muted transition-colors hover:bg-surface-2 hover:text-ink cursor-pointer"
                    aria-label={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
                    title={isFullscreen ? 'Exit fullscreen (Esc)' : 'Fullscreen'}
                >
                    {isFullscreen ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
                </button>
            </div>

            {/* Toolbar */}
            <div className="flex items-center gap-3 px-4 md:px-6 py-2 md:py-3 bg-surface-2 border-b border-hairline flex-shrink-0">
                <select
                    value={language}
                    onChange={e => handleLanguageChange(e.target.value)}
                    className="h-9 rounded-lg border border-hairline-strong bg-surface px-2 text-sm text-ink-soft transition-colors hover:border-muted-soft focus:border-brand focus:outline-none min-w-0 max-w-[50vw] md:max-w-none"
                >
                    {LANGUAGE_OPTIONS.map(l => <option key={l.value} value={l.value}>{l.label}</option>)}
                </select>
                <div className="flex-1" />
            </div>

            {/* Main content area: editor + drawer + footer */}
            <div className="relative flex flex-col flex-1 min-h-0 overflow-hidden">
                {/* Code Editor — takes remaining space */}
                <div className="flex-1 min-h-[180px] min-w-0 h-full">
                    <CodeEditor
                        language={language}
                        value={code}
                        onChange={handleCodeChange}
                        height="100%"
                    />
                </div>

                {/* Console Drawer — slides up from bottom, above the 56px footer */}
                <div
                    className={`absolute bottom-14 left-0 right-0 bg-surface-2 border-t border-hairline transition-[height] duration-200 ease-in-out overflow-hidden z-20 shadow-[0_-4px_12px_rgba(0,0,0,0.15)] dark:shadow-[0_-4px_12px_rgba(0,0,0,0.4)] flex flex-col ${consoleOpen
                        ? 'h-[200px] md:h-[320px]'
                        : 'h-0'
                        }`}
                >
                    {/* Drawer Header: tabs + close */}
                    <div className="flex items-center justify-between bg-surface-3 border-b border-hairline px-4 flex-shrink-0">
                        <div className="flex gap-4">
                            <button
                                onClick={() => setConsoleTab('custom')}
                                className={`px-0 py-2.5 text-[11px] font-semibold uppercase tracking-wider border-b-2 transition-colors cursor-pointer ${consoleTab === 'custom'
                                    ? 'text-ink dark:text-white border-blue-500'
                                    : 'text-muted border-transparent hover:text-ink'
                                    }`}
                            >
                                Custom Test
                            </button>
                            <button
                                onClick={() => setConsoleTab('result')}
                                className={`px-0 py-2.5 text-[11px] font-semibold uppercase tracking-wider border-b-2 transition-colors cursor-pointer ${consoleTab === 'result'
                                    ? 'text-ink dark:text-white border-blue-500'
                                    : 'text-muted border-transparent hover:text-ink'
                                    }`}
                            >
                                Result
                            </button>
                        </div>
                        <button
                            onClick={() => setConsoleOpen(false)}
                            aria-label="Close console"
                            className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors hover:bg-surface-2 hover:text-ink cursor-pointer"
                            title="Close console"
                        >
                            <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <line x1="18" y1="6" x2="6" y2="18" />
                                <line x1="6" y1="6" x2="18" y2="18" />
                            </svg>
                        </button>
                    </div>

                    {/* Drawer Body */}
                    <div className="flex-1 min-h-0 relative">
                        {/* Custom Test pane */}
                        <div className={`absolute inset-0 p-3 md:p-4 overflow-y-auto ${consoleTab === 'custom' ? 'block' : 'hidden'}`}>
                            <textarea
                                value={customInput}
                                onChange={e => setCustomInput(e.target.value)}
                                placeholder="Paste custom input here..."
                                className="w-full h-full resize-none p-3 text-sm font-mono border border-hairline-strong rounded bg-surface text-ink focus:outline-none focus:ring-2 focus-visible:outline-[var(--ring)]"
                            />
                        </div>

                        {/* Result pane */}
                        <div className={`absolute inset-0 p-3 md:p-4 overflow-y-auto ${consoleTab === 'result' ? 'block' : 'hidden'}`}>
                            {error && (
                                <div className="p-3 bg-bad/10 border border-bad/30 rounded-lg text-sm text-bad mb-3">
                                    {error}
                                </div>
                            )}

                            {output ? (
                                (() => {
                                    const isError = output.status === 'CE' || output.status === 'RE' || output.status === 'TLE' || output.status === 'MLE' || output.status === 'SE'
                                    const errorText = (output.compile_output as string) || (output.stderr as string)
                                    return (
                                        <div className="space-y-3">
                                            {/* Stats */}
                                            <div className="flex flex-wrap items-center gap-2 md:gap-4 text-xs text-muted bg-surface-3 rounded p-2">
                                                <span className="flex items-center gap-1">
                                                    <Clock className="w-3.5 h-3.5" />
                                                    {typeof output.time_used === 'number' && output.time_used > 0 ? `${output.time_used}ms` : '—'}
                                                    {output.status === 'TLE' && <span className="text-bad">(limit {IDE_TIME_LIMIT_MS / 1000}s)</span>}
                                                </span>
                                                <span className="flex items-center gap-1">
                                                    <Cpu className="w-3.5 h-3.5" />
                                                    {typeof output.memory_used === 'number' && output.memory_used > 0 ? `${Math.round(output.memory_used / 1024)}MB` : '—'}
                                                    {output.status === 'MLE' && <span className="text-bad">(limit {IDE_MEMORY_LIMIT_KB / 1024}MB)</span>}
                                                </span>
                                                {/* This pill used to know only CE/RE/TLE/MLE/SE and fell
                                                    through to a green "Executed" for everything else — so a
                                                    wrong answer was reported as a success. `passed` from the
                                                    run endpoint is the authoritative signal when expected
                                                    output was supplied. */}
                                                <span className="ml-auto">
                                                    <VerdictBadge status={runVerdict(output)} />
                                                </span>
                                            </div>

                                            {errorText && (
                                                <div>
                                                    <span className="block text-xs font-semibold text-bad mb-1">
                                                        {output.status === 'CE' ? 'Compilation Error' : 'Error Output'}
                                                    </span>
                                                    <pre className="bg-bad/10 text-bad text-xs p-3 rounded overflow-x-auto font-mono whitespace-pre-wrap">
                                                        {errorText}
                                                    </pre>
                                                </div>
                                            )}

                                            {(!!output.stdout || !isError) && (
                                                <div>
                                                    <span className="block text-xs font-semibold text-muted mb-1">stdout</span>
                                                    <pre className="bg-code text-code-ink text-sm p-3 rounded overflow-x-auto font-mono whitespace-pre-wrap">
                                                        {(output.stdout as string) || '(no output)'}
                                                    </pre>
                                                </div>
                                            )}

                                            {!!output.stderr && (output.stderr as string) !== errorText && (
                                                <div>
                                                    <span className="block text-xs font-semibold text-warn mb-1">stderr</span>
                                                    <pre className="bg-warn/10 text-warn text-xs p-3 rounded overflow-x-auto font-mono whitespace-pre-wrap">
                                                        {output.stderr as string}
                                                    </pre>
                                                </div>
                                            )}
                                        </div>
                                    )
                                })()
                            ) : (
                                <div className="text-center py-8 text-muted-soft text-sm">
                                    <Play className="w-6 h-6 mx-auto mb-2 opacity-50" />
                                    <p>Click <strong>Run</strong> to see results here</p>
                                </div>
                            )}
                        </div>
                    </div>
                </div>

                {/* Action Footer */}
                <div className="h-14 bg-surface-3 border-t border-hairline flex items-center justify-end gap-2 px-4 flex-shrink-0 z-30">
                    <div className="flex items-center gap-2">
                        <button
                            onClick={openCustomTest}
                            className="inline-flex h-10 items-center gap-2 rounded-lg border border-hairline-strong bg-surface-3 px-3 text-sm font-medium text-ink-soft transition-colors hover:bg-hairline cursor-pointer"
                        >
                            <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <path d="M12 20h9" />
                                <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
                            </svg>
                            Custom Test
                        </button>
                        <button
                            onClick={handleRun}
                            disabled={running || !code.trim()}
                            className="inline-flex h-10 items-center gap-2 whitespace-nowrap rounded-lg bg-brand px-3 text-sm font-medium text-on-brand transition-colors hover:bg-brand-hover disabled:opacity-50 cursor-pointer md:px-4"
                        >
                            <Play className="w-4 h-4" />
                            {running ? 'Running...' : 'Run'}
                        </button>
                    </div>
                </div>
            </div>
        </>
    )

    if (isFullscreen) {
        return (
            <div className="fixed inset-0 z-50 flex flex-col bg-canvas">
                {content}
            </div>
        )
    }

    return (
        <div className="flex-1 min-h-0 flex flex-col w-full overflow-hidden">
            {content}
        </div>
    )
}
