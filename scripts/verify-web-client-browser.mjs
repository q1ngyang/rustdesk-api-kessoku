const [devtoolsPort, clientURL, apiURL, accountUsername, accountPassword, targetID, targetPassword] = process.argv.slice(2);

if ([devtoolsPort, clientURL, apiURL, accountUsername, accountPassword, targetID, targetPassword].some((value) => !value)) {
  throw new Error("usage: node verify-web-client-browser.mjs DEVTOOLS_PORT CLIENT_URL API_URL USERNAME ACCOUNT_PASSWORD TARGET_ID TARGET_PASSWORD");
}

const devtoolsOrigin = `http://127.0.0.1:${devtoolsPort}`;

function delay(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

class CDP {
  constructor(url) {
    this.url = url;
    this.nextID = 1;
    this.pending = new Map();
    this.listeners = new Map();
    this.socket = undefined;
  }

  async connect() {
    this.socket = new WebSocket(this.url);
    await new Promise((resolve, reject) => {
      this.socket.addEventListener("open", resolve, { once: true });
      this.socket.addEventListener("error", () => reject(new Error("DevTools WebSocket connection failed")), { once: true });
    });
    this.socket.addEventListener("message", (event) => {
      const message = JSON.parse(String(event.data));
      if (message.id !== undefined) {
        const pending = this.pending.get(message.id);
        if (pending !== undefined) {
          this.pending.delete(message.id);
          if (message.error !== undefined) pending.reject(new Error(message.error.message));
          else pending.resolve(message.result);
        }
        return;
      }
      for (const listener of this.listeners.get(message.method) ?? []) listener(message.params);
    });
  }

  on(method, listener) {
    const listeners = this.listeners.get(method) ?? [];
    listeners.push(listener);
    this.listeners.set(method, listeners);
  }

  call(method, params = {}) {
    if (this.socket === undefined) throw new Error("DevTools client is not connected");
    const id = this.nextID++;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`DevTools command timed out: ${method}`));
      }, 15000);
      this.pending.set(id, {
        resolve: (value) => { clearTimeout(timer); resolve(value); },
        reject: (error) => { clearTimeout(timer); reject(error); },
      });
      this.socket.send(JSON.stringify({ id, method, params }));
    });
  }

  async evaluate(expression) {
    const response = await this.call("Runtime.evaluate", {
      expression,
      awaitPromise: true,
      returnByValue: true,
    });
    if (response.exceptionDetails !== undefined) {
      throw new Error(response.exceptionDetails.exception?.description ?? response.exceptionDetails.text ?? "Browser evaluation failed");
    }
    return response.result.value;
  }

  close() {
    this.socket?.close();
    this.socket = undefined;
  }
}

async function pages() {
  const response = await fetch(`${devtoolsOrigin}/json/list`, { cache: "no-store" });
  if (!response.ok) throw new Error("Unable to list Chrome targets");
  return (await response.json()).filter((target) => target.type === "page");
}

async function waitForPage(predicate, timeoutMilliseconds = 20000) {
  const deadline = Date.now() + timeoutMilliseconds;
  while (Date.now() < deadline) {
    const match = (await pages()).find(predicate);
    if (match !== undefined) return match;
    await delay(100);
  }
  throw new Error("Expected Chrome target did not appear");
}

async function attach(target) {
  const client = new CDP(target.webSocketDebuggerUrl);
  await client.connect();
  const responses = [];
  const failures = [];
  const windowOpens = [];
  client.on("Network.responseReceived", ({ response }) => responses.push({ url: response.url, status: response.status }));
  client.on("Network.loadingFailed", ({ errorText, type }) => failures.push({ errorText, type }));
  client.on("Page.windowOpen", (event) => windowOpens.push({ url: event.url, userGesture: event.userGesture, windowName: event.windowName }));
  await client.call("Runtime.enable");
  await client.call("Page.enable");
  await client.call("Network.enable");
  return { client, responses, failures, windowOpens };
}

async function waitFor(client, expression, description, timeoutMilliseconds = 60000) {
  const deadline = Date.now() + timeoutMilliseconds;
  let last;
  while (Date.now() < deadline) {
    last = await client.evaluate(expression);
    if (last) return last;
    await delay(200);
  }
  const status = await client.evaluate(`document.querySelector('.status-text')?.textContent ?? document.body.innerText.slice(0, 500)`);
  throw new Error(`${description} timed out; browser state: ${String(status ?? last)}`);
}

