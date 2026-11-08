# Legal documents

These are the operator-facing copies of AIOJ's legal pages. The versions the app
actually serves are embedded from `internal/api/handler/legal/*.md` at build
time and exposed as `GET /api/legal/{terms_of_service,privacy_policy,dmca}`.

**Keep the two in sync.** Edit the served copy, then mirror it:

```bash
cp internal/api/handler/legal/terms_of_service.md docs/legal/TERMS_OF_SERVICE.md
cp internal/api/handler/legal/privacy_policy.md   docs/legal/PRIVACY_POLICY.md
cp internal/api/handler/legal/dmca.md             docs/legal/DMCA.md
```

`web/src/pages/legal/legalLinks.test.ts` reads the served files and fails the
frontend test suite if a cross-document anchor no longer resolves, so run
`make test-frontend` after editing headings.

## Before you launch an instance

The documents are written to be usable as-is, but they describe **an operator**
rather than naming one. A deployment should fill in:

1. **Identity and contact.** Privacy Policy §14 and Terms §17 point at "the
   operator's published contact channel". Publish one (support page, footer link,
   or an address in your deployment docs) and, if you are a company, add your
   registered name and address.
2. **Governing law.** Terms §13 uses the law and courts of the operator's
   jurisdiction. Name the jurisdiction explicitly if you want certainty.
3. **DMCA agent.** DMCA §4 assumes a designated agent. Instances serving US users
   should register one with the U.S. Copyright Office and keep the registration
   current; the § 512(c) safe harbour depends on it.
4. **Integrations.** Privacy §6 lists what *can* leave the instance. Disable
   what you do not run, or say plainly which of OAuth, SMTP, Sentry, MOSS,
   virtual judge, and webhooks are active on your deployment.
5. **Retention.** Privacy §8 states defaults (14-day AI-generation records,
   short-lived logs). Adjust if your configuration differs.
6. **Age limits.** Terms §2 and Privacy §11 set 13 as the floor, with the higher
   EEA/UK digital-consent age. Course and school deployments may need a higher
   age or guardian consent.
7. **Local law.** GDPR/UK GDPR, CCPA/CPRA, DSA, and consumer-protection rules all
   add obligations this template only sketches. Have counsel review before a
   public launch.

## Not legal advice

These documents are a well-structured starting point, not a substitute for advice
from a lawyer licensed in your jurisdiction.
