# Security Policy

## Deployment Boundaries

PlugStream accepts a remote video `url` via HTTP query parameters and passes it to `ffmpeg` and `ffprobe`. When deploying this service, operators should enforce the following controls:

- **Restrict source URLs**: Place PlugStream behind a reverse proxy or API gateway that validates allowed domains and blocks requests targeting private networks or loopback addresses (SSRF protection).
- **Require authentication and rate limiting**: Do not expose an unauthenticated instance directly to the public internet.
- **Bound resource usage**: Tune `server.max_concurrent_tasks` and `server.timeout` in `config.yaml` according to host CPU and disk capacity.

## Reporting a Vulnerability

Please do not open a public GitHub issue for security vulnerabilities.

Instead, report vulnerabilities privately through [GitHub Security Advisories](https://github.com/icestr/PlugStream/security/advisories/new) with reproduction details, affected versions, and potential impact.
