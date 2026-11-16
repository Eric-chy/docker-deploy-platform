import { createI18n } from "vue-i18n";
import enUS from "../locales/en-US";
import zhCN from "../locales/zh-CN";

export type AppLocale = "zh-CN" | "en-US";
const localeStorageKey = "deploy-platform-locale";

const getInitialLocale = (): AppLocale =>
  localStorage.getItem(localeStorageKey) === "en-US" ? "en-US" : "zh-CN";

export const i18n = createI18n({
  legacy: false,
  locale: getInitialLocale(),
  fallbackLocale: "zh-CN",
  messages: {
    "zh-CN": zhCN,
    "en-US": enUS
  }
});

export function translate(key: string, named?: Record<string, unknown>) {
  return i18n.global.t(key, named ?? {});
}

export function translateTitle(title?: string | false) {
  if (typeof title !== "string" || title.length === 0) return "";
  if (i18n.global.locale.value === "zh-CN") return title;
  const { t, te } = i18n.global;
  return te(title) ? t(title) : title;
}

export function setLocale(locale: AppLocale) {
  i18n.global.locale.value = locale;
  localStorage.setItem(localeStorageKey, locale);
  document.documentElement.lang = locale;
}
