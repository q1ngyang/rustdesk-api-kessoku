import "./styles.css";
import defaultIconDark from "./assets/starrydesk-icon-dark.svg";
import defaultIconLight from "./assets/starrydesk-icon-light.svg";
import defaultLogoDark from "./assets/starrydesk-logo-dark.svg";
import defaultLogoLight from "./assets/starrydesk-logo-light.svg";
import { browserSession, browserSessionGrant, browserSessionLogout, completeTwoFactor, establishBrowserSession, finishConnectionAudit, login, logout, saveBrowserPreferences, startConnectionAudit, type BrowserSession, type ConnectionAudit } from "./auth";
import { utf8, wipe } from "./bytes";
import { keyEvent, RemoteClient, type ClientState } from "./client";
import { ControlKey } from "./generated/kessoku_wire";
import { receiveAdminGrant } from "./grant";
import { formDisabledState } from "./form_state";
import { loadProfile, type ClientProfile } from "./profile";
import { configureLocale, documentLocale, localeOptions, t } from "./i18n";
import { LIMITS } from "./limits";
import { applyTheme, resolveTheme, type ThemePreference } from "./preferences";
import { normalizePeerId } from "./wire";

let initialProfile: ClientProfile | undefined;
let initialProfileError: unknown;
try {
  initialProfile = await loadProfile();
  configureLocale(initialProfile.preferences.language);
} catch (error: unknown) {
  initialProfileError = error;
}
let activeTheme: ThemePreference = resolveTheme(initialProfile?.preferences.theme);
applyTheme(activeTheme);
document.documentElement.lang = documentLocale();

function element<K extends keyof HTMLElementTagNameMap>(tag: K, className?: string, text?: string): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className !== undefined) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

interface ThemedImage {
  readonly root: HTMLSpanElement;
  readonly light: HTMLImageElement;
  readonly dark: HTMLImageElement;
}

function themedImage(className: string, lightUrl: string, darkUrl: string): ThemedImage {
  const root = element("span", `${className} theme-image`);
  const light = element("img", "theme-image__light");
  const dark = element("img", "theme-image__dark");
  light.alt = "";
  dark.alt = "";
  if (lightUrl !== "") light.src = lightUrl;
  if (darkUrl !== "") dark.src = darkUrl;
  root.append(light, dark);
  return { root, light, dark };
}

function setThemedImage(image: ThemedImage, lightUrl: string, darkUrl: string): void {
  if (lightUrl === "") image.light.removeAttribute("src");
  else image.light.src = lightUrl;
  if (darkUrl === "") image.dark.removeAttribute("src");
  else image.dark.src = darkUrl;
  image.root.hidden = lightUrl === "" && darkUrl === "";
}

const defaultFooterHTML = '<a href="https://github.com/q1ngyang/rustdesk-api-kessoku" target="_blank" rel="noopener noreferrer"><span>RustDesk API Kessoku</span><span>Github</span></a>';

function renderSafeFooter(container: HTMLElement, source: string): void {
  const parsed = new DOMParser().parseFromString(source || defaultFooterHTML, "text/html");
  const allowed = new Set(["A", "SPAN", "STRONG", "EM"]);
  const copyNode = (node: Node): Node => {
    if (node.nodeType === Node.TEXT_NODE) return document.createTextNode(node.textContent || "");
    const fragment = document.createDocumentFragment();
    if (!(node instanceof HTMLElement)) return fragment;
    let destination: HTMLElement | DocumentFragment = fragment;
    if (allowed.has(node.tagName)) {
      const elementName = node.tagName.toLowerCase() as "a" | "span" | "strong" | "em";
      const clean = document.createElement(elementName);
      if (clean instanceof HTMLAnchorElement) {
        try {
          const target = new URL(node.getAttribute("href") || "", location.href);
          if (target.protocol !== "https:" || target.username !== "" || target.password !== "") throw new Error("unsafe footer link");
          clean.href = target.href;
          clean.target = "_blank";
          clean.rel = "noopener noreferrer";
        } catch { /* Render link text without navigation. */ }
      }
      destination = clean;
    }
    for (const child of node.childNodes) destination.append(copyNode(child));
    return destination;
  };
  container.replaceChildren(...Array.from(parsed.body.childNodes, copyNode));
}

