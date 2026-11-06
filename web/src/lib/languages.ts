/**
 * Canonical judge languages.
 *
 * `key` and `name` mirror `lang/*.yaml` on the backend, which is the only place
 * a language is actually defined. Before this module existed the list was
 * copy-pasted into IDE, ProblemDetail, ContestProblem, Submissions and
 * SubmissionDetail, and the copies had drifted in ways that mattered:
 *
 *   * every copy advertised stale compiler versions ("GNU G++17 7.3.0",
 *     "Python 3.8.10") that no longer matched what the judge runs;
 *   * the same submission read "C++ 20 (GCC)" in the submission list and
 *     "GNU G++17 7.3.0 (64 bit)" in the IDE;
 *   * ContestProblem's list was missing Clang, PyPy, JavaScript and C 32-bit,
 *     so a contestant could not submit in those languages in a contest even
 *     though the judge supports them everywhere else.
 *
 * Adding a language now means adding one entry here plus the `lang/*.yaml`.
 */

export interface Language {
    /** Judge key — what goes on the wire. */
    key: string
    /** Display name, matching `lang/<key>.yaml`. */
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

export const LANGUAGES: Language[] = [
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

const BY_KEY = new Map(LANGUAGES.map(l => [l.key, l]))

/** Display name for a judge key, falling back to the raw key. */
export function languageName(key: string | undefined | null): string {
    if (!key) return '—'
    return BY_KEY.get(key)?.name ?? key
}

/** Starter code for a judge key. */
export function languageTemplate(key: string | undefined | null): string {
    if (!key) return ''
    return BY_KEY.get(key)?.template ?? ''
}

/** Options for a `<select>`. */
export const LANGUAGE_OPTIONS = LANGUAGES.map(l => ({ value: l.key, label: l.name }))
