import { useCallback, useEffect, useMemo, useState } from 'react'
import Editor, { type OnMount } from '@monaco-editor/react'
import { Settings, Eye, EyeOff, WrapText } from 'lucide-react'
import { useTheme } from '../context/ThemeContext'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface EditorSettings {
  fontSize: number
  tabSize: number
  wordWrap: 'on' | 'off'
  minimap: boolean
  theme: 'light' | 'vs-dark' | 'hc-black'
}

interface CodeEditorProps {
  language: string
  value: string
  onChange: (code: string) => void
  height?: string
  readOnly?: boolean
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const STORAGE_KEY = 'aioj_editor_settings'

const FONT_SIZES = [12, 13, 14, 15, 16, 18, 20, 22, 24]
const TAB_SIZES = [2, 4, 8]

const THEME_OPTIONS: { value: EditorSettings['theme']; label: string }[] = [
  { value: 'light', label: 'Light' },
  { value: 'vs-dark', label: 'Dark' },
  { value: 'hc-black', label: 'High Contrast' },
]

const MONACO_LANG_MAP: Record<string, string> = {
  'cpp-gcc-64': 'cpp',
  'cpp-clang-64': 'cpp',
  'c-gcc': 'c',
  'csharp': 'csharp',
  'python': 'python',
  'pypy': 'python',
  'java': 'java',
  'rust': 'rust',
  'nodejs': 'javascript',
  'javascript': 'javascript',
  'typescript': 'typescript',
  'go': 'go',
  'kotlin': 'kotlin',
  'ruby': 'ruby',
  'php': 'php',
  'swift': 'swift',
  'scala': 'scala',
  'sql': 'sql',
  'html': 'html',
  'css': 'css',
}

function mapLanguage(lang: string): string {
  return MONACO_LANG_MAP[lang] ?? 'plaintext'
}

// ---------------------------------------------------------------------------
// Persistence helpers
// ---------------------------------------------------------------------------

function loadSettings(): EditorSettings {
  const defaults: EditorSettings = {
    fontSize: 14,
    tabSize: 4,
    wordWrap: 'off',
    minimap: true,
    theme: 'vs-dark',
  }
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return { ...defaults, ...JSON.parse(raw) }
  } catch {
    // ignore corrupt data
  }
  return defaults
}