let faviconLight = defaultIconLight;
let faviconDark = defaultIconDark;
function syncFavicon(): void {
  let favicon = document.querySelector<HTMLLinkElement>("link[rel='icon']");
  if (favicon === null) {
    favicon = element("link") as HTMLLinkElement;
    favicon.rel = "icon";
    document.head.append(favicon);
  }
  favicon.href = activeTheme === "dark" ? faviconDark : faviconLight;
}
syncFavicon();

function field(label: string, type: string, autocomplete: HTMLInputElement["autocomplete"], revealable = false): { wrap: HTMLLabelElement; input: HTMLInputElement; reveal?: HTMLButtonElement } {
  const wrap = element("label", "field");
  wrap.append(element("span", "field-label", label));
  const input = element("input", "field-input");
  input.type = type;
  input.autocomplete = autocomplete;
  input.required = true;
  input.spellcheck = false;
  if (!revealable) {
    wrap.append(input);
    return { wrap, input };
  }
  const control = element("span", "field-control");
  const reveal = element("button", "field-reveal", t("showPassword"));
  reveal.type = "button";
  reveal.setAttribute("aria-pressed", "false");
  reveal.addEventListener("click", () => {
    const showing = input.type === "text";
    input.type = showing ? "password" : "text";
    reveal.textContent = showing ? t("showPassword") : t("hidePassword");
    reveal.setAttribute("aria-pressed", String(!showing));
    input.focus();
  });
  control.append(input, reveal);
  wrap.append(control);
  return { wrap, input, reveal };
}

const root = document.querySelector<HTMLElement>("#app");
if (root === null) throw new Error("Application root is missing");

const pageBackground = themedImage("page-background", "", "");
const shell = element("section", "shell");
const header = element("header", "topbar");
const brand = element("div", "brand");
const brandIcon = themedImage("brand-icon", defaultIconLight, defaultIconDark);
const brandName = element("strong", "brand-name", "Kessoku Remote");
brand.append(brandIcon.root, brandName);
const headerActions = element("div", "topbar-actions");
const preferenceControls = element("div", "preference-controls");
const languageMenu = element("div", "language-menu");
const languageControl = element("button", "preference-control preference-control--language");
languageControl.type = "button";
languageControl.title = t("language");
languageControl.setAttribute("aria-label", t("language"));
languageControl.setAttribute("aria-haspopup", "menu");
languageControl.setAttribute("aria-expanded", "false");
languageControl.append(element("span", "preference-control__glyph", "文/A"));
const languagePopup = element("div", "language-popup");
languagePopup.setAttribute("role", "menu");
languagePopup.hidden = true;
for (const option of localeOptions) {
  const node = element("button", "language-option");
  node.type = "button";
  node.dataset.locale = option.value;
  node.setAttribute("role", "menuitemradio");
  node.setAttribute("aria-checked", String(option.value === documentLocale()));
  node.append(element("span", "", option.label), element("span", "language-option__check", option.value === documentLocale() ? "✓" : ""));
  node.addEventListener("click", () => {
    if (profile === undefined || option.value === documentLocale()) {
      languagePopup.hidden = true;
      languageControl.setAttribute("aria-expanded", "false");
      return;
    }
    languagePopup.querySelectorAll("button").forEach(button => { (button as HTMLButtonElement).disabled = true; });
    void saveBrowserPreferences(profile, { language: option.value }).then(() => location.reload()).catch(() => {
      languagePopup.querySelectorAll("button").forEach(button => { (button as HTMLButtonElement).disabled = false; });
    });
  });
  languagePopup.append(node);
}
languageMenu.append(languageControl, languagePopup);
const themeButton = element("button", "preference-control preference-control--theme");
themeButton.type = "button";
const renderThemeButton = (): void => {
  const useDark = activeTheme === "light";
  themeButton.textContent = useDark ? "☾" : "☀";
  themeButton.title = t(useDark ? "switchToDark" : "switchToLight");
  themeButton.setAttribute("aria-label", themeButton.title);
};
renderThemeButton();
preferenceControls.append(languageMenu, themeButton);
const status = element("div", "status");
const statusDot = element("span", "status-dot");
const statusText = element("span", "status-text", t("loading"));
status.append(statusDot, statusText);
headerActions.append(preferenceControls, status);
header.append(brand, headerActions);