async function navigate(client, url) {
  const navigation = await client.call("Page.navigate", { url });
  if (navigation.errorText !== undefined) throw new Error(`Navigation failed: ${navigation.errorText}`);
  await waitFor(client, `document.readyState === "complete"`, `navigation to ${new URL(url).origin}`, 30000);
}

async function trustedClick(client, expression) {
  const point = await client.evaluate(`(() => {
    const element = (${expression});
    if (!element) throw new Error('Trusted click target is missing');
    element.scrollIntoView({block:'center', inline:'center'});
    const rect = element.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) throw new Error('Trusted click target is not visible');
    const x = rect.left + rect.width / 2;
    const y = rect.top + rect.height / 2;
    const hit = document.elementFromPoint(x, y);
    return {
      x, y,
      text: element.textContent?.trim() ?? '',
      disabled: element.matches(':disabled') || element.getAttribute('aria-disabled') === 'true',
      hit: hit === element || element.contains(hit),
      elementHTML: element.outerHTML.slice(0, 500),
      hitHTML: hit?.outerHTML?.slice(0, 500) ?? '',
      viewport: [innerWidth, innerHeight],
    };
  })()`);
  if (point.disabled || !point.hit) throw new Error(`Trusted click target is unavailable: ${JSON.stringify(point)}`);
  await client.call("Input.dispatchMouseEvent", { type: "mouseMoved", x: point.x, y: point.y });
  await client.call("Input.dispatchMouseEvent", { type: "mousePressed", x: point.x, y: point.y, button: "left", buttons: 1, clickCount: 1 });
  await client.call("Input.dispatchMouseEvent", { type: "mouseReleased", x: point.x, y: point.y, button: "left", buttons: 0, clickCount: 1 });
  return point;
}

const initialPages = await pages();
const directTarget = initialPages.find((target) => target.url.startsWith(clientURL) || target.url === "about:blank") ?? initialPages[0];
if (directTarget === undefined) throw new Error("Chrome has no page target for the browser verification");
process.stderr.write("browser_phase=attach_direct\n");
const { client: direct, responses: directResponses, failures: directFailures, windowOpens: directWindowOpens } = await attach(directTarget);
process.stderr.write("browser_phase=navigate_direct\n");
await navigate(direct, clientURL);
process.stderr.write("browser_phase=profile_direct\n");
await waitFor(direct, `document.querySelector('.status')?.dataset.state === "idle"`, "Web Client profile load");

const browserPolicy = await direct.evaluate(`(async () => ({
  localStorageEntries: localStorage.length,
  sessionStorageEntries: sessionStorage.length,
  serviceWorkers: 'serviceWorker' in navigator ? (await navigator.serviceWorker.getRegistrations()).length : 0,
  indexedDatabases: 'databases' in indexedDB ? (await indexedDB.databases()).length : 0,
  vp9: 'VideoDecoder' in globalThis && (await VideoDecoder.isConfigSupported({codec:'vp09.00.10.08'})).supported === true,
  opener: window.opener !== null,
}))()`);
if (browserPolicy.localStorageEntries !== 0 || browserPolicy.sessionStorageEntries !== 0 || browserPolicy.serviceWorkers !== 0 || browserPolicy.indexedDatabases !== 0 || !browserPolicy.vp9 || browserPolicy.opener) {
  throw new Error(`Direct client browser policy failed: ${JSON.stringify(browserPolicy)}`);
}

await direct.evaluate(`(() => {
  const values = ${JSON.stringify([accountUsername, accountPassword, targetID, targetPassword])};
  const inputs = [...document.querySelectorAll('.field-input')];
  if (inputs.length !== values.length) throw new Error('Unexpected Web Client form');
  inputs.forEach((input, index) => {
    input.value = values[index];
    input.dispatchEvent(new Event('input', {bubbles:true}));
  });
  document.querySelector('form').requestSubmit();
  return true;
})()`);
await waitFor(direct, `document.querySelector('.status')?.dataset.state === "connected"`, "direct forced-Relay connection");

const directCanvas = await waitFor(direct, `(() => {
  const canvas = document.querySelector('canvas');
  if (!canvas || canvas.hidden || canvas.width < 1 || canvas.height < 1) return false;
  const context = canvas.getContext('2d');
  const pixels = context.getImageData(0, 0, Math.min(canvas.width, 64), Math.min(canvas.height, 64)).data;
  let nonBlack = 0;
  for (let index = 0; index < pixels.length; index += 4) if (pixels[index] || pixels[index + 1] || pixels[index + 2]) nonBlack += 1;
  return nonBlack > 0 ? {width:canvas.width,height:canvas.height,nonBlack} : false;
})()`, "decoded VP9 canvas");