function saveSettings(settings: EditorSettings): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings))
  } catch {
    // storage full or unavailable – silently ignore
  }
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function Toolbar({
  settings,
  onSettingsChange,
}: {
  settings: EditorSettings
  onSettingsChange: (next: EditorSettings) => void
}) {
  const [open, setOpen] = useState(false)

  const update = useCallback(
    <K extends keyof EditorSettings>(key: K, value: EditorSettings[K]) => {
      const next = { ...settings, [key]: value }
      onSettingsChange(next)
    },
    [settings, onSettingsChange],
  )

  return (
    <div className="relative flex items-center gap-2 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-1.5 text-xs">
      {/* Font size */}
      <label className="flex items-center gap-1 text-gray-500 dark:text-gray-400">
        <span className="font-medium">Font</span>
        <select
          value={settings.fontSize}
          onChange={(e) => update('fontSize', Number(e.target.value))}
          className="rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-1.5 py-0.5 text-gray-800 dark:text-gray-200 focus:outline-none focus:ring-1 focus:ring-blue-500"
        >
          {FONT_SIZES.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
      </label>

      {/* Tab size */}
      <label className="flex items-center gap-1 text-gray-500 dark:text-gray-400">
        <span className="font-medium">Tab</span>
        <select
          value={settings.tabSize}
          onChange={(e) => update('tabSize', Number(e.target.value))}
          className="rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-1.5 py-0.5 text-gray-800 dark:text-gray-200 focus:outline-none focus:ring-1 focus:ring-blue-500"
        >
          {TAB_SIZES.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
      </label>

      {/* Word-wrap toggle */}
      <button
        onClick={() =>
          update('wordWrap', settings.wordWrap === 'on' ? 'off' : 'on')
        }
        className="flex items-center gap-1 rounded px-1.5 py-0.5 transition-colors hover:bg-gray-200 dark:hover:bg-gray-600"
        title="Toggle word wrap"
      >
        <WrapText size={14} />
        <span className="text-gray-600 dark:text-gray-300">
          Wrap {settings.wordWrap === 'on' ? 'On' : 'Off'}
        </span>
      </button>

      {/* Minimap toggle */}
      <button
        onClick={() => update('minimap', !settings.minimap)}
        className="flex items-center gap-1 rounded px-1.5 py-0.5 transition-colors hover:bg-gray-200 dark:hover:bg-gray-600"
        title="Toggle minimap"
      >
        {settings.minimap ? <Eye size={14} /> : <EyeOff size={14} />}
        <span className="text-gray-600 dark:text-gray-300">
          Map {settings.minimap ? 'On' : 'Off'}
        </span>
      </button>

      {/* Theme selector */}
      <label className="flex items-center gap-1 text-gray-500 dark:text-gray-400">
        <span className="font-medium">Theme</span>
        <select
          value={settings.theme}
          onChange={(e) =>
            update('theme', e.target.value as EditorSettings['theme'])
          }
          className="rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-1.5 py-0.5 text-gray-800 dark:text-gray-200 focus:outline-none focus:ring-1 focus:ring-blue-500"
        >
          {THEME_OPTIONS.map((t) => (
            <option key={t.value} value={t.value}>{t.label}</option>
          ))}
        </select>
      </label>

      {/* Settings gear (currently decorative – all controls are visible inline) */}
      <button
        onClick={() => setOpen(!open)}
        className="ml-auto rounded p-1 transition-colors hover:bg-gray-200 dark:hover:bg-gray-600"
        title="Editor settings"
      >
        <Settings size={14} className="text-gray-500 dark:text-gray-400" />
      </button>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export default function CodeEditor({
  language,
  value,
  onChange,
  height = '400px',
  readOnly = false,
}: CodeEditorProps) {
  const { theme: appTheme } = useTheme()
  const [settings, setSettings] = useState<EditorSettings>(loadSettings)

  // Sync settings to localStorage whenever they change
  useEffect(() => {
    saveSettings(settings)
  }, [settings])

  // Resolve the Monaco theme from application theme when no explicit choice
  const resolvedTheme = useMemo(() => {
    if (settings.theme !== 'vs-dark') return settings.theme
    return appTheme === 'dark' ? 'vs-dark' : 'light'
  }, [settings.theme, appTheme])

  const monacoLang = mapLanguage(language)

  const handleMount: OnMount = useCallback(
    (editor) => {
      editor.updateOptions({
        fontSize: settings.fontSize,
        tabSize: settings.tabSize,
        wordWrap: settings.wordWrap,
        minimap: { enabled: settings.minimap },
        readOnly,
      })
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [readOnly],
  )

  const handleChange = useCallback(
    (val: string | undefined) => {
      onChange(val ?? '')
    },
    [onChange],
  )

  return (
    <div
      className="flex flex-col overflow-hidden rounded border border-gray-300 dark:border-gray-600"
      style={{ height }}
    >
      <Toolbar settings={settings} onSettingsChange={setSettings} />

      <div className="flex-1 min-h-0">
        <Editor
          height="100%"
          language={monacoLang}
          value={value}
          theme={resolvedTheme}
          onChange={handleChange}
          onMount={handleMount}
          loading={
            <div className="flex items-center justify-center h-full text-sm text-gray-400">
              Loading editor…
            </div>
          }
          options={{
            fontSize: settings.fontSize,
            tabSize: settings.tabSize,
            wordWrap: settings.wordWrap,
            minimap: { enabled: settings.minimap },
            readOnly,
            automaticLayout: true,
            scrollBeyondLastLine: false,
            padding: { top: 8 },
            renderLineHighlight: 'all',
            bracketPairColorization: { enabled: true },
            guides: { bracketPairs: true },
          }}
        />
      </div>
    </div>
  )
}
