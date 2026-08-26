# Caddy edge — migration and cutover

## Why

The wildcard certificate is issued by lego over Hostinger DNS-01. That can only
ever cover domains in our own Hostinger account, so it cannot cover a client's
own domain: their DNS is at their registrar, where we have no API access. Those
certificates have to be obtained over HTTP/TLS-ALPN instead — per domain, on
first request, without a human writing a vhost for every new client.

nginx can do this with certbot per domain, but every new client then needs a
vhost written and a reload run. Caddy's on-demand TLS does it automatically,
gated by an endpoint that decides whether a hostname is allowed.

## What is deliberately unchanged

The existing wildcard keeps being issued and renewed by the existing lego timer
(`deploy/tls/`), and Caddy loads it from disk. The proven certificate path is
not touched, and Caddy needs no DNS provider plugin. On-demand issuance applies
**only** to hostnames we do not own.

## The gate

Caddy asks before obtaining a certificate for an unseen hostname, calling
`http://127.0.0.1:9100/internal/tls-authorize?domain=<name>`. A 200 authorises
issuance. The endpoint answers exactly as strictly as routing does: the domain
must be verified **and** on a plan that includes custom domains.

Without the gate, anyone could point a DNS record at the server and make it
request certificates on their behalf, burning Let's Encrypt rate limits until
issuance stops working for real clients. It answers the same 403 for "unknown"
and "not entitled", so it cannot be used to probe which plan a domain is on, and
it is never routed publicly.

## Cutover — manual and deliberate

This changes what terminates TLS for every existing tenant, so it is not part of
the normal deploy. Do it at a quiet hour, with a rollback ready.

1. **Install Caddy** on the VPS (`apt install caddy`). Do not enable it yet —
   nginx still holds :80 and :443.
2. **Copy the config**: `install -m 0644 deploy/caddy/Caddyfile /etc/caddy/Caddyfile`
3. **Validate before anything is live**: `caddy validate --config /etc/caddy/Caddyfile`
4. **Confirm the gate answers** while nginx is still serving:
   `curl -i "http://127.0.0.1:9100/internal/tls-authorize?domain=<a verified domain>"` → 200,
   and an unknown domain → 403.
5. **Stop nginx, start Caddy**: `systemctl stop nginx && systemctl start caddy`
6. **Verify each hostname class immediately**: apex, `www`, `api`, `assets`, a
   tenant subdomain, and a reserved subdomain (expects 404).
7. **Then** point a client domain's A record at the server and load it. The
   first request will pause briefly while the certificate is obtained.

### Rollback

`systemctl stop caddy && systemctl start nginx`

nginx keeps its config the whole time, so rollback is a service swap, not a
restore. Keep this to hand during the cutover.

## After the cutover

`deploy.yml` still installs the nginx config. That step should be switched to
`deploy/caddy/install-caddy` once Caddy is the live edge — but only then, or a
deploy will promote a config nothing is reading.

## Verified so far

The Caddyfile adapts and provisions cleanly (`caddy validate` against the real
binary, with certificates present) and the gate was exercised against the real
API: verified + entitled → 200; unverified → 403; unknown → 403; missing
parameter → 400; and after a plan downgrade → 403.

Nothing here has been run against production.
