# AIOJ Terms of Service

**Last updated:** 2026-11-08

## The short version

- Be honest, be kind, and do not attack the platform or other people.
- **Your code is yours.** We take only the licence needed to compile, judge, and
  display it — never ownership, never resale, never model training.
- Judging is automated; contest decisions are made by humans and are final after
  review. Ratings can be recalculated when cheating is confirmed.
- The service is provided as-is, without warranties, and our liability is
  limited. Keep your own copy of anything you cannot afford to lose.
- We may suspend accounts that cheat, abuse the sandbox, harass people, or break
  the law.

## 1. Agreement to these terms

By creating an account, signing in, submitting code, or otherwise using this AIOJ
instance (the "Instance"), you agree to these Terms of Service ("Terms") and to
our [Privacy Policy](/legal/privacy). If you do not agree, do not use the
Instance.

These Terms are an agreement between **you** and the **operator** of this
Instance — the person or organisation that runs it. They are not an agreement
with the AIOJ project or its contributors, who license the software and do not
operate your Instance. See
[About the AIOJ software and self-hosted instances](#14-about-the-aioj-software-and-self-hosted-instances).

Additional rules apply where they are published: contest rules, class and
organisation rules, community guidelines, and problem-setter guidelines. Those
rules supplement these Terms; if they conflict, the more specific rule wins for
the activity it governs.

## 2. Eligibility and accounts

You must be at least 13 years old. If you are under the age of majority where you
live, you may use the Instance only with the consent and involvement of a parent,
guardian, or teacher. If you are using the Instance on behalf of an organisation,
you confirm you are authorised to accept these Terms for it.

You agree to:

- give accurate registration information and keep your email address current, so
  verification, password-reset, and security notices reach you;
- keep your password and two-factor codes confidential, and not share, sell, or
  transfer your account;
- use **one account per person**. Organisations may structure their members
  through the Instance's organisation and class features, but you may not run
  alternate accounts to
  evade a ban, a rating limit, or a division restriction, or to manipulate
  standings;
- protect your account with two-factor authentication, especially if it holds
  API keys or organisation admin rights;
- tell the operator promptly if you believe your account or an API key has been
  compromised.

**API keys and webhooks are credentials.** You are responsible for what they do.
Keep them out of public repositories and client-side code, rotate them if they
leak, and remember that a webhook endpoint you register receives event data that
may include usernames and submission metadata.

We may require email verification, invalidate sessions, or force a password reset
when we believe an account is at risk.

## 3. Acceptable use

You may use the Instance to learn, practise, compete, teach, and organise
contests. You may not:

**Cheat or distort competition**

- use another person, another account, or an AI service to obtain answers during
  a rated contest where the rules forbid it;
- share, request, or publish solutions or test data during a contest's active
  period;
- copy another person's submission, or republish someone else's solution as your
  own, including across platforms;
- manipulate ratings, standings, hacks, or votes through collusion, sockpuppets,
  or coordinated submissions;
- scrape other users' submissions or editorials to build a solution corpus for
  contest use.

**Attack the platform or its users**

- attempt to escape, weaken, or overload the judge sandbox; run code designed to
  reach the host, the network, or other users' data; or exploit a judge weakness
  to obtain a verdict you did not earn;
- probe, scan, or test the Instance for vulnerabilities without the operator's
  prior written permission, or run denial-of-service, credential-stuffing, or
  brute-force attacks;
- bypass rate limits, CSRF protection, authentication, or access controls, or
  scrape the API or site at a rate that degrades service for others;
- upload malware, ransomware, miners, or any code whose purpose is to harm.

**Post prohibited content**

- harassment, threats, hate speech, or targeted abuse;
- doxxing or publishing another person's private information without consent;
- sexual content involving minors, non-consensual intimate content, or content
  that is illegal where you or the operator is located;
- spam, deceptive links, phishing, or impersonation of another person, the
  operator, or the AIOJ project;
- content that infringes copyright, trademark, or other rights — see our
  [DMCA / Copyright Policy](/legal/dmca).

**Evade moderation**

- create new accounts, or use VPNs or proxies, to get around a suspension;
- remove or alter copyright, attribution, or licence notices;
- encourage or help anyone else do any of the above.

Reading and modifying the AIOJ source code is expressly allowed — it is
open-source software. Using knowledge of the code to break the sandbox, game
verdicts, or attack a running Instance is not.

If you break these rules we may, in proportion to the harm: remove content, void
a submission or contest result, recalculate rating, restrict features, suspend or
terminate the account, and report serious abuse to the relevant authorities or
institution. Where a contest or class has its own disciplinary process, we may
defer to it.

## 4. Your content and intellectual property

**You keep your rights.** You retain all copyright and other rights in the
source code, comments, blogs, editorials, problem statements, images, and other
material you post ("Your Content"). Nothing in these Terms transfers ownership to
the operator.

**The licence we need — and only that.** By posting Your Content, you grant the
operator a worldwide, non-exclusive, royalty-free, revocable licence to host,
store, reproduce, compile, execute, test, display, format, and distribute Your
Content **solely as needed to operate and promote the Instance's core features**:
judging submissions, running contests, publishing standings and solutions
according to the visibility you choose, producing judge artefacts (compiled
binaries, checker output, plagiarism fingerprints), and delivering feeds and
notifications. This licence includes the right to let other users **view** Your
Content where you have made it public; it does not let anyone republish it as
their own.

**What the licence does not allow.** The operator may not sell, sublicense, or
redistribute Your Content as a product, use it for advertising, or use it to
train machine-learning models. There is no blanket licence to your code beyond
what judging and display require.

**When the licence ends.** It ends when you delete the content or close your
account, with these exceptions: (a) public content may persist in search-engine
and third-party caches; (b) backups expire on the operator's rotation schedule;
(c) contest results, standings, and rating history may be retained with your
username and submission summary because other users' ratings depend on them;
(d) copies may be kept where law, a security investigation, or an unresolved
dispute requires it.

**You are responsible for what you post.** You confirm that you have the rights
to post Your Content, that it does not infringe anyone's rights, and that any
material you import from another judge or problem archive is used under a licence
that permits it, with attribution preserved. Problem statements imported through
FPS or similar tools carry the obligations of their source.

**Other users' content.** You may read public solutions and editorials for
learning. You may not republish them in bulk, sell them, or feed them into a
competing service.

**Feedback.** If you send suggestions or bug reports, we may use them without
obligation or compensation. Do not include confidential material in feedback.

## 5. Contests, judging, and ratings

**Rules per contest.** Each contest publishes its own rules, format, division
eligibility, and time limits. Formats include ACM, OI, IOI, AtCoder, and
Codeforces styles, and they score and penalise differently. By registering, you
accept that contest's rules.

**Automated judging.** Verdicts are produced by executing your code against test
data in an isolated sandbox and applying the configured checker. Automated
verdicts can be wrong: tests may be weak, limits may be misconfigured, or
infrastructure may fail. Report suspected judge errors through the contest's
clarification channel or the support channel rather than by resubmitting
repeatedly.

**Rejudging and corrections.** We may rejudge submissions, fix test data, void
results, or adjust scores and ratings when there is a judging bug, a weak test
set, a judge outage, or confirmed cheating. We aim to announce material
corrections, but they may happen after a contest ends.

**Hacking.** Hacking is permitted only within the rules of a contest that offers
it. You may not use a hack to attack another user's account, obtain their private
data, or interfere with the judge. Counter-tests become part of the problem's
test data and may be published.

**Plagiarism detection.** Submissions may be compared with each other and with
external corpora, automatically and by human review. Confirmed plagiarism or
unauthorised collaboration can lead to disqualification, a rating penalty, loss of
achievements, or a ban. Because automated similarity flags can be false
positives, you may appeal to the operator with an explanation before a permanent
penalty is recorded.

**Ratings are not property.** Ratings and ranks are a measure of contest
performance, calculated with an Elo-style system and recalculated when results
change. They carry no monetary value, create no entitlement, and are not
compensation if lost or adjusted.

**Decisions.** Contest organisers and the operator decide disputes. Their
decisions on eligibility, scoring, and disqualification are final after review.

## 6. Virtual judge and remote platforms

Virtual judge lets you submit to Codeforces, AtCoder, CSES, Toph, and QOJ through
the Instance's bot accounts. When you do:

- your source code is transmitted to that platform, and **you are responsible for
  complying with its terms, including any rules about automated submission or
  account sharing**;
- the platform's verdict, availability, rate limits, and account policies are
  outside our control, and a submission may fail or be rejected for reasons we
  cannot fix;
- the operator may disable virtual judge, a platform, or a bot account at any
  time, including after repeated failures or a platform's request.

AIOJ is not affiliated with, endorsed by, or sponsored by any of those platforms.
Their names and marks belong to their owners and are used only to describe
interoperability.

## 7. Our content and the licence to the software

The Instance's design, interface, text (other than Your Content), and marks are
owned by the operator or its licensors and may not be copied or scraped for a
competing service without permission.

Problem statements, editorials, and blogs belong to the people who wrote them.
If you reuse them, keep the attribution and respect the stated licence.

The **AIOJ software itself is released under the MIT License**. That licence
governs your use of the source code; these Terms govern your use of this hosted
Instance. Nothing here restricts rights the MIT License gives you in the software.

## 8. Availability, changes, and backups

The Instance is provided on a **best-effort basis**. There is no service-level
agreement, no guaranteed uptime, and no guarantee that data will never be lost.
Maintenance, upgrades, network problems, and judge outages happen.

Keep your own copy of any solution or statement you care about. Do not use the
Instance as your only storage for a thesis, coursework, or a contest you are
preparing.

We may add, change, or remove features, and we may retire the Instance. Where
practical we will give notice before removing something you rely on, and a
reasonable window to export your data. Demo or seeded environments may be reset
without notice.

## 9. Suspension and termination

**You** may stop using the Instance at any time and delete your account in
Settings. Deletion requires your password when your account has one and an
explicit confirmation.

**We** may suspend or terminate your access if you materially breach these Terms
or a contest rule, if we are required to by law, if your account poses a security
risk, or if the Instance is being discontinued. Except where the breach is
serious (cheating, attacks, harassment, or illegal content), we will give notice
and a chance to respond.

On termination, your right to use the Instance ends. Sections that by their nature
should survive do survive: your content licence to the extent described in
[section 4](#4-your-content-and-intellectual-property), disclaimers, limitation of
liability, indemnity, governing law, and accrued rights. Data handling after
termination is described in the [Privacy Policy](/legal/privacy).

You may appeal a suspension by contacting the operator. Bans for confirmed
cheating or attacks on the platform may be permanent.

## 10. Disclaimers

To the fullest extent permitted by law, the Instance and all content on it are
provided **"as is" and "as available"**, without warranties of any kind, express
or implied, including merchantability, fitness for a particular purpose, title,
accuracy, and non-infringement.

We do not warrant that the Instance will be uninterrupted, secure, or error-free;
that verdicts, ratings, or standings will be accurate; that user-posted content is
correct, lawful, or safe; or that data will be preserved. Content posted by other
users is theirs, not the operator's. Nothing on the Instance is legal,
academic, or professional advice, and a verdict is not a grade or a credential.

Some jurisdictions do not allow certain disclaimers. In those places, the
disclaimers apply only to the extent permitted, and your statutory rights are not
affected.

## 11. Limitation of liability

To the fullest extent permitted by law, the operator and the AIOJ project's
contributors will not be liable for indirect, incidental, special, consequential,
exemplary, or punitive damages, or for loss of profits, data, code, rating,
standing, opportunity, reputation, or academic outcome, arising out of or
relating to your use of the Instance — whether based on contract, tort,
negligence, strict liability, or any other theory, and even if advised of the
possibility.

The operator's total aggregate liability for all claims relating to the Instance
will not exceed the greater of (a) the amount you paid the operator for the
Instance in the twelve months before the event giving rise to the claim, or
(b) **USD 100**.

Nothing in these Terms limits liability that cannot lawfully be limited,
including liability for death or personal injury caused by negligence, for fraud
or fraudulent misrepresentation, or for breach of non-waivable consumer rights.
If your local consumer law does not allow a cap on certain damages, the cap
applies only to the extent the law allows.

## 12. Indemnity

To the extent permitted by law, you agree to indemnify and hold harmless the
operator from claims, damages, losses, and reasonable legal costs arising from
(a) Your Content, (b) your breach of these Terms or of another person's rights,
or (c) your misuse of the Instance — including a claim that content you posted
infringes copyright or that your submission to a remote judge violated that
platform's terms.

We will notify you of a claim covered by this section and may ask you to
cooperate in the defence; we may also take over the defence at our own expense.
This section does not apply to the extent the claim arises from our own
negligence or wilful misconduct, and it does not limit consumer rights.

## 13. Governing law and disputes

These Terms are governed by the laws of the jurisdiction in which the operator of
this Instance is established, without regard to conflict-of-law rules. The courts
of that jurisdiction have exclusive jurisdiction, unless mandatory law where you
live gives you the right to sue, or be sued, in your local courts.

Before filing a claim, contact the operator and try to resolve the dispute
informally for at least 30 days. Most problems — a wrong verdict, a missing
rating update, a moderation decision — are resolved faster that way.

To the extent permitted by law, each party may bring claims only in an individual
capacity, not as part of a class or representative action. This does not prevent a
consumer-protection authority from acting, and it does not apply where such a
waiver is unenforceable.

Nothing in this section prevents either party from seeking urgent injunctive
relief to stop abuse of the Instance or infringement of intellectual property.

## 14. About the AIOJ software and self-hosted instances

AIOJ is free, open-source, self-hostable software, licensed under the **MIT
License**. The AIOJ project publishes the code; it does **not** operate every
Instance built from it and does not control what an operator configures.

If you use an Instance run by a university, club, company, or individual, that
operator is your counterparty under these Terms and the data controller under the
Privacy Policy. Their configuration decides which features exist, which
integrations are enabled, and how long data is kept. If an operator adds terms of
their own, those terms apply to your use of their Instance alongside these.

If **you** run an Instance for other people, you are the operator: you are
responsible for the content, the configuration, security, data protection, and
compliance with the law that applies to you, and for adapting these Terms and the
Privacy Policy to your deployment. The AIOJ project and its contributors provide
the software without warranty and have no liability for your Instance.

## 15. Changes to these terms

We may update these Terms to reflect new features, new law, or a change in how the
Instance operates. The date at the top shows the current version. For material
changes we will give notice through the interface or by email before they take
effect, where practical. Continuing to use the Instance after the effective date
means you accept the updated Terms. If you do not, stop using the Instance and
delete your account; you may export your data first.

## 16. General

- **Entire agreement.** These Terms, the Privacy Policy, and any contest or
  organisation rules incorporated by reference are the whole agreement about the
  Instance and supersede prior discussions on the same subject.
- **Severability.** If a provision is held unenforceable, it is limited or removed
  to the minimum extent necessary and the rest stays in force.
- **No waiver.** Failing to enforce a provision is not a waiver of it.
- **Assignment.** You may not assign your rights under these Terms. The operator
  may assign them to a successor that takes over the Instance, with notice.
- **Force majeure.** Neither party is liable for delay or failure caused by
  events beyond its reasonable control, such as outages, network failures,
  natural disasters, or legal action.
- **Notices.** We may give notice by email to the address on your account or by a
  prominent in-app message. You give notice by contacting the operator.
- **Language.** These Terms are written in English. Translations, where provided,
  are for convenience; the English version prevails to the extent the law allows.
- **Headings and examples.** Headings are for navigation only. Examples are
  illustrative, not exhaustive.
- **No third-party beneficiaries.** Apart from the AIOJ project's contributors,
  who may rely on the disclaimers and liability limits in these Terms, no third
  party has rights under them.

## 17. Contact

Questions about these Terms, appeals, abuse reports, and legal notices go to the
operator's published contact channel for this Instance. Security reports should go
to the operator's security contact, or privately to the AIOJ project for
vulnerabilities in the software itself.
