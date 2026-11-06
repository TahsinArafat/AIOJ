#!/usr/bin/env python3
"""
Map the legacy `x dark:y` Tailwind pairs onto the semantic design tokens
introduced in web/src/global.css.

Why a codemod instead of hand-editing: the same twelve pairs appear ~1700
times across ~90 files, and they all mean the same three things (a surface, a
text level, a border). Editing them by hand is both enormous and error-prone;
a single ordered mapping is reviewable and idempotent.

Design rules encoded here:
  * surfaces:  bg-white -> surface, bg-gray-50 -> surface-2, bg-gray-100/200 -> surface-3
  * text:      gray-900/800 -> ink, gray-700 -> ink-soft, gray-600/500 -> muted, gray-400 -> muted-soft
  * borders:   gray-100/200 -> hairline, gray-300 -> hairline-strong
  * brand:     blue-600/700 -> brand / brand-hover, blue-50 -> brand-soft,
               and white text on a brand fill becomes on-brand (white on the
               light blue dark-mode brand is unreadable).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

WEB_SRC = Path(__file__).resolve().parent.parent / "src"

# Files already rebuilt by hand on the token system — never touch them.
SKIP_FILES = {
    "App.tsx",
    "components/Navbar.tsx",
    "components/ActivityFeed.tsx",
    "components/EmptyState.tsx",
    "components/AuthLayout.tsx",
    "components/LanguageSwitcher.tsx",
    "components/ui/Button.tsx",
    "components/ui/buttonStyles.ts",
    "components/ui/Card.tsx",
    "components/ui/Badge.tsx",
    "components/ui/Field.tsx",
    "components/ui/Table.tsx",
    "components/ui/PageHeader.tsx",
    "components/ui/ui.test.tsx",
    "pages/ProblemList.tsx",
    "pages/ContestList.tsx",
    "pages/Rankings.tsx",
    "pages/Login.tsx",
    "pages/Register.tsx",
    "pages/ForgotPassword.tsx",
    "pages/ResetPassword.tsx",
    "pages/Login.test.tsx",
    "global.css",
}

# Ordered: longest / most specific first so a pair is never half-rewritten.
REPLACEMENTS: list[tuple[str, str]] = [
    # ── text ────────────────────────────────────────────────────────────────
    ("text-gray-950 dark:text-gray-50", "text-ink"),
    ("text-gray-900 dark:text-gray-50", "text-ink"),
    ("text-gray-900 dark:text-gray-100", "text-ink"),
    ("text-gray-800 dark:text-gray-100", "text-ink"),
    ("text-gray-800 dark:text-gray-200", "text-ink"),
    ("text-gray-800 dark:text-gray-300", "text-ink"),
    ("text-gray-700 dark:text-gray-200", "text-ink-soft"),
    ("text-gray-700 dark:text-gray-300", "text-ink-soft"),
    ("text-gray-600 dark:text-gray-300", "text-muted"),
    ("text-gray-600 dark:text-gray-400", "text-muted"),
    ("text-gray-500 dark:text-gray-400", "text-muted"),
    ("text-gray-400 dark:text-gray-500", "text-muted-soft"),
    ("text-gray-300 dark:text-gray-600", "text-muted-soft"),
    # ── surfaces ────────────────────────────────────────────────────────────
    ("bg-white dark:bg-gray-800", "bg-surface"),
    ("bg-white dark:bg-gray-900", "bg-surface"),
    ("bg-white dark:bg-gray-700", "bg-surface"),
    ("bg-gray-50 dark:bg-gray-700", "bg-surface-2"),
    ("bg-gray-50 dark:bg-gray-800", "bg-surface-2"),
    ("bg-gray-50 dark:bg-gray-900", "bg-surface-2"),
    ("bg-gray-100 dark:bg-gray-700", "bg-surface-3"),
    ("bg-gray-100 dark:bg-gray-800", "bg-surface-3"),
    ("bg-gray-100 dark:bg-gray-900", "bg-surface-3"),
    ("bg-gray-200 dark:bg-gray-700", "bg-surface-3"),
    ("bg-gray-200 dark:bg-gray-800", "bg-surface-3"),
    ("bg-gray-300 dark:bg-gray-600", "bg-hairline-strong"),
    ("bg-gray-400 dark:bg-gray-500", "bg-muted-soft"),
    # ── borders ─────────────────────────────────────────────────────────────
    ("border-gray-100 dark:border-gray-700", "border-hairline"),
    ("border-gray-100 dark:border-gray-800", "border-hairline"),
    ("border-gray-200 dark:border-gray-600", "border-hairline"),
    ("border-gray-200 dark:border-gray-700", "border-hairline"),
    ("border-gray-200 dark:border-gray-800", "border-hairline"),
    ("border-gray-300 dark:border-gray-600", "border-hairline-strong"),
    ("border-gray-300 dark:border-gray-700", "border-hairline-strong"),
    # ── dividers ────────────────────────────────────────────────────────────
    ("divide-gray-100 dark:divide-gray-700", "divide-hairline"),
    ("divide-gray-200 dark:divide-gray-700", "divide-hairline"),
    # ── hovers ──────────────────────────────────────────────────────────────
    ("hover:bg-gray-50 dark:hover:bg-gray-700", "hover:bg-surface-2"),
    ("hover:bg-gray-50 dark:hover:bg-gray-800", "hover:bg-surface-2"),
    ("hover:bg-gray-50 dark:hover:bg-gray-900", "hover:bg-surface-2"),
    ("hover:bg-gray-100 dark:hover:bg-gray-700", "hover:bg-surface-2"),
    ("hover:bg-gray-100 dark:hover:bg-gray-800", "hover:bg-surface-2"),
    ("hover:bg-gray-200 dark:hover:bg-gray-600", "hover:bg-surface-3"),
    ("hover:bg-gray-200 dark:hover:bg-gray-700", "hover:bg-surface-3"),
    ("hover:bg-gray-300 dark:hover:bg-gray-700", "hover:bg-surface-3"),
    ("hover:bg-gray-700 dark:hover:bg-gray-500", "hover:bg-surface-3"),
    # ── brand accents ───────────────────────────────────────────────────────
    ("hover:bg-blue-50 dark:hover:bg-blue-900/20", "hover:bg-brand-soft"),
    ("hover:bg-blue-100 dark:hover:bg-blue-900/50", "hover:bg-brand-soft"),
    ("bg-blue-50 dark:bg-blue-900/20", "bg-brand-soft"),
    ("bg-blue-50 dark:bg-blue-900/30", "bg-brand-soft"),
    ("bg-blue-100 dark:bg-blue-900/50", "bg-brand-soft"),
    ("bg-blue-600 dark:bg-blue-500", "bg-brand"),
    ("hover:bg-blue-700 dark:hover:bg-blue-600", "hover:bg-brand-hover"),
    ("hover:bg-blue-700 dark:hover:bg-blue-500", "hover:bg-brand-hover"),
    ("hover:bg-blue-700", "hover:bg-brand-hover"),
    ("bg-blue-600", "bg-brand"),
    ("text-blue-700 dark:text-blue-300", "text-brand"),
    ("text-blue-600 dark:text-blue-400", "text-brand"),
    ("hover:text-blue-600 dark:hover:text-blue-400", "hover:text-brand"),
    ("hover:text-blue-800 dark:hover:text-blue-300", "hover:text-brand"),
    ("hover:text-blue-600", "hover:text-brand"),
    ("focus:ring-blue-500 dark:focus:ring-blue-400", "focus-visible:outline-[var(--ring)]"),
    ("focus:ring-blue-500", "focus-visible:outline-[var(--ring)]"),
    ("focus:border-blue-500 dark:focus:border-blue-400", "focus:border-brand"),
    ("focus:border-blue-500", "focus:border-brand"),
    # ── stray accent tints that pointed at the old canvas ───────────────────
    ("text-gray-850 dark:text-gray-205", "text-ink"),
    ("text-gray-700 dark:text-gray-350", "text-ink-soft"),
    ("border-gray-750", "border-hairline"),
]

# Any quoted class list that now paints a brand fill must use on-brand text:
# `text-white` on the light-blue dark-mode brand is ~2:1 contrast.
CLASS_LITERAL = re.compile(r"""(['"])([^'"]*\bbg-brand\b[^'"]*)\1""")
TOKEN_CLASS = re.compile(r"\b(bg|text|border|divide|ring|outline)-")


def convert(text: str) -> str:
    for old, new in REPLACEMENTS:
        text = text.replace(old, new)

    def fix_brand_text(match: re.Match[str]) -> str:
        quote, body = match.group(1), match.group(2)
        if "text-white" in body:
            body = body.replace("text-white", "text-on-brand")
        return f"{quote}{body}{quote}"

    return CLASS_LITERAL.sub(fix_brand_text, text)


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
