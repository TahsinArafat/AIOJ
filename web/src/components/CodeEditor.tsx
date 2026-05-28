import { useEffect, useRef } from 'react'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { oneDark } from '@codemirror/theme-one-dark'
import { cpp } from '@codemirror/lang-cpp'
import { python } from '@codemirror/lang-python'
import { java } from '@codemirror/lang-java'
import { rust } from '@codemirror/lang-rust'
import { javascript } from '@codemirror/lang-javascript'

interface CodeEditorProps {
    language: string
    value: string
    onChange: (code: string) => void
    height?: string
    readOnly?: boolean
}

function getLangExtension(lang: string) {
    if (lang.startsWith('cpp') || lang.startsWith('c-')) return cpp()
    if (lang === 'python' || lang === 'pypy') return python()
    if (lang === 'java') return java()
    if (lang === 'rust') return rust()
    if (lang === 'nodejs') return javascript()
    if (lang === 'csharp') return cpp()
    return cpp()
}

export default function CodeEditor({
    language,
    value,
    onChange,
    height = '400px',
    readOnly = false
}: CodeEditorProps) {
    const editorRef = useRef<HTMLDivElement>(null)
    const viewRef = useRef<EditorView | null>(null)
    const onChangeRef = useRef(onChange)

    // Sync onChange callback ref to avoid effect trigger when it changes
    useEffect(() => {
        onChangeRef.current = onChange
    }, [onChange])

    useEffect(() => {
        if (!editorRef.current) return

        // Destroy old view
        viewRef.current?.destroy()

        // Create listener extension
        const updateListener = EditorView.updateListener.of((update) => {
            if (update.docChanged) {
                onChangeRef.current(update.state.doc.toString())
            }
        })

        // Config state extensions
        const extensions = [
            basicSetup,
            oneDark,
            getLangExtension(language),
            updateListener,
            EditorView.theme({
                '&': { height: height, maxHeight: height },
                '.cm-scroller': { overflow: 'auto' }
            })
        ]

        if (readOnly) {
            extensions.push(EditorState.readOnly.of(true))
        }

        const state = EditorState.create({
            doc: value,
            extensions
        })

        const view = new EditorView({
            state,
            parent: editorRef.current
        })

        viewRef.current = view

        return () => {
            view.destroy()
        }
    }, [language, height, readOnly])

    // Update code value programmatically if it diverges from doc
    useEffect(() => {
        if (viewRef.current) {
            const currentDoc = viewRef.current.state.doc.toString()
            if (value !== currentDoc) {
                viewRef.current.dispatch({
                    changes: { from: 0, to: currentDoc.length, insert: value }
                })
            }
        }
    }, [value])

    return (
        <div 
            ref={editorRef} 
            className="border border-gray-300 rounded overflow-hidden text-left"
            style={{ minHeight: height }}
        />
    )
}
