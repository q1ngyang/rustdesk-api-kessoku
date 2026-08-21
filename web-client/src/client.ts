import { frameBytes, utf8, wipe } from "./bytes";
import { createBox, openSignedMessage, passwordChallenge, SecretChannel } from "./crypto";
import {
  ConnType,
  ControlKey,
  IdPk,
  ImageQuality,
  KeyboardMode,
  Message,
  NatType,
  OptionMessage_BoolOption,
  RendezvousMessage,
  SupportedDecoding_PreferCodec,
  type KeyEvent,
  type MouseEvent,
  type RelayResponse,
  type TestDelay,
} from "./generated/kessoku_wire";
import { CLIENT_VERSION, LIMITS } from "./limits";
import { approvedRelay, type ClientProfile } from "./profile";
import { boundedText, countAppVariants, onlyRelayResponse, topLevelFieldNumbers, validateCursorMessage, validatePeerId, validatePeerInfo } from "./wire";
import { Vp9CanvasRenderer } from "./video";

export type ClientState = "idle" | "rendezvous" | "relay" | "handshake" | "authenticating" | "connected" | "disconnected" | "error";

export interface ConnectionInput {
  peerId: string;
  token: string;
  remotePassword: Uint8Array;
}

export interface ClientEvents {
  state(state: ClientState, detail: string): void;
  peer(name: string, platform: string): void;
}

function encodeRendezvous(message: Parameters<typeof RendezvousMessage.encode>[0]): Uint8Array {
  return RendezvousMessage.encode(message).finish();
}

function encodeMessage(message: Parameters<typeof Message.encode>[0]): Uint8Array {
  return Message.encode(message).finish();
}

export function relayPairingRequest(peerId: string, uuid: string, licenceKey: string): Parameters<typeof RendezvousMessage.encode>[0] {
  return {
    punchHoleRequest: undefined,
    punchHoleResponse: undefined,
    requestRelay: {
      id: peerId,
      uuid,
      socketAddr: new Uint8Array(0),
      relayServer: "",
      secure: false,
      licenceKey,
      connType: ConnType.DEFAULT_CONN,
      token: "",
    },
    relayResponse: undefined,
  };
}

export function testDelayResponse(message: TestDelay): TestDelay | undefined {
  // RustDesk echoes server-originated probes byte-for-byte. A frame marked
  // from_client is already the response to a probe initiated by this side;
  // this MVP does not initiate probes, so it is safe to ignore.
  return message.fromClient ? undefined : { ...message };
}

function sendBytes(socket: WebSocket, bytes: Uint8Array): void {
  if (socket.readyState !== WebSocket.OPEN) throw new Error("WebSocket is not open");
  if (bytes.byteLength > LIMITS.websocketFrame) throw new Error("Outbound frame exceeds limit");
  socket.send(bytes);
}

function openSocket(url: string): Promise<WebSocket> {
  return new Promise((resolve, reject) => {
    const socket = new WebSocket(url);
    socket.binaryType = "arraybuffer";
    const timer = window.setTimeout(() => {
      socket.close(1008, "timeout");
      reject(new Error("WebSocket connection timed out"));
    }, LIMITS.handshakeTimeoutMs);
    socket.addEventListener("open", () => {
      window.clearTimeout(timer);
      resolve(socket);
    }, { once: true });
    socket.addEventListener("error", () => {
      window.clearTimeout(timer);
      reject(new Error("WebSocket connection failed"));
    }, { once: true });
  });
}

function nextFrame(socket: WebSocket): Promise<Uint8Array> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => finish(new Error("Handshake response timed out")), LIMITS.handshakeTimeoutMs);
    const cleanup = (): void => {
      window.clearTimeout(timer);
      socket.removeEventListener("message", onMessage);
      socket.removeEventListener("close", onClose);
      socket.removeEventListener("error", onError);
    };
    const finish = (error: Error, bytes?: Uint8Array): void => {
      cleanup();
      if (bytes === undefined) reject(error); else resolve(bytes);
    };
    const onMessage = (event: MessageEvent<unknown>): void => {
      const size = event.data instanceof Blob ? event.data.size : event.data instanceof ArrayBuffer ? event.data.byteLength : LIMITS.websocketFrame + 1;
      if (size > LIMITS.websocketFrame) {
        finish(new Error("Inbound frame exceeds limit"));
        return;
      }
      void frameBytes(event.data).then((bytes) => finish(new Error("unused"), bytes), (error: unknown) => finish(error instanceof Error ? error : new Error("Invalid frame")));
    };
    const onClose = (): void => finish(new Error("WebSocket closed during handshake"));
    const onError = (): void => finish(new Error("WebSocket failed during handshake"));
    socket.addEventListener("message", onMessage);
    socket.addEventListener("close", onClose);
    socket.addEventListener("error", onError);
  });
}

