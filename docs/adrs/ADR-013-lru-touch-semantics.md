# ADR-013: Store holds received messages only; LRU touched by receive and view

## Status
Accepted

## Context
Max 100 users; least-recently-used user evicted when a 101st user must be added. Need to define
"use" and what the store holds.

## Decision
- Store holds only messages received from Kafka (other side), keyed by username. Sent messages
  are not stored.
- A user's recency is refreshed when (1) a message for that user is received, or (2) the UI
  fetches that user's messages.
- Fetching an unknown user returns an empty list and does NOT create an entry (otherwise
  arbitrary lookups could evict real users).
- Adding a new user when 100 exist evicts the least-recently-used user with all its messages.

## Alternatives
- Receive-only touch — GET side-effect free, but a watched idle user can be evicted.
- View-only touch — users receiving messages but not viewed get evicted; counter-intuitive.

## Consequences
- GET /api/messages mutates LRU order (documented side effect).
- Sending never affects the local store.
