type Messages = Record<string, string>;
type LocaleKey = "zh-CN" | "zh-TW" | "en" | "fr" | "es" | "ru" | "ko" | "ja";

const en: Messages = {
  loading: "Loading secure profile…", remoteAria: "Remote desktop. Focus to send keyboard input.",
  heroTitle: "A private path to your desktop", heroCopy: "Forced Relay, authenticated encryption, and VP9 WebCodecs. Secrets stay in memory.",
  connect: "Connect", signInCopy: "Sign in to mint a short-lived connection grant.", account: "Account", accountPassword: "Account password",
  remoteId: "Remote ID", remotePassword: "Remote password", disconnect: "Disconnect", safety: "No direct connection · Audio and clipboard disabled",
  ready: "Ready · profile {generation}", grantReady: "Grant ready · profile {generation}", grantReceived: "Connection grant received. Enter only the remote desktop password.",
  stateRendezvous: "Contacting the ID server", stateRelay: "Opening the approved Relay", stateHandshake: "Verifying the remote identity", stateAuthenticating: "Authenticating the remote session", stateConnected: "Connected",
  grantRelaunch: "This one-time grant is no longer valid. Return to the Kessoku device page and open Link again.", requesting: "Requesting short-lived grant",
  grantExpired: "The connection grant expired; launch the client again", connected: "Connected · {platform}", connectedTo: "Connected to {name} · {platform}",
  profileRejected: "Profile rejected", disconnectedByUser: "Disconnected by user", twoFactor: "Authenticator code",
  twoFactorCopy: "Enter the six-digit authenticator code, then enter the remote password again.", showPassword: "Show", hidePassword: "Hide",
  signOut: "Sign out", signedInAs: "Signed in as {name}", sessionResumed: "Your Kessoku sign-in is remembered. Enter the remote ID and password.", signOutFailed: "Could not sign out",
  supportChat: "Support chat", endToEndSession: "Remote session", chatEmpty: "Messages with the remote user appear here.", chatPlaceholder: "Enter inserts a new line · Ctrl+Enter sends", send: "Send", you: "You", remote: "Remote",
  language: "Language", lightMode: "Standard mode", darkMode: "Dark mode", switchToLight: "Use standard mode", switchToDark: "Use dark mode",
};
const zhCN: Messages = { ...en, loading: "正在加载安全配置…", remoteAria: "远程桌面，聚焦后可发送键盘输入。", heroTitle: "安全连接您的远程桌面", heroCopy: "强制中继、认证加密与 VP9 WebCodecs，敏感信息仅保存在内存中。", connect: "连接", signInCopy: "登录后签发短时连接授权。", account: "账户", accountPassword: "账户密码", remoteId: "远程 ID", remotePassword: "远程固定密码", disconnect: "断开连接", safety: "不允许直连 · 音频与剪贴板已禁用", ready: "已就绪 · 配置 {generation}", grantReady: "授权已就绪 · 配置 {generation}", grantReceived: "已收到一次性连接授权，只需输入远程桌面固定密码。", stateRendezvous: "正在联系 ID 服务器", stateRelay: "正在打开已批准的中继", stateHandshake: "正在验证远程设备身份", stateAuthenticating: "正在认证远程会话", stateConnected: "已连接", grantRelaunch: "本次一次性授权已失效，请返回 Kessoku 设备页面重新点击“链接”。", requesting: "正在申请短时授权", grantExpired: "连接授权已过期，请重新从后台打开 WebClient", connected: "已连接 · {platform}", connectedTo: "已连接到 {name} · {platform}", profileRejected: "安全配置被拒绝", disconnectedByUser: "已由用户断开", twoFactor: "身份验证器代码", twoFactorCopy: "输入身份验证器中的六位代码，然后重新输入远程固定密码。", showPassword: "显示", hidePassword: "隐藏", signOut: "退出登录", signedInAs: "已登录：{name}", sessionResumed: "Kessoku 登录状态已保留，请输入远程 ID 和固定密码。", signOutFailed: "退出登录失败", supportChat: "协助对话", endToEndSession: "当前远程会话", chatEmpty: "与远程用户的消息会显示在这里。", chatPlaceholder: "Enter 换行 · Ctrl+Enter 发送", send: "发送", you: "我", remote: "远程用户", language: "语言", lightMode: "标准模式", darkMode: "夜间模式", switchToLight: "切换到标准模式", switchToDark: "切换到夜间模式" };
const zhTW: Messages = { ...zhCN, loading: "正在載入安全設定…", connect: "連線", disconnect: "中斷連線", account: "帳戶", accountPassword: "帳戶密碼", remotePassword: "遠端固定密碼", grantReceived: "已收到一次性連線授權，只需輸入遠端桌面固定密碼。", chatPlaceholder: "Enter 換行 · Ctrl+Enter 傳送", language: "語言", lightMode: "標準模式", darkMode: "夜間模式" };
const fr: Messages = { ...en, loading: "Chargement du profil sécurisé…", connect: "Connexion", disconnect: "Déconnexion", account: "Compte", accountPassword: "Mot de passe du compte", remoteId: "ID distant", remotePassword: "Mot de passe distant", signInCopy: "Connectez-vous pour créer une autorisation de courte durée.", language: "Langue", lightMode: "Mode standard", darkMode: "Mode sombre" };
const es: Messages = { ...en, loading: "Cargando perfil seguro…", connect: "Conectar", disconnect: "Desconectar", account: "Cuenta", accountPassword: "Contraseña de la cuenta", remoteId: "ID remoto", remotePassword: "Contraseña remota", signInCopy: "Inicie sesión para crear una autorización de corta duración.", language: "Idioma", lightMode: "Modo estándar", darkMode: "Modo oscuro" };
const ru: Messages = { ...en, loading: "Загрузка защищённого профиля…", connect: "Подключиться", disconnect: "Отключиться", account: "Учётная запись", accountPassword: "Пароль учётной записи", remoteId: "Удалённый ID", remotePassword: "Удалённый пароль", language: "Язык", lightMode: "Светлая тема", darkMode: "Тёмная тема" };
const ko: Messages = { ...en, loading: "보안 프로필을 불러오는 중…", connect: "연결", disconnect: "연결 해제", account: "계정", accountPassword: "계정 비밀번호", remoteId: "원격 ID", remotePassword: "원격 비밀번호", language: "언어", lightMode: "표준 모드", darkMode: "다크 모드" };
const ja: Messages = { ...en, loading: "セキュアプロファイルを読み込み中…", remoteAria: "リモートデスクトップ。フォーカスするとキー入力を送信できます。", heroTitle: "安全にリモートデスクトップへ接続", heroCopy: "強制リレー、認証済み暗号化、VP9 WebCodecs を使用します。機密情報はメモリ内だけに保持されます。", connect: "接続", signInCopy: "サインインして短時間の接続許可を発行します。", account: "アカウント", accountPassword: "アカウントのパスワード", remoteId: "リモート ID", remotePassword: "リモート固定パスワード", disconnect: "切断", safety: "直接接続なし · 音声とクリップボードは無効", ready: "準備完了 · プロファイル {generation}", stateRendezvous: "ID サーバーに接続中", stateRelay: "承認済みリレーを開いています", stateHandshake: "リモート端末の身元を確認中", stateAuthenticating: "リモートセッションを認証中", stateConnected: "接続済み", requesting: "接続許可を取得中", connected: "接続済み · {platform}", connectedTo: "{name} に接続済み · {platform}", twoFactor: "認証アプリのコード", twoFactorCopy: "認証アプリの6桁コードを入力し、リモート固定パスワードをもう一度入力してください。", showPassword: "表示", hidePassword: "非表示", signOut: "サインアウト", signedInAs: "{name} としてサインイン中", sessionResumed: "Kessoku のサインイン状態を保持しています。リモート ID と固定パスワードを入力してください。", supportChat: "サポートチャット", endToEndSession: "現在のリモートセッション", chatEmpty: "リモートユーザーとのメッセージがここに表示されます。", chatPlaceholder: "Enter で改行 · Ctrl+Enter で送信", send: "送信", you: "自分", remote: "リモート", language: "言語", lightMode: "標準モード", darkMode: "ダークモード", switchToLight: "標準モードに切り替え", switchToDark: "ダークモードに切り替え" };