export class RemoteClient {
  readonly #canvas: HTMLCanvasElement;
  readonly #events: ClientEvents;
  #state: ClientState = "idle";
  #profile: ClientProfile | undefined;
  #peerId = "";
  #token = "";
  #password: Uint8Array | undefined;
  #rendezvous: WebSocket | undefined;
  #relay: WebSocket | undefined;
  #channel: SecretChannel | undefined;
  #renderer: Vp9CanvasRenderer | undefined;
  #authStep: "hash" | "login" | "done" = "hash";
  #display = 0;
  #authTimer = 0;
  #receiveChain = Promise.resolve();

  constructor(canvas: HTMLCanvasElement, events: ClientEvents) {
    this.#canvas = canvas;
    this.#events = events;
  }

  get state(): ClientState { return this.#state; }

  async connect(profile: ClientProfile, input: ConnectionInput): Promise<void> {
    if (this.#state !== "idle" && this.#state !== "disconnected" && this.#state !== "error") throw new Error("A connection is already active");
    this.#clear(false);
    this.#profile = profile;
    this.#peerId = validatePeerId(input.peerId);
    if (input.token.length === 0 || input.token.length > LIMITS.token) throw new Error("Invalid connection token");
    if (input.remotePassword.byteLength === 0 || input.remotePassword.byteLength > LIMITS.passwordBytes) throw new Error("Invalid remote password");
    this.#token = input.token;
    this.#password = input.remotePassword.slice();
    wipe(input.remotePassword);
    try {
      const relayResponse = await this.#rendezvousPhase();
      await this.#relayPhase(relayResponse);
    } catch (error) {
      this.#fatal(error);
      throw error;
    }
  }

  async #rendezvousPhase(): Promise<RelayResponse> {
    const profile = this.#profile!;
    this.#setState("rendezvous", "Contacting rendezvous");
    const socket = await openSocket(profile.rendezvousUrl);
    this.#rendezvous = socket;
    const pending = nextFrame(socket);
    sendBytes(socket, encodeRendezvous({
      punchHoleRequest: {
        id: this.#peerId,
        natType: NatType.SYMMETRIC,
        licenceKey: profile.serverPublicKeyText,
        connType: ConnType.DEFAULT_CONN,
        token: this.#token,
        version: CLIENT_VERSION,
        udpPort: 0,
        forceRelay: true,
        upnpPort: 0,
        socketAddrV6: new Uint8Array(0),
      },
      punchHoleResponse: undefined,
      requestRelay: undefined,
      relayResponse: undefined,
    }));
    const frame = await pending;
    const response = onlyRelayResponse(RendezvousMessage.decode(frame));
    approvedRelay(profile, response.relayServer);
    socket.close(1000, "relay selected");
    this.#rendezvous = undefined;
    return response;
  }

  async #relayPhase(response: RelayResponse): Promise<void> {
    const profile = this.#profile!;
    this.#setState("relay", "Opening approved Relay");
    const socket = await openSocket(approvedRelay(profile, response.relayServer));
    this.#relay = socket;
    const pending = nextFrame(socket);
    sendBytes(socket, encodeRendezvous(relayPairingRequest(this.#peerId, response.uuid, profile.serverPublicKeyText)));
    this.#setState("handshake", "Verifying peer identity");
    const firstFrame = await pending;
    const first = Message.decode(firstFrame);
    if (countAppVariants(first) !== 1 || first.signedId === undefined ||
      first.signedId.id.byteLength < 65 || first.signedId.id.byteLength > LIMITS.controlText) {
      throw new Error("First Relay message was not a bounded SignedId");
    }

    const signedIdentity = await openSignedMessage(response.pk!, profile.serverPublicKey);
    const signingIdentity = IdPk.decode(signedIdentity);
    if (signingIdentity.id !== this.#peerId || signingIdentity.pk.byteLength !== 32) throw new Error("Server-signed peer identity mismatch");
    const signedSession = await openSignedMessage(first.signedId.id, signingIdentity.pk);
    const sessionIdentity = IdPk.decode(signedSession);
    if (sessionIdentity.id !== this.#peerId || sessionIdentity.pk.byteLength !== 32) throw new Error("Peer session identity mismatch");

    const sessionKey = crypto.getRandomValues(new Uint8Array(32));
    const box = await createBox(sessionIdentity.pk, sessionKey);
    sendBytes(socket, encodeMessage({
      signedId: undefined,
      publicKey: { asymmetricValue: box.publicKey, symmetricValue: box.sealed },
      testDelay: undefined,
      videoFrame: undefined,
      loginRequest: undefined,
      loginResponse: undefined,
      hash: undefined,
      mouseEvent: undefined,
      cursorData: undefined,
      cursorPosition: undefined,
      cursorId: undefined,
      keyEvent: undefined,
      misc: undefined,
      peerInfo: undefined,
    }));
    this.#channel = new SecretChannel(sessionKey);
    wipe(sessionKey); wipe(signedIdentity); wipe(signedSession); wipe(signingIdentity.pk); wipe(sessionIdentity.pk);
    this.#setState("authenticating", "Authenticating remote session");
    socket.addEventListener("message", (event) => {
      this.#receiveChain = this.#receiveChain.then(() => this.#receive(event)).catch((error: unknown) => this.#fatal(error));
    });
    socket.addEventListener("close", () => {
      if (this.#state !== "disconnected" && this.#state !== "error") this.#fatal(new Error("Relay connection closed"));
    });
    socket.addEventListener("error", () => this.#fatal(new Error("Relay connection failed")));
    this.#armAuthTimeout();
  }

  async #receive(event: MessageEvent<unknown>): Promise<void> {
    const size = event.data instanceof Blob ? event.data.size : event.data instanceof ArrayBuffer ? event.data.byteLength : LIMITS.websocketFrame + 1;
    if (size > LIMITS.websocketFrame) throw new Error("Inbound frame exceeds limit");
    const encrypted = await frameBytes(event.data);
    const plaintext = this.#channel!.open(encrypted);
    const fieldNumbers = topLevelFieldNumbers(plaintext);
    const message = Message.decode(plaintext);
    if (countAppVariants(message) !== 1) {
      throw new Error(`Unsupported application message field(s): ${fieldNumbers.join(",") || "none"}`);
    }
    // RustDesk sends TestDelay independently of authentication progress.  It
    // may arrive between the password challenge and LoginResponse, so handle
    // it before applying the authentication state machine.
    if (message.testDelay !== undefined) {
      const response = testDelayResponse(message.testDelay);
      if (response !== undefined) {
        this.#sendEncrypted({
          signedId: undefined, publicKey: undefined,
          testDelay: response,
          videoFrame: undefined, loginRequest: undefined, loginResponse: undefined, hash: undefined,
          mouseEvent: undefined, cursorData: undefined, cursorPosition: undefined, cursorId: undefined,
          keyEvent: undefined, misc: undefined, peerInfo: undefined,
        });
      }
      return;
    }
    if (this.#authStep === "hash") {
      if (message.hash === undefined) throw new Error("Expected password challenge");
      await this.#answerChallenge(message.hash.salt, message.hash.challenge);
      this.#armAuthTimeout();
      return;
    }
    if (this.#authStep === "login") {
      if (message.loginResponse === undefined) throw new Error("Expected LoginResponse");
      const response = message.loginResponse;
      const choices = Number(response.error !== undefined) + Number(response.peerInfo !== undefined);
      if (choices !== 1 || response.error !== undefined || response.peerInfo === undefined) {
        const detail = response.error === undefined ? "" : boundedText(response.error, "Login error");
        throw new Error(detail === "" ? "Remote login was rejected" : `Remote login was rejected: ${detail}`);
      }
      const display = validatePeerInfo(response.peerInfo);
      this.#display = response.peerInfo.currentDisplay;
      this.#renderer = await Vp9CanvasRenderer.create(this.#canvas, () => this.#sendVideoAck(), (error) => this.#fatal(error));
      this.#renderer.setDisplay(display.width, display.height);
      this.#events.peer(response.peerInfo.hostname, response.peerInfo.platform);
      this.#authStep = "done";
      window.clearTimeout(this.#authTimer);
      this.#authTimer = 0;
      this.#setState("connected", "Connected");
      return;
    }
    if (message.cursorData !== undefined || message.cursorPosition !== undefined || message.cursorId !== undefined) {
      validateCursorMessage(message);
      return;
    }
    if (message.peerInfo !== undefined) {
      const display = validatePeerInfo(message.peerInfo);
      this.#display = message.peerInfo.currentDisplay;
      this.#renderer!.setDisplay(display.width, display.height);
      return;
    }
    if (message.videoFrame !== undefined) {
      if (message.videoFrame.vp9s === undefined || message.videoFrame.display !== this.#display) throw new Error("Unsupported video stream");
      this.#renderer!.decode(message.videoFrame.vp9s);
      return;
    }
    if (message.misc?.closeReason !== undefined) {
      boundedText(message.misc.closeReason, "Close reason");
      this.disconnect(message.misc.closeReason || "Peer disconnected");
    }
  }

  async #answerChallenge(salt: string, challenge: string): Promise<void> {
    boundedText(salt, "Challenge salt");
    boundedText(challenge, "Challenge");
    if (this.#password === undefined) throw new Error("Remote password is unavailable");
    const response = await passwordChallenge(this.#password, salt, challenge);
    wipe(this.#password);
    this.#password = undefined;
    this.#sendEncrypted({
      signedId: undefined,
      publicKey: undefined,
      testDelay: undefined,
      videoFrame: undefined,
      loginRequest: {
        // For a rendezvous/Relay session RustDesk uses `username` as the
        // destination identity guard, not as an operating-system account.
        username: this.#peerId,
        password: response,
        myId: String(100_000_000 + crypto.getRandomValues(new Uint32Array(1))[0]! % 900_000_000),
        myName: "Kessoku Web",
        option: {
          imageQuality: ImageQuality.Balanced,
          showRemoteCursor: OptionMessage_BoolOption.No,
          disableAudio: OptionMessage_BoolOption.Yes,
          disableClipboard: OptionMessage_BoolOption.Yes,
          supportedDecoding: {
            abilityVp9: 1,
            abilityH264: 0,
            abilityH265: 0,
            prefer: SupportedDecoding_PreferCodec.VP9,
            abilityVp8: 0,
            abilityAv1: 0,
            i444: { vp8: false, vp9: false, av1: false, h264: false, h265: false },
          },
          disableKeyboard: OptionMessage_BoolOption.No,
        },
        videoAckRequired: true,
        version: CLIENT_VERSION,
        myPlatform: "Web",
      },
      loginResponse: undefined,
      hash: undefined,
      mouseEvent: undefined,
      cursorData: undefined,
      cursorPosition: undefined,
      cursorId: undefined,
      keyEvent: undefined,
      misc: undefined,
      peerInfo: undefined,
    });
    wipe(response);
    this.#authStep = "login";
  }

  #sendEncrypted(message: Parameters<typeof Message.encode>[0]): void {
    if (this.#relay === undefined || this.#channel === undefined) throw new Error("Secure channel is unavailable");
    const plaintext = encodeMessage(message);
    const encrypted = this.#channel.seal(plaintext);
    wipe(plaintext);
    sendBytes(this.#relay, encrypted);
  }

  #armAuthTimeout(): void {
    window.clearTimeout(this.#authTimer);
    this.#authTimer = window.setTimeout(() => this.#fatal(new Error("Remote authentication timed out")), LIMITS.handshakeTimeoutMs);
  }

  #sendVideoAck(): void {
    if (this.#state !== "connected") return;
    this.#sendEncrypted({
      signedId: undefined, publicKey: undefined, testDelay: undefined, videoFrame: undefined,
      loginRequest: undefined, loginResponse: undefined, hash: undefined, mouseEvent: undefined,
      cursorData: undefined, cursorPosition: undefined, cursorId: undefined, keyEvent: undefined,
      misc: { switchDisplay: undefined, option: undefined, closeReason: undefined, refreshVideo: undefined, videoReceived: true },
      peerInfo: undefined,
    });
  }

  sendMouse(event: MouseEvent): void {
    if (this.#state !== "connected") return;
    if (!Number.isInteger(event.x) || !Number.isInteger(event.y) || event.x < -LIMITS.displayDimension || event.y < -LIMITS.displayDimension ||
      event.x > LIMITS.displayDimension || event.y > LIMITS.displayDimension || !Number.isInteger(event.mask) || event.mask < 0 || event.mask > 255) return;
    this.#sendEncrypted({
      signedId: undefined, publicKey: undefined, testDelay: undefined, videoFrame: undefined,
      loginRequest: undefined, loginResponse: undefined, hash: undefined,
      mouseEvent: event, cursorData: undefined, cursorPosition: undefined, cursorId: undefined,
      keyEvent: undefined, misc: undefined, peerInfo: undefined,
    });
  }

  sendKey(event: KeyEvent): void {
    if (this.#state !== "connected") return;
    if (event.modifiers.length > 4 || event.seq !== undefined && utf8.encode(event.seq).byteLength > 32) return;
    this.#sendEncrypted({
      signedId: undefined, publicKey: undefined, testDelay: undefined, videoFrame: undefined,
      loginRequest: undefined, loginResponse: undefined, hash: undefined,
      mouseEvent: undefined, cursorData: undefined, cursorPosition: undefined, cursorId: undefined,
      keyEvent: event, misc: undefined, peerInfo: undefined,
    });
  }

  disconnect(detail = "Disconnected"): void {
    if (this.#state === "disconnected") return;
    this.#clear(true);
    this.#setState("disconnected", detail);
  }

  #fatal(error: unknown): void {
    if (this.#state === "error" || this.#state === "disconnected") return;
    const message = error instanceof Error ? error.message : "Connection failed closed";
    this.#clear(true);
    this.#setState("error", message);
  }

  #clear(closeSockets: boolean): void {
    if (closeSockets) {
      if (this.#rendezvous?.readyState === WebSocket.OPEN) this.#rendezvous.close(1000, "client disconnect");
      if (this.#relay?.readyState === WebSocket.OPEN) this.#relay.close(1000, "client disconnect");
    }
    this.#rendezvous = undefined;
    this.#relay = undefined;
    this.#channel?.close();
    this.#channel = undefined;
    this.#renderer?.close();
    this.#renderer = undefined;
    wipe(this.#password);
    this.#password = undefined;
    this.#token = "";
    this.#peerId = "";
    this.#authStep = "hash";
    this.#display = 0;
    window.clearTimeout(this.#authTimer);
    this.#authTimer = 0;
  }

  #setState(state: ClientState, detail: string): void {
    this.#state = state;
    this.#events.state(state, detail);
  }
}

export function keyboardModifiers(event: KeyboardEvent): ControlKey[] {
  const output: ControlKey[] = [];
  if (event.altKey) output.push(ControlKey.Alt);
  if (event.shiftKey) output.push(ControlKey.Shift);
  if (event.ctrlKey) output.push(ControlKey.Control);
  if (event.metaKey) output.push(ControlKey.Meta);
  return output;
}

export function keyEvent(event: KeyboardEvent, down: boolean): KeyEvent | undefined {
  if (event.isComposing || event.repeat) return undefined;
  const controls: Readonly<Record<string, ControlKey>> = {
    Alt: ControlKey.Alt, Backspace: ControlKey.Backspace, CapsLock: ControlKey.CapsLock, Control: ControlKey.Control,
    Delete: ControlKey.Delete, ArrowDown: ControlKey.DownArrow, End: ControlKey.End, Escape: ControlKey.Escape,
    F1: ControlKey.F1, F2: ControlKey.F2, F3: ControlKey.F3, F4: ControlKey.F4, F5: ControlKey.F5, F6: ControlKey.F6,
    F7: ControlKey.F7, F8: ControlKey.F8, F9: ControlKey.F9, F10: ControlKey.F10, F11: ControlKey.F11, F12: ControlKey.F12,
    Home: ControlKey.Home, ArrowLeft: ControlKey.LeftArrow, Meta: ControlKey.Meta, PageDown: ControlKey.PageDown,
    PageUp: ControlKey.PageUp, Enter: ControlKey.Return, ArrowRight: ControlKey.RightArrow, Shift: ControlKey.Shift,
    " ": ControlKey.Space, Tab: ControlKey.Tab, ArrowUp: ControlKey.UpArrow, Insert: ControlKey.Insert,
  };
  const base = { down, press: false, modifiers: keyboardModifiers(event), mode: KeyboardMode.Legacy };
  const controlKey = controls[event.key];
  if (controlKey !== undefined) return { ...base, controlKey, chr: undefined, unicode: undefined, seq: undefined };
  const points = Array.from(event.key);
  if (points.length === 1) {
    return { ...base, controlKey: undefined, chr: points[0]!.codePointAt(0), unicode: undefined, seq: undefined };
  }
  return undefined;
}
