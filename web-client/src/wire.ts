import { utf8 } from "./bytes";
import { BinaryReader } from "@bufbuild/protobuf/wire";
import { LIMITS } from "./limits";
import type { DisplayInfo, Message as AppMessage, PeerInfo, RelayResponse, RendezvousMessage as RendezvousEnvelope } from "./generated/kessoku_wire";

export function validatePeerId(value: string): string {
  if (value.length === 0 || value.length > LIMITS.peerId || !/^[A-Za-z0-9_-]+$/.test(value)) {
    throw new Error("Peer ID contains unsupported characters");
  }
  return value;
}

export function boundedText(value: string, label: string): string {
  if (utf8.encode(value).byteLength > LIMITS.controlText) throw new Error(`${label} is too large`);
  return value;
}

export function compatiblePeerVersion(value: string): boolean {
  const match = /^(\d+)\.(\d+)\.(\d+)(?:[-+][0-9A-Za-z.-]+)?$/.exec(value);
  if (match === null) return false;
  const major = Number(match[1]);
  const minor = Number(match[2]);
  return major === 1 && minor >= 2;
}

export function onlyRelayResponse(message: RendezvousEnvelope): RelayResponse {
  const variants = [message.punchHoleRequest, message.punchHoleResponse, message.requestRelay, message.relayResponse]
    .filter((value) => value !== undefined).length;
  if (variants !== 1 || message.relayResponse === undefined) throw new Error("Unexpected rendezvous response");
  const response = message.relayResponse;
  boundedText(response.refuseReason, "Relay refusal");
  if (response.refuseReason !== "") throw new Error("Relay request was refused");
  if (response.uuid.length === 0) throw new Error("Relay response omitted UUID");
  boundedText(response.uuid, "Relay UUID");
  if (response.relayServer.length === 0) throw new Error("Relay response omitted Relay name");
  boundedText(response.relayServer, "Relay name");
  const identityChoices = Number(response.id !== undefined) + Number(response.pk !== undefined);
  if (identityChoices !== 1 || response.pk === undefined || response.pk.byteLength < 65 || response.pk.byteLength > LIMITS.controlText) {
    throw new Error("Relay response omitted signed peer identity");
  }
  if (!compatiblePeerVersion(response.version)) throw new Error("Incompatible peer version");
  return response;
}

export function countAppVariants(message: AppMessage): number {
  return [
    message.signedId,
    message.publicKey,
    message.testDelay,
    message.videoFrame,
    message.loginRequest,
    message.loginResponse,
    message.hash,
    message.mouseEvent,
    message.cursorData,
    message.cursorPosition,
    message.cursorId,
    message.keyEvent,
    message.misc,
    message.peerInfo,
  ].filter((value) => value !== undefined).length;
}

export function topLevelFieldNumbers(input: Uint8Array): number[] {
  const reader = new BinaryReader(input);
  const fields: number[] = [];
  while (reader.pos < reader.len) {
    if (fields.length >= 8) throw new Error("Application message has too many top-level fields");
    const tag = reader.uint32();
    const field = tag >>> 3;
    const wireType = tag & 7;
    if (field === 0 || wireType === 3 || wireType === 4) throw new Error("Invalid application message tag");
    fields.push(field);
    reader.skip(wireType);
  }
  return fields;
}

export function validateCursorMessage(message: AppMessage): void {
  if (message.cursorData !== undefined) {
    const cursor = message.cursorData;
    if (!Number.isInteger(cursor.width) || !Number.isInteger(cursor.height) ||
      cursor.width <= 0 || cursor.height <= 0 ||
      cursor.width > LIMITS.cursorDimension || cursor.height > LIMITS.cursorDimension ||
      cursor.width * cursor.height > LIMITS.cursorPixels ||
      !Number.isInteger(cursor.hotx) || !Number.isInteger(cursor.hoty) ||
      cursor.hotx < 0 || cursor.hoty < 0 || cursor.hotx >= cursor.width || cursor.hoty >= cursor.height ||
      cursor.colors.byteLength === 0 || cursor.colors.byteLength > LIMITS.cursorEncodedBytes) {
      throw new Error("Peer supplied invalid cursor data");
    }
  }
  if (message.cursorPosition !== undefined) {
    const { x, y } = message.cursorPosition;
    if (!Number.isInteger(x) || !Number.isInteger(y) ||
      x < -LIMITS.displayDimension || y < -LIMITS.displayDimension ||
      x > LIMITS.displayDimension || y > LIMITS.displayDimension) {
      throw new Error("Peer supplied invalid cursor position");
    }
  }
}

export function validDisplay(display: DisplayInfo): boolean {
  return Number.isInteger(display.width) && Number.isInteger(display.height) &&
    display.width > 0 && display.height > 0 &&
    display.width <= LIMITS.displayDimension && display.height <= LIMITS.displayDimension &&
    display.width * display.height <= LIMITS.displayPixels &&
    utf8.encode(display.name).byteLength <= LIMITS.controlText;
}

export function validatePeerInfo(info: PeerInfo): DisplayInfo {
  for (const text of [info.username, info.hostname, info.platform, info.version]) boundedText(text, "Peer information");
  if (info.displays.length === 0 || info.displays.length > 32) throw new Error("Peer has no bounded display");
  if (!info.displays.every(validDisplay)) throw new Error("Peer supplied invalid display dimensions");
  if (!Number.isInteger(info.currentDisplay) || info.currentDisplay < 0 || info.currentDisplay >= info.displays.length) {
    throw new Error("Peer selected an invalid display");
  }
  return info.displays[info.currentDisplay]!;
}
