# Internal authentication fixtures

These response fixtures are safe for cross-repository Starry contract tests.
They contain no private key or live credential. Every inactive condition has
the same external shape so consumers cannot depend on an enumerable reason.

- `introspection-active.json`: minimal active response.
- `introspection-expired.json`, `introspection-revoked.json`,
  `introspection-disabled.json`, and `introspection-rotated-key.json`: identical
  inactive responses for distinct server-side causes.
- `jwks-current-previous.json`: public-only current/previous Ed25519 key shape.
