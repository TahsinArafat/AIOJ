# Monaco Editor Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the CodeMirror-based CodeEditor component with a fully customizable Monaco Editor wrapper supporting custom font size, theme override, tab size, line wrapping, and minimap toggle, persisted globally via localStorage.

**Architecture:** Use `@monaco-editor/react` as the core editor component wrapper. Maintain identical interface props so no adjustments are needed in calling code.

**Tech Stack:** React, TypeScript, Tailwind CSS, Lucide React (already installed for icons), `@monaco-editor/react`.

---

### Task 1: Package Dependencies Cleanup and Installation

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install `@monaco-editor/react` and remove CodeMirror dependencies**

Run:
```bash
npm install @monaco-editor/react
```
And check `web/package.json` to verify dependencies.

- [ ] **Step 2: Commit changes**

Run:
```bash
git add web/package.json web/package-lock.json
git commit -m "chore: install @monaco-editor/react"
```

---

### Task 2: CodeEditor Component Implementation

**Files:**
- Modify: `web/src/components/CodeEditor.tsx`

- [ ] **Step 1: Rewrite CodeEditor.tsx with the customizable Monaco Editor implementation**

Implement:
```typescript
import { useState } from 'react'
import MonacoEditor from '@monaco-editor/react'
import { Settings, Eye, EyeOff, WrapText } from 'lucide-react'
import { useTheme } from '../context/ThemeContext'

interface CodeEditorProps {
    language: string
    value: string
    onChange: (code: string) => void
    height?: string
    readOnly?: boolean
}

interface EditorSettings {
    themeOverride: 'system' | 'vs-dark' | 'vs'
    fontSize: number
    tabSize: 2 | 4 | 8
    wordWrap: 'on' | 'off'
    minimap: boolean
}

const DEFAULT_SETTINGS: EditorSettings = {
    themeOverride: 'system',
    fontSize: 14,
    tabSize: 4,
    wordWrap: 'off',
    minimap: false
}

function getMonacoLanguage(lang: string): string {
    if (lang.startsWith('cpp') || lang.startsWith('c-')) return 'cpp'
    if (lang === 'python' || lang === 'pypy') return 'python'
    if (lang === 'java') return 'java'
    if (lang === 'rust') return 'rust'
    if (lang === 'nodejs') return 'javascript'
    if (lang === 'csharp') return 'csharp'
    return 'cpp'
}

export default function CodeEditor({
    language,
    value,
    onChange,
    height = '400px',
    readOnly = false
}: CodeEditorProps) {
    const { theme } = useTheme()
    const [settings, setSettings] = useState<EditorSettings>(() => {
        try {
            const saved = localStorage.getItem('aioj_editor_settings')
            return saved ? JSON.parse(saved) : DEFAULT_SETTINGS
        } catch {
            return DEFAULT_SETTINGS
        }
    })
    const [showSettings, setShowSettings] = useState(false)

    const updateSetting = <K extends keyof EditorSettings>(key: K, val: EditorSettings[K]) => {
        const next = { ...settings, [key]: val }
        setSettings(next)
        localStorage.setItem('aioj_editor_settings', JSON.stringify(next))
    }

    const activeTheme = settings.themeOverride === 'system'
        ? (theme === 'dark' ? 'vs-dark' : 'light')
        : settings.themeOverride

    return (
        <div className="border border-gray-300 dark:border-gray-700 rounded-lg overflow-hidden flex flex-col bg-white dark:bg-gray-800 text-left">
            <div className="flex items-center justify-between px-3 py-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 text-xs text-gray-500">
                <span className="font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 select-none">
                    Source Code ({getMonacoLanguage(language)})
                </span>
                <div className="flex items-center gap-3 relative">
                    <button
                        onClick={() => updateSetting('wordWrap', settings.wordWrap === 'on' ? 'off' : 'on')}
                        className={`p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors cursor-pointer ${settings.wordWrap === 'on' ? 'text-blue-500' : 'text-gray-400'}`}
                        title="Toggle Word Wrap"
                    >
                        <WrapText size={16} />
                    </button>
                    <button
                        onClick={() => updateSetting('minimap', !settings.minimap)}
                        className={`p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors cursor-pointer ${settings.minimap ? 'text-blue-500' : 'text-gray-400'}`}
                        title="Toggle Minimap"
                    >
                        {settings.minimap ? <Eye size={16} /> : <EyeOff size={16} />}
                    </button>
                    <button
                        onClick={() => setShowSettings(!showSettings)}
                        className={`p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors cursor-pointer ${showSettings ? 'text-blue-500' : 'text-gray-400'}`}
                        title="Editor Settings"
                    >
                        <Settings size={16} />
                    </button>

                    {showSettings && (
                        <div className="absolute right-0 top-full mt-2 w-56 p-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg z-50 flex flex-col gap-3">
                            <h4 className="font-bold text-gray-700 dark:text-gray-300 pb-1 border-b border-gray-150 dark:border-gray-700 select-none">Editor Options</h4>
                            <div className="flex flex-col gap-1">
                                <label className="text-gray-500 text-[10px] font-semibold uppercase select-none">Editor Theme</label>
                                <select
                                    value={settings.themeOverride}
                                    onChange={(e) => updateSetting('themeOverride', e.target.value as any)}
                                    className="p-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:outline-none"
                                >
                                    <option value="system">Match Site Theme</option>
                                    <option value="vs-dark">Dark (vs-dark)</option>
                                    <option value="vs">Light (vs)</option>
                                </select>
                            </div>
                            <div className="flex flex-col gap-1">
                                <label className="text-gray-500 text-[10px] font-semibold uppercase select-none">Font Size</label>
                                <select
                                    value={settings.fontSize}
                                    onChange={(e) => updateSetting('fontSize', Number(e.target.value))}
                                    className="p-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:outline-none"
                                >
                                    {[12, 13, 14, 15, 16, 17, 18, 20].map(sz => (
                                        <option key={sz} value={sz}>{sz}px</option>
                                    ))}
                                </select>
                            </div>
                            <div className="flex flex-col gap-1">
                                <label className="text-gray-500 text-[10px] font-semibold uppercase select-none">Tab Size</label>
                                <select
                                    value={settings.tabSize}
                                    onChange={(e) => updateSetting('tabSize', Number(e.target.value) as any)}
                                    className="p-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:outline-none"
                                >
                                    <option value={2}>2 spaces</option>
                                    <option value={4}>4 spaces</option>
                                    <option value={8}>8 spaces</option>
                                </select>
                            </div>
                        </div>
                    )}
                </div>
            </div>
            <div style={{ height }}>
                <MonacoEditor
                    height="100%"
                    language={getMonacoLanguage(language)}
                    value={value}
                    onChange={(val) => onChange(val ?? '')}
                    theme={activeTheme}
                    options={{
                        readOnly,
                        minimap: { enabled: settings.minimap },
                        fontSize: settings.fontSize,
                        tabSize: settings.tabSize,
                        wordWrap: settings.wordWrap,
                        lineNumbers: 'on',
                        automaticLayout: true,
                        scrollBeyondLastLine: false,
                        insertSpaces: true,
                    }}
                />
            </div>
        </div>
    )
}
```

- [ ] **Step 2: Build project using Vite to confirm no TypeScript compilation or configuration issues**

Run:
```bash
npm run build
```
Expected: Succeeds without compile errors.

- [ ] **Step 3: Commit implementation**

Run:
```bash
git add web/src/components/CodeEditor.tsx
git commit -m "feat: replace CodeEditor with customizable Monaco editor wrapper"
```
