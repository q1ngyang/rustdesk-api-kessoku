import { describe, expect, it } from "vitest";
import { keyEvent, relayPairingRequest } from "./client";
import { ControlKey, KeyEvent, KeyboardMode, Message, RendezvousMessage } from "./generated/kessoku_wire";
import { compatiblePeerVersion, countAppVariants, onlyRelayResponse, topLevelFieldNumbers, validateCursorMessage, validateDisabledSideChannel, validatePeerId, validDisplay } from "./wire";
import { webCodecsTimestamp } from "./video";

function keyboard(key: string, extra: Partial<KeyboardEvent> = {}): KeyboardEvent {
  return { key, altKey: false, shiftKey: false, ctrlKey: false, metaKey: false, isComposing: false, repeat: false, ...extra } as KeyboardEvent;
}

describe("wire policy", () => {
  it("accepts only bounded RustDesk IDs", () => {
    expect(validatePeerId("123_ABC-9")).toBe("123_ABC-9");
    expect(() => validatePeerId("bad/id")).toThrow();
    expect(() => validatePeerId("")).toThrow();
  });

  it("bounds the compatibility range", () => {
    expect(compatiblePeerVersion("1.4.9")).toBe(true);
    expect(compatiblePeerVersion("1.2.0-patch.1")).toBe(true);
    expect(compatiblePeerVersion("2.0.0")).toBe(false);
  });

  it("ignores unknown protobuf fields but rejects a non-Relay variant", () => {
    expect(RendezvousMessage.decode(new Uint8Array([8, 1]))).toEqual({
      punchHoleRequest: undefined, punchHoleResponse: undefined, requestRelay: undefined, relayResponse: undefined,
    });
    expect(() => onlyRelayResponse({
      punchHoleRequest: undefined, punchHoleResponse: undefined, requestRelay: undefined, relayResponse: undefined,
    })).toThrow();
  });

  it("enforces display resource bounds", () => {
    expect(validDisplay({ x: 0, y: 0, width: 1920, height: 1080, name: "main", online: true })).toBe(true);
    expect(validDisplay({ x: 0, y: 0, width: 8192, height: 8192, name: "huge", online: true })).toBe(false);
  });

  it("recognizes and bounds non-rendered cursor service messages", () => {
    const cursor = Message.create({
      cursorData: { id: 1n, hotx: 1, hoty: 1, width: 2, height: 2, colors: new Uint8Array([1, 2, 3]) },
    });
    expect(countAppVariants(cursor)).toBe(1);
    expect(() => validateCursorMessage(cursor)).not.toThrow();
    expect(() => validateCursorMessage(Message.create({
      cursorData: { id: 1n, hotx: 0, hoty: 0, width: 513, height: 1, colors: new Uint8Array(0) },
    }))).toThrow("invalid cursor data");
  });

  it("reports only bounded protobuf field numbers for unsupported messages", () => {
    expect(topLevelFieldNumbers(new Uint8Array([0x62, 0x00]))).toEqual([12]);
    expect(() => topLevelFieldNumbers(new Uint8Array([0x0b]))).toThrow("Invalid application message tag");
  });

  it("recognizes bounded Cliprdr field 20 bootstrap traffic while clipboard stays disabled", () => {
    const readyBytes = new Uint8Array([0xa2, 0x01, 0x02, 0x0a, 0x00]);
    expect(topLevelFieldNumbers(readyBytes)).toEqual([20]);
    const ready = Message.decode(readyBytes);
    expect(countAppVariants(ready)).toBe(1);
    expect(ready.cliprdr?.ready).toEqual({});
    expect(() => validateDisabledSideChannel(ready)).not.toThrow();

    expect(() => validateDisabledSideChannel(Message.create({
      cliprdr: {
        ready: undefined,
        formatList: undefined,
        formatListResponse: undefined,
        formatDataRequest: undefined,
        formatDataResponse: { msgFlags: 0, formatData: new Uint8Array(1_052_673) },
        fileContentsRequest: undefined,
        fileContentsResponse: undefined,
        tryEmpty: undefined,
        files: undefined,
      },
    }))).toThrow("oversized clipboard frame");
  });

  it("round-trips RustDesk chat as Misc field 19 without colliding with Cliprdr field 20", () => {
    const encoded = Message.encode(Message.create({
      misc: {
        chatMessage: { text: "Remote assistance message" },
        switchDisplay: undefined,
        option: undefined,
        closeReason: undefined,
        refreshVideo: undefined,
        videoReceived: undefined,
      },
    })).finish();

    expect(topLevelFieldNumbers(encoded)).toEqual([19]);
    const decoded = Message.decode(encoded);
    expect(countAppVariants(decoded)).toBe(1);
    expect(decoded.misc?.chatMessage?.text).toBe("Remote assistance message");
  });

  it("encodes printable and control keys in legacy mode", () => {
    expect(keyEvent(keyboard("é"), true)).toMatchObject({ chr: 233, down: true, press: false, mode: KeyboardMode.Legacy });
    expect(keyEvent(keyboard("é"), false)).toMatchObject({ chr: 233, down: false, press: false, mode: KeyboardMode.Legacy });
    expect(keyEvent(keyboard("Enter", { shiftKey: true }), true)).toMatchObject({
      controlKey: ControlKey.Return, down: true, press: false, modifiers: [ControlKey.Shift], mode: KeyboardMode.Legacy,
    });
  });

  it("uses the RustDesk 1.4.9 modifier and keyboard-mode field numbers", () => {
    expect(Array.from(KeyEvent.encode({
      down: true,
      press: false,
      controlKey: undefined,
      chr: 115,
      unicode: undefined,
      seq: undefined,
      win2winHotkey: undefined,
      modifiers: [ControlKey.Control],
      mode: KeyboardMode.Translate,
    }).finish())).toEqual([8, 1, 32, 115, 66, 1, 4, 72, 2]);
  });

  it("builds the HBBR pairing frame with default security and no token or address", () => {
    expect(relayPairingRequest("123456", "relay-uuid", "server-key").requestRelay).toEqual({
      id: "123456",
      uuid: "relay-uuid",
      socketAddr: new Uint8Array(0),
      relayServer: "",
      secure: false,
      licenceKey: "server-key",
      connType: 0,
      token: "",
    });
  });

  it("converts RustDesk millisecond PTS to WebCodecs microseconds", () => {
    expect(webCodecsTimestamp(42n)).toBe(42_000);
    expect(() => webCodecsTimestamp(-1n)).toThrow();
  });
});
