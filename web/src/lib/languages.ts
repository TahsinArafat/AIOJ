/**
 * Judge languages, as configured on the server.
 *
 * `lang/*.yaml` on the backend is the only place a language is defined, and
 * GET /api/languages serves that same directory live — so an admin edit in
 * admin#languages, or a language added there, shows up in the IDE and the
 * submit forms without a rebuild.
 *
 * History worth keeping: this module used to be a hardcoded array, copied again
 * into IDE, ProblemDetail, ContestProblem, Submissions and SubmissionDetail.
 * The copies drifted — stale compiler versions, the same submission reading
 * "C++ 20 (GCC)" in one place and "GNU G++17 7.3.0" in another, and a contest
 * picker that hid Clang, PyPy, JavaScript and C 32-bit entirely. Then the array
 * itself became the problem: it was still shipped in the bundle, so the panel
 * could add or rename a language and every dropdown kept offering the old set.
 *
 * The data now comes from the server. FALLBACK_LANGUAGES exists only so a
 * dropdown is never empty when the request fails; it is deliberately the same
 * twelve keys the repo ships, and a test pins it to `lang/*.yaml` so it cannot
 * drift into a second, wrong source of truth.
 */

export interface Language {
    /** Judge key — what goes on the wire. */
    key: string
    /** Display name, as set in `lang/<key>.yaml`. */
    name: string
    /** Starter template offered in the IDE and the submit pane. */
    template: string
}

const CPP_TEMPLATE = `#include <iostream>
using namespace std;

int main() {
    // Your code here

    return 0;
}`

const C_TEMPLATE = `#include <stdio.h>

int main() {
    // Your code here

    return 0;
}`

const JAVA_TEMPLATE = `import java.util.Scanner;

public class Main {
    public static void main(String[] args) {
        Scanner sc = new Scanner(System.in);
        // Your code here

    }
}`

const CSHARP_TEMPLATE = `using System;

class Program {
    static void Main() {
        // Your code here

    }
}`

/**
 * Templates are editor conveniences, not judge config — the server has no
 * opinion about what starter code to show. Keyed by language key, with a
 * family fallback in `templateFor` for keys this table has not seen (an admin
 * can add any key they like, and a submission still needs something to start
 * from).
 */
const TEMPLATES: Record<string, string> = {
    'cpp-gpp-64': CPP_TEMPLATE,
    'cpp-gpp-32': CPP_TEMPLATE,
    'cpp-clang': CPP_TEMPLATE,
    'c-gcc-64': C_TEMPLATE,
    'c-gcc-32': C_TEMPLATE,
    java: JAVA_TEMPLATE,
    csharp: CSHARP_TEMPLATE,
    rust: 'fn main() {\n    // Your code here\n\n}\n',
    python: '# Your code here\n',
    pypy: '# Your code here\n',
    nodejs: '// Your code here\n',
    javascript: '// Your code here\n',
}

/** Family fallback for keys the TEMPLATES table does not know. */
function templateFor(key: string, extensions: string[] = []): string {
    const known = TEMPLATES[key]
    if (known) return known
    // Order matters: check the language families before the key, because an
    // admin is free to name a C++ key anything at all.
    const ext = extensions[0] ?? ''
    if (ext === '.cpp' || ext === '.cc' || ext === '.cxx') return CPP_TEMPLATE
    if (ext === '.c') return C_TEMPLATE
    if (ext === '.java') return JAVA_TEMPLATE
    if (ext === '.cs') return CSHARP_TEMPLATE
    if (ext === '.rs') return 'fn main() {\n    // Your code here\n\n}\n'
    if (ext === '.py' || ext === '.js' || ext === '.ts') return '# Your code here\n'
    if (ext === '.go') return 'package main\n\nimport "fmt"\n\nfunc main() {\n    // Your code here\n    _ = fmt.Sprint()\n}\n'
    return ''
}

/**
 * Shipped languages, kept only as the offline fallback.
 *
 * A test asserts these keys match `lang/*.yaml` exactly, so this cannot become
 * stale the way the old copies did. Do not add a language here — add it to
 * `lang/` and it will arrive over the API.
 */
export const FALLBACK_LANGUAGES: Language[] = [
    { key: 'cpp-gpp-64', name: 'C++ (G++ 64-bit)', template: CPP_TEMPLATE },
    { key: 'cpp-gpp-32', name: 'C++ (G++ 32-bit)', template: CPP_TEMPLATE },
    { key: 'cpp-clang', name: 'C++ (Clang)', template: CPP_TEMPLATE },
    { key: 'c-gcc-64', name: 'C (GCC 64-bit)', template: C_TEMPLATE },
    { key: 'c-gcc-32', name: 'C (GCC 32-bit)', template: C_TEMPLATE },
    { key: 'python', name: 'Python 3', template: '# Your code here\n' },
    { key: 'pypy', name: 'PyPy 3', template: '# Your code here\n' },
    { key: 'java', name: 'Java', template: JAVA_TEMPLATE },
    { key: 'rust', name: 'Rust', template: 'fn main() {\n    // Your code here\n\n}\n' },
    { key: 'nodejs', name: 'Node.js', template: '// Your code here\n' },
    { key: 'javascript', name: 'JavaScript', template: '// Your code here\n' },
    { key: 'csharp', name: 'C# (Mono)', template: CSHARP_TEMPLATE },
]

export const DEFAULT_LANGUAGE = 'cpp-gpp-64'

/** A row from GET /api/languages. */
interface PublicLanguageRow {
    key: string
    name: string
    extensions?: string[]
}

/** Turn the server's rows into the shape the UI expects. */
export function toLanguages(rows: PublicLanguageRow[] | undefined | null): Language[] {
    if (!rows || rows.length === 0) return FALLBACK_LANGUAGES
    return rows.map(r => ({
        key: r.key,
        name: r.name || r.key,
        template: templateFor(r.key, r.extensions),
    }))
}

/**
 * Display name for a judge key.
 *
 * Falls back to the raw key rather than blanking: a submission in a language
 * that has since been deleted from `lang/` must still render as something, and
 * the key is at least truthful.
 */
export function languageName(key: string | undefined | null, languages: Language[] = FALLBACK_LANGUAGES): string {
    if (!key) return '—'
    return languages.find(l => l.key === key)?.name ?? key
}

/** Starter code for a judge key. */
export function languageTemplate(key: string | undefined | null, languages: Language[] = FALLBACK_LANGUAGES): string {
    if (!key) return ''
    return languages.find(l => l.key === key)?.template ?? ''
}

/** Options for a `<select>`. */
export function languageOptions(languages: Language[] = FALLBACK_LANGUAGES) {
    return languages.map(l => ({ value: l.key, label: l.name }))
}

// --- Backwards-compatible module-level exports -----------------------------
//
// Several pages import these directly. They keep working against the fallback
// until each page moves to the `useLanguages()` hook, which is what actually
// reads the server. New code should use the hook, not these.

/** @deprecated Use `useLanguages()` — this is the offline fallback only. */
export const LANGUAGES = FALLBACK_LANGUAGES

/** @deprecated Use `useLanguages()` — this is the offline fallback only. */
export const LANGUAGE_OPTIONS = languageOptions(FALLBACK_LANGUAGES)
