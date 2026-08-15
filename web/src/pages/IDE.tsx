import { useState, useEffect, useRef } from 'react'
import { Link } from 'react-router-dom'
import CodeEditor from '../components/CodeEditor'
import { api, getAccessToken } from '../lib/api'
import { Play, Clock, Cpu, AlertTriangle, Maximize2, Minimize2 } from 'lucide-react'

const IDE_TIME_LIMIT_MS = 5000
const IDE_MEMORY_LIMIT_KB = 524288
const COOLDOWN_MS = 5000
const MAX_OUTPUT_LENGTH = 100000

const LANGS = [
    { value: 'cpp-gpp-64', label: 'GNU G++17 7.3.0 (64 bit)' },
    { value: 'cpp-gpp-32', label: 'GNU G++14 6.4.0 (32 bit)' },
    { value: 'c-gcc-64', label: 'GNU GCC C11 9.2.0 (64 bit)' },
    { value: 'python', label: 'Python 3.8.10' },
    { value: 'java', label: 'Java 11.0.6' },
    { value: 'rust', label: 'Rust 1.75.0' },
    { value: 'nodejs', label: 'Node.js 18.16.1' },
    { value: 'csharp', label: 'Mono C# 6.12.0' },
]

const TEMPLATE_CODE: Record<string, string> = {
    'cpp-gpp-64': `#include <iostream>
using namespace std;

int main() {
    // Your code here
    
    return 0;
}`,
    'cpp-gpp-32': `#include <iostream>
using namespace std;

int main() {
    // Your code here
    
    return 0;
}`,
    'c-gcc-64': `#include <stdio.h>

int main() {
    // Your code here
    
    return 0;
}`,
    'python': `# Your code here
`,
    'java': `import java.util.Scanner;

public class Main {
    public static void main(String[] args) {
        Scanner sc = new Scanner(System.in);
        // Your code here
        
    }
}`,
    'rust': `fn main() {
    // Your code here
    
}`,
    'nodejs': `// Your code here
`,
    'csharp': `using System;

class Program {
    static void Main() {
        // Your code here
        
    }
}`,
}

