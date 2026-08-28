export type ThemePreference = "light" | "dark";

export function resolveTheme(preference = ""): ThemePreference {
  if (preference === "light" || preference === "dark") return preference;
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches === true ? "dark" : "light";
}

export function applyTheme(theme: ThemePreference): void {
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
}
