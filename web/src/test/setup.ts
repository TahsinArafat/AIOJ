import '@testing-library/jest-dom'

const localStorageMock = (function () {
    let store: Record<string, string> = {}
    return {
        getItem(key: string) {
            return store[key] || null
        },
        setItem(key: string, value: string) {
            store[key] = value.toString()
        },
        clear() {
            store = {}
        },
        removeItem(key: string) {
            delete store[key]
        }
    }
})()

Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock })
Object.defineProperty(window, 'localStorage', { value: localStorageMock })

// jsdom lacks matchMedia (used by ThemeProvider / ProblemDetail)
if (typeof window !== 'undefined' && !window.matchMedia) {
    Object.defineProperty(window, 'matchMedia', {
        writable: true,
        value: (query: string) => ({
            matches: false,
            media: query,
            onchange: null,
            addListener: () => { },
            removeListener: () => { },
            addEventListener: () => { },
            removeEventListener: () => { },
            dispatchEvent: () => false,
        }),
    })
}

// prosemirror-view calls document.elementFromPoint (not in jsdom)
if (typeof document !== 'undefined' && !document.elementFromPoint) {
    document.elementFromPoint = () => null
}
