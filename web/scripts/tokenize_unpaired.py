#!/usr/bin/env python3
"""
Second pass: retire the *unpaired* legacy greys.

`tokenize_classes.py` only rewrote `x dark:y` pairs. Plenty of call sites wrote
a single light-only colour (`bg-white`, `text-gray-600`, `border-gray-200`),
which is exactly the class of bug the token system exists to prevent: on the
dark canvas `text-gray-600` is ~2.6:1 and `bg-white` is a glowing white block.

Deliberately excluded:
  * translucent overlays — `bg-white/20`, `bg-white/70` are painted *on* a dark
    surface and must stay translucent;
  * `bg-gray-900` / `bg-black/..` blocks — those are intentional fixed-dark
    surfaces (code viewers, modal scrims), not surfaces that should follow the
    theme;
  * anything already carrying a `dark:` variant, which pass one handled.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

WEB_SRC = Path(__file__).resolve().parent.parent / "src"

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
    "global.css",
}

# Ordered longest-first. Each pattern requires the class to end at a boundary so
# `bg-white/20` and `bg-gray-900` are left alone.
SIMPLE: list[tuple[str, str]] = [
    # text
    ("text-gray-900", "text-ink"),
    ("text-gray-800", "text-ink"),
    ("text-gray-700", "text-ink-soft"),
    ("text-gray-600", "text-muted"),
    ("text-gray-500", "text-muted"),
    ("hover:text-black", "hover:text-ink"),
    ("text-black", "text-ink"),
    # surfaces
    ("hover:bg-gray-200", "hover:bg-surface-3"),
    ("hover:bg-gray-100", "hover:bg-surface-2"),
    ("hover:bg-gray-50", "hover:bg-surface-2"),
    ("bg-gray-50", "bg-surface-2"),
    ("bg-gray-100", "bg-surface-3"),
    ("bg-gray-200", "bg-surface-3"),
    ("bg-white", "bg-surface"),
    # borders / dividers
    ("border-gray-200", "border-hairline"),
    ("border-gray-300", "border-hairline-strong"),
    ("divide-gray-200", "divide-hairline"),
    ("divide-gray-100", "divide-hairline"),
]

# A class name continues until whitespace or a quote; `bg-white/20` therefore
# must not match `bg-white`. `bg-gray-900` must not match `bg-gray-90x` etc.
PATTERNS: list[tuple[re.Pattern[str], str]] = [
    (re.compile(rf"(?<![\w:-]){re.escape(old)}(?![\w/])"), new) for old, new in SIMPLE
]

# Fixed-dark surfaces: never rewrite anything on these lines.
FIXED_DARK = re.compile(r"bg-(gray-900|gray-800|black)\b")


def convert(text: str) -> str:
    out_lines: list[str] = []
    for line in text.splitlines(keepends=True):
        # Skip lines that deliberately paint a dark surface (code viewers,
        # scrims, the scoreboard header band).
        if FIXED_DARK.search(line) and "text-" not in line.split("className")[0]:
            out_lines.append(line)
            continue
        for pattern, new in PATTERNS:
            line = pattern.sub(new, line)
        out_lines.append(line)
    return "".join(out_lines)


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
