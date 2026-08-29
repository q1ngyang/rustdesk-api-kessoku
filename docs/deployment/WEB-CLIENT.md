# Built-in Kessoku Web Client

**English** | [简体中文](WEB-CLIENT.zh-CN.md)

Kessoku v2.8.3 includes a repository-owned, MIT-licensed browser remote-
desktop MVP. Its reviewed TypeScript source is in `web-client/`; release builds
place only the reproducible production output in `resources/client`. It does
not use historical `resources/web`, `resources/web2`, hosted WebClient2/V2,
Flutter Web output, externally built JavaScript, or downloaded WASM.

## Supported MVP

- outgoing remote-control sessions through forced Relay over WSS;
- RustDesk 1.4.9 protocol compatibility against Starry patch-v1.2.2;
- signed peer-key verification and fail-closed encrypted sessions;
- VP9 video through browser WebCodecs and Canvas 2D rendering; and
- bounded mouse plus basic keyboard input.

Direct/P2P TCP or UDP, incoming/host mode, file transfer, clipboard, audio,
terminal, port forwarding, printing, display switching, mobile touch, IME,
H.264/H.265/AV1, and software decoding are not part of the v2.8.3 MVP.

## Origin and token boundary

The client listener defaults to `127.0.0.1:21122` for a native Linux
deployment. In a container it may listen on `0.0.0.0:21122` only while Compose
binds the host port to `127.0.0.1`. Put a dedicated HTTPS reverse proxy and
hostname in front of it. `web-client.public-origin` must differ from
`web-client.api-origin`; do not publish the client below the admin/API origin
as a path.

The client obtains non-secret connection endpoints and the server public key
from same-origin `GET /config/v1.json`. It accepts login or an already
authenticated connection-only grant through:

- `POST /api/web-client/v1/login` with strict username/password JSON;
- `POST /api/web-client/v1/grants` with an existing RustAuth bearer; and
- `POST /api/web-client/v1/logout` with the connection bearer.

The returned token has audience `rustdesk-connect`, scope `connect:initiate`,
a default 15-minute lifetime, and a maximum one-hour configured lifetime.
Admin launch reads the trusted client origin from Kessoku application config,
opens that exact origin, and delivers
`kessoku.web-client.grant.v1` with a strict `postMessage` target origin. Peer ID,
token, password, keys, and session state remain in memory; none is put in a URL,
cookie, localStorage, sessionStorage, IndexedDB, Cache API, service worker, or
log. The client origin receives no ambient API/admin cookie.

The API/admin response uses `Cross-Origin-Opener-Policy:
same-origin-allow-popups` so the authenticated opener survives only long
enough for the exact-origin ready/grant/accepted handshake. The independent
client listener deliberately sends no COOP header (the browser default is
`unsafe-none`); it does not copy the admin header. After one accepted grant the
client removes the listener and attempts to clear `window.opener`. A timeout,
navigation error, or missing acknowledgement closes the popup and makes a
best-effort `/api/web-client/v1/logout` call before clearing the admin's token
reference.

## Configuration

```yaml
web-client:
  mode: builtin
  listen: "127.0.0.1:21122"
  public-origin: "https://client.example.com"
  api-origin: "https://api.example.com"
  rendezvous-wss-url: "wss://rustdesk.example.com/ws/id"
  relay-wss-urls:
    "rustdesk.example.com:21117": "wss://rustdesk.example.com/ws/relay"
  server-public-key: "BASE64_ED25519_PUBLIC_KEY"
  profile-generation: 1
  connection-token-ttl: 15m
```

Every URL is exact HTTPS/WSS. A Relay name returned by rendezvous must be in
`relay-wss-urls`; the browser never derives a destination from untrusted input.
Increase `profile-generation` for a reviewed endpoint/key profile change.

## Build, licence, and evidence

Node 24.15.0 and npm 11.12.1 are fixed. CI uses `npm ci`, lint, tests,
production audit, registry-signature audit, two identical builds, a dedicated
CycloneDX SBOM, distribution checksums, and MIT/third-party licence evidence.
`web-client/LICENSE` covers repository-owned client code; the complete client
dependency/build graph and its licences are recorded in the client SBOM and
`WEB-CLIENT-NOTICE.md` release notice. The complete Apache-2.0 and BSD-3-Clause
text required by the runtime dependency is shipped at
`resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt` in every
archive, Debian package, and container image.

See [Web Client deployment](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Web-Client),
[Security model](../security/SECURITY-MODEL.md), and
[the wire profile](../development/WEB-CLIENT-WIRE-SPEC.md).
