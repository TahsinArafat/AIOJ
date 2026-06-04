# Design Spec: Profile settings, teams, groups, and rankings customization and control

## Overview
This document specifies the design for implementing:
1. **Profile settings & Customization** under `/profile` (avatar, bio, first/last name, country, city, organization, GitHub link, display email setting, hide problem tags option, and password change).
2. **Rankings customization**: Dynamic filtering of users on `/rankings` by country and organization.
3. **Private Teams & Invitations**: Inviting members to teams, requesting to join private teams, and full team management (Update/Delete).
4. **Private Groups & Controls**: Group manager roles, group contest management, group invite code/link, and join policies (auto-approve vs manual approve).

---

## 1. Database Schema Changes

We will introduce a database migration (`internal/store/migrations/000048_profile_teams_groups_customization.up.sql`) to implement the following changes:

### A. User Profile Settings
Add customization fields to `user_profiles`:
```sql
ALTER TABLE user_profiles 
ADD COLUMN first_name VARCHAR(64) DEFAULT '',
ADD COLUMN last_name VARCHAR(64) DEFAULT '',
ADD COLUMN country VARCHAR(64) DEFAULT '',
ADD COLUMN city VARCHAR(64) DEFAULT '',
ADD COLUMN organization VARCHAR(128) DEFAULT '',
ADD COLUMN github_url VARCHAR(256) DEFAULT '',
ADD COLUMN show_email BOOLEAN DEFAULT FALSE,
ADD COLUMN show_tags BOOLEAN DEFAULT TRUE;
```

### B. Team Settings and Membership States
Add `is_public` to `teams` and update `team_members` to allow pending state roles:
```sql
-- Add is_public column to teams
ALTER TABLE teams ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT TRUE;

-- Update role constraint on team_members to include invitation/request states
ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_role_check;
ALTER TABLE team_members ADD CONSTRAINT team_members_role_check CHECK (role IN ('owner', 'captain', 'member', 'invited', 'requested'));
```

### C. Group Settings and Join Policies
Add invite link generation capabilities and dynamic join policies to `groups`:
```sql
-- Add invite_code and join_policy to groups
ALTER TABLE groups 
ADD COLUMN invite_code VARCHAR(8) UNIQUE,
ADD COLUMN join_policy VARCHAR(16) NOT NULL DEFAULT 'auto_approve' CHECK (join_policy IN ('auto_approve', 'manual_approve'));

-- Update role constraint on group_members to include manager, invited, requested
ALTER TABLE group_members DROP CONSTRAINT IF EXISTS group_members_role_check;
ALTER TABLE group_members ADD CONSTRAINT group_members_role_check CHECK (role IN ('owner', 'manager', 'member', 'invited', 'requested'));
```

---

## 2. Backend Go REST API Additions

### A. Profile Settings & Password Change
We will introduce API endpoints for profile mutation:
* `GET /api/users/profile` -> Returns full user details and profile info for the logged-in user.
* `PUT /api/users/profile` -> Updates profile fields (`first_name`, `last_name`, `country`, `city`, `organization`, `github_url`, `bio`, `avatar_url`, `show_email`, `show_tags`).
* `PUT /api/users/profile/password` -> Updates user password. Takes `{ current_password, new_password }`.

### B. Rankings Filtering
Modify `GET /api/rankings` to support query params:
* `?country={country}` -> Filters the rankings list by selected country.
* `?organization={org}` -> Filters the rankings list by selected organization.

### C. Teams Customization
Modify `/api/teams` endpoints:
* `PUT /api/teams/{id}` -> Updates team name, description, avatar, `is_public`. Only available to team owner/captain.
* `DELETE /api/teams/{id}` -> Deletes the team. Only available to team owner.
* `POST /api/teams/{id}/invite` -> Send invitation. Request: `{ username }`. Adds user to `team_members` with role `invited`.
* `POST /api/teams/{id}/request` -> Request to join a private team. Adds user to `team_members` with role `requested`.
* `POST /api/teams/{id}/respond` -> Approve or reject an invite/request. Request: `{ user_id, action: "approve" | "reject" | "accept" | "decline" }`.
* Modify `GET /api/teams/{id}` -> Should hide/exclude private team members unless the current user is already a member.

### D. Groups Customization
Modify `/api/groups` endpoints:
* Add `invite_code` generation on group creation.
* `POST /api/groups/{id}/invite` -> Manager/Owner invites a user by username (role `invited`).
* `POST /api/groups/{id}/respond` -> Respond to invitations/join requests. Request: `{ user_id, action: "approve" | "reject" | "accept" | "decline" }`.
* `POST /api/groups/join-code` -> Join a group using an `invite_code`. Request: `{ invite_code }`. If group policy is `auto_approve`, user is added directly as a `member`. If `manual_approve`, added as `requested`.
* Modify `AddContest` and `RemoveContest` -> Authenticated user must have role `owner` or `manager` in the group (or be a global admin).

---

## 3. Frontend React Pages Updates

### A. Profile Settings Page (`web/src/pages/Profile.tsx`)
* Revamp `/profile` into a modern settings view with sub-panels:
  * **Edit Profile**: Fields for First Name, Last Name, Country, City, Organization, GitHub, Avatar URL, Bio, and display email toggle.
  * **Preference Settings**: Spoiler toggle for "Show problem tags".
  * **Change Password**: Fields for current and new password.
  * **My Invites & Requests**: View pending invitations to teams and groups and click "Accept" or "Decline".

### B. Rankings Page (`web/src/pages/Rankings.tsx`)
* Add Country filter dropdown.
* Add Organization filter dropdown.
* Support instant search and fetch rankings matching these filters.

### C. Teams Management (`web/src/pages/TeamDetail.tsx` and `web/src/pages/TeamCreate.tsx`)
* **Create Team**: Add `is_public` toggle.
* **Team Details**:
  * Add "Settings" section for Owners/Captains to update name, description, toggle visibility, or delete team.
  * Add "Invite Member" input (autocomplete or input username) to invite another person.
  * Add "Join Requests" panel showing pending user requests with "Approve" / "Reject" buttons.
  * For private teams, show a "Request to Join" button instead of "Join Team" for non-members.

### D. Groups Management (`web/src/pages/GroupDetail.tsx` and `web/src/pages/GroupCreate.tsx`)
* **Create Group**: Add `join_policy` toggle (Auto-Approve vs Manual-Approve).
* **Group Details**:
  * Display the Invite Code / Invite Link (e.g. `/groups/join?code=XYZ`) to managers/owners so they can copy and share it.
  * Add "Invite Member" option for managers/owners.
  * Add "Join Requests" tab for managers/owners to review manual join requests.
  * Modify Contests tab so both `owner` and `manager` roles can add or remove group contests.

---

## 4. Verification & Testing Plan
1. **Schema Check**: Apply migration `000048_profile_teams_groups_customization.up.sql` and verify database tables.
2. **API Verification**: Check that update profile, password change, invitation, join request, and rankings filters function correctly.
3. **Frontend Integration**: Build and verify all page layouts.
