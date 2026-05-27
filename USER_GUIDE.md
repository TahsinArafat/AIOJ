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

---

## 3. Problem Setter Workspace (`/setter`)

Accessible to both `teacher` and `admin` roles. Click **Setter Workspace** in the top navigation bar.

### Managing Problems
- The workspace lists all problems in the system.
- Click **+ Create Problem** to add a new problem to the judge (Note: the detailed creation form is being actively developed).
- You can view and edit problems. 

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
