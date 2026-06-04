# Design Document: Direct Editorial Creation on Frontend

Add direct editorial creation capability on the frontend for Admins, Judges, and Problem Collaborators.

## Objectives
- Allow privileged users to directly create editorials for a problem.
- Implement editorial creation workspace in Setter Problem Workspace.
- Implement quick inline editorial creation in public Problem Detail view.
- Support markdown text editing, solution code, time/space complexity definitions, and official/unofficial toggles.

---

## 1. Context & User Flow

### Flow 1: Setter Problem Workspace (Tab)
1. Problem Setter/Admin navigates to `/setter/problems/:slug#editorial`.
2. A new "Editorial" tab appears in the sidebar.
3. If an editorial exists, it shows the list/preview. If none, it prompts to write one.
4. Clicking "Add Editorial" loads a form to input Title, Content, Solution Code, Language, Complexity, and toggle Official state (Admin only).
5. Saving persists the editorial via `POST /api/editorials`.

### Flow 2: Public Problem View (Modal)
1. Privileged users (Admin, Judge, or Creator/Collaborator of problem) view the problem at `/problems/:slug`.
2. Under the "Editorials" tab, if they have permissions, they see an "Add Editorial" button.
3. Clicking this opens an overlay Modal containing the same Editorial Form.
4. Saving automatically appends it to the editorials list under the tab.

---

## 2. Component Design

### 2.1 Reusable Form: `EditorialForm`
- **Location**: `/web/src/components/EditorialForm.tsx`
- **Inputs**:
  - `title` (text, required)
  - `content` (textarea / markdown, required)
  - `approach` (textarea, optional)
  - `solutionCode` (textarea, optional)
  - `solutionLanguage` (select dropdown of supported language labels, optional)
  - `timeComplexity` (text, optional)
  - `spaceComplexity` (text, optional)
  - `isOfficial` (checkbox, only enabled/rendered for Admins)
- **Props**:
  - `problemId: string`
  - `onSuccess: () => void`
  - `onCancel?: () => void`

### 2.2 Setter Workspace Tab: `EditorialTab`
- **Location**: `/web/src/components/SetterWorkspace/EditorialTab.tsx`
- **Behavior**:
  - Fetches existing editorials using `api.editorials.getByProblem(problemId)`.
  - Displays list of current editorials (title, author, is_official badge).
  - Offers "Create Editorial" switch/button to reveal `EditorialForm`.

### 2.3 Inline Modal: `AddEditorialModal`
- **Location**: `/web/src/components/AddEditorialModal.tsx`
- **Behavior**:
  - Standard modal dialog window wrapping `EditorialForm`.
  - Dispatched from `ProblemDetail.tsx` when the "Add Editorial" button is clicked.

---

## 3. API Integration

Verify client methods in `/web/src/lib/api.ts` or add them if not fully structured:
```typescript
// Already present in api.ts
editorials: {
  list: (offset: number, limit: number) => Promise<{ data: Editorial[], total: number }>,
  get: (id: string) => Promise<Editorial>,
  getByProblem: (problemId: string) => Promise<{ data: Editorial[] }>,
  create: (data: CreateEditorialRequest) => Promise<Editorial>
}
```

Authentication header: Include JWT tokens using `getAccessToken()` for authorization.

---

## 4. Permission Checking

Admin, Judge, and Collaborators:
- Decode token / fetch user status.
- Admin status check: `user.role === 'admin'`.
- Judge/Collaborator check: Check if current user is owner, co_author, or tester of the problem.
- On public `ProblemDetail.tsx`, fetch permissions from `api.problems.getPermissions(slug)` or check user context info.

---

## 5. Testing & Verification

- Test case: Admin adding an official editorial.
- Test case: Normal user viewing the public problem (does not see the "Add Editorial" button).
- Test case: Space/Time complexity inputs validating correct format.