await direct.evaluate(`(() => {
  const canvas = document.querySelector('canvas');
  const rect = canvas.getBoundingClientRect();
  canvas.dispatchEvent(new PointerEvent('pointermove', {
    bubbles:true, pointerId:1, pointerType:'mouse', buttons:0,
    clientX:rect.left + rect.width * 0.25, clientY:rect.top + rect.height * 0.30,
  }));
  canvas.focus();
  canvas.dispatchEvent(new KeyboardEvent('keydown', {bubbles:true,key:'K',code:'KeyK'}));
  canvas.dispatchEvent(new KeyboardEvent('keyup', {bubbles:true,key:'K',code:'KeyK'}));
  canvas.dispatchEvent(new KeyboardEvent('keydown', {bubbles:true,key:'s',code:'KeyS',ctrlKey:true}));
  canvas.dispatchEvent(new KeyboardEvent('keyup', {bubbles:true,key:'s',code:'KeyS',ctrlKey:true}));
  return true;
})()`);
await delay(1500);

await direct.evaluate(`document.querySelector('button.secondary').click()`);
await waitFor(direct, `document.querySelector('.status')?.dataset.state === "disconnected"`, "direct disconnect");
await waitFor(direct, `localStorage.length === 0 && sessionStorage.length === 0`, "client secret cleanup", 10000);
await delay(1000);
if (!directResponses.some(({ url, status }) => url.endsWith("/api/web-client/v1/logout") && status === 204)) {
  throw new Error("Direct Web Client logout was not acknowledged");
}

await navigate(direct, `${apiURL}_admin/#/login`);
await waitFor(direct, `document.querySelectorAll('.login-input input').length === 2`, "admin login form", 30000);
await direct.evaluate(`(() => {
  const values = ${JSON.stringify([accountUsername, accountPassword])};
  const inputs = [...document.querySelectorAll('.login-input input')];
  inputs.forEach((input, index) => {
    input.value = values[index];
    input.dispatchEvent(new Event('input', {bubbles:true}));
  });
  document.querySelector('.login-button').click();
  return true;
})()`);
await waitFor(direct, `typeof localStorage.getItem('access_token') === 'string' && localStorage.getItem('access_token').length > 0`, "admin login", 30000);
await waitFor(direct, `location.hash === '#/' && document.querySelector('.login-button') === null`, "admin post-login navigation", 30000);

const createPeer = await direct.evaluate(`(async () => {
  const token = localStorage.getItem('access_token');
  const headers = {'Content-Type':'application/json','api-token':token};
  const listed = await fetch('/api/admin/peer/list?page=1&page_size=100&id=${encodeURIComponent(targetID)}', {headers});
  const listedBody = await listed.json();
  if (!listed.ok || listedBody.code !== 0) return {status:listed.status, body:listedBody};
  if (listedBody.data?.list?.some((peer) => peer.id === ${JSON.stringify(targetID)})) {
    return {status:200, body:{code:0, data:{existing:true}}};
  }
  const response = await fetch('/api/admin/peer/create', {
    method:'POST',
    headers,
    body:JSON.stringify({id:${JSON.stringify(targetID)},hostname:'browser-matrix',os:'Linux',uuid:'browser-matrix-target',version:'1.4.9'}),
  });
  return {status:response.status, body:await response.json()};
})()`);
if (createPeer.status !== 200 || createPeer.body.code !== 0) throw new Error(`Unable to create admin peer fixture: ${JSON.stringify(createPeer)}`);

await navigate(direct, `${apiURL}_admin/#/user/peer`);
await waitFor(direct, `location.hash === '#/user/peer'`, "admin peer route", 30000);
await waitFor(direct, `document.body.innerText.includes(${JSON.stringify(targetID)})`, "admin peer row", 30000);
await waitFor(direct, `document.querySelector('.el-loading-mask') === null`, "admin peer loading overlay", 10000);
await waitFor(direct, `(() => {
  const row = [...document.querySelectorAll('.el-table__row')].find((candidate) => candidate.innerText.includes(${JSON.stringify(targetID)}));
  return Boolean(row?.querySelector('button.el-button--success'));
})()`, "admin Web Client launch button", 10000);
const existingPageIDs = new Set((await pages()).map(({ id }) => id));
const launchClick = await trustedClick(direct, `(() => {
  const row = [...document.querySelectorAll('.el-table__row')].find((candidate) => candidate.innerText.includes(${JSON.stringify(targetID)}));
  return row?.querySelector('button.el-button--success');
})()`);

