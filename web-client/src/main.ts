import "./styles.css";
import { login, logout } from "./auth";
import { utf8, wipe } from "./bytes";
import { keyEvent, RemoteClient, type ClientState } from "./client";
import { ControlKey } from "./generated/kessoku_wire";
import { receiveAdminGrant } from "./grant";
import { formDisabledState } from "./form_state";
import { loadProfile, type ClientProfile } from "./profile";

function element<K extends keyof HTMLElementTagNameMap>(tag: K, className?: string, text?: string): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className !== undefined) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function field(label: string, type: string, autocomplete: HTMLInputElement["autocomplete"]): { wrap: HTMLLabelElement; input: HTMLInputElement } {
  const wrap = element("label", "field");
  wrap.append(element("span", "field-label", label));
  const input = element("input", "field-input");
  input.type = type;
  input.autocomplete = autocomplete;
  input.required = true;
  input.spellcheck = false;
  wrap.append(input);
  return { wrap, input };
}

const root = document.querySelector<HTMLElement>("#app");
if (root === null) throw new Error("Application root is missing");

const shell = element("section", "shell");
const header = element("header", "topbar");
const brand = element("div", "brand");
brand.append(element("span", "brand-mark", "K"), element("strong", "brand-name", "Kessoku Remote"));
const status = element("div", "status");
const statusDot = element("span", "status-dot");
const statusText = element("span", "status-text", "Loading secure profile…");
status.append(statusDot, statusText);
header.append(brand, status);

const stage = element("div", "stage");
const canvas = element("canvas", "remote-canvas");
canvas.tabIndex = 0;
canvas.setAttribute("aria-label", "Remote desktop. Focus to send keyboard input.");
const empty = element("div", "empty-state");
empty.append(element("div", "empty-icon", "⌁"), element("h1", "empty-title", "A private path to your desktop"),
  element("p", "empty-copy", "Forced Relay, authenticated encryption, and VP9 WebCodecs. Secrets stay in memory."));
stage.append(canvas, empty);

const panel = element("aside", "panel");
panel.append(element("h2", "panel-title", "Connect"), element("p", "panel-copy", "Sign in to mint a short-lived connection grant."));
const form = element("form", "connect-form");
const username = field("Account", "text", "username");
const accountPassword = field("Account password", "password", "current-password");
const peerId = field("Remote ID", "text", "off");
const remotePassword = field("Remote password", "password", "off");
peerId.input.inputMode = "text";
const actions = element("div", "actions");
const connectButton = element("button", "primary", "Connect");
connectButton.type = "submit";
connectButton.disabled = true;
const disconnectButton = element("button", "secondary", "Disconnect");
disconnectButton.type = "button";
disconnectButton.hidden = true;
actions.append(connectButton, disconnectButton);
const safety = element("p", "safety", "No direct connection · Audio and clipboard disabled");
form.append(username.wrap, accountPassword.wrap, peerId.wrap, remotePassword.wrap, actions, safety);
panel.append(form);

const layout = element("div", "layout");
layout.append(stage, panel);
shell.append(header, layout);
root.append(shell);

let profile: ClientProfile | undefined;
let connectionToken = "";
let connectionTokenExpiresAt = 0;
let busy = false;
let accountGrantDelivered = false;
let stopGrantReceiver = (): void => undefined;

function updateInputDisabled(active: boolean): void {
  const disabled = formDisabledState(active, busy, accountGrantDelivered);
  username.input.disabled = disabled.account;
  accountPassword.input.disabled = disabled.account;
  peerId.input.disabled = disabled.connection;
  remotePassword.input.disabled = disabled.connection;
}

function setState(state: ClientState | "loading", detail: string): void {
  status.dataset.state = state;
  statusText.textContent = detail;
  const active = state === "connected";
  empty.hidden = active;
  canvas.hidden = !active;
  disconnectButton.hidden = !active;
  connectButton.hidden = active;
  connectButton.disabled = busy || profile === undefined;
  updateInputDisabled(active);
}

async function revokeToken(): Promise<void> {
  const token = connectionToken;
  connectionToken = "";
  connectionTokenExpiresAt = 0;
  if (profile !== undefined && token !== "") {
    try { await logout(profile, token); } catch { /* Token expires shortly; never log it. */ }
  }
  if (accountGrantDelivered) {
    accountGrantDelivered = false;
    username.wrap.hidden = false;
    accountPassword.wrap.hidden = false;
    peerId.input.readOnly = false;
    panel.querySelector<HTMLElement>(".panel-copy")!.textContent = "Sign in to mint a short-lived connection grant.";
    updateInputDisabled(false);
  }
}

const client = new RemoteClient(canvas, {
  state(state, detail) {
    setState(state, detail);
    if (state === "error" || state === "disconnected") void revokeToken();
  },
  peer(name, platform) {
    statusText.textContent = name === "" ? `Connected · ${platform}` : `Connected to ${name} · ${platform}`;
  },
});

