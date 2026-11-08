# Legal documents

These are the operator-facing copies of AIOJ's legal pages. The versions the app serves are the markdown files in `internal/api/handler/legal/`, which the Go API embeds and exposes as `GET /api/legal/{terms_of_service,privacy_policy,dmca}`.

**Keep the two in sync.** Edit the markdown in `internal/api/handler/legal/`, then mirror it here:

```bash
cp internal/api/handler/legal/terms_of_service.md docs/legal/TERMS_OF_SERVICE.md
cp internal/api/handler/legal/privacy_policy.md   docs/legal/PRIVACY_POLICY.md
cp internal/api/handler/legal/dmca.md             docs/legal/DMCA.md
```

The frontend serves the same files: `web/src/pages/legal/legalSources.ts` compiles them into the bundle, and `npm run build` prerenders each document into static HTML (`web/src/prerender/legalEntry.tsx`). There is still exactly one source of truth.

Run `make test-frontend` after editing. Two contract tests read these files and fail on drift:

- `legalLinks.test.ts` — every cross-document anchor (`/legal/terms#4-your-content-and-intellectual-property`) must resolve to a real heading.
- `legalEntry.test.ts` — the prerendered HTML must contain the full policy text, the required privacy topics, and no placeholder wording.

## How the pages are served

The app is a client-rendered SPA, so a URL served straight from `index.html` contains an empty `<div id="root">`. That is invisible to a browser but not to a crawler that does not execute JavaScript — it sees a page with no policy text and rejects it as "does not have sufficient content".

The build fixes that: `npm run build` runs an SSR build (`vite.prerender.config.ts`) and then `scripts/prerender-legal.mjs`, which writes:

```
dist/legal/privacy.html
dist/legal/terms.html
dist/legal/dmca.html
```

`web/nginx.conf` serves them with `try_files $uri $uri.html /index.html;`, so `/legal/privacy` returns the complete document in the response body, with no redirect and no JavaScript. The in-app route renders the same component for client-side navigation.

Verify after a build:

```bash
curl -s https://your-domain/legal/privacy | wc -c          # ~42 kB, not ~2 kB
curl -s https://your-domain/legal/privacy | grep -c '<h2'  # ~19 headings
```

## Before you launch an instance

The documents are written to be usable as-is, but they describe **an operator** rather than naming one. A deployment should fill in:

1. **Identity and contact.** Privacy Policy §17 and Terms §17 point at "the contact details published for this Instance". Publish one (support page, footer link, or an address in your deployment docs) and, if you are a company, add your registered name and address.
2. **Governing law.** Terms §13 uses the law and courts of the operator's jurisdiction. Name the jurisdiction explicitly if you want certainty.
3. **DMCA agent.** DMCA §4 assumes a designated agent. Instances serving US users should register one with the U.S. Copyright Office and keep the registration current; the § 512(c) safe harbour depends on it.
4. **Integrations.** Privacy §7 lists what *can* leave the instance. Disable what you do not run, or say plainly which of OAuth, SMTP, Sentry, MOSS, virtual judge, and webhooks are active on your deployment.
5. **Retention.** Privacy §9 states defaults (14-day AI-generation records, short-lived logs). Adjust if your configuration differs.
6. **Age limits.** Terms §2 and Privacy §14 set 13 as the floor, with the higher EEA/UK digital-consent age. Course and school deployments may need a higher age or guardian consent.
7. **Local law.** GDPR/UK GDPR, CCPA/CPRA, DSA, and consumer-protection rules all add obligations this template only sketches. Have counsel review before a public launch.

## App-store declarations

Privacy §5 ("Device permissions and app stores") declares **no sensitive permissions**, matching the software: AIOJ is a website and requests no location, contacts, camera, microphone, storage, advertising-ID, or push-notification access. The same section carries the data-categories table that maps to a store data-safety form.

If you ship a native wrapper that adds a permission, update §5 before the store review — the declaration and the build have to agree.

## Not legal advice

These documents are a well-structured starting point, not a substitute for advice from a lawyer licensed in your jurisdiction.
