# EditorialForm Component — Design Spec

## Overview

Reusable React form component for creating editorials, located at `web/src/components/EditorialForm.tsx`. Follows existing codebase patterns (BlogCreate, ProblemCreate) with individual `useState` hooks and Tailwind styling.

## Props Interface

```typescript
interface EditorialFormProps {
  problemId: string       // Required — associates editorial with a problem
  onSuccess: () => void   // Required — called after successful API submission
  onCancel?: () => void   // Optional — renders Cancel button when provided
}
```

## State

Individual `useState` hooks (consistent with existing form components):

| State               | Type      | Default |
|----------------------|-----------|---------|
| `title`              | `string`  | `''`    |
| `content`            | `string`  | `''`    |
| `approach`           | `string`  | `''`    |
| `solutionCode`       | `string`  | `''`    |
| `solutionLanguage`   | `string`  | `''`    |
| `timeComplexity`     | `string`  | `''`    |
| `spaceComplexity`    | `string`  | `''`    |
| `isOfficial`         | `boolean` | `false` |
| `submitting`         | `boolean` | `false` |
| `error`              | `string`  | `''`    |

## Validation

- **Required fields:** `title` and `content` (trimmed non-empty)
- Submit button disabled when: `submitting || !title.trim() || !content.trim()`

## API Mapping

```typescript
await api.editorials.create({
  problem_id: problemId,
  title,
  content,
  approach: approach || undefined,
  solution_code: solutionCode || undefined,
  solution_language: solutionLanguage || undefined,
  time_complexity: timeComplexity || undefined,
  space_complexity: spaceComplexity || undefined,
  is_official: isOfficial,
})
```

Optional fields omitted from payload when empty (JSON.stringify drops `undefined`).

## UI Layout

Single-column form, `max-w-2xl mx-auto`, matching BlogCreate/ProblemCreate patterns.

1. **Error alert** — red banner above form (shown when `error` is set)
2. **Title** — `<input type="text">`, required
3. **Content** — `<textarea rows={10}>`, required
4. **Approach** — `<textarea rows={3}>`, placeholder "Describe your approach..."
5. **Solution Code** — `<textarea rows={8}>`, monospace (`font-mono`)
6. **Solution Language** — `<input type="text">`, placeholder "e.g. cpp, python"
7. **Complexity row** — 2-column grid: Time Complexity + Space Complexity `<input type="text">`
8. **Is Official** — `<input type="checkbox">` with label
9. **Button row** — Submit (blue, disabled/loading state) + Cancel (gray, only if `onCancel` provided)

## Behavior

- On submit: set `submitting=true`, clear `error`, call `api.editorials.create(...)`, on success call `onSuccess()`, on failure set `error` to message, finally set `submitting=false`
- Cancel button: calls `onCancel()`
- No `as any` or type suppressions

## Constraints

- No new dependencies
- Tailwind-only styling (no new CSS files)
- Single file: `web/src/components/EditorialForm.tsx`
