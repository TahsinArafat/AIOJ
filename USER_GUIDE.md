# AIOJ - User & Administration Guide

Welcome to AIOJ! This guide explains how to use the platform as a normal user, a problem setter (teacher), and an administrator.

## 1. Logging In

- Open the website.
- Click **Login** in the top right.
- To access the admin and setter panels immediately, log in with the default admin credentials:
  - **Username:** `admin`
  - **Password:** `admin_secret`

*Note: Once you log in, the navigation bar will automatically update to show your role-specific links.*

---

## 2. Admin Dashboard (`/admin`)

Only users with the `admin` role can access this panel. Click **Admin** in the top navigation bar.

### User Management
- View a list of all registered users.
- Use the **Role** dropdown to change a user's role:
  - `user`: Standard contestant.
  - `teacher`: Problem Setter (can create problems and contests).
  - `admin`: Full system access.
  - `bot`: Reserved for VJudge remote submission bots.

### Setter Applications
- Normal users can apply to become Problem Setters.
- In the Admin Dashboard, you will see a table of **Setter Applications**.
- Click **Approve** to grant them the `teacher` role, or **Reject** to decline.

### AI Models (`/admin#ai-models`)

This tab lets you register and manage the AI endpoints that power the **AI Problem Generator**. Multiple models can be added; the one with the highest **priority** that is currently **enabled** is automatically selected for each generation request.

**Adding a model:**
1. Open **Admin → AI Models** and click **+ Add Model**.
2. Fill in the fields:
   | Field | Description |
   |---|---|
   | **Display Name** | A friendly label shown in the panel (e.g. `CF Generator v1`). |
   | **Endpoint URL** | The OpenAI-compatible base URL of your model server, e.g. `http://localhost:8080/v1`. Must serve `POST /chat/completions`. |
   | **API Key** | Authentication token. Leave blank for self-hosted models that don't require one. |
   | **Model Name** | The exact model ID sent in every API request (e.g. `codeforces-problem-generator-v1` or `gpt-4o`). |
   | **Priority** | Integer — higher value = preferred when multiple models are enabled. Default is `0`. |
   | **Description** | Optional notes about the model's training, strengths, or intended use. |
   | **Enabled** | Tick to make this model available for generation. Untick to disable without deleting. |
3. Click **Test Connection** to send a trivial ping to the endpoint. A green ✓ means the server responded correctly; a red ✗ shows the error message inline.
4. Click **Add Model** to save.

**Managing existing models:**
- Click the **Enabled / Disabled** chip in the table to toggle a model on or off instantly (no page reload needed).
- Click the **pencil** icon to edit all fields. Leave the API Key blank to preserve the stored secret.
- Click the **trash** icon to permanently delete a model (confirmation required).

**How the active model is resolved:**
- Every generation request queries the database at call time for the enabled model with the highest priority.
- If no model is enabled, the generation endpoint returns: `"no AI model is enabled — add one in Admin › AI Models"`.
- API keys are stored in the database and are never exposed in the list view; they are only retrievable by opening the edit form.

---

## 3. Problem Setter Workspace (`/setter`)

Accessible to both `teacher` and `admin` roles. Click **Setter Workspace** in the top navigation bar.

### Managing Problems
- The workspace lists all problems in the system.
- Click **+ Create Problem** to add a new problem to the judge.
- You can view and edit problems.

### AI Problem Generator (`/generate/problem`)

Admins and setters can generate a complete Codeforces-style problem — statement, reference solution, and editorial — using the configured AI model. Access it via the **✦ Generate with AI** button in the Setter Workspace header.

**How to generate a problem:**

1. **Algorithmic Topics** — Click quick-pick chips (dp, graphs, greedy, etc.) or type a custom tag and press Enter. At least one tag is required.
2. **Difficulty Rating** — Drag the slider or click a rating chip (800–3500 Codeforces scale). The colour and label indicate the tier (Pupil, Expert, Master, etc.).
3. **Problem Type** — Choose `standard` (batch judge), `interactive` (online interactor), or `output_only`.
4. **Test Case Count** — Number of example/hidden test cases to describe in the statement (1–100).
5. **Subtask Scoring** — Tick this to ask the model to write 2–4 subtasks with partial credit.
6. **Additional Guidance** — Optional free-form hint, e.g. `"grid DP on N×M array"` or `"the insight involves modular inverses"`. The AI incorporates this into the problem scenario.

Click **✦ Generate Problem**. Generation takes roughly 15–60 seconds depending on the model.

**After generation:**
- A success banner appears with a **"Open in Setter Workspace →"** button that takes you directly to the draft problem.
- The problem is saved as a **draft** (`visible = false`) and is never visible to contestants until an admin or setter explicitly publishes it.
- A reference C++17 solution and editorial are generated and saved automatically alongside the problem.
- Every generated field (title, statement, time/memory limits, solution, editorial) can be edited freely in the Setter Workspace before publishing.

> **Note:** At least one AI model must be added and enabled in **Admin → AI Models** before the generator will work.

### Collaboration & Privacy (Codeforces Standard)
AIOJ uses a granular permission system for problems:
- **Owner**: The creator of the problem. Can edit, delete, and add other collaborators.
- **Co-author**: Has edit access to the problem statement and can upload testcases.
- **Tester**: Has read-only access to private problems to test them before a contest.
- *Privacy*: Problems are hidden from normal users until they are marked as `visible` or added to an active contest.

---

## 4. Contests (`/contests`)

- Click **Contests** in the navigation bar to see a list of upcoming, running, and ended contests.
- **Registration**: You must register for a contest before you can participate.
- **Scoreboard**: 
  - Each contest features a real-time scoreboard.
  - **Freeze Time**: Contests can have a "freeze time" (e.g., the last 1 hour). During this time, the scoreboard will not update for other users, and pending submissions will appear as `?`. The full results are revealed when the contest ends.

---

## 5. Solving Problems (`/problems`)

- Browse the list of public problems.
- Click on a problem to read the description, input/output formats, and sample cases.
- Use the integrated **CodeMirror** editor to write your solution.
- Supported languages include C++, Python, Java, Rust, Node.js, and more.
- Click **Submit** to send your code to the secure sandbox. The result (e.g., `Accepted`, `Wrong Answer`, `Time Limit Exceeded`) will update in real time!