languageControl.addEventListener("click", () => {
  languagePopup.hidden = !languagePopup.hidden;
  languageControl.setAttribute("aria-expanded", String(!languagePopup.hidden));
});
document.addEventListener("click", event => {
  if (languageMenu.contains(event.target as Node)) return;
  languagePopup.hidden = true;
  languageControl.setAttribute("aria-expanded", "false");
});
document.addEventListener("keydown", event => {
  if (event.key !== "Escape" || languagePopup.hidden) return;
  languagePopup.hidden = true;
  languageControl.setAttribute("aria-expanded", "false");
  languageControl.focus();
});
themeButton.addEventListener("click", () => {
  const previous = activeTheme;
  activeTheme = previous === "dark" ? "light" : "dark";
  applyTheme(activeTheme);
  syncFavicon();
  renderThemeButton();
  if (profile !== undefined) void saveBrowserPreferences(profile, { theme: activeTheme }).catch(() => {
    activeTheme = previous;
    applyTheme(activeTheme);
    syncFavicon();
    renderThemeButton();
  });
});

const stage = element("div", "stage");
const canvas = element("canvas", "remote-canvas");
canvas.tabIndex = 0;
canvas.setAttribute("aria-label", t("remoteAria"));
const empty = element("div", "empty-state");
empty.append(element("div", "empty-icon", "⌁"), element("h1", "empty-title", t("heroTitle")),
  element("p", "empty-copy", t("heroCopy")));
stage.append(canvas, empty);

const panel = element("aside", "panel");
const panelIntro = element("div", "panel-intro");
const panelIntroCopy = element("div", "panel-intro__copy");
const panelCopy = element("p", "panel-copy", t("signInCopy"));
panelIntroCopy.append(element("h2", "panel-title", t("connect")), panelCopy);
const panelBrandLogo = themedImage("panel-brand-logo", defaultLogoLight, defaultLogoDark);
panelIntro.append(panelIntroCopy, panelBrandLogo.root);
panel.append(panelIntro);
const sessionBar = element("div", "session-bar");
sessionBar.hidden = true;
const sessionIdentity = element("span", "session-identity");
const sessionLogoutButton = element("button", "session-logout", t("signOut"));
sessionLogoutButton.type = "button";
sessionBar.append(sessionIdentity, sessionLogoutButton);
const form = element("form", "connect-form");
const username = field(t("account"), "text", "username");
const accountPassword = field(t("accountPassword"), "password", "current-password", true);
const twoFactor = field(t("twoFactor"), "text", "one-time-code");
twoFactor.input.inputMode = "numeric";
twoFactor.input.pattern = "[0-9]{6}";
twoFactor.input.maxLength = 6;
twoFactor.input.required = false;
twoFactor.wrap.hidden = true;
const peerId = field(t("remoteId"), "text", "off");
const remotePassword = field(t("remotePassword"), "password", "off", true);
peerId.input.inputMode = "text";
const actions = element("div", "actions");
const connectButton = element("button", "primary", t("connect"));
connectButton.type = "submit";
connectButton.disabled = true;
const disconnectButton = element("button", "secondary", t("disconnect"));
disconnectButton.type = "button";
disconnectButton.hidden = true;
actions.append(connectButton, disconnectButton);
const safety = element("p", "safety", t("safety"));
form.append(username.wrap, accountPassword.wrap, twoFactor.wrap, peerId.wrap, remotePassword.wrap, actions, safety);
panel.append(sessionBar, form);

