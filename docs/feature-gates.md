# Feature gates

Every optional surface of AIOJ — contests, gym, IDE, blog, AI generation, and so
on — can be switched off, limited to a set of roles, or limited to a rating
range, without a deploy. This document covers how the mechanism is put together
and how to add a new gateable feature.

## The three layers

| Layer | Lives in | What it is |
|-------|----------|------------|
| **Feature** | `internal/feature/feature.go` (registry) | Static description: key, label, category, default policy, whether the API enforces it. Compiled in. |
| **Policy** | `system_settings` table, key `feature.<key>` | The admin's decision: `enabled`, `roles`, `min_rating`, `max_rating`, `message`. |
| **Decision** | computed per request | The answer for one principal at one moment. |

Keeping them apart is what makes the system modular: the registry is the single
source of truth, and it drives the API gate, the SPA's `/api/features` call and
the admin panel at the same time. A feature cannot be half-wired.

Policies reuse the existing `system_settings` table, so there is no migration
and every change is already captured by the admin audit log.

## Decision order

`feature.Decide` applies the rules in this order, and the order is part of the
contract:

1. **Master switch.** `enabled: false` denies everyone, including admins. "Turn
   this off" has to mean off.
2. **Admin bypass.** Admins skip the role list and the rating window. An admin
   must always be able to reach the surface they are configuring — otherwise a
   rating gate could lock the only person who can undo it out of the page that
   explains what happened.
3. **Requires login**, then **roles**, then **rating window**.

Anonymous callers hitting a role- or rating-restricted feature get
`login_required` rather than `role`/`rating`, because "sign in" is the action
they can actually take.

## Enforcement

**Server.** `middleware.FeatureGate(jwtManager, resolver, key)` is mounted per
route group in `internal/api/router.go`. It resolves the caller itself (it reads
the bearer token when no auth middleware has run yet), because most gated groups
mix public and authenticated routes. A denial is `403` with a machine-readable
body:

```json
{ "error": "this feature is disabled", "feature": "gym", "reason": "disabled", "message": "" }
```

`403` rather than `404`: hiding the route would make a misconfiguration
indistinguishable from a typo in a link, and the point of the gate is a policy
decision the caller is allowed to understand.

**Client.** `FeaturesProvider` (`web/src/context/FeaturesContext.tsx`) caches
`GET /api/features`, which returns the verdict already resolved for the caller.
`<FeatureGate feature="gym">` wraps routes, and the navbar hides denied entries.
The client **fails open**: if the request fails, gates render their children.
The API still enforces, so the worst case is a clear error page instead of a
blank site.

## Adding a feature

1. Add an entry to the registry in `internal/feature/feature.go` and a `Key…`
   constant next to the others. Default it to `Unrestricted` — a new gate must
   never ship closed, or an existing surface disappears on deploy.
2. Set `APIEnforced: true` and mount `r.Use(gate(feature.KeyX))` in
   `internal/api/router.go` if a dedicated API exists. If the surface has no
   API of its own (the IDE, for example), leave it `false`: the admin panel then
   labels the row **UI only** so nobody believes they have locked down more than
   they have.
3. Wrap the route in `web/src/App.tsx` with `<FeatureGate feature="x">` and, if
   it is in the navigation, add it to the `show` map in `Navbar.tsx`.
4. Run `go test ./internal/feature/...` — `TestRegistryIsConsistent` fails if
   the key is duplicated, the default is invalid, or a constant points at a
   missing entry.

## Operational notes

- **Propagation.** Writes invalidate the resolver on the instance that served
  them, so a single-replica deployment applies a change immediately. With
  several replicas the worst case is `DefaultTTL` (15s).
- **Availability over policy.** If `system_settings` cannot be read, the
  resolver falls back to the registry defaults (everything open) rather than
  denying the site. The same applies to a failed rating lookup: it is treated as
  "unrated", which keeps a rating gate closed rather than open.
- **Admin endpoints are never gated.** `/api/admin/features` stays reachable
  regardless of policy, so a bad decision is always recoverable from the UI.
- **Bulk reset.** Deleting a `feature.<key>` row (via `DELETE
  /api/admin/features/{key}`) restores the built-in default.
