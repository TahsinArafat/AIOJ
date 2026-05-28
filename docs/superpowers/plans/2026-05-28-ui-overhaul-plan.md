# AIOJ Codeforces UI/UX Alignment & Polygon Setter Workspace Overhaul Plan

## Goal
Transform AIOJ's user and developer workspaces to compete with Codeforces by matching UI/UX conventions, providing a multi-tab rich Codeforces Polygon-style Setter Workspace, and wiring dynamic judges to execute SPJ or specialized line/float checkers.

## Wave 1 (Base Refactorings & Core Implementations)
1. **Navbar Separation**: Extract `Navbar` component from `App.tsx` into `web/src/components/Navbar.tsx`.
2. **CodeEditor Extraction**: Re-package inline CodeMirror logic into `web/src/components/CodeEditor.tsx`.
3. **Problem Markdown/LaTeX**: Integrate rich statement rendering on `ProblemDetail.tsx` (using react-markdown, KaTeX, and syntax highlighting).
4. **Polling Memory Leak Fix**: Fix polling leak with mount refs and AbortController.
5. **Tags & Metadata Sidebar**: Add a right-hand details pane on `/problems/:slug`.
6. **Setter Multi-Tab Workspace**: Revamp `/setter` list and `/setter/:slug` dashboard tabs (Statement, Test Data, Checker/SPJ, Collaborators, Settings).
7. **Dynamic Checker Selection**: Upgrade Go `worker.go` to dynamically select lines/float/SPJ checkers.
8. **Testing Rigor**: Setup Vitest frontend unit and integration tests.

## Wave 2 (Aesthetic Upgrades & API integrations)
9. **Navbar Icons (Lucide)**: Wire comprehensive, modern iconography to links & a user avatar dropdown.
10. **Mobile Hamburger Drawer**: Build full responsive navigation menu.
11. **My Submissions tab**: Add tab to `/problems/:slug` view.
12. **Custom Test Runner**: Build a scratchpad interface for setters & solvers to run code instantly.
13. **Testcase Manager (Scores & ZIP Upload)**: Visual table supporting bulk scoring & multipart `.in`/`.out` ZIP upload.
14. **SPJ C++ Compilation**: Allow backend to compile & execute arbitrary Special Judge code in sandbox.
15. **Collaborator management UI**: Add co-author and tester permissions.

## Verification
- All UI states must compile cleanly (`npm run build`).
- Go judging engine tests must pass successfully.
