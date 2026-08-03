# Security policy

## Supported versions

Security fixes are applied to the latest release.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Use GitHub private vulnerability reporting for this repository.

Include the affected version, reproduction steps, impact, and any suggested mitigation. Acknowledgement should arrive within five business days.

## Threat model

The service processes untrusted HTML inside a headless browser. Remote resources and JavaScript are disabled by default.

Remote resources are enabled only for exact hostnames listed in `ALLOWED_ASSET_HOSTS`. These hostnames must be controlled or trusted by the operator. Literal loopback, private, link-local, and unspecified IP addresses remain blocked. DNS rebinding protection must also be enforced by the deployment network because hostname resolution occurs inside Chrome.

The Docker image runs as a non-root user with a read-only filesystem. `CHROME_NO_SANDBOX=true` is set in the image for container compatibility, so container isolation and the supplied runtime restrictions are part of the security boundary. Prefer enabling the Chrome sandbox when the target runtime supports it.

For an internet-facing deployment, configure `API_KEY`, a reverse-proxy rate limit, TLS, egress filtering, and explicit `ALLOWED_ORIGINS`. Do not expose `/metrics` publicly without protecting it at the reverse proxy.
