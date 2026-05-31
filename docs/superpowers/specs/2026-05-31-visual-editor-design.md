# Visual WYSIWYG Editor for Problem Setter

**Date:** 2026-05-31  
**Status:** Approved  
**Author:** Sisyphus

## Overview

Replace plain textareas in the problem setter workspace with a rich WYSIWYG editor powered by TipTap (ProseMirror). Add a visual math editor dialog for inserting LaTeX formulas via toolbar button.

## Goals

1. Rich text editing experience (like Google Docs) for problem statements
2. Visual math formula insertion with live KaTeX preview
3. Apply to all text fields: Description, Input Format, Output Format, Hint
4. Example/test page at `/setter/test1` with pre-loaded sample problem

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  StatementTab.tsx                                            │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  VisualEditor (TipTap WYSIWYG)                          ││
│  │  ┌─────────────────────────────────────────────────────┐ ││
│  │  │  Toolbar: [B] [I] [H1] [H2] [List] [Code] [Math]  │ ││
│  │  ├─────────────────────────────────────────────────────┤ ││
│  │  │  Editor Content Area                                │ ││
│  │  │  - Bold, italic, headings render in-place           │ ││
│  │  │  - Math formulas render as KaTeX blocks             │ ││
│  │  │  - Code blocks with syntax highlighting             │ ││
│  │  └─────────────────────────────────────────────────────┘ ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  [Math Dialog - opens when clicking Math toolbar button]     │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  LaTeX Input: [________________________]                ││
│  │  Preview:     [Rendered KaTeX output]                   ││
│  │  Common Symbols: [α] [β] [∑] [∫] [√] [→]              ││
│  │  [Cancel] [Insert Inline] [Insert Block]                ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## Component Structure

### New Files

```
web/src/components/VisualEditor/
├── index.tsx              # Main VisualEditor component
├── Toolbar.tsx            # Editor toolbar with formatting buttons
├── MathDialog.tsx         # Math insertion dialog with LaTeX input + preview
├── extensions/
│   ├── MathExtension.ts   # Custom TipTap math extension
│   └── MarkdownOutput.ts  # HTML to Markdown conversion
└── presets.ts             # Common math formula templates

web/src/pages/
└── SetterTestPage.tsx     # Example page at /setter/test1
```

### Modified Files

```
web/src/components/SetterWorkspace/StatementTab.tsx  # Replace textareas with VisualEditor
web/src/App.tsx                                      # Add /setter/test1 route
web/package.json                                     # Add TipTap dependencies
```

## Editor Features

### Toolbar Buttons

| Category | Buttons | Description |
|----------|---------|-------------|
| Text | **B**, *I*, ~~S~~ | Bold, Italic, Strikethrough |
| Headings | H1, H2, H3 | Section headings |
| Lists | • List, 1. List | Bullet and numbered lists |
| Code | `code`, ```code``` | Inline code and code blocks |
| **Math** | **∑** | Opens MathDialog |
| History | ↶ ↷ | Undo/Redo |

### Math Dialog

**Trigger:** Click ∑ button in toolbar or type `$$` and press Enter

**Layout:**
```
┌─────────────────────────────────────────────────────────┐
│  Insert Math Formula                                     │
├─────────────────────────────────────────────────────────┤
│  LaTeX Input:                                            │
│  ┌─────────────────────────────────────────────────────┐│
│  │ \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}                ││
│  └─────────────────────────────────────────────────────┘│
│                                                          │
│  Preview:                                                │
│  ┌─────────────────────────────────────────────────────┐│
│  │         -b ± √(b² - 4ac)                           ││
│  │        ─────────────────                            ││
│  │                2a                                    ││
│  └─────────────────────────────────────────────────────┘│
│                                                          │
│  Quick Insert:                                           │
│  [α][β][γ][δ][ε][θ][λ][μ][π][σ][φ][ω]                  │
│  [∑][∫][√][∞][±][×][÷][≠][≈][≤][≥]                     │
│  [→][←][↑][↓][⇒][⇐]                                    │
│  [frac][sqrt][sum][int][lim][log]                        │
│                                                          │
│  Templates:                                              │
│  [Fraction] [Square Root] [Sum] [Integral] [Matrix]      │
│                                                          │
│  [Cancel]                    [Insert $...$] [Insert $$...$$]
└─────────────────────────────────────────────────────────┘
```

