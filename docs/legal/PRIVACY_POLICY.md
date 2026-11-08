# AIOJ Privacy Policy

**Last updated:** 2026-11-08

## The short version

- We collect what a judge needs to work: your account, your submissions, and
  enough technical data to keep the service fast and safe.
- **We do not sell your data.** There are no advertising networks, no cross-site
  trackers, no fingerprinting, and no data brokers on this instance.
- **We do not train machine-learning models on your code or content.**
- **You own your code.** We only take the narrow licence needed to compile, run,
  store, and display it — see our [Terms of Service](/legal/terms).
- **You can export or delete your data yourself**, at any time, from Settings.
- AIOJ is [open-source](/legal/terms#14-about-the-aioj-software-and-self-hosted-instances)
  and self-hostable. If you are using an instance run by someone other than the
  AIOJ project (a university, a club, a company), **that operator is the data
  controller**, and this page describes what the software does by default.

## 1. Scope and who is responsible

This policy covers the AIOJ web client and the API that serves it (together,
the "Instance"). It does not cover third-party sites we link to, including the
remote judges you can submit to.

The **operator** of this Instance — the person or organisation that deploys and
administers it — decides why and how personal data is processed. The operator's
contact details are the ones published for this Instance (its documentation,
footer, about page, or your administrator). Where this policy says "we", it means
that operator.

If your organisation runs AIOJ for you (for example a class or a private contest
group), that organisation may have its own policy that adds to this one. Their
policy prevails for data they control.

**This policy is public.** You can read it without an account, and it is not
behind a login or an app install. The same is true of our
[Terms of Service](/legal/terms) and
[DMCA / Copyright Policy](/legal/dmca).

## 2. What we collect

| Category | What it includes |
|---|---|
| **Account** | Username, email address, password hash (bcrypt with a per-user salt — never the password itself), role, account timestamps, email-verification state. If you enable two-factor authentication, a TOTP secret and hashed one-time backup codes. If you sign in with GitHub or Google, the provider's subject identifier and the profile fields you consent to share (such as email and avatar). |
| **Profile** | Display name, bio, country, avatar URL, links, preferred language and theme. |
| **Activity** | Submissions (source code, language, verdict, per-test timing and memory), code runs, hacks and counter-tests, contest registrations and standings, rating history, virtual contests, problems and contests you author, comments, blogs, editorials, clarifications, teams, groups, classes, friendships, achievements. |
| **Settings** | Notification preferences (which events email you and which appear in-app), API keys (stored hashed), webhook endpoints and delivery records, and remote-judge bot linkage. |
| **AI-assisted tooling** | Prompts and generated output when you use problem-generation features. Generation records are kept only for the configured retention window (14 days by default) unless you save the result. |
| **Technical** | IP address, user agent, request path, response status, and timestamps in server logs; IP address and actor in the admin audit log (administrative actions only). These support rate limiting, abuse prevention, and security investigations. |
| **Browser storage** | Access and refresh tokens, a CSRF token cookie, your cookie-consent choice, and UI preferences (theme, language, editor layout). See [Cookies and local storage](#6-cookies-and-local-storage). |

## 3. What we do not collect, and what we do not do

**Data we do not ask for and do not want.** Do not post special-category data
(health, religion, political opinions, biometrics, sexual orientation) or anyone
else's personal information. If you do, you are responsible for it, and we may
remove it.

**Things we do not do with your data:**

- We do **not** sell, rent, or trade personal data.
- We do **not** run advertising or cross-site tracking, and we do not embed
  third-party ad pixels, social widgets, or session-replay scripts.
- We do **not** use your submissions, blogs, editorials, or messages to train
  models, ours or anyone else's.
- We do **not** build a shadow profile from your browsing on other sites. There
  is no advertising identifier attached to your account.
- We do **not** read your code except as needed to judge it, to run a plagiarism
  check that you or a contest organiser triggered, or to investigate abuse.
- We do **not** publish private content. What other people can see is decided by
  the visibility settings on each item; see
  [Public content and search engines](#8-public-content-and-search-engines).
- We do **not** send marketing email. Every message we send is transactional or a
  notification you asked for; see
  [Email, notifications and your choices](#13-email-notifications-and-your-choices).

## 4. Why we use your data

| Purpose | Data used | Legal basis (where GDPR/UK GDPR applies) |
|---|---|---|
| Create and secure your account, sign you in, keep your session alive | Account, browser storage | Performance of a contract |
| Compile and judge submissions, run contests, score and rate participants | Activity, technical | Performance of a contract |
| Deliver contest reminders, rating changes, problem updates, and comment replies you subscribed to | Account, settings | Performance of a contract |
| Detect cheating and plagiarism, defend contests from abuse | Activity, technical | Legitimate interests (fair competition) and contract |
| Rate-limit, block attacks, investigate security incidents | Technical, account | Legitimate interests (security) and legal obligation |
| Send account email (verification, password reset, two-factor changes, security notices) | Account | Contract and legal obligation |
| Answer support requests and appeals | Whatever you send us | Legitimate interests (support) or consent |
| Optional product analytics | Aggregated usage events | **Consent only** — nothing runs before you accept |
| Comply with law and enforce our terms | Any of the above | Legal obligation and legitimate interests |

We do not make decisions with legal or similarly significant effects about you
by automated means. Automated judging decides whether a program is correct, not
whether you are entitled to a service. Contest disqualifications are reviewed by
a human.

## 5. Device permissions and app stores

AIOJ is a web application. It runs in your browser, and **it requests no device
permissions.**

- We do **not** access your location (precise or approximate), contacts, camera,
  microphone, photos or media library, files outside the ones you pick, SMS,
  call logs, calendar, health data, or body sensors.
- We do **not** read your installed applications or your advertising ID, and we
  do **not** ask for access to "all files" on your device.
- When you choose to upload something — a problem statement, a test-case archive,
  an avatar — your browser's file picker hands us exactly the file you selected.
  That is a user-initiated transfer, not background access.
- Notifications are delivered inside the app over an authenticated WebSocket
  while you are signed in. AIOJ does not ask for system push-notification
  permission.
- If a companion mobile or desktop build of AIOJ is published for an app store,
  this section is the permission declaration that applies to it: it declares no
  sensitive permissions. Any build that needs more must update this policy first.

### Data categories at a glance

This table maps what the Service collects to the categories used by app-store
data-safety forms. It describes the software's default behaviour; an operator may
disable optional integrations.

| Data type | Collected | Linked to your identity | Used for | Shared with third parties |
|---|---|---|---|---|
| Email address, username, display name | Yes | Yes | Account and sign-in | Only the mail provider that delivers account email |
| Password | Stored only as a salted bcrypt hash | Yes | Authentication | No |
| Source code and submissions | Yes | Yes | Judging, standings, plagiarism checks | Only the remote platform you explicitly submit to (virtual judge) |
| Contest and practice activity, ratings | Yes | Yes | Scoring, standings, profile | No |
| Content you post (blogs, comments, editorials, problems) | Yes | Yes | Publishing and discussion | Public, if you publish it |
| IP address and user agent | Yes | Yes, in logs | Security, rate limiting, abuse prevention | Only the hosting/monitoring provider |
| Crash and error diagnostics | Only when an operator enables Sentry | Limited | Stability | The configured error-monitoring provider |
| Analytics events | Only after you accept the banner | Aggregated | Product improvement | No |
| Location, contacts, health, biometrics, advertising ID | **No** | — | — | — |

## 6. Cookies and local storage

We use the minimum needed to keep you signed in and the site working. There are
**no third-party cookies** and no tracking cookies.

| Name | Where | Purpose | Type |
|---|---|---|---|
| `csrf` | Cookie | Proves that a state-changing request came from our own pages (CSRF protection). | Essential |
| `access_token` | localStorage | Short-lived signed token that authenticates your API requests. | Essential |
| `refresh_token` | localStorage | Lets you stay signed in without re-entering your password; revoked on sign-out. | Essential |
| `cookie-consent` | localStorage | Remembers whether you accepted or declined optional analytics, so we do not ask again. | Essential |
| theme / language / editor preferences | localStorage | Remembers how you like the interface. | Functional |

**Optional analytics.** If the operator enables analytics, it is first-party,
aggregated, and loaded only after you press **Accept**. Declining or dismissing
the banner leaves every feature working. You can change your mind at any time by
clearing the `cookie-consent` value in your browser's site-data settings, which
makes the banner appear again.

Because sign-in tokens are kept in browser storage, treat a shared or public
computer as untrusted: always sign out, and never paste a token into a script or
a bug report. Signing out revokes the refresh token on the server as well as
clearing it locally.

## 7. When data leaves this Instance

We share data only with the services needed to run AIOJ, and only the minimum
each one needs. Depending on how the operator has configured the Instance, that
can include:

- **Sign-in providers** — GitHub or Google, if you choose OAuth sign-in. They
  receive the fact that you are signing in and return your basic profile.
- **Email delivery** — an SMTP provider, to deliver verification, password-reset,
  and notification mail. It sees your email address and the message content.
- **Error monitoring** — Sentry, when enabled. Error reports can include request
  metadata (path, user agent, truncated identifiers). They do not include your
  password.
- **Plagiarism checking** — a check compares submissions with an internal LCS
  engine when a contest organiser requests one. If the external MOSS service is
  enabled, the submissions being compared are also sent to it. MOSS is off
  unless configured.
- **Remote judges (virtual judge)** — when you submit through virtual judge,
  your source code is sent to the platform you chose (Codeforces, AtCoder, CSES,
  Toph, or QOJ) using the bot account that platform requires. This is you
  instructing a transfer, and the target platform's terms and privacy policy
  apply to it.
- **Webhooks you configure** — your own endpoints (and, for organisations, the
  endpoints their administrators configure) receive event payloads.
- **Hosting, database, backups, and monitoring** — the infrastructure provider
  the operator uses to run the Instance.
- **Legal and safety** — disclosures when required by law, to enforce our terms,
  or to protect users from harm. We aim to notify you unless we are legally
  barred from doing so.

We never share data for advertising or for someone else's marketing, and we do
not disclose your email address to other users.

## 8. Public content and search engines

A contest platform is partly public by design. The following are visible to
anyone, including people who are not signed in:

- your username, public profile, rating, and rating history;
- standings, contest results, and achievements;
- submissions, runs, and solutions whose visibility is public (this is how
  upsolving works after a contest);
- blogs, editorials, comments, clarifications, and any problem or contest you
  author or publish.

The Instance exposes a sitemap and Atom feeds for public content, so search
engines and feed readers may index and cache it. Removing a post stops us serving
it, but we cannot force third parties to drop their cached copies. **Do not post
personal information you are not prepared to make public.**

Private submissions, draft problems, hidden test data, and direct messages (where
the Instance provides them) are not included in the sitemap or feeds.

## 9. How long we keep data

- **Account and profile data** — while your account exists.
- **Submissions, results, and rating history** — while your account exists,
  because standings and rating history are only meaningful as a continuous
  record.
- **Deleted accounts** — deletion is a hard delete. Rows owned by you are
  removed by cascade; a few shared references are set to null so that other
  users' standings and comment threads are not destroyed. Copies in rotating
  backups expire on the operator's backup schedule.
- **AI-generation records** — 14 days by default, unless you save the result.
- **Server logs** — a short operational window, typically days to weeks.
- **Admin audit records** — kept longer, because they are the record of who
  changed what.
- **Security and abuse records** — kept for as long as needed to defend the
  Instance, and to honour bans that protect contests.
- **Legal holds** — data may be preserved longer where law or a live dispute
  requires it.

We do not keep a second copy for analytics, and we do not retain data "just in
case" after you delete your account.

## 10. How we protect it

- Passwords are stored as salted bcrypt hashes (cost 12). There are no
  plaintext or reversible password stores, and OAuth-only accounts have no
  password at all.
- Sessions use short-lived signed access tokens (15 minutes by default) and
  revocable refresh tokens. Signing out revokes the refresh token server-side,
  not just in your browser, and a password reset revokes every session.
- Two-factor authentication (TOTP plus one-time backup codes) is available for
  every account.
- Submitted code executes in a cgroup-isolated sandbox with CPU, memory, and
  process limits, and it is compiled in a separate image from the sandbox.
- Production deployments require strong, non-default secrets; the server refuses
  to boot with a weak or published `JWT_SECRET`.
- State-changing requests are CSRF-protected, submissions and runs are
  rate-limited per user, and administrative actions are recorded in an audit log.
- Transport is encrypted with TLS on any properly configured deployment.

No system is perfectly secure. If you find a vulnerability, please report it
privately to the operator instead of publishing it, and never include real user
data or working credentials in a report. Do not put secrets, API keys, or
personal data in submitted code — submissions are stored and may be public.

## 11. Security incidents

If we become aware of a breach that affects your personal data, we will notify
you without undue delay — by email to the address on your account and by a
notice in the interface — describing what happened, what data was involved, what
we have done, and what you should do (for example, reset your password and
revoke API keys).

Where the law requires it, we also notify the competent supervisory authority.
Because the AIOJ project does not operate your Instance, breach notifications
for a self-hosted deployment come from that deployment's operator.

## 12. Your rights and choices

Depending on where you live, you may have some or all of the following rights:

- **Access and portability.** Settings → *Export my data* returns a JSON snapshot
  of your account, profile, and submission statistics
  (`GET /api/users/me/export`).
- **Deletion.** See [Deleting your account](#deleting-your-account) below.
- **Correction.** Edit your profile and account settings directly. If a field is
  locked, ask the operator.
- **Withdraw consent.** Change your analytics choice through the cookie banner;
  nothing else depends on it.
- **Object or restrict.** Contact the operator if you want processing limited
  while a dispute is resolved.
- **Complain.** You may lodge a complaint with your local data-protection
  authority. We would appreciate the chance to fix the problem first.

We respond to verified requests within 30 days, or the shorter period your local
law requires. We will not degrade your service or your contest standing because
you exercised a privacy right.

Some data cannot be removed on request: a contest result that other users'
ratings depend on, a resolved cheating finding, or records we must keep by law.
Where that applies, we will explain why and minimise what remains.

### Deleting your account

Two paths, both permanent:

1. **In the app:** sign in → Settings → *Delete account* → confirm. You may be
   asked for your password if your account has one. This removes your account and
   the content you own (`DELETE /api/users/me`).
2. **By request:** contact the operator from the email address on your account.
   We verify ownership before acting.

Deletion is immediate for live data and **cannot be undone**. Content you made
public may remain in search-engine caches and in copies other people saved.
Submissions that stand in a finished contest may be retained in anonymised or
minimised form so that standings and ratings stay correct — see
[How long we keep data](#9-how-long-we-keep-data).

## 13. Email, notifications and your choices

**Account email is not optional.** We must be able to send you address
verification, password resets, two-factor change confirmations, and security
notices. If you block these, you may lose access to your account.

**Notification email is optional.** In Settings → Notifications you choose
whether to receive email for contest reminders, rating changes, problem updates,
and blog comments, and separately which of those appear in the app. Every
non-essential email has a matching switch, and turning one off does not affect
your account or contest access.

We do not send marketing or promotional email, and we do not share your address
with advertisers.

## 14. Children

AIOJ is not directed at children under 13. You must be at least 13 to create an
account; in the European Economic Area and the United Kingdom, the digital
consent age (13–16, depending on the country) applies. If you are below the age
of majority where you live, use the Instance only with the involvement of a
parent, guardian, or teacher.

If we learn that a child below the applicable age created an account without the
required consent, we will delete the account and its data. Teachers and
organisers running AIOJ for minors are responsible for obtaining the consents
their law requires.

## 15. International transfers

The Instance stores data where the operator hosts it, which may not be your
country. Some providers listed in
[When data leaves this Instance](#7-when-data-leaves-this-instance) process data
in other countries. Where required, the operator relies on appropriate
safeguards such as standard contractual clauses or an adequacy decision. Ask the
operator for details of the safeguards that apply to this Instance.

## 16. Changes to this policy

We may update this policy to reflect new features or new law. The date at the top
always shows the current version. For material changes we will post an in-app
notice or send an email before the change takes effect. If you disagree with a
change, you can export your data and delete your account.

Earlier versions are available from the operator on request, and every version
of this document is kept in the project's public source repository.

## 17. Contact

- **Privacy questions, access, and deletion requests:** the contact details
  published for this Instance (documentation, footer, about page, or your
  administrator).
- **Security reports:** the operator's security contact, or the AIOJ project's
  private vulnerability channel for bugs in the software itself.
- **Complaints:** the operator first, then your local supervisory authority.

We aim to acknowledge privacy requests within 5 working days and to resolve them
within 30 days.

## 18. If you run this software for other people

Operators of self-hosted instances are independent data controllers. You must
publish your own identity and contact details, adopt a policy that matches the
features you enable, and honour the law that applies to you. The optional
integrations described above (external MOSS, Sentry, analytics, OAuth, remote
judges, webhooks) are inactive until you configure them — enable only what your
policy discloses. The AIOJ project does not receive data from your Instance and
is not a party to your relationship with your users.
