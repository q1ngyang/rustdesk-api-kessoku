# Kessoku Web Client

An MIT-licensed, forced-Relay browser client for the Kessoku RustDesk compatibility profile. The MVP supports authenticated encrypted sessions, remote-password challenge response, VP9 WebCodecs rendering, mouse/basic keyboard input, connection status, and explicit disconnect.

The Kessoku-authored client is MIT licensed. Its runtime protobuf dependency, `@bufbuild/protobuf` 2.9.0, is separately licensed under `(Apache-2.0 AND BSD-3-Clause)`; complete terms are included in `public/third-party-licenses/@bufbuild-protobuf-2.9.0.txt` and copied into `dist/third-party-licenses/` by the production build.

## Reproducible build

Use exactly Node 24.15.0 and npm 11.12.1:

```sh
npm ci
npm run check
npm audit
```

The generated codec is committed. Regeneration additionally requires protobuf compiler 28.3:

```sh
npm run generate:proto
```

## Deployment configuration

The hosting origin must serve immutable JSON at `/config/v1.json`:

```json
{
  "schema_version": 1,
  "profile_generation": 1,
  "api_origin": "https://api.example.invalid",
  "rendezvous_wss_url": "wss://id.example.invalid/ws/id",
  "relay_wss_urls": {
    "relay-1": "wss://relay.example.invalid/ws/relay"
  },
  "server_public_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  "server_key_fingerprint": "sha256:66687aadf862bd776c8fc18b8e9f8e20089714856ee233b3902a591d0d5f2925"
}
```

The fingerprint is `sha256:` followed by lowercase hexadecimal SHA-256 of the decoded 32-byte Ed25519 public key. All endpoints are absolute and fixed: the client rejects URL credentials, query strings, fragments, unexpected paths, redirects, unknown fields, and Relay names missing from the exact allowlist.

The API must implement `/api/web-client/v1/login`, `/api/web-client/v1/grants`, and `/api/web-client/v1/logout` as described by the Kessoku API contract. Configure CORS for the deployment origin. Production hosting must also send the CSP from `index.html` as an HTTP response header (including `frame-ancestors`, which browsers do not enforce from a meta element).

## Browser requirements

A secure context with WebSocket, Web Crypto Ed25519/X25519, VP9 WebCodecs, and Canvas 2D is required. There is no cryptographic or video fallback. Unsupported browsers fail closed.

## Live interoperability checklist

The deterministic codec, crypto primitives, validation rules, and build are covered locally. Before production rollout, exercise a real Starry `patch-v1.2.2` deployment and RustDesk 1.4.9 peer to confirm the complete signed-ID exchange, Relay binary frame boundaries, the peer's VP9 ability bit interpretation and timestamp unit, `video_received` pacing, legacy keyboard behavior across target operating systems, pointer/wheel direction, and logout/CORS behavior. These checks require live endpoints and cannot be proven by the repository-only unit suite.
