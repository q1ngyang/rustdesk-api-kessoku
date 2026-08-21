import { concatBytes, constantTimeEqual, utf8, wipe } from "./bytes";

const ZERO_16 = new Uint8Array(16);
const ZERO_24 = new Uint8Array(24);
const P1305 = (1n << 130n) - 5n;
const MASK_128 = (1n << 128n) - 1n;

function source(input: Uint8Array): ArrayBuffer {
  return new Uint8Array(input).buffer;
}

function read32(input: Uint8Array, offset: number): number {
  return (input[offset]! | (input[offset + 1]! << 8) | (input[offset + 2]! << 16) | (input[offset + 3]! << 24)) >>> 0;
}

function write32(output: Uint8Array, offset: number, value: number): void {
  output[offset] = value;
  output[offset + 1] = value >>> 8;
  output[offset + 2] = value >>> 16;
  output[offset + 3] = value >>> 24;
}

function rotate(value: number, bits: number): number {
  return ((value << bits) | (value >>> (32 - bits))) >>> 0;
}

function salsaRounds(state: Uint32Array): Uint32Array {
  const x = new Uint32Array(state);
  for (let round = 0; round < 10; round += 1) {
    x[4] ^= rotate((x[0]! + x[12]!) >>> 0, 7); x[8] ^= rotate((x[4]! + x[0]!) >>> 0, 9);
    x[12] ^= rotate((x[8]! + x[4]!) >>> 0, 13); x[0] ^= rotate((x[12]! + x[8]!) >>> 0, 18);
    x[9] ^= rotate((x[5]! + x[1]!) >>> 0, 7); x[13] ^= rotate((x[9]! + x[5]!) >>> 0, 9);
    x[1] ^= rotate((x[13]! + x[9]!) >>> 0, 13); x[5] ^= rotate((x[1]! + x[13]!) >>> 0, 18);
    x[14] ^= rotate((x[10]! + x[6]!) >>> 0, 7); x[2] ^= rotate((x[14]! + x[10]!) >>> 0, 9);
    x[6] ^= rotate((x[2]! + x[14]!) >>> 0, 13); x[10] ^= rotate((x[6]! + x[2]!) >>> 0, 18);
    x[3] ^= rotate((x[15]! + x[11]!) >>> 0, 7); x[7] ^= rotate((x[3]! + x[15]!) >>> 0, 9);
    x[11] ^= rotate((x[7]! + x[3]!) >>> 0, 13); x[15] ^= rotate((x[11]! + x[7]!) >>> 0, 18);
    x[1] ^= rotate((x[0]! + x[3]!) >>> 0, 7); x[2] ^= rotate((x[1]! + x[0]!) >>> 0, 9);
    x[3] ^= rotate((x[2]! + x[1]!) >>> 0, 13); x[0] ^= rotate((x[3]! + x[2]!) >>> 0, 18);
    x[6] ^= rotate((x[5]! + x[4]!) >>> 0, 7); x[7] ^= rotate((x[6]! + x[5]!) >>> 0, 9);
    x[4] ^= rotate((x[7]! + x[6]!) >>> 0, 13); x[5] ^= rotate((x[4]! + x[7]!) >>> 0, 18);
    x[11] ^= rotate((x[10]! + x[9]!) >>> 0, 7); x[8] ^= rotate((x[11]! + x[10]!) >>> 0, 9);
    x[9] ^= rotate((x[8]! + x[11]!) >>> 0, 13); x[10] ^= rotate((x[9]! + x[8]!) >>> 0, 18);
    x[12] ^= rotate((x[15]! + x[14]!) >>> 0, 7); x[13] ^= rotate((x[12]! + x[15]!) >>> 0, 9);
    x[14] ^= rotate((x[13]! + x[12]!) >>> 0, 13); x[15] ^= rotate((x[14]! + x[13]!) >>> 0, 18);
  }
  return x;
}

