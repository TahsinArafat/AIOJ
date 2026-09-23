
Next: A.31 cookie consent banner.
- **Account deletion (A.30): DONE** — DELETE /api/users/me with password re-auth + confirm=true; migration 000062 (personal CASCADE, shared authors SET NULL); UsersDeletionHandler + UserCascadeDeleter; Profile Danger Zone UI. Tests: 403 wrong password, 400 no confirm, 200 happy, 401. Verified: go build/test, tsc. Next: A.31 cookie consent banner.
