import { describe, expect, it } from "vitest";

import { configureLocale, localeOptions, t } from "./i18n";

describe("WebClient connection status translations", () => {
	it("uses the same human-first language order and Chinese names as the admin console", () => {
		expect(localeOptions).toEqual([
			{ value: "zh-CN", label: "简体中文" }, { value: "zh-TW", label: "繁体中文" }, { value: "en", label: "English" },
			{ value: "ja", label: "日本語" }, { value: "ko", label: "한국어" }, { value: "fr", label: "Français" },
			{ value: "es", label: "Español" }, { value: "ru", label: "Русский" },
		]);
	});
  it("localizes every connection phase in Simplified Chinese", () => {
    configureLocale("zh-CN");
    expect([
      t("stateRendezvous"),
      t("stateRelay"),
      t("stateHandshake"),
      t("stateAuthenticating"),
      t("stateConnected"),
    ]).toEqual([
      "正在联系 ID 服务器",
      "正在打开已批准的中继",
      "正在验证远程设备身份",
      "正在认证远程会话",
      "已连接",
    ]);
  });

  it("provides Japanese connection-state copy", () => {
    configureLocale("ja");
    expect(t("stateConnected")).toBe("接続済み");
    expect(t("stateAuthenticating")).toBe("リモートセッションを認証中");
  });
});
