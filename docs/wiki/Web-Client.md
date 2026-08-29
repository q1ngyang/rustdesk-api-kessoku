# Built-in Web Client

**English** | [简体中文](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/ZH-CN-Web-Client)

Kessoku v3.0.6 builds its MIT-licensed browser client from the repository's
`web-client/` source and packages it as `resources/client`. The management UI
in `admin-web/` remains a separate application. Historical `resources/web`,
`resources/web2`, WebClient2/V2, and remote browser-client downloads are
permanently rejected by build and packaging policy.

## What the MVP does

The client initiates forced-Relay WSS sessions, verifies signed peer identity,
uses the encrypted RustDesk session, decodes VP9 with WebCodecs, renders with
Canvas 2D, and sends bounded mouse/basic keyboard input. Compatibility targets
RustDesk 1.4.9 and Starry patch-v1.2.2.

It intentionally excludes direct/P2P, host mode, file transfer, clipboard,
audio, terminal, port forwarding, printing, display switching, touch/IME,
non-VP9 codecs, and a software decoder. Treat these as unsupported—not hidden
configuration switches.

## Deploy two HTTPS origins

Use one origin for API/admin and one for the Web Client, for example:

```text
https://api.example.com     -> 127.0.0.1:21114
https://client.example.com  -> 127.0.0.1:21122
```

The native listener default is `127.0.0.1:21122`. The Docker image exposes
21122; recommended Compose keeps it host-local. A container-side
`0.0.0.0:21122` bind is acceptable only behind that host-local mapping and the
dedicated HTTPS proxy. Do not co-host the client below an API path, publish
21122 directly, enable proxy credentials, or reuse the internal mTLS port.

The client origin needs a CSP-compatible HTTPS connection to the API origin
and WSS connections to the exact rendezvous/Relay endpoints. Proxy WSS upgrade
headers and preserve the fixed `/ws/id` and `/ws/relay` paths. The client
rejects a Relay name not present in its configured exact map.

## Enable the profile

Set `web-client.mode: builtin`, exact `public-origin`/`api-origin`, listener,
WSS URLs, base64 Ed25519 server public key, positive profile generation, and a
connection-token TTL no longer than one hour or the global auth maximum. The
full example and field contract are in [`WEB-CLIENT.md`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/deployment/WEB-CLIENT.md)
and the [configuration reference](https://github.com/q1ngyang/rustdesk-api-kessoku/wiki/Configuration-Reference).

`GET /config/v1.json` on the client origin contains public endpoints, public
key/fingerprint, schema version, and profile generation only. The fingerprint
is `sha256:` plus lowercase hexadecimal SHA-256 of the decoded 32-byte Ed25519
key. It contains no token, password, private key, listener, TTL, or client-
origin value. On the wire the configured public-key string is used only as
`PunchHoleRequest.licence_key` and `RequestRelay.licence_key`; forced Relay
keeps `RequestRelay.socket_addr` empty.

## Launch and authentication

Users may log in directly through the client. An already authenticated admin
page instead calls `POST /api/web-client/v1/grants` with its RustAuth bearer,
opens only the deployment-owned `web_client_public_origin`, and sends the
short-lived connection grant through exact-origin `postMessage`. The peer ID
and connection token are carried in memory and never in a URL or persistent
browser storage. The grant is limited to audience `rustdesk-connect` and scope
`connect:initiate`; it is not an admin/API bearer.

If launch fails, close the popup, revoke/logout the connection grant when
possible, and correct origin/configuration. Never fall back to a query-string
token, wildcard `postMessage`, shared cookie, or relaxed CORS.

The API/admin response must use COOP `same-origin-allow-popups`. The separate
client response intentionally omits COOP (`unsafe-none` default) until the
ready/grant/accepted exchange has completed; the two origins do not send the
same COOP value. Client code validates both exact API/admin origin and opener
source, accepts once, removes the listener, then tries to clear the opener.
Admin timeout, navigation error, or missing acknowledgement triggers
best-effort logout/revocation; a successful acknowledgement transfers token
lifecycle ownership to the client.

## Acceptance checks

- both frontend lockfiles pass lint/test/audit/signatures and two-build
  reproducibility;
- separate admin/client distribution checksums, CycloneDX SBOMs, and MIT/
  third-party licence evidence exist;
- image, archive, and DEB contain `resources/client/index.html` and the full
  `resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt`, and
  contain neither historical resource directory;
- 21122 is host-local and served externally only as the configured HTTPS
  client origin;
- profile JSON contains no secret and responses have the expected CSP,
  no-store/referrer, frame, and content-type protections;
- a forced-Relay VP9 desktop session completes with mouse and keyboard input,
  then logout/disconnect clears the token and password from memory.

The detailed protocol/security limits are in
[`docs/development/WEB-CLIENT-WIRE-SPEC.md`](https://github.com/q1ngyang/rustdesk-api-kessoku/blob/master/docs/development/WEB-CLIENT-WIRE-SPEC.md).