const chatSection = element("section", "chat-panel");
chatSection.hidden = true;
const chatHeader = element("div", "chat-header");
chatHeader.append(element("strong", "", t("supportChat")), element("span", "", t("endToEndSession")));
const chatMessages = element("div", "chat-messages");
chatMessages.setAttribute("aria-live", "polite");
const chatEmpty = element("p", "chat-empty", t("chatEmpty"));
chatMessages.append(chatEmpty);
const chatForm = element("form", "chat-form");
const chatInput = element("textarea", "chat-input");
chatInput.placeholder = t("chatPlaceholder");
chatInput.maxLength = 2000;
chatInput.rows = 2;
const chatSend = element("button", "chat-send", t("send"));
chatSend.type = "submit";
chatForm.append(chatInput, chatSend);
chatSection.append(chatHeader, chatMessages, chatForm);
panel.append(chatSection);
const panelFooter = element("footer", "panel-footer");
renderSafeFooter(panelFooter, "");
panel.append(panelFooter);

const layout = element("div", "layout");
layout.append(stage, panel);
shell.append(header, layout);
root.append(pageBackground.root, shell);

let profile: ClientProfile | undefined = initialProfile;
let connectionToken = "";
let connectionTokenExpiresAt = 0;
let busy = false;
let accountGrantDelivered = false;
let launchedWithAdminGrant = false;
let requiresRelaunch = false;
let loginChallenge = "";
let accountSessionActive = false;
let localDisplayName = t("you");
let localAvatar = "";
let remoteDisplayName = t("remote");
let reportedRemoteName = "";
let registeredRemoteHostname = "";
let remotePlatform = "";
let stopGrantReceiver = (): void => undefined;
let activeAuditPromise: Promise<ConnectionAudit | undefined> | undefined;
let activeAuditToken = "";
let activePeerID = "";

function beginConnectionAudit(): void {
  if (profile === undefined || connectionToken === "" || activePeerID === "" || activeAuditPromise !== undefined) return;
  activeAuditToken = connectionToken;
  const peerID = activePeerID;
  activeAuditPromise = startConnectionAudit(profile, activeAuditToken, peerID).then((audit) => {
    if (activePeerID === peerID && client.state === "connected" && audit.peerHostname !== "") {
      registeredRemoteHostname = audit.peerHostname;
      renderRemoteIdentity();
    }
    return audit;
  }).catch(() => undefined);
}

async function closeConnectionAudit(): Promise<void> {
  const pending = activeAuditPromise;
  const token = activeAuditToken;
  activeAuditPromise = undefined;
  activeAuditToken = "";
  activePeerID = "";
  if (profile === undefined || pending === undefined || token === "") return;
  const audit = await pending;
  if (audit !== undefined) await finishConnectionAudit(profile, token, audit).catch(() => undefined);
}

function updateInputDisabled(active: boolean): void {
  const disabled = formDisabledState(active, busy, accountGrantDelivered);
  const accountDisabled = disabled.account || accountSessionActive;
  username.input.disabled = accountDisabled;
  accountPassword.input.disabled = accountDisabled;
	if (accountPassword.reveal !== undefined) accountPassword.reveal.disabled = accountDisabled;
	twoFactor.input.disabled = accountDisabled;
  peerId.input.disabled = disabled.connection;
  remotePassword.input.disabled = disabled.connection;
	if (remotePassword.reveal !== undefined) remotePassword.reveal.disabled = disabled.connection;
}

