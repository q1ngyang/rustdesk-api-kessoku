# Kessoku Web Client wire profile

Status: implementation input for the Kessoku v2.8.0 browser client.

This document describes the observable wire contract used by the browser
client. It deliberately contains protocol facts and state transitions only;
it is not copied source code or a compatibility promise for features outside
the profile below.

## Compatibility inputs

- RustDesk client compatibility target: `1.4.9`, source revision
  `6c578292e8ebbbec708b76986ba8c4bc7c509747`.
- That tag pins its protocol submodule to
  `7e1c392c62d39c364127307cd408421dd5f8cfb0`.
- Starry compatibility target: published `patch-v1.2.0` WSS paths.
- Browser transport is forced Relay over WSS. Direct TCP, UDP and P2P are not
  part of this profile.

The minimal protobuf declaration in `web-client/proto/kessoku_wire.proto` is
the normative codec input for this repository. Unknown fields must be ignored
and unsupported message variants must never be reflected back to the peer.

## Fixed endpoints

The client obtains one public, immutable JSON profile from its own origin. It
contains an HTTPS API origin, one `wss://.../ws/id` rendezvous URL, an exact
map from server-returned Relay names to approved `wss://.../ws/relay` URLs,
the server Ed25519 public key, and a positive profile generation. Neither the
profile nor a URL may contain a token, password or private key.
`server_key_fingerprint` is exactly `sha256:` followed by lowercase hexadecimal
SHA-256 of the decoded 32-byte Ed25519 public key.

The browser rejects a rendezvous-selected Relay that is absent from the exact
map. It never derives a new hostname, port or path from untrusted input.

## Connection state machine

1. Hold the short-lived `rustdesk-connect` token and remote password in memory
   only. Reject an empty peer ID, an oversized value, or a value outside the
   documented RustDesk ID character set.
2. Open the fixed rendezvous WSS URL and send `PunchHoleRequest` with the peer
   ID, symmetric NAT, default connection type, the configured server public-key
   string in `licence_key`, token, client version and `force_relay=true`.
3. Accept only a successful `RelayResponse`. Require a non-empty UUID, an
   allowlisted Relay name, a non-empty signed peer key and a compatible peer
   version. A refusal, timeout, unknown response or closed socket is terminal.
4. Open the allowlisted Relay WSS URL and send `RequestRelay` with the peer ID,
   UUID, the configured server public-key string in `licence_key` and default
   connection type. Keep `socket_addr` empty for forced Relay. Do not put the
   bearer token in this Relay request. The same configured public-key string is
   used in `PunchHoleRequest.licence_key`; it is not derived or rewritten.
5. Verify the rendezvous-provided signed `IdPk` with the configured server
   Ed25519 public key, and require its ID to equal the requested peer ID.
6. Require the first Relay application message to be `SignedId`. Verify it
   using the key from step 5, decode the enclosed `IdPk`, require the same peer
   ID and require a 32-byte peer Curve25519 public key.
7. Create an ephemeral Curve25519 key pair and a random 32-byte secretbox key.
   Seal the secret key with a 24-byte all-zero nonce, the peer public key and
   the ephemeral private key. Send `PublicKey` containing the ephemeral public
   key and sealed secret. Any error closes the socket; insecure fallback is
   forbidden.
8. Subsequent application protobuf messages are encrypted with XSalsa20-
   Poly1305 secretbox. Each direction owns a 64-bit counter starting at one;
   encode it little-endian into the first eight bytes of a zero-filled 24-byte
   nonce. Counter wrap, authentication failure or decode failure is terminal.
9. Receive `Hash`, compute SHA-256(password UTF-8 || salt UTF-8), then
   SHA-256(first digest || challenge UTF-8), and send it in `LoginRequest`.
   Password and derived hashes are erased on disconnect.
10. Require a successful `LoginResponse` with a bounded `PeerInfo` and at
    least one valid display. Advertise VP9 decoding, disable audio and
    clipboard, and request video acknowledgement.
11. Decode only VP9 frames through browser WebCodecs. Enforce encoded-frame,
    queue, display-dimension and decoded-frame bounds. Acknowledge consumed
    video, draw with Canvas 2D, and close every `VideoFrame` object.
12. Send bounded mouse and basic keyboard messages only while connected. Mouse
    masks use `buttons << 3 | type`: move=0, down=1, up=2, wheel=3; left=1,
    right=2 and middle=4. Absolute move/down/up coordinates are display-space
    signed integers; wheel deltas occupy `x` and `y`. Printable keyboard input
    uses the Unicode code point in `KeyEvent.chr`, `press=true` and legacy mode.
    Non-printable keys use the declared `ControlKey`, with `down=true` on
    keydown and both flags false on keyup. Modifier lists contain only the
    currently held Alt, Shift, Control and Meta values. Browser auto-repeat is
    bounded and IME composition is not part of the first profile.
13. Echo a received `TestDelay` whose `from_client` flag is false. After a VP9
    frame batch has been consumed and drawn, send `Misc.video_received=true`;
    on a decoder reset send a bounded `Misc.refresh_video=true` request.
14. Explicit disconnect closes sockets and decoders and clears all secrets.

## Security and resource limits

- WSS and HTTPS only; exact origins, hosts and paths; no ambient proxy or URL
  override.
- No token, password, derived password, key or session state in URL,
  localStorage, sessionStorage, IndexedDB, Cache API, service workers or logs.
- No analytics, remote script, dynamic CDN, `eval`, inline event handler or
  unsafe HTML insertion.
- Fail closed on key mismatch, unsigned sessions or unsupported codecs.
- Maximum WSS frame: 16 MiB; maximum peer/control text: 4 KiB; maximum display
  dimension: 8192; maximum pixels: 33,554,432; maximum queued video chunks:
  32; maximum encoded video chunk: 8 MiB.
- MVP exclusions: direct/P2P, incoming mode, file transfer, clipboard, audio,
  terminal, port forwarding, printing, multi-display switching, mobile touch,
  H.264/H.265/AV1 and software decoder fallback.

## Independent implementation boundary

The implementation may consume only this document and the repository-owned
minimal protobuf declaration. It must not read, copy, import, download or
derive from historical `resources/web`, hosted WebClient2/V2 assets,
`web_deps.tar.gz`, RustDesk Flutter Web output, or externally built JS/WASM.
All UI, state-machine, transport, rendering and tests are repository-owned
source with an explicit licence and reproducible lockfile.
