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
    const [output, setOutput] = useState<any>(null)
    const [running, setRunning] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [lastRunTime, setLastRunTime] = useState(0)
    const [cooldownRemaining, setCooldownRemaining] = useState(0)
    const [isFullscreen, setIsFullscreen] = useState(false)
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
                stdout: truncate(res.stdout),
                stderr: truncate(res.stderr),
                compile_output: truncate(res.compile_output),
            })
        } catch (e: any) {
            setError(e.message || 'Run failed')
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
            <div className="flex items-center gap-3 px-4 md:px-6 py-2 md:py-3 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
                <select
                    value={language}
                    onChange={e => handleLanguageChange(e.target.value)}
                    className="border border-gray-300 dark:border-gray-600 rounded px-2 py-1.5 text-sm bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 min-w-0 max-w-[50vw] md:max-w-none"
                >
                    {LANGS.map(l => <option key={l.value} value={l.value}>{l.label}</option>)}
                </select>
                <div className="flex-1" />
                <button
                    onClick={handleRun}
                    disabled={running || !code.trim()}
                    className="inline-flex items-center gap-2 bg-blue-600 text-white px-3 md:px-4 py-1.5 md:py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer whitespace-nowrap"
                >
                    <Play className="w-4 h-4" />
                    {running ? 'Running...' : 'Run'}
                </button>
            </div>

            {/* Main content: editor + output side by side (stacked on mobile) */}
            <div className="flex flex-col md:flex-row flex-1 min-h-0 overflow-auto">
                {/* Left: Code Editor */}
                <div className="flex-1 min-w-0 border-b md:border-b-0 md:border-r border-gray-200 dark:border-gray-700 min-h-[40vh] md:min-h-0">
                    <CodeEditor
                        language={language}
                        value={code}
                        onChange={handleCodeChange}
                        height="100%"
                    />
                </div>

                {/* Right: Input + Output */}
                <div className="w-full md:w-[420px] md:flex-shrink-0 flex flex-col min-h-0">
                    {/* Custom Input */}
                    <div className="border-b border-gray-200 dark:border-gray-700 p-3 md:p-4">
                        <h3 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">Custom Input (stdin)</h3>
                        <textarea
                            value={customInput}
                            onChange={e => setCustomInput(e.target.value)}
                            placeholder="Enter input for your program..."
                            rows={4}
                            className="w-full border border-gray-300 dark:border-gray-600 rounded p-2 text-sm bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 font-mono focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                        />
                    </div>

                    {/* Output */}
                    <div className="flex-1 overflow-y-auto p-3 md:p-4">
                        <h3 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">Output</h3>

                        {error && (
                            <div className="p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg text-sm text-red-700 dark:text-red-300">
                                {error}
                            </div>
                        )}

                        {output ? (
                            (() => {
                                const isError = output.status === 'CE' || output.status === 'RE' || output.status === 'TLE' || output.status === 'MLE' || output.status === 'SE'
                                const errorText = output.compile_output || output.stderr
                                return (
                                    <div className="space-y-3">
                                        {/* Stats */}
                                        <div className="flex flex-wrap items-center gap-2 md:gap-4 text-xs text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 rounded p-2">
                                            <span className="flex items-center gap-1">
                                                <Clock className="w-3.5 h-3.5" />
                                                {output.time_used > 0 ? `${output.time_used}ms` : '—'}
                                                {output.status === 'TLE' && <span className="text-red-500">(limit {IDE_TIME_LIMIT_MS / 1000}s)</span>}
                                            </span>
                                            <span className="flex items-center gap-1">
                                                <Cpu className="w-3.5 h-3.5" />
                                                {output.memory_used > 0 ? `${Math.round(output.memory_used / 1024)}MB` : '—'}
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
                                                <pre className="bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 text-xs p-3 rounded overflow-x-auto font-mono max-h-60 whitespace-pre-wrap">
                                                    {errorText}
                                                </pre>
                                            </div>
                                        )}

                                        {(output.stdout || !isError) && (
                                            <div>
                                                <span className="block text-xs font-semibold text-gray-500 dark:text-gray-400 mb-1">stdout</span>
                                                <pre className="bg-gray-900 text-green-400 text-sm p-3 rounded overflow-x-auto font-mono max-h-60 whitespace-pre-wrap">
                                                    {output.stdout || '(no output)'}
                                                </pre>
                                            </div>
                                        )}

                                        {output.stderr && output.stderr !== errorText && (
                                            <div>
                                                <span className="block text-xs font-semibold text-yellow-600 dark:text-yellow-400 mb-1">stderr</span>
                                                <pre className="bg-yellow-50 dark:bg-yellow-900/20 text-yellow-700 dark:text-yellow-300 text-xs p-3 rounded overflow-x-auto font-mono max-h-60 whitespace-pre-wrap">
                                                    {output.stderr}
                                                </pre>
                                            </div>
                                        )}
                                    </div>
                                )
                            })()
                        ) : (
                            <div className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">
                                <Play className="w-8 h-8 mx-auto mb-2 opacity-50" />
                                <p>Click <strong>Run</strong> to execute your code</p>
                                <p className="mt-1 text-xs">Output will appear here</p>
                            </div>
                        )}
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