function renderRemoteIdentity(): void {
  const name = registeredRemoteHostname || reportedRemoteName;
  remoteDisplayName = name || t("remote");
  if (client.state === "connected") {
    statusText.textContent = name === "" ? t("connected", { platform: remotePlatform }) : t("connectedTo", { name, platform: remotePlatform });
  }
}

function setAccountSession(active: boolean, account = "", displayName = "", avatar = ""): void {
  accountSessionActive = active;
  if (active) {
    localDisplayName = displayName || account || t("you");
    localAvatar = avatar;
  } else {
    localDisplayName = t("you");
    localAvatar = "";
  }
  sessionBar.hidden = !active;
  sessionIdentity.textContent = active ? t("signedInAs", { name: account }) : "";
  sessionLogoutButton.disabled = busy || client.state === "connected";
  if (!accountGrantDelivered) {
    username.wrap.hidden = active;
    accountPassword.wrap.hidden = active;
    username.input.required = !active;
    accountPassword.input.required = !active;
    if (active) {
      twoFactor.wrap.hidden = true;
      twoFactor.input.required = false;
      panelCopy.textContent = t("sessionResumed");
    } else {
      username.input.readOnly = false;
      panelCopy.textContent = t("signInCopy");
    }
  }
  updateInputDisabled(client.state === "connected");
}

function syncAccountPreferences(loaded: ClientProfile, session: BrowserSession, reloadForLanguage: boolean): void {
  if (!session.authenticated) return;
  const languageChanged = session.language !== "" && session.language !== documentLocale();
  const themeChanged = session.theme !== "" && session.theme !== activeTheme;
  if (!languageChanged && !themeChanged) return;
  if (themeChanged) {
    activeTheme = session.theme as ThemePreference;
    applyTheme(activeTheme);
    syncFavicon();
    renderThemeButton();
  }
  const preferences: { language?: string; theme?: "light" | "dark" } = {};
  if (languageChanged) preferences.language = session.language;
  if (themeChanged) preferences.theme = session.theme;
  void saveBrowserPreferences(loaded, preferences).then(() => {
    // Reload only during passive session restoration. A reload while accepting
    // a one-time admin grant or submitting a password would discard the target
    // and its short-lived authorization.
    if (languageChanged && reloadForLanguage) location.reload();
  }).catch(() => undefined);
}

function setState(state: ClientState | "loading", detail: string): void {
  status.dataset.state = state;
  statusText.textContent = detail;
  const active = state === "connected";
  empty.hidden = active;
  canvas.hidden = !active;
  disconnectButton.hidden = !active;
  connectButton.hidden = active;
  connectButton.disabled = busy || profile === undefined || requiresRelaunch;
  remotePassword.wrap.hidden = active;
  remotePassword.input.required = !active;
  chatSection.hidden = !active;
  sessionLogoutButton.disabled = busy || active;
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
	if (launchedWithAdminGrant) {
		requiresRelaunch = true;
		panelCopy.textContent = t("grantRelaunch");
	} else {
		username.wrap.hidden = false;
		accountPassword.wrap.hidden = false;
		peerId.input.readOnly = false;
		panelCopy.textContent = t("signInCopy");
	}
    updateInputDisabled(false);
  }
}

const client = new RemoteClient(canvas, {
  state(state, detail) {
    const progressKey: Partial<Record<ClientState, string>> = {
      rendezvous: "stateRendezvous",
      relay: "stateRelay",
      handshake: "stateHandshake",
      authenticating: "stateAuthenticating",
      connected: "stateConnected",
    };
    setState(state, progressKey[state] === undefined ? detail : t(progressKey[state]!));
    if (state === "connected") beginConnectionAudit();
    if (state === "error" || state === "disconnected") void closeConnectionAudit().finally(() => revokeToken());
  },
  peer(name, platform) {
    reportedRemoteName = name;
    remotePlatform = platform;
    renderRemoteIdentity();
  },
  chat(text) { appendChat(text, "remote"); },
});

