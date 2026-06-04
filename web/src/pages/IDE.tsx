import { useState, useEffect } from 'react'
import CodeEditor from '../components/CodeEditor'
import { api } from '../lib/api'
import { Play, Clock, Cpu } from 'lucide-react'

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
        setRunning(true)
        setError(null)
        setOutput(null)
        try {
            const res = await api.submissions.run({
                source_code: code,
                language,
                input: customInput,
            })
            setOutput(res)
        } catch (e: any) {
            setError(e.message || 'Run failed')
        } finally {
            setRunning(false)
        }
    }

    return (
        <div className="h-full flex flex-col">
            {/* Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900">
                <div className="flex items-center gap-4">
                    <h1 className="text-xl font-bold text-gray-900 dark:text-gray-100">IDE</h1>
                    <span className="text-xs text-gray-400 dark:text-gray-500 bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded">
                        Write, run, and test code instantly
                    </span>
                </div>
            </div>

            {/* Toolbar */}
            <div className="flex items-center gap-3 px-6 py-3 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
                <select
                    value={language}
                    onChange={e => handleLanguageChange(e.target.value)}
                    className="border border-gray-300 dark:border-gray-600 rounded px-2 py-1.5 text-sm bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                    {LANGS.map(l => <option key={l.value} value={l.value}>{l.label}</option>)}
                </select>
                <div className="flex-1" />
                <button
                    onClick={handleRun}
                    disabled={running || !code.trim()}
                    className="inline-flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
                >
                    <Play className="w-4 h-4" />
                    {running ? 'Running...' : 'Run'}
                </button>
            </div>

            {/* Main content: editor + output side by side */}
            <div className="flex flex-1 min-h-0">
                {/* Left: Code Editor */}
                <div className="flex-1 min-w-0 border-r border-gray-200 dark:border-gray-700">
                    <CodeEditor
                        language={language}
                        value={code}
                        onChange={handleCodeChange}
                        height="100%"
                    />
                </div>

                {/* Right: Input + Output */}
                <div className="w-[420px] flex-shrink-0 flex flex-col min-h-0">
                    {/* Custom Input */}
                    <div className="border-b border-gray-200 dark:border-gray-700 p-4">
                        <h3 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">Custom Input (stdin)</h3>
                        <textarea
                            value={customInput}
                            onChange={e => setCustomInput(e.target.value)}
                            placeholder="Enter input for your program..."
                            rows={5}
                            className="w-full border border-gray-300 dark:border-gray-600 rounded p-2 text-sm bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 font-mono focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                        />
                    </div>

                    {/* Output */}
                    <div className="flex-1 overflow-y-auto p-4">
                        <h3 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">Output</h3>

                        {error && (
                            <div className="p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg text-sm text-red-700 dark:text-red-300">
                                {error}
                            </div>
                        )}

                        {output ? (
                            <div className="space-y-3">
                                {/* Stats */}
                                <div className="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 rounded p-2">
                                    <span className="flex items-center gap-1">
                                        <Clock className="w-3.5 h-3.5" />
                                        {output.time_used > 0 ? `${output.time_used}ms` : '—'}
                                    </span>
                                    <span className="flex items-center gap-1">
                                        <Cpu className="w-3.5 h-3.5" />
                                        {output.memory_used > 0 ? `${Math.round(output.memory_used / 1024)}MB` : '—'}
                                    </span>
                                    <span className={`ml-auto font-medium px-2 py-0.5 rounded text-xs ${
                                        output.status === 'ac' || output.status === 'success' || !output.status
                                            ? 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20'
                                            : 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20'
                                    }`}>
                                        {output.status === 'ce' ? 'Compile Error' : 
                                         output.status === 're' ? 'Runtime Error' :
                                         output.status === 'tle' ? 'Time Limit Exceeded' :
                                         output.status === 'mle' ? 'Memory Limit Exceeded' :
                                         'Executed'}
                                    </span>
                                </div>

                                {/* Compile Output */}
                                {output.compile_output && (
                                    <div>
                                        <span className="block text-xs font-semibold text-red-600 dark:text-red-400 mb-1">Compile Output</span>
                                        <pre className="bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 text-xs p-3 rounded overflow-x-auto font-mono max-h-40">
                                            {output.compile_output}
                                        </pre>
                                    </div>
                                )}

                                {/* Stdout */}
                                <div>
                                    <span className="block text-xs font-semibold text-gray-500 dark:text-gray-400 mb-1">stdout</span>
                                    <pre className="bg-gray-900 text-green-400 text-sm p-3 rounded overflow-x-auto font-mono max-h-60 whitespace-pre-wrap">
                                        {output.stdout || '(no output)'}
                                    </pre>
                                </div>

                                {/* Stderr */}
                                {output.stderr && (
                                    <div>
                                        <span className="block text-xs font-semibold text-yellow-600 dark:text-yellow-400 mb-1">stderr</span>
                                        <pre className="bg-yellow-50 dark:bg-yellow-900/20 text-yellow-700 dark:text-yellow-300 text-xs p-3 rounded overflow-x-auto font-mono max-h-40">
                                            {output.stderr}
                                        </pre>
                                    </div>
                                )}
                            </div>
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
        </div>
    )
}
