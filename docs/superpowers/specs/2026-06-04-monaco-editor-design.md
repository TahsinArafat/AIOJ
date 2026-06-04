# Design Spec: Monaco Editor Integration with Customization Options

This design specification details the integration of `monaco-editor` as the default source code editor in AIOJ, replacing CodeMirror 6, and adds user-configurable options (theme, font size, tab size, word wrapping, and minimap toggle) with persistent user preference storage.

## Objectives
- Replace the CodeMirror 6 editor wrapper in `web/src/components/CodeEditor.tsx` with a customizable Monaco Editor using `@monaco-editor/react`.
- Provide an integrated Settings Toolbar with preferences stored in `localStorage`.
- Support seamless layout resizing (crucial for side-by-side resizing in problem pages).
- Ensure zero boilerplate changes are needed in calling components (`ProblemDetail.tsx`, `ContestProblem.tsx`, `CheckerTab.tsx`).

## Proposed Features

### 1. Monaco Editor Integration
- Use `@monaco-editor/react` to dynamically load Monaco Editor from CDNs.
- Map the AIOJ language modes to built-in Monaco language identifiers.
- Sync content changes and support a read-only mode.

### 2. User Configuration Panel
- **Theme Selection**: Match Site Theme (default, tracks global ThemeContext), vs-dark (Dark), vs (Light).
- **Font Size Options**: Adjustable from 12px to 20px (default 14px).
- **Tab Size Options**: Configurable between 2, 4, or 8 spaces (default 4).
- **Word Wrap**: Toggle wrap on or off.
- **Minimap**: Toggle the visual overview map.

## File Changes
- **web/package.json**: Add `@monaco-editor/react` to dependencies and remove old codemirror-specific dependencies.
- **web/src/components/CodeEditor.tsx**: Replace implementation with the new customizable Monaco Editor version.
