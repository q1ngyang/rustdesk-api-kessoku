export const LIMITS = Object.freeze({
  websocketFrame: 16 * 1024 * 1024,
  controlText: 4 * 1024,
  peerId: 128,
  token: 4 * 1024,
  passwordBytes: 4 * 1024,
  displayDimension: 8192,
  displayPixels: 33_554_432,
  cursorDimension: 512,
  cursorPixels: 262_144,
  cursorEncodedBytes: 1_052_672,
  queuedVideoChunks: 32,
  encodedVideoChunk: 8 * 1024 * 1024,
  handshakeTimeoutMs: 12_000,
});

export const CLIENT_VERSION = "1.4.9";
