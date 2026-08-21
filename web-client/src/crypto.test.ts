import { describe, expect, it } from "vitest";
import { createBox, hsalsa20, openSignedMessage, passwordChallenge, poly1305, SecretChannel, secretboxOpen, secretboxSeal } from "./crypto";
import { concatBytes, utf8 } from "./bytes";

function hex(value: string): Uint8Array {
  return Uint8Array.from(value.match(/../g) ?? [], (pair) => Number.parseInt(pair, 16));
}

function hexOf(value: Uint8Array): string {
  return Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

describe("cryptographic primitives", () => {
  it("matches the RFC 7539 Poly1305 vector", () => {
    const key = hex("85d6be7857556d337f4452fe42d506a80103808afb0db2fd4abff6af4149f51b");
    expect(hexOf(poly1305(utf8.encode("Cryptographic Forum Research Group"), key)))
      .toBe("a8061dc1305136c6c22b8baf0c0127a9");
  });

  it("authenticates XSalsa20-Poly1305 and rejects modification", () => {
    const key = Uint8Array.from({ length: 32 }, (_, index) => index);
    const nonce = Uint8Array.from({ length: 24 }, (_, index) => 23 - index);
    const message = utf8.encode("independent Kessoku secretbox implementation");
    const box = secretboxSeal(message, nonce, key);
    expect(secretboxOpen(box, nonce, key)).toEqual(message);
    box[20] ^= 1;
    expect(secretboxOpen(box, nonce, key)).toBeUndefined();
  });

  it("matches an independent libsodium XSalsa20-Poly1305 vector", () => {
    const key = Uint8Array.from({ length: 32 }, (_, index) => index);
    const nonce = Uint8Array.from({ length: 24 }, (_, index) => index);
    const message = utf8.encode("kessoku-secretbox-interop");
    expect(hexOf(secretboxSeal(message, nonce, key)))
      .toBe("070b8f9e86f56600f92c9c88a21b62b3359a4b3ca8a1d73dc859ec4c0dfb28f82a882fb1f8404d8067");
  });

  it("uses independent ordered nonce counters", () => {
    const key = new Uint8Array(32).fill(7);
    const sender = new SecretChannel(key);
    const receiver = new SecretChannel(key);
    const first = sender.seal(utf8.encode("one"));
    const second = sender.seal(utf8.encode("two"));
    expect(new TextDecoder().decode(receiver.open(first))).toBe("one");
    expect(new TextDecoder().decode(receiver.open(second))).toBe("two");
    sender.close();
    receiver.close();
  });

  it("matches the two-stage SHA-256 challenge vector", async () => {
    const result = await passwordChallenge(utf8.encode("remote-password"), "salt-value", "challenge-value");
    expect(hexOf(result)).toBe("6dad153d6b7142c481713c92dc9a0f55c189729d1e40bcfaefdd5f9a12a17a67");
  });

  it("opens an Ed25519 signed-message envelope with Web Crypto", async () => {
    const pair = await crypto.subtle.generateKey({ name: "Ed25519" }, true, ["sign", "verify"]) as CryptoKeyPair;
    const message = utf8.encode("signed IdPk payload");
    const signature = new Uint8Array(await crypto.subtle.sign("Ed25519", pair.privateKey, new Uint8Array(message).buffer));
    const publicKey = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey));
    expect(await openSignedMessage(concatBytes(signature, message), publicKey)).toEqual(message);
  });

  it("seals the session key for a peer X25519 key", async () => {
    const peer = await crypto.subtle.generateKey({ name: "X25519" }, true, ["deriveBits"]) as CryptoKeyPair;
    const peerPublic = new Uint8Array(await crypto.subtle.exportKey("raw", peer.publicKey));
    const secret = new Uint8Array(32).fill(42);
    const box = await createBox(peerPublic, secret);
    const ephemeral = await crypto.subtle.importKey("raw", new Uint8Array(box.publicKey).buffer, { name: "X25519" }, false, []);
    const rawShared = new Uint8Array(await crypto.subtle.deriveBits({ name: "X25519", public: ephemeral }, peer.privateKey, 256));
    const shared = hsalsa20(rawShared, new Uint8Array(16));
    expect(secretboxOpen(box.sealed, new Uint8Array(24), shared)).toEqual(secret);
  });
});