const profileReady = initialProfile === undefined ? Promise.reject(initialProfileError) : Promise.resolve(initialProfile);
void profileReady.then((loaded) => {
  profile = loaded;
	if (loaded.branding.title !== "") {
		brandName.textContent = loaded.branding.title;
		document.title = loaded.branding.title;
	}
	setThemedImage(brandIcon, loaded.branding.iconLightUrl || defaultIconLight, loaded.branding.iconDarkUrl || defaultIconDark);
	faviconLight = loaded.branding.iconLightUrl || defaultIconLight;
	faviconDark = loaded.branding.iconDarkUrl || defaultIconDark;
	syncFavicon();
	setThemedImage(panelBrandLogo, loaded.branding.logoLightUrl || defaultLogoLight, loaded.branding.logoDarkUrl || defaultLogoDark);
	setThemedImage(pageBackground, loaded.branding.backgroundLightUrl, loaded.branding.backgroundDarkUrl);
	renderSafeFooter(panelFooter, loaded.branding.footerHtml);
  setState("idle", t("ready", { generation: loaded.generation }));
  stopGrantReceiver = receiveAdminGrant(loaded, (grant) => {
    if (busy || client.state !== "idle") throw new Error("Client is not ready for an admin grant");
    connectionToken = grant.token;
    connectionTokenExpiresAt = grant.expiresAt;
    accountGrantDelivered = true;
	launchedWithAdminGrant = true;
		requiresRelaunch = false;
		void establishBrowserSession(loaded, grant.token).then(() => browserSession(loaded)).then(session => {
			if (session.authenticated) {
				syncAccountPreferences(loaded, session, false);
				setAccountSession(true, session.username, session.displayName, session.avatar);
			}
		}).catch(() => undefined);
    peerId.input.value = grant.peerId;
    peerId.input.readOnly = true;
    username.input.value = "";
    accountPassword.input.value = "";
    username.wrap.hidden = true;
    accountPassword.wrap.hidden = true;
    username.input.required = false;
    accountPassword.input.required = false;
    panelCopy.textContent = t("grantReceived");
    setState("idle", t("grantReady", { generation: loaded.generation }));
    remotePassword.input.focus();
  });
  void browserSession(loaded).then(session => {
    if (session.authenticated) {
      syncAccountPreferences(loaded, session, !accountGrantDelivered);
      localDisplayName = session.displayName || session.username;
      localAvatar = session.avatar;
      if (!accountGrantDelivered) setAccountSession(true, session.username, session.displayName, session.avatar);
    }
  }).catch(() => undefined);
}, (error: unknown) => {
  setState("error", error instanceof Error ? error.message : t("profileRejected"));
  connectButton.disabled = true;
});