let openedTarget;
try {
  openedTarget = await waitForPage((target) => !existingPageIDs.has(target.id), 12000);
} catch (error) {
  const pageMessage = await direct.evaluate(`document.querySelector('.el-message__content')?.textContent ?? ''`);
  throw new Error(`Web Client popup did not open: target=${JSON.stringify(launchClick)} message=${JSON.stringify(pageMessage)} events=${JSON.stringify(directWindowOpens)} responses=${JSON.stringify(directResponses.slice(-12))} failures=${JSON.stringify(directFailures.slice(-12))}`, { cause: error });
}
let popupTarget;
try {
  popupTarget = await waitForPage((target) => target.id === openedTarget.id && target.url.startsWith(clientURL), 30000);
} catch (error) {
  const pageMessage = await direct.evaluate(`document.querySelector('.el-message__content')?.textContent ?? ''`);
  throw new Error(`Web Client popup did not navigate: opened=${JSON.stringify({id:openedTarget.id,url:openedTarget.url})} current=${JSON.stringify(await pages())} message=${JSON.stringify(pageMessage)} events=${JSON.stringify(directWindowOpens)} responses=${JSON.stringify(directResponses.slice(-20))} failures=${JSON.stringify(directFailures.slice(-20))}`, { cause: error });
}
const { client: popup, responses: popupResponses, failures: popupFailures } = await attach(popupTarget);
await waitFor(popup, `document.querySelector('.status-text')?.textContent?.startsWith('Grant ready') === true`, "exact-origin admin grant handoff", 30000);
const popupGrant = await popup.evaluate(`(() => {
  const inputs = [...document.querySelectorAll('.field-input')];
  return {
    peerID: inputs[2]?.value,
    peerReadOnly: inputs[2]?.readOnly,
    accountHidden: inputs[0]?.closest('label')?.hidden && inputs[1]?.closest('label')?.hidden,
    localStorageEntries: localStorage.length,
    sessionStorageEntries: sessionStorage.length,
    openerCleared: window.opener === null,
  };
})()`);
if (popupGrant.peerID !== targetID || !popupGrant.peerReadOnly || !popupGrant.accountHidden || popupGrant.localStorageEntries !== 0 || popupGrant.sessionStorageEntries !== 0 || !popupGrant.openerCleared) {
  throw new Error(`Admin grant boundary failed: ${JSON.stringify(popupGrant)}`);
}

await popup.evaluate(`(() => {
  const input = [...document.querySelectorAll('.field-input')][3];
  input.value = ${JSON.stringify(targetPassword)};
  input.dispatchEvent(new Event('input', {bubbles:true}));
  document.querySelector('form').requestSubmit();
  return true;
})()`);
await waitFor(popup, `document.querySelector('.status')?.dataset.state === "connected"`, "admin-granted forced-Relay connection");
const popupCanvas = await waitFor(popup, `(() => {
  const canvas = document.querySelector('canvas');
  if (!canvas || canvas.hidden || canvas.width < 1 || canvas.height < 1) return false;
  const pixels = canvas.getContext('2d').getImageData(0, 0, Math.min(canvas.width, 64), Math.min(canvas.height, 64)).data;
  let nonBlack = 0;
  for (let index = 0; index < pixels.length; index += 4) if (pixels[index] || pixels[index + 1] || pixels[index + 2]) nonBlack += 1;
  return nonBlack > 0 ? {width:canvas.width,height:canvas.height,nonBlack} : false;
})()`, "admin-granted VP9 canvas");
await popup.evaluate(`document.querySelector('button.secondary').click()`);
await waitFor(popup, `document.querySelector('.status')?.dataset.state === "disconnected"`, "admin-granted disconnect");
await delay(1000);
if (!popupResponses.some(({ url, status }) => url.endsWith("/api/web-client/v1/logout") && status === 204)) {
  throw new Error("Admin-granted Web Client logout was not acknowledged");
}

const result = {
  direct: { state: "connected_then_logged_out", canvas: directCanvas },
  adminPopup: { state: "grant_accepted_connected_then_logged_out", canvas: popupCanvas },
  browserPolicy,
  popupGrant,
  networkFailures: { direct: directFailures, popup: popupFailures },
};
process.stdout.write(`${JSON.stringify(result)}\n`);
popup.close();
direct.close();
