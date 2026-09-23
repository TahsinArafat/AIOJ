
Next: A.31 cookie consent banner.
- **Account deletion (A.30): DONE** — `DELETE /api/users/me` (password re-auth + `confirm=true`; OAuth-only may blank password). Migration `000062_user_delete_cascade` (personal rows CASCADE, shared authors SET NULL). `UsersDeletionHandler` + `UserCascadeDeleter`. Profile Danger Zone UI → clearTokens → home. Tests: 403 wrong password, 400 no confirm, 200 happy, 401 no claims. Verified: go build/test, tsc. Next: A.31 cookie consent banner.
cookie consent banner.