export default function IDE() {
    const [language, setLanguage] = useState(() => localStorage.getItem('aioj_ide_lang') || 'cpp-gpp-64')
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
        setCode(saved || TEMPLATE_CODE[language] || '')
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
                    <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100">Login Required</h2>
                    <p className="text-gray-500 dark:text-gray-400">You must be logged in to use the IDE.</p>
                    <Link to="/login" className="inline-block bg-blue-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-blue-700 transition-colors">
                        Log In
                    </Link>
                </div>
            </div>
        )
    }

    const content = (
        <>
            {/* Header */}
            <div className="flex items-center justify-between px-4 md:px-6 py-3 md:py-4 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 flex-shrink-0">
                <div className="flex items-center gap-2 md:gap-4 min-w-0">
                    <h1 className="text-lg md:text-xl font-bold text-gray-900 dark:text-gray-100 whitespace-nowrap">IDE</h1>
                    <span className="text-xs text-gray-400 dark:text-gray-500 hidden sm:inline">|</span>
                    <span className="text-xs text-gray-400 dark:text-gray-500 hidden sm:inline whitespace-nowrap">
                        Limits: {IDE_TIME_LIMIT_MS / 1000}s / {IDE_MEMORY_LIMIT_KB / 1024}MB
                    </span>
                    {cooldownRemaining > 0 && (
                        <span className="text-xs text-yellow-600 dark:text-yellow-400 whitespace-nowrap">
                            Cooldown: {Math.ceil(cooldownRemaining / 1000)}s
                        </span>
                    )}
                </div>
                <button
                    onClick={() => setIsFullscreen(!isFullscreen)}
                    className="p-2 rounded hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-gray-500 dark:text-gray-400 cursor-pointer"
                    title={isFullscreen ? 'Exit fullscreen (Esc)' : 'Fullscreen'}
                >
                    {isFullscreen ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
                </button>
            </div>

            {/* Toolbar */}
            <div className="flex items-center gap-3 px-4 md:px-6 py-2 md:py-3 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
                <select
                    value={language}
                    onChange={e => handleLanguageChange(e.target.value)}
                    className="border border-gray-300 dark:border-gray-600 rounded px-2 py-1.5 text-sm bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 min-w-0 max-w-[50vw] md:max-w-none"
                >
                    {LANGS.map(l => <option key={l.value} value={l.value}>{l.label}</option>)}
                </select>
                <div className="flex-1" />
            </div>

            {/* Main content area: editor + drawer + footer */}
            <div className="relative flex flex-col flex-1 min-h-0">
                {/* Code Editor — takes remaining space */}
                <div className="flex-1 min-h-0 min-w-0">
                    <CodeEditor
                        language={language}
                        value={code}
                        onChange={handleCodeChange}
                        height="100%"
                    />
                </div>

                {/* Console Drawer — slides up from bottom, above the 56px footer */}
                <div
                    className={`absolute bottom-14 left-0 right-0 bg-gray-100 dark:bg-gray-950 border-t border-gray-200 dark:border-gray-700 transition-[height] duration-200 ease-in-out overflow-hidden z-20 shadow-[0_-4px_12px_rgba(0,0,0,0.15)] dark:shadow-[0_-4px_12px_rgba(0,0,0,0.4)] flex flex-col ${
                        consoleOpen
                            ? 'h-[200px] md:h-[320px]'
                            : 'h-0'
                    }`}
                >
                    {/* Drawer Header: tabs + close */}
                    <div className="flex items-center justify-between bg-gray-200 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 flex-shrink-0">
                        <div className="flex gap-4">
                            <button
                                onClick={() => setConsoleTab('custom')}
                                className={`px-0 py-2.5 text-[11px] font-semibold uppercase tracking-wider border-b-2 transition-colors cursor-pointer ${
                                    consoleTab === 'custom'
                                        ? 'text-gray-900 dark:text-white border-blue-500'
                                        : 'text-gray-500 dark:text-gray-400 border-transparent hover:text-gray-700 dark:hover:text-gray-300'
                                }`}
                            >
                                Custom Test
                            </button>
                            <button
                                onClick={() => setConsoleTab('result')}
                                className={`px-0 py-2.5 text-[11px] font-semibold uppercase tracking-wider border-b-2 transition-colors cursor-pointer ${
                                    consoleTab === 'result'
                                        ? 'text-gray-900 dark:text-white border-blue-500'
                                        : 'text-gray-500 dark:text-gray-400 border-transparent hover:text-gray-700 dark:hover:text-gray-300'
                                }`}
                            >
                                Result
                            </button>
                        </div>
                        <button
                            onClick={() => setConsoleOpen(false)}
                            className="text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white p-2 cursor-pointer transition-colors"
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
                                className="w-full h-full resize-none p-3 text-sm font-mono border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
                            />
                        </div>

                        {/* Result pane */}
                        <div className={`absolute inset-0 p-3 md:p-4 overflow-y-auto ${consoleTab === 'result' ? 'block' : 'hidden'}`}>
                            {error && (
                                <div className="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg text-sm text-red-700 dark:text-red-300 mb-3">
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
                                            <div className="flex flex-wrap items-center gap-2 md:gap-4 text-xs text-gray-500 dark:text-gray-400 bg-gray-200 dark:bg-gray-800 rounded p-2">
                                                <span className="flex items-center gap-1">
                                                    <Clock className="w-3.5 h-3.5" />
                                                    {typeof output.time_used === 'number' && output.time_used > 0 ? `${output.time_used}ms` : '—'}
                                                    {output.status === 'TLE' && <span className="text-red-500">(limit {IDE_TIME_LIMIT_MS / 1000}s)</span>}
                                                </span>
                                                <span className="flex items-center gap-1">
                                                    <Cpu className="w-3.5 h-3.5" />
                                                    {typeof output.memory_used === 'number' && output.memory_used > 0 ? `${Math.round(output.memory_used / 1024)}MB` : '—'}
                                                    {output.status === 'MLE' && <span className="text-red-500">(limit {IDE_MEMORY_LIMIT_KB / 1024}MB)</span>}
                                                </span>
                                                <span className={`ml-auto font-medium px-2 py-0.5 rounded text-xs ${
                                                    isError
                                                        ? 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20'
                                                        : 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20'
                                                }`}>
                                                    {output.status === 'CE' ? 'Compile Error' :
                                                     output.status === 'RE' ? 'Runtime Error' :
                                                     output.status === 'TLE' ? 'Time Limit Exceeded' :
                                                     output.status === 'MLE' ? 'Memory Limit Exceeded' :
                                                     output.status === 'SE' ? 'System Error' :
                                                     'Executed'}
                                                </span>
                                            </div>

                                            {errorText && (
                                                <div>
                                                    <span className="block text-xs font-semibold text-red-600 dark:text-red-400 mb-1">
                                                        {output.status === 'CE' ? 'Compilation Error' : 'Error Output'}
                                                    </span>
                                                    <pre className="bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 text-xs p-3 rounded overflow-x-auto font-mono whitespace-pre-wrap">
                                                        {errorText}
                                                    </pre>
                                                </div>
                                            )}

                                            {(!!output.stdout || !isError) && (
                                                <div>
                                                    <span className="block text-xs font-semibold text-gray-500 dark:text-gray-400 mb-1">stdout</span>
                                                    <pre className="bg-gray-900 text-green-400 text-sm p-3 rounded overflow-x-auto font-mono whitespace-pre-wrap">
                                                        {(output.stdout as string) || '(no output)'}
                                                    </pre>
                                                </div>
                                            )}

                                            {!!output.stderr && (output.stderr as string) !== errorText && (
                                                <div>
                                                    <span className="block text-xs font-semibold text-yellow-600 dark:text-yellow-400 mb-1">stderr</span>
                                                    <pre className="bg-yellow-50 dark:bg-yellow-900/20 text-yellow-700 dark:text-yellow-300 text-xs p-3 rounded overflow-x-auto font-mono whitespace-pre-wrap">
                                                        {output.stderr as string}
                                                    </pre>
                                                </div>
                                            )}
                                        </div>
                                    )
                                })()
                            ) : (
                                <div className="text-center py-8 text-gray-400 dark:text-gray-500 text-sm">
                                    <Play className="w-6 h-6 mx-auto mb-2 opacity-50" />
                                    <p>Click <strong>Run</strong> to see results here</p>
                                </div>
                            )}
                        </div>
                    </div>
                </div>

                {/* Action Footer */}
                <div className="h-14 bg-gray-100 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between px-4 flex-shrink-0 z-30">
                    <button
                        onClick={() => setConsoleOpen(prev => !prev)}
                        className={`inline-flex items-center gap-2 px-3 py-1.5 rounded text-sm font-medium transition-colors cursor-pointer border ${
                            consoleOpen
                                ? 'bg-gray-200 dark:bg-gray-700 text-gray-900 dark:text-white border-gray-300 dark:border-gray-600'
                                : 'bg-gray-200 dark:bg-gray-800 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-300 dark:hover:bg-gray-700'
                        }`}
                    >
                        <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <polyline points="4 17 10 11 4 5" />
                            <line x1="12" y1="19" x2="20" y2="19" />
                        </svg>
                        Console
                    </button>
                    <div className="flex items-center gap-2">
                        <button
                            onClick={openCustomTest}
                            className="inline-flex items-center gap-2 px-3 py-1.5 rounded text-sm font-medium bg-gray-200 dark:bg-gray-800 text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 hover:bg-gray-300 dark:hover:bg-gray-700 transition-colors cursor-pointer"
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
                            className="inline-flex items-center gap-2 bg-blue-600 text-white px-3 md:px-4 py-1.5 md:py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer whitespace-nowrap"
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
            <div className="fixed inset-0 z-50 flex flex-col bg-white dark:bg-gray-950">
                {content}
            </div>
        )
    }

    return (
        <div className="h-full flex flex-col">
            {content}
        </div>
    )
}