function salsaState(key: Uint8Array, input: Uint8Array): Uint32Array {
  if (key.byteLength !== 32 || input.byteLength !== 16) throw new Error("Invalid Salsa20 input");
  const state = new Uint32Array(16);
  state[0] = 0x61707865; state[5] = 0x3320646e; state[10] = 0x79622d32; state[15] = 0x6b206574;
  state[1] = read32(key, 0); state[2] = read32(key, 4); state[3] = read32(key, 8); state[4] = read32(key, 12);
  state[11] = read32(key, 16); state[12] = read32(key, 20); state[13] = read32(key, 24); state[14] = read32(key, 28);
  state[6] = read32(input, 0); state[7] = read32(input, 4); state[8] = read32(input, 8); state[9] = read32(input, 12);
  return state;
}

export function hsalsa20(key: Uint8Array, input: Uint8Array): Uint8Array {
  const x = salsaRounds(salsaState(key, input));
  const output = new Uint8Array(32);
  const positions = [0, 5, 10, 15, 6, 7, 8, 9] as const;
  positions.forEach((position, index) => write32(output, index * 4, x[position]!));
  x.fill(0);
  return output;
}

function salsaBlock(key: Uint8Array, nonce8: Uint8Array, counter: bigint): Uint8Array {
  const input = new Uint8Array(16);
  input.set(nonce8, 0);
  let value = counter;
  for (let index = 0; index < 8; index += 1) {
    input[8 + index] = Number(value & 0xffn);
    value >>= 8n;
  }
  const state = salsaState(key, input);
  const x = salsaRounds(state);
  const output = new Uint8Array(64);
  for (let index = 0; index < 16; index += 1) write32(output, index * 4, (x[index]! + state[index]!) >>> 0);
  state.fill(0); x.fill(0); input.fill(0);
  return output;
}

function xsalsaStream(key: Uint8Array, nonce: Uint8Array, length: number): Uint8Array {
  if (key.byteLength !== 32 || nonce.byteLength !== 24) throw new Error("Invalid secretbox input");
  const subkey = hsalsa20(key, nonce.subarray(0, 16));
  const output = new Uint8Array(length);
  let offset = 0;
  let counter = 0n;
  while (offset < length) {
    const block = salsaBlock(subkey, nonce.subarray(16), counter);
    const count = Math.min(64, length - offset);
    output.set(block.subarray(0, count), offset);
    block.fill(0);
    offset += count;
    counter += 1n;
  }
  subkey.fill(0);
  return output;
}

function littleEndian(input: Uint8Array): bigint {
  let value = 0n;
  for (let index = input.byteLength - 1; index >= 0; index -= 1) value = (value << 8n) | BigInt(input[index]!);
  return value;
}

export function poly1305(message: Uint8Array, key: Uint8Array): Uint8Array {
  if (key.byteLength !== 32) throw new Error("Invalid Poly1305 key");
  const r = littleEndian(key.subarray(0, 16)) & 0x0ffffffc0ffffffc0ffffffc0fffffffn;
  const s = littleEndian(key.subarray(16));
  let accumulator = 0n;
  for (let offset = 0; offset < message.byteLength; offset += 16) {
    const chunk = message.subarray(offset, Math.min(offset + 16, message.byteLength));
    accumulator = ((accumulator + littleEndian(chunk) + (1n << BigInt(chunk.byteLength * 8))) * r) % P1305;
  }
  let tag = (accumulator + s) & MASK_128;
  const output = new Uint8Array(16);
  for (let index = 0; index < output.byteLength; index += 1) {
    output[index] = Number(tag & 0xffn);
    tag >>= 8n;
  }
  return output;
}

export function secretboxSeal(message: Uint8Array, nonce: Uint8Array, key: Uint8Array): Uint8Array {
  const stream = xsalsaStream(key, nonce, message.byteLength + 32);
  const ciphertext = new Uint8Array(message.byteLength);
  for (let index = 0; index < message.byteLength; index += 1) ciphertext[index] = message[index]! ^ stream[index + 32]!;
  const tag = poly1305(ciphertext, stream.subarray(0, 32));
  stream.fill(0);
  return concatBytes(tag, ciphertext);
}

export function secretboxOpen(box: Uint8Array, nonce: Uint8Array, key: Uint8Array): Uint8Array | undefined {
  if (box.byteLength < 16) return undefined;
  const ciphertext = box.subarray(16);
  const stream = xsalsaStream(key, nonce, ciphertext.byteLength + 32);
  const expected = poly1305(ciphertext, stream.subarray(0, 32));
  if (!constantTimeEqual(box.subarray(0, 16), expected)) {
    stream.fill(0); expected.fill(0);
    return undefined;
  }
  const message = new Uint8Array(ciphertext.byteLength);
  for (let index = 0; index < ciphertext.byteLength; index += 1) message[index] = ciphertext[index]! ^ stream[index + 32]!;
  stream.fill(0); expected.fill(0);
  return message;
}