void loadProfile().then((loaded) => {
  profile = loaded;
  setState("idle", `Ready · profile ${loaded.generation}`);
  stopGrantReceiver = receiveAdminGrant(loaded, (grant) => {
    if (busy || client.state !== "idle") throw new Error("Client is not ready for an admin grant");
    connectionToken = grant.token;
    connectionTokenExpiresAt = grant.expiresAt;
    accountGrantDelivered = true;
    peerId.input.value = grant.peerId;
    peerId.input.readOnly = true;
    username.input.value = "";
    accountPassword.input.value = "";
    username.wrap.hidden = true;
    accountPassword.wrap.hidden = true;
    panel.querySelector<HTMLElement>(".panel-copy")!.textContent = "Connection grant received. Enter only the remote desktop password.";
    setState("idle", `Grant ready · profile ${loaded.generation}`);
    remotePassword.input.focus();
  });
}, (error: unknown) => {
  setState("error", error instanceof Error ? error.message : "Profile rejected");
  connectButton.disabled = true;
});

form.addEventListener("submit", (event) => {
  event.preventDefault();
  if (busy || profile === undefined) return;
  busy = true;
  setState("loading", "Requesting short-lived grant");
  const accountName = username.input.value;
  const accountSecret = accountPassword.input.value;
  const target = peerId.input.value;
  const remoteSecret = utf8.encode(remotePassword.input.value);
  accountPassword.input.value = "";
  remotePassword.input.value = "";
  const granted = accountGrantDelivered && connectionToken !== "";
  if (granted && connectionTokenExpiresAt <= Math.floor(Date.now() / 1000)) {
    wipe(remoteSecret);
    busy = false;
    void revokeToken();
    setState("error", "The connection grant expired; launch the client again");
    return;
  }
  const tokenRequest = granted ? Promise.resolve(connectionToken) : login(profile, accountName, accountSecret);
  void tokenRequest.then(async (token) => {
    connectionToken = token;
    await client.connect(profile!, { peerId: target, token, remotePassword: remoteSecret });
  }).catch(async (error: unknown) => {
    wipe(remoteSecret);
    client.disconnect(error instanceof Error ? error.message : "Connection failed");
    await revokeToken();
  }).finally(() => {
    busy = false;
    const locked = client.state !== "idle" && client.state !== "disconnected" && client.state !== "error";
    updateInputDisabled(locked);
    connectButton.disabled = locked || profile === undefined;
  });
});

disconnectButton.addEventListener("click", () => client.disconnect("Disconnected by user"));

function position(event: PointerEvent): { x: number; y: number } {
  const rect = canvas.getBoundingClientRect();
  return {
    x: Math.max(0, Math.min(canvas.width - 1, Math.round((event.clientX - rect.left) * canvas.width / rect.width))),
    y: Math.max(0, Math.min(canvas.height - 1, Math.round((event.clientY - rect.top) * canvas.height / rect.height))),
  };
}

function modifiers(event: PointerEvent): ControlKey[] {
  const output: ControlKey[] = [];
  if (event.altKey) output.push(ControlKey.Alt);
  if (event.ctrlKey) output.push(ControlKey.Control);
  if (event.metaKey) output.push(ControlKey.Meta);
  if (event.shiftKey) output.push(ControlKey.Shift);
  return output;
}

let pendingMove: PointerEvent | undefined;
let moveFrame = 0;
canvas.addEventListener("pointermove", (event) => {
  pendingMove = event;
  if (moveFrame !== 0) return;
  moveFrame = requestAnimationFrame(() => {
    const move = pendingMove;
    pendingMove = undefined;
    moveFrame = 0;
    if (move !== undefined) client.sendMouse({ mask: move.buttons << 3, ...position(move), modifiers: modifiers(move) });
  });
});
canvas.addEventListener("pointerdown", (event) => {
  canvas.focus();
  canvas.setPointerCapture(event.pointerId);
  const button = event.button === 0 ? 1 : event.button === 2 ? 2 : event.button === 1 ? 4 : 0;
  if (button !== 0) client.sendMouse({ mask: (button << 3) | 1, ...position(event), modifiers: modifiers(event) });
  event.preventDefault();
});
canvas.addEventListener("pointerup", (event) => {
  const button = event.button === 0 ? 1 : event.button === 2 ? 2 : event.button === 1 ? 4 : 0;
  if (button !== 0) client.sendMouse({ mask: (button << 3) | 2, ...position(event), modifiers: modifiers(event) });
  event.preventDefault();
});
canvas.addEventListener("contextmenu", (event) => event.preventDefault());
canvas.addEventListener("wheel", (event) => {
  const clamp = (value: number): number => Math.max(-8192, Math.min(8192, Math.round(value)));
  client.sendMouse({ mask: (event.buttons << 3) | 3, x: clamp(event.deltaX), y: clamp(event.deltaY), modifiers: modifiers(event as unknown as PointerEvent) });
  event.preventDefault();
}, { passive: false });
canvas.addEventListener("keydown", (event) => {
  const wireEvent = keyEvent(event, true);
  if (wireEvent !== undefined) {
    client.sendKey(wireEvent);
    event.preventDefault();
  }
});
canvas.addEventListener("keyup", (event) => {
  const wireEvent = keyEvent(event, false);
  if (wireEvent !== undefined) {
    client.sendKey(wireEvent);
    event.preventDefault();
  }
});

window.addEventListener("pagehide", () => {
  stopGrantReceiver();
  client.disconnect("Page closed");
  void revokeToken();
});