const dictionaries: Record<LocaleKey, Messages> = { "zh-CN": zhCN, "zh-TW": zhTW, en, fr, es, ru, ko, ja };
export const localeOptions: ReadonlyArray<{ value: LocaleKey; label: string }> = [
  { value: "zh-CN", label: "简体中文" }, { value: "zh-TW", label: "繁体中文" }, { value: "en", label: "English" },
  { value: "ja", label: "日本語" }, { value: "ko", label: "한국어" }, { value: "fr", label: "Français" },
  { value: "es", label: "Español" }, { value: "ru", label: "Русский" },
];

function normalizeLocale(input: string): LocaleKey {
  const value = input.toLowerCase();
  if (value === "zh-tw" || value.startsWith("zh-hk")) return "zh-TW";
  if (value.startsWith("zh")) return "zh-CN";
  return (["en", "fr", "es", "ru", "ko", "ja"] as const).find(item => value.startsWith(item)) ?? "en";
}

let locale: LocaleKey = normalizeLocale(navigator.language);
let active = dictionaries[locale];

export function t(key: string, values: Record<string, string | number> = {}): string {
  return (active[key] ?? en[key] ?? key).replace(/\{(\w+)\}/g, (_, name: string) => String(values[name] ?? `{${name}}`));
}

export function documentLocale(): LocaleKey { return locale; }

export function configureLocale(next: string): void {
  locale = localeOptions.some(option => option.value === next) ? next as LocaleKey : normalizeLocale(navigator.language);
  active = dictionaries[locale];
}