export async function openSignedMessage(signed: Uint8Array, publicKey: Uint8Array): Promise<Uint8Array> {
  if (publicKey.byteLength !== 32 || signed.byteLength < 64) throw new Error("Invalid Ed25519 signed message");
  const key = await crypto.subtle.importKey("raw", source(publicKey), { name: "Ed25519" }, false, ["verify"]);
  const signature = signed.slice(0, 64);
  const message = signed.slice(64);
  if (!(await crypto.subtle.verify("Ed25519", key, source(signature), source(message)))) throw new Error("Ed25519 signature rejected");
  signature.fill(0);
  return message;
}

export async function createBox(peerPublicKey: Uint8Array, secret: Uint8Array): Promise<{ publicKey: Uint8Array; sealed: Uint8Array }> {
  if (peerPublicKey.byteLength !== 32 || secret.byteLength !== 32) throw new Error("Invalid Curve25519 input");
  const pair = await crypto.subtle.generateKey({ name: "X25519" }, true, ["deriveBits"]) as CryptoKeyPair;
  const peer = await crypto.subtle.importKey("raw", source(peerPublicKey), { name: "X25519" }, false, []);
  const rawShared = new Uint8Array(await crypto.subtle.deriveBits({ name: "X25519", public: peer }, pair.privateKey, 256));
  const shared = hsalsa20(rawShared, ZERO_16);
  wipe(rawShared);
  const sealed = secretboxSeal(secret, ZERO_24, shared);
  wipe(shared);
  const publicKey = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey));
  if (publicKey.byteLength !== 32) throw new Error("Invalid generated Curve25519 key");
  return { publicKey, sealed };
}

export async function passwordChallenge(password: Uint8Array, salt: string, challenge: string): Promise<Uint8Array> {
  const saltBytes = utf8.encode(salt);
  const challengeBytes = utf8.encode(challenge);
  const firstInput = concatBytes(password, saltBytes);
  const first = new Uint8Array(await crypto.subtle.digest("SHA-256", source(firstInput)));
  const secondInput = concatBytes(first, challengeBytes);
  const result = new Uint8Array(await crypto.subtle.digest("SHA-256", source(secondInput)));
  wipe(saltBytes); wipe(challengeBytes); wipe(firstInput); wipe(first); wipe(secondInput);
  return result;
}

export class SecretChannel {
  #sendCounter = 1n;
  #receiveCounter = 1n;
  #key: Uint8Array | undefined;

  constructor(key: Uint8Array) {
    if (key.byteLength !== 32) throw new Error("Invalid channel key");
    this.#key = key.slice();
  }

  #nonce(counter: bigint): Uint8Array {
    if (counter < 1n || counter > 0xffffffffffffffffn) throw new Error("Secretbox counter exhausted");
    const nonce = new Uint8Array(24);
    let value = counter;
    for (let index = 0; index < 8; index += 1) {
      nonce[index] = Number(value & 0xffn);
      value >>= 8n;
    }
    return nonce;
  }

  seal(message: Uint8Array): Uint8Array {
    if (this.#key === undefined) throw new Error("Channel is closed");
    const nonce = this.#nonce(this.#sendCounter);
    const result = secretboxSeal(message, nonce, this.#key);
    if (this.#sendCounter === 0xffffffffffffffffn) this.close();
    else this.#sendCounter += 1n;
    return result;
  }

  open(box: Uint8Array): Uint8Array {
    if (this.#key === undefined) throw new Error("Channel is closed");
    const nonce = this.#nonce(this.#receiveCounter);
    const result = secretboxOpen(box, nonce, this.#key);
    if (result === undefined) throw new Error("Secretbox authentication failed");
    if (this.#receiveCounter === 0xffffffffffffffffn) this.close();
    else this.#receiveCounter += 1n;
    return result;
  }

  close(): void {
    wipe(this.#key);
    this.#key = undefined;
    this.#sendCounter = 0n;
    this.#receiveCounter = 0n;
  }
}
