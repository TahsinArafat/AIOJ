#!/usr/bin/env python3
"""
Third pass: map the *status* colour families onto semantic tones.

Passes 1 and 2 handled the neutral greys and the brand blue. What is left is
the semantic palette — red/green/yellow/amber/purple — which carries meaning
(success, failure, warning, accent) and therefore has a token of its own:
`bad` / `ok` / `warn` / `accent`.

Two kinds of breakage are fixed here:

  1. Unreadable pairs. `bg-green-100 text-green-800` has no dark variant, so on
     the dark canvas it is dark-green text on a pale-green fill. Mapping the
     pair to `bg-ok/12 text-ok` is correct in both themes by construction.

  2. Silently dropped classes. `bg-green-900/200` and `bg-amber-900/200` use an
     opacity step that does not exist, so Tailwind emits nothing and the element
     loses its background entirely.

Solid fills (`bg-green-600 text-white`) are mapped to the `-fill` tokens, which
exist precisely because a *text* tone is not a usable fill colour in dark mode.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

WEB_SRC = Path(__file__).resolve().parent.parent / "src"

# Files already rebuilt by hand on the primitives.
SKIP_FILES = {
    "App.tsx",
    "components/Navbar.tsx",
    "components/ActivityFeed.tsx",
    "components/EmptyState.tsx",
    "components/AuthLayout.tsx",
    "components/LanguageSwitcher.tsx",
    "components/ConfirmDialog.tsx",
    "components/Toast.tsx",
    "components/GlobalSearch.tsx",
    "components/CodeEditor.tsx",
    "components/VisualEditor/Toolbar.tsx",
    "components/VisualEditor/MathDialog.tsx",
    "components/ui/Button.tsx",
    "components/ui/buttonStyles.ts",
    "components/ui/Card.tsx",
    "components/ui/Badge.tsx",
    "components/ui/Field.tsx",
    "components/ui/Table.tsx",
    "components/ui/PageHeader.tsx",
    "components/ui/VerdictBadge.tsx",
    "pages/ProblemList.tsx",
    "pages/ContestList.tsx",
    "pages/Rankings.tsx",
    "pages/Login.tsx",
    "pages/Register.tsx",
    "pages/ForgotPassword.tsx",
    "pages/ResetPassword.tsx",
    "pages/Submissions.tsx",
    "pages/SubmissionDetail.tsx",
    "pages/Profile.tsx",
    "global.css",
}

# ── Paired form (light + dark halves) ────────────────────────────────────────
# Longest / most specific first. Each entry is (old, new) and both halves must
# match exactly, so a half-rewritten pair can never occur.
PAIRS: list[tuple[str, str]] = [
    # red / rose -> bad
    ("bg-red-50 dark:bg-red-900/20", "bg-bad/10"),
    ("bg-red-50 dark:bg-red-900/30", "bg-bad/10"),
    ("bg-red-100 dark:bg-red-900/30", "bg-bad/12"),
    ("bg-red-100 dark:bg-red-900/20", "bg-bad/12"),
    ("text-red-700 dark:text-red-300", "text-bad"),
    ("text-red-700 dark:text-red-400", "text-bad"),
    ("text-red-600 dark:text-red-400", "text-bad"),
    ("text-red-800 dark:text-red-300", "text-bad"),
    ("border-red-200 dark:border-red-800", "border-bad/30"),
    ("border-red-200 dark:border-red-700", "border-bad/30"),
    ("border-red-300 dark:border-red-700", "border-bad/40"),
    ("hover:bg-red-700 dark:hover:bg-red-600", "hover:bg-danger-fill-hover"),
    # green / emerald -> ok
    ("bg-green-50 dark:bg-green-900/20", "bg-ok/10"),
    ("bg-green-50 dark:bg-green-900/30", "bg-ok/10"),
    ("bg-green-100 dark:bg-green-900/30", "bg-ok/12"),
    ("bg-green-100 dark:bg-green-900/40", "bg-ok/12"),
    ("bg-emerald-50 dark:bg-emerald-900/20", "bg-ok/10"),
    ("bg-emerald-100 dark:bg-emerald-900/30", "bg-ok/12"),
    ("text-green-700 dark:text-green-300", "text-ok"),
    ("text-green-700 dark:text-green-400", "text-ok"),
    ("text-green-600 dark:text-green-400", "text-ok"),
    ("text-green-800 dark:text-green-300", "text-ok"),
    ("text-emerald-700 dark:text-emerald-300", "text-ok"),
    ("text-emerald-600 dark:text-emerald-400", "text-ok"),
    ("border-green-200 dark:border-green-800", "border-ok/30"),
    ("border-green-200 dark:border-green-700", "border-ok/30"),
    ("border-green-300 dark:border-green-700", "border-ok/40"),
    ("border-emerald-200 dark:border-emerald-800", "border-ok/30"),
    ("border-emerald-200 dark:border-emerald-700", "border-ok/30"),
    # yellow / amber -> warn
    ("bg-yellow-50 dark:bg-yellow-900/20", "bg-warn/10"),
    ("bg-yellow-50 dark:bg-yellow-900/30", "bg-warn/10"),
    ("bg-yellow-100 dark:bg-yellow-900/30", "bg-warn/12"),
    ("bg-yellow-100 dark:bg-yellow-900/20", "bg-warn/12"),
    ("bg-amber-50 dark:bg-amber-900/20", "bg-warn/10"),
    ("bg-amber-50 dark:bg-amber-900/30", "bg-warn/10"),
    ("bg-amber-100 dark:bg-amber-900/30", "bg-warn/12"),
    ("bg-amber-100 dark:bg-amber-900/20", "bg-warn/12"),
    ("text-yellow-700 dark:text-yellow-300", "text-warn"),
    ("text-yellow-600 dark:text-yellow-400", "text-warn"),
    ("text-yellow-800 dark:text-yellow-300", "text-warn"),
    ("text-amber-700 dark:text-amber-300", "text-warn"),
    ("text-amber-800 dark:text-amber-300", "text-warn"),
    ("text-amber-600 dark:text-amber-400", "text-warn"),
    ("border-yellow-200 dark:border-yellow-700", "border-warn/30"),
    ("border-yellow-300 dark:border-yellow-700", "border-warn/40"),
    ("border-amber-200 dark:border-amber-700", "border-warn/30"),
    ("border-amber-300 dark:border-amber-700", "border-warn/40"),
    # purple / violet / indigo -> accent (dark-safe violet)
    ("bg-purple-50 dark:bg-purple-900/20", "bg-violet-500/10"),
    ("bg-purple-100 dark:bg-purple-900/30", "bg-violet-500/12"),
    ("text-purple-700 dark:text-purple-300", "text-accent"),
    ("text-purple-800 dark:text-purple-300", "text-accent"),
    ("text-purple-600 dark:text-purple-400", "text-accent"),
    ("border-purple-200 dark:border-purple-700", "border-accent/30"),
    # blue / sky that pass 1 did not reach (non-brand semantic uses)
    ("bg-sky-50 dark:bg-sky-900/30", "bg-brand-soft"),
    ("bg-sky-100 dark:bg-sky-900/30", "bg-brand-soft"),
    ("text-sky-700 dark:text-sky-300", "text-brand"),
    ("border-sky-200 dark:border-sky-800", "border-brand/30"),
]

# ── Invalid classes Tailwind silently drops ──────────────────────────────────
INVALID: list[tuple[str, str]] = [
    # Opacity steps above 100 do not exist.
    ("bg-green-900/200", "bg-ok"),
    ("bg-amber-900/200", "bg-warn"),
    ("bg-red-900/200", "bg-bad"),
    # Typo left a status dot with no colour at all.
    ("bg-brand-soft0", "bg-brand"),
]

# ── Unpaired leftovers ───────────────────────────────────────────────────────
SIMPLE: list[tuple[str, str]] = [
    ("text-red-600", "text-bad"),
    ("text-red-500", "text-bad"),
    ("text-red-400", "text-bad"),
    ("text-red-300", "text-bad"),
    ("text-green-600", "text-ok"),
    ("text-green-400", "text-ok"),
    ("text-green-300", "text-ok"),
    ("text-emerald-400", "text-ok"),
    ("text-yellow-600", "text-warn"),
    ("text-yellow-400", "text-warn"),
    ("text-yellow-300", "text-warn"),
    ("text-amber-400", "text-warn"),
    ("text-amber-300", "text-warn"),
    ("text-purple-300", "text-accent"),
    ("bg-red-600", "bg-danger-fill"),
    ("bg-green-600", "bg-ok-fill"),
    ("bg-green-700", "bg-ok-fill-hover"),
    ("bg-amber-600", "bg-warn-fill"),
    ("border-red-200", "border-bad/30"),
    ("border-green-200", "border-ok/30"),
    ("border-yellow-200", "border-warn/30"),
]

# Lines that intentionally paint a fixed dark surface (code viewers, scrims).
FIXED_DARK = re.compile(r"bg-(gray-900|gray-950|black|code)\b")


def apply(text: str, table: list[tuple[str, str]]) -> str:
    for old, new in table:
        text = text.replace(old, new)
    return text


def convert(text: str) -> str:
    text = apply(text, INVALID)
    text = apply(text, PAIRS)

    out: list[str] = []
    for line in text.splitlines(keepends=True):
        if FIXED_DARK.search(line):
            out.append(line)
            continue
        for old, new in SIMPLE:
            line = re.sub(rf"(?<![\w:-]){re.escape(old)}(?![\w/])", new, line)
        out.append(line)
    return "".join(out)


def main() -> int:
    changed: list[tuple[str, int]] = []
    for path in sorted(WEB_SRC.rglob("*.tsx")):
        rel = path.relative_to(WEB_SRC).as_posix()
        if rel in SKIP_FILES or ".test." in rel:
            continue
        original = path.read_text()
        updated = convert(original)
        if updated != original:
            path.write_text(updated)
            delta = sum(1 for a, b in zip(original.splitlines(), updated.splitlines()) if a != b)
            changed.append((rel, delta))

    for rel, delta in changed:
        print(f"{delta:5d}  {rel}")
    print(f"\n{len(changed)} files rewritten")
    return 0


if __name__ == "__main__":
    sys.exit(main())
