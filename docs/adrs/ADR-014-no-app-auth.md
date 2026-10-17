# ADR-014: No application-level authentication; network-restricted exposure

## Status
Accepted

## Context
Usernames are labels, not identities. Anyone reaching the URL can read any username's received
messages and send as any username. Instances run on developer VMs in a trusted network.

## Decision
- No auth in the app.
- Exposure limited by deployment: internal-only OpenShift Route (edge TLS) / Ingress, and a
  NetworkPolicy allowing ingress to the pod only from the router/ingress namespace.

## Alternatives
- Shared bearer token from a Secret — ~20 lines, cheap insurance; not needed in trusted net.
- OpenShift oauth-proxy sidecar — real SSO, but extra mirrored image, not testable on kind.

## Consequences
- Anyone on the internal network with route access can read pasted text.
- Revisit if exposure widens (would add option B: shared token).
