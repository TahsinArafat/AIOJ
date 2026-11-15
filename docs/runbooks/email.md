# Transactional email runbook

Covers outbound mail only: email verification, password reset, and 2FA notices.
Inbound mail and mailboxes are out of scope.

## How delivery is wired

`mail.driver` selects one of three `mail.Sender` implementations
(`cmd/aioj/main.go`):

| Driver | Behaviour | Use |
|--------|-----------|-----|
| `smtp` | Delivers through a relay | Any environment where users must receive mail |
| `catcher` | Captured in memory, readable at `/api/dev/mail` | Local dev (**dev only**) |
| `noop` | Discarded silently | Never, in production |

The production compose file defaults to `MAIL_DRIVER=noop`
(`docker-compose.dokploy.yml`). With that default, a reset request returns
`200 {"message":"If the email exists, a reset link has been sent"}` while
nothing is sent — the response is deliberately enumeration-safe, so it looks
identical to success. **A misconfigured relay and a working one are
indistinguishable to the user.** This is the single most common cause of
"AIOJ email is broken".

## Diagnosis order

1. **Check the driver in the boot log.** `cmd/aioj/main.go` logs the active
   driver at startup. `driver=noop` or `driver=catcher` in production is the
   whole answer — no further debugging needed.
2. **Check for send errors.** Since the hardening pass, failed sends log
   `send password reset email failed` / `send verification email failed` with
   the `user_id` and the wrapped `err`. A missing log line plus a delivered
   mail means the path is healthy.
3. **Check the boot warnings.** `Config.Validate` warns when `driver=noop` or
   empty in production, and when `driver=smtp` has an empty `mail.host`.
4. **Check the link host.** `MAIL_PUBLIC_URL` (alias `PUBLIC_ORIGIN`) is baked
   into every email link. A `localhost` value in production produces working
   sends with unusable links.

## Brevo setup

1. Create a Brevo account and add the sending domain under **Senders, Domains
   & Dedicated IPs**.
2. Add the **SPF**, **DKIM**, and **DMARC** DNS records Brevo gives you. Verify
   in the dashboard before sending — unverified domains are rejected.
3. Create an **SMTP key** under **SMTP & API → SMTP keys**. This is the value
   for `MAIL_PASSWORD`; the account password will not authenticate.
4. Set the environment:

   ```bash
   MAIL_DRIVER=smtp
   MAIL_HOST=smtp-relay.brevo.com
   MAIL_PORT=587                 # 587 = STARTTLS, 465 = implicit TLS
   MAIL_USERNAME=<brevo account login>
   MAIL_PASSWORD=<SMTP key, not the account password>
   MAIL_FROM=noreply@<verified-domain>
   MAIL_PUBLIC_URL=https://<DOMAIN>
   ```

5. Redeploy and confirm the boot log shows `driver=smtp`.

## Verify a send end to end

```bash
# 1. Driver is live
docker compose logs backend | grep "mail driver configured"

# 2. Request a reset (always returns 200 — do not trust the response)
curl -sS -X POST https://<DOMAIN>/api/auth/password-reset/request \
  -H 'Content-Type: application/json' \
  -d '{"email":"<your address>"}'

# 3. Watch for a send failure
docker compose logs -f backend | grep -i "send .* email failed"
```

A send failure logs a wrapped cause that names the stage — `dial` (network or
port blocked), `auth` (bad SMTP key), `MAIL FROM`/`RCPT TO` (sender or recipient
rejected, usually an unverified domain). If no failure logs and no mail arrives,
inspect the Brevo dashboard's transaction log: a provider-side rejection
(reputation, suppression list) is visible there and nowhere in AIOJ.

## Ports and TLS

- `587` — plaintext connect, upgraded with STARTTLS. The sender **requires** the
  upgrade before sending credentials and fails closed if the server offers none.
- `465` — implicit TLS, negotiated before the SMTP greeting. Handled explicitly;
  `net/smtp.SendMail` cannot do this, so the connection is built by hand.
- `25` — not supported for submitting. Blocked by most cloud providers and
  unsuitable for authenticated relay.
- All connections carry a 10s deadline (`mail.timeout` overrides). A hung relay
  fails the send rather than pinning the request goroutine.

## Local development

`catcher` is the local default. Captured mail is inspectable without a relay:

```bash
docker compose up -d backend
curl -sS http://localhost:8080/api/dev/mail | jq '.data[].Subject'
```

`/api/dev/mail` is mounted only when `driver == catcher`, so it is absent in
production by construction. For a full mailbox UI, `docker-compose.sim.yml`
runs MailHog. To exercise the real SMTP path locally, run a catcher container
that speaks SMTP (Mailpit on `:1025`) and point `MAIL_HOST`/`MAIL_PORT` at it —
that validates this sender's protocol code without touching a provider.

## Escalation

| Symptom | Likely cause |
|---------|--------------|
| Boot log says `noop` | `MAIL_DRIVER` unset — see the compose default |
| `dial` error | Wrong host/port, or egress to that port blocked |
| `auth as ...` error | `MAIL_PASSWORD` is the account password, not an SMTP key |
| `STARTTLS` error | Relay does not offer STARTTLS on this port; use 465 |
| `RCPT TO` rejected | Recipient suppressed, or `MAIL_FROM` domain not verified |
| Sends fine, no delivery | Check the provider's transaction log and spam folder |
| Link points at localhost | `MAIL_PUBLIC_URL` is unset in production |
