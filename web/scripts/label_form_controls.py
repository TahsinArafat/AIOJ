#!/usr/bin/env python3
"""Give form controls an accessible name when their <label> is a sibling.

The codebase renders most fields as

    <label className="...">Title</label>
    <input ... />

which looks labelled but is not: the label has no `htmlFor` and the control has
no `id`, so assistive tech announces an unnamed text box. This codemod adds an
`aria-label` to the control derived from the label text.

`aria-label` rather than `htmlFor`/`id` on purpose: several of these fields are
rendered inside `.map()` over sample cases / test cases, where a static `id`
would produce duplicate ids in the DOM. `aria-label` is always unique-safe.

Only touches controls that have none of: aria-label, aria-labelledby, id,
placeholder. Re-runnable: a second pass is a no-op.

Usage:
    python3 web/scripts/label_form_controls.py            # dry run
    python3 web/scripts/label_form_controls.py --write    # apply
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

CONTROL_TAGS = ("input", "select", "textarea")
LABEL_RE = re.compile(r"<label\b([^>]*)>", re.S)


def skip_ws(src: str, i: int) -> int:
    while i < len(src) and src[i] in " \t\r\n":
        i += 1
    return i


def find_close_tag(src: str, start: int) -> int:
    """Index just past the `>` that closes the tag opening at `start`."""
    i = start
    depth = 0
    quote: str | None = None
    while i < len(src):
        ch = src[i]
        if quote:
            if ch == "\\":
                i += 2
                continue
            if ch == quote:
                quote = None
        elif ch in "\"'`":
            quote = ch
        elif ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
        elif ch == ">" and depth == 0:
            return i + 1
        i += 1
    return -1


def label_text(inner: str) -> str:
    """Visible text of a label body, with JSX tags/expressions removed."""
    text = re.sub(r"<[^>]*>", " ", inner)
    text = re.sub(r"\{[^{}]*\}", " ", text)
    text = text.replace("&nbsp;", " ")
    text = re.sub(r"\s+", " ", text).strip()
    text = text.strip("*").strip()
    return text


def process(src: str) -> tuple[str, list[str]]:
    out: list[str] = []
    changes: list[str] = []
    pos = 0
    while True:
        m = LABEL_RE.search(src, pos)
        if not m:
            out.append(src[pos:])
            break
        out.append(src[pos : m.start()])

        attrs = m.group(1)
        body_start = m.end()
        body_end = src.find("</label>", body_start)
        if body_end == -1:
            out.append(src[m.start() : m.end()])
            pos = m.end()
            continue
        inner = src[body_start:body_end]
        after = body_end + len("</label>")

        # A label that wraps its own control already names it.
        wraps_control = re.search(r"<(input|select|textarea)\b", inner) is not None
        has_for = re.search(r"\bhtmlFor\s*=", attrs) is not None
        text = label_text(inner)

        if wraps_control or has_for or not text or len(text) > 60:
            out.append(src[m.start() : after])
            pos = after
            continue

        # The whole label element is kept verbatim; only the control changes.
        label_src = src[m.start() : after]
        j = skip_ws(src, after)
        tag = None
        for candidate in CONTROL_TAGS:
            if src.startswith("<" + candidate, j) and not src[j + 1 + len(candidate)].isalnum():
                tag = candidate
                break
        if tag is None:
            out.append(label_src)
            pos = after
            continue

        end = find_close_tag(src, j)
        if end == -1:
            out.append(label_src)
            pos = after
            continue
        opening = src[j:end]
        # A placeholder still counts as a weak name, but the visible label is
        # the better one, so it does not stop us here.
        already = re.search(r"\b(aria-label|aria-labelledby)\s*=|\bid\s*=", opening)
        if already:
            out.append(label_src)
            pos = after
            continue

        escaped = text.replace('"', "&quot;")
        insert_at = j + 1 + len(tag)
        out.append(label_src)
        out.append(src[after:insert_at])
        out.append(f' aria-label="{escaped}"')
        pos = insert_at
        changes.append(f'{tag}: "{escaped}"')

    return "".join(out), changes


def main() -> int:
    write = "--write" in sys.argv
    root = Path(__file__).resolve().parents[1] / "src"
    total_files = 0
    total_changes = 0
    for path in sorted(root.rglob("*.tsx")):
        if ".test." in path.name:
            continue
        src = path.read_text()
        if "<label" not in src:
            continue
        new, changes = process(src)
        if not changes:
            continue
        total_files += 1
        total_changes += len(changes)
        print(f"{path.relative_to(root.parent)}: {len(changes)} control(s)")
        for c in changes:
            print(f"    {c}")
        if write:
            path.write_text(new)
    print(f"\n{total_changes} control(s) in {total_files} file(s){'' if write else ' (dry run)'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
