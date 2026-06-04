# Design Document: AtCoder, Toph, and QOJ VJudge Bots & Problem Importers

## Background
AIOJ (Lightweight Online Judge) supports VJudge bots to submit code to remote competitive programming platforms on behalf of bot accounts. Codeforces and CSES bots are already implemented. AtCoder, Toph, and QOJ bots are currently stubs and need complete implementations.

---

## 1. AtCoder Bot (`atcoder.go` & `parser.go` & `import.go`)

### Authentication & Cloudflare Bypass
AtCoder uses Cloudflare Turnstile for both the login page (`/login`) and the submission page (`/submit`).
- **Approach**: Cookie-based authentication.
- **Implementation**: The admin dashboard allows saving `session_data` (cookies) for bot accounts. We will extract the `REPSESSID` (or relevant session cookies) and inject them into the HTTP client.
- **State Check**: `IsLoggedIn` will fetch `https://atcoder.jp/settings` and check if the username/logout link is present.
- **Login Flow**: If cookies are invalid/expired and credentials are provided, we will return an error instructing the user to refresh session cookies manually via the Admin dashboard.

### Submission Flow
1. Fetch the submission page to retrieve the CSRF token:
   - URL: `https://atcoder.jp/contests/{contest_id}/submit` or generic `https://atcoder.jp/submit`.
   - Extract CSRF token from the input field `csrf_token`.
2. Send a POST request to `https://atcoder.jp/contests/{contest_id}/submit` containing:
   - `csrf_token`: extracted CSRF token
   - `data.TaskScreenName`: problem ID (e.g. `abc300_a`)
   - `data.LanguageId`: mapped language ID
   - `sourceCode`: the code payload
3. Extract the submission ID from the redirect/response.

### Polling Flow
1. Fetch `https://atcoder.jp/contests/{contest_id}/submissions/me` or specific submission page `https://atcoder.jp/contests/{contest_id}/submissions/{remote_id}`.
2. Scrape the status/verdict from HTML (e.g., `AC`, `WA`, `TLE`, `MLE`, `CE`, `RE`, `WJ` for waiting).
3. If verdict is complete, parse memory (KB) and time (ms) used.

### Problem Parser (`parser.go`)
- Method: `ParseAtCoderProblem(ctx, contestID, problemID)`
- Scrape `https://atcoder.jp/contests/{contestID}/tasks/{problemID}`.
- Parse title (e.g. `A - Generalized ABC`), description (convert HTML to markdown), sample cases, time/memory limits.

---

## 2. Toph Bot (`toph.go` & `parser.go` & `import.go`)

### Authentication
Toph uses standard session cookie auth.
- **Login Flow**:
  1. GET `https://toph.co/login`.
  2. Scrape the HTML form for CSRF token.
  3. POST to `https://toph.co/login` with `nick`/`email`, `password`, and CSRF.
  4. Save returned session cookies.
- **State Check**: Fetch the home page and verify login status.

### Submission Flow
1. Fetch the problem submit page: `https://toph.co/p/{problem_id}`.
2. Scrape CSRF token.
3. Send POST multipart or urlencoded request to submit code.
4. Scrape the resulting submission ID from redirect.

### Polling Flow
1. Fetch `https://toph.co/s/{submission_id}`.
2. Scrape verdict details, execution time, and memory.

### Problem Parser (`parser.go`)
- Method: `ParseTophProblem(ctx, problemID)`
- Scrape `https://toph.co/p/{problemID}`.
- Parse title, description (convert HTML to markdown), sample cases, time/memory limits.

---

## 3. QOJ Bot (`qoj.go` & `parser.go` & `import.go`)

### Authentication
QOJ is based on Universal Online Judge (UOJ).
- **Login Flow**:
  1. GET `https://qoj.ac/login`.
  2. Scrape CSRF token (`_token`).
  3. POST to `https://qoj.ac/login` with `username`, `password`, and `_token`.
  4. Maintain session cookies.
- **State Check**: GET `https://qoj.ac/profile` and verify login status.

### Submission Flow
1. Fetch `https://qoj.ac/problem/{problem_id}`.
2. Scrape CSRF token (`_token`).
3. POST to `https://qoj.ac/problem/{problem_id}/submit` with `_token`, `answer`, `language`.
4. Scrape the submission ID from redirect.

### Polling Flow
1. Fetch `https://qoj.ac/submission/{submission_id}`.
2. Scrape verdict, memory, and time.

### Problem Parser & Importer (`parser.go` & `import.go`)
- Method: `ParseQOJProblem(ctx, problemID)`
- Scrape `https://qoj.ac/problem/{problemID}`.
- **PDF Problem Statements**:
  - If the statement is a PDF, find the direct PDF link (e.g. `https://qoj.ac/problems/files/{id}/problem.pdf`).
  - Set `problem.Description = "https://qoj.ac/problems/files/{id}/problem.pdf"`.
  - Do NOT download the PDF to AIOJ servers.
- **HTML Problem Statements**:
  - If HTML, convert description, input/output formats, and constraints to markdown.

---

## 4. Frontend PDF Rendering

In both `web/src/pages/ProblemDetail.tsx` and `web/src/pages/ContestProblem.tsx`:
- Before rendering `ReactMarkdown`, check if `problem.description` is a direct PDF link:
  ```ts
  const isPdf = problem.description && (
      problem.description.startsWith('http') && 
      (problem.description.endsWith('.pdf') || problem.description.includes('/problem.pdf'))
  );
  ```
- If `isPdf` is true, render a PDF viewer instead of markdown:
  ```tsx
  <div className="space-y-4">
      <div className="flex justify-between items-center bg-gray-50 dark:bg-gray-800 p-3 rounded-lg border border-gray-200 dark:border-gray-700">
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300">PDF Problem Statement</span>
          <a 
              href={problem.description} 
              target="_blank" 
              rel="noreferrer" 
              className="text-sm text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 font-semibold"
          >
              Download PDF
          </a>
      </div>
      <iframe 
          src={problem.description} 
          className="w-full h-[800px] border-0 rounded-lg shadow-sm bg-white" 
          title="Problem Statement"
      />
  </div>
  ```