form.addEventListener("submit", (event) => {
  event.preventDefault();
  if (busy || profile === undefined) return;
  busy = true;
  reportedRemoteName = "";
  registeredRemoteHostname = "";
  remotePlatform = "";
  remoteDisplayName = t("remote");
  setState("loading", t("requesting"));
  const accountName = username.input.value;
  const accountSecret = accountPassword.input.value;
  const target = normalizePeerId(peerId.input.value);
  peerId.input.value = target;
  activePeerID = target;
  const remoteSecret = utf8.encode(remotePassword.input.value);
  accountPassword.input.value = "";
  remotePassword.input.value = "";
  const granted = accountGrantDelivered && connectionToken !== "";
  if (granted && connectionTokenExpiresAt <= Math.floor(Date.now() / 1000)) {
    wipe(remoteSecret);
    busy = false;
    void revokeToken();
    setState("error", t("grantExpired"));
    return;
  }
  const tokenRequest = granted ? Promise.resolve(connectionToken)
    : accountSessionActive ? browserSessionGrant(profile)
      : loginChallenge !== "" ? completeTwoFactor(profile, accountName, loginChallenge, twoFactor.input.value)
        : login(profile, accountName, accountSecret);
  void tokenRequest.then(async (result) => {
    if (typeof result !== "string") {
      loginChallenge = result.challenge;
      accountPassword.wrap.hidden = true;
      accountPassword.input.required = false;
      twoFactor.wrap.hidden = false;
      twoFactor.input.required = true;
      username.input.readOnly = true;
      panelCopy.textContent = t("twoFactorCopy");
      wipe(remoteSecret);
      setState("idle", t("twoFactor"));
      twoFactor.input.focus();
      return;
    }
    loginChallenge = "";
    twoFactor.input.value = "";
    twoFactor.wrap.hidden = true;
    twoFactor.input.required = false;
    connectionToken = result;
    if (!granted && !accountSessionActive) {
      setAccountSession(true, accountName);
      const session = await browserSession(profile!).catch(() => undefined);
      if (session?.authenticated) {
        syncAccountPreferences(profile!, session, false);
        setAccountSession(true, session.username, session.displayName, session.avatar);
      }
    }
    await client.connect(profile!, { peerId: target, token: result, remotePassword: remoteSecret });
  }).catch(async (error: unknown) => {
    wipe(remoteSecret);
    if (loginChallenge !== "") {
      setState("error", error instanceof Error ? error.message : "Authentication failed");
      twoFactor.input.value = "";
      twoFactor.input.focus();
    } else if (client.state !== "error" && client.state !== "disconnected") {
      client.disconnect(error instanceof Error ? error.message : "Connection failed");
    }
  }).finally(() => {
    busy = false;
    const locked = client.state !== "idle" && client.state !== "disconnected" && client.state !== "error";
    updateInputDisabled(locked);
    connectButton.disabled = locked || profile === undefined || requiresRelaunch;
  });
});

disconnectButton.addEventListener("click", () => client.disconnect(t("disconnectedByUser")));

sessionLogoutButton.addEventListener("click", () => {
  if (profile === undefined || busy || client.state === "connected") return;
  busy = true; sessionLogoutButton.disabled = true;
  void browserSessionLogout(profile).then(() => {
    setAccountSession(false);
    username.input.value = "";
    accountPassword.input.value = "";
    username.input.focus();
  }).catch((error: unknown) => setState("error", error instanceof Error ? error.message : t("signOutFailed"))).finally(() => { busy = false; sessionLogoutButton.disabled = false; });
});

function appendChat(text: string, direction: "local" | "remote"): void {
  chatEmpty.remove();
  const row = element("div", `chat-message chat-message--${direction}`);
  const avatar = element("span", "chat-message__avatar");
  const displayName = direction === "local" ? localDisplayName : remoteDisplayName;
  if (direction === "local" && localAvatar !== "") {
    const image = element("img");
    image.src = localAvatar;
    image.alt = "";
    avatar.append(image);
  } else {
    avatar.textContent = Array.from(displayName.trim())[0]?.toUpperCase() || (direction === "local" ? "K" : "R");
  }
  const body = element("div", "chat-message__body");
  const meta = element("div", "chat-message__meta");
  meta.append(element("span", "chat-message__author", displayName), element("time", "", new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })));
  body.append(meta, element("p", "chat-message__bubble", text));
  if (direction === "local") row.append(body, avatar);
  else row.append(avatar, body);
  chatMessages.append(row);
  while (chatMessages.children.length > 100) chatMessages.firstElementChild?.remove();
  chatMessages.scrollTop = chatMessages.scrollHeight;
}

chatForm.addEventListener("submit", event => {
  event.preventDefault();
  const text = chatInput.value.trim();
  if (text === "" || utf8.encode(text).byteLength > LIMITS.chatText || client.state !== "connected") return;
  client.sendChat(text);
  appendChat(text, "local");
  chatInput.value = "";
  chatInput.focus();
});

chatInput.addEventListener("keydown", event => {
  if (event.key === "Enter" && event.ctrlKey && !event.isComposing) {
    event.preventDefault();
    chatForm.requestSubmit();
  }
});

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
});