**Features:**
- Live KaTeX preview as user types LaTeX
- Quick-insert symbol palette (Greek letters, operators, arrows)
- Common function buttons (frac, sqrt, sum, etc.)
- Template presets (fraction, quadratic formula, matrix)
- Insert as inline math (`$...$`) or block math (`$$...$$`)

## Data Flow

```
User types in VisualEditor
  ↓
TipTap generates HTML (internal representation)
  ↓
Convert HTML → Markdown (using turndown library)
  ↓
onUpdate('description', markdownValue)
  ↓
formState.description = markdownValue
  ↓
On "Save Statement" → api.problems.update(slug, payload)
  ↓
Backend stores markdown in database
```

### HTML to Markdown Conversion

The editor internally works with HTML (ProseMirror), but the storage format is Markdown (matching existing CSES/Codeforces import format). A conversion layer using `turndown` library handles:

- `<strong>` → `**text**`
- `<em>` → `*text*`
- `<h1>` → `# text`
- `<code>` → `` `text` ``
- `<math-inline>` → `$LaTeX$`
- `<math-block>` → `$$LaTeX$$`

## Integration with Existing Workspace

### StatementTab.tsx Changes

**Before (current):**
```tsx
<textarea
  value={formState.description}
  onChange={e => onUpdate('description', e.target.value)}
  rows={8}
  className="..."
/>
```

**After (new):**
```tsx
<VisualEditor
  content={formState.description}
  onChange={(markdown) => onUpdate('description', markdown)}
  placeholder="Write your problem statement..."
/>
```

### Preview Panel

The right-side preview panel becomes optional since WYSIWYG shows rendered output directly. Options:
1. **Remove preview panel** - WYSIWYG is the preview
2. **Keep as markdown view** - Show raw markdown for power users
3. **Toggle** - Button to switch between WYSIWYG and markdown view

**Recommendation:** Keep preview panel as "Markdown View" toggle for debugging/advanced use.

## Dependencies

```json
{
  "@tiptap/react": "^2.6.0",
  "@tiptap/starter-kit": "^2.6.0",
  "@tiptap/extension-mathematics": "^2.6.0",
  "@tiptap/extension-code-block-lowlight": "^2.6.0",
  "@tiptap/extension-placeholder": "^2.6.0",
  "katex": "^0.17.0",
  "lowlight": "^3.0.0",
  "turndown": "^7.1.0"
}
```

## /setter/test1 Example Page

A standalone page at `/setter/test1` that:

1. Pre-loads sample problem ("Weird Algorithm" from CSES)
2. Shows VisualEditor in action with real content
3. Has "Copy Markdown" button to see generated markdown
4. Useful for testing and demonstrating the editor

**Route:** `/setter/test1` → `SetterTestPage.tsx`

**Layout:**
```
┌─────────────────────────────────────────────────────────────┐
│  Visual Editor Test Page                    [Copy Markdown] │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐│
│  │  VisualEditor with pre-loaded "Weird Algorithm"         ││
│  │  - Title: Weird Algorithm                               ││
│  │  - Description: Full problem statement with math        ││
│  │  - Input/Output/Hint: Pre-filled                        ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  Generated Markdown (read-only)                         ││
│  │  ┌─────────────────────────────────────────────────────┐││
│  │  │ # Weird Algorithm                                   │││
│  │  │                                                     │││
│  │  │ Consider an algorithm that takes...                 │││
│  │  │ $n$ is even... $$3 \rightarrow 10 \rightarrow 5$$  │││
│  │  └─────────────────────────────────────────────────────┘││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## Error Handling

| Scenario | Handling |
|----------|----------|
| Invalid LaTeX in MathDialog | Show error message in preview area, prevent insert |
| Empty editor content | Allow save (some fields optional) |
| Large content (>100KB) | Warning toast, allow continue |
| Turndown conversion failure | Fallback to raw HTML, log error |
| TipTap initialization error | Show error state, fallback to textarea |

## Testing Strategy

1. **Unit tests:** VisualEditor component renders, toolbar buttons work
2. **Integration tests:** MathDialog opens/inserts, HTML→Markdown conversion
3. **E2E tests:** Full workflow - type text, insert math, save, verify stored markdown
4. **Manual testing:** /setter/test1 page with sample problem

## Future Enhancements (Out of Scope)

- Collaborative editing (multiple users editing simultaneously)
- Version history / diff view
- Export to PDF
- Custom LaTeX macros
- Image drag-and-drop upload
- Table editor
