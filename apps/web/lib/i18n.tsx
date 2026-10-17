"use client";

import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  formatDateTime as formatDateTimeValue,
  formatNumber as formatNumberValue,
} from "@/lib/format";
import type { TimeZonePreference } from "@/lib/format";

const LOCALE_KEY = "sorolens_locale";
const TIMEZONE_KEY = "sorolens_timezone";

const messages = {
  en: {
    "nav.live": "Live",
    "nav.contracts": "Contracts",
    "nav.events": "Events",
    "nav.watchdog": "Watchdog",
    "nav.groups": "Groups",
    "nav.rules": "Rules",
    "nav.playground": "Playground",
    "nav.settings": "Settings",
    "shell.community": "Built for the Stellar developer community.",
    "preferences.language": "Language",
    "preferences.timezone": "Time zone",
    "preferences.local": "Local time",
    "preferences.utc": "UTC",
    "playground.title": "API Playground",
    "playground.description":
      "Pick an endpoint, fill in the parameters, and send a request. A saved API key from Settings is pre-filled.",
    "playground.request": "Request",
    "playground.noParameters": "This endpoint has no parameters.",
    "playground.body": "Request body (JSON)",
    "playground.apiKey": "API key (optional)",
    "playground.pasteKey": "Paste your key…",
    "playground.sending": "Sending…",
    "playground.send": "Send",
    "playground.copyCurl": "Copy as curl",
    "playground.curl": "curl",
    "playground.response": "Response",
    "playground.responseHint":
      "Send a request to see the status, headers, and body.",
    "playground.headers": "Headers",
    "playground.responseBody": "Body",
  },
  es: {
    "nav.live": "En vivo",
    "nav.contracts": "Contratos",
    "nav.events": "Eventos",
    "nav.watchdog": "Vigilancia",
    "nav.groups": "Grupos",
    "nav.rules": "Reglas",
    "nav.playground": "Explorador API",
    "nav.settings": "Configuración",
    "shell.community":
      "Creado para la comunidad de desarrolladores de Stellar.",
    "preferences.language": "Idioma",
    "preferences.timezone": "Zona horaria",
    "preferences.local": "Hora local",
    "preferences.utc": "UTC",
    "playground.title": "Explorador de API",
    "playground.description":
      "Elige un endpoint, completa los parámetros y envía una solicitud. La clave guardada en Configuración se completa automáticamente.",
    "playground.request": "Solicitud",
    "playground.noParameters": "Este endpoint no tiene parámetros.",
    "playground.body": "Cuerpo de la solicitud (JSON)",
    "playground.apiKey": "Clave API (opcional)",
    "playground.pasteKey": "Pega tu clave…",
    "playground.sending": "Enviando…",
    "playground.send": "Enviar",
    "playground.copyCurl": "Copiar como curl",
    "playground.curl": "curl",
    "playground.response": "Respuesta",
    "playground.responseHint":
      "Envía una solicitud para ver el estado, los encabezados y el cuerpo.",
    "playground.headers": "Encabezados",
    "playground.responseBody": "Cuerpo",
  },
} as const;

export type Locale = keyof typeof messages;
export type MessageKey = keyof typeof messages.en;

interface LocaleContextValue {
  locale: Locale;
  timeZone: TimeZonePreference;
  setLocale: (locale: Locale) => void;
  setTimeZone: (timeZone: TimeZonePreference) => void;
  t: (key: MessageKey) => string;
}

const LocaleContext = createContext<LocaleContextValue | null>(null);

export function LocaleProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>("en");
  const [timeZone, setTimeZoneState] = useState<TimeZonePreference>("UTC");

  useEffect(() => {
    const savedLocale = window.localStorage.getItem(LOCALE_KEY);
    const savedTimeZone = window.localStorage.getItem(TIMEZONE_KEY);
    if (savedLocale === "en" || savedLocale === "es") {
      setLocaleState(savedLocale);
    }
    if (savedTimeZone === "UTC" || savedTimeZone === "local") {
      setTimeZoneState(savedTimeZone);
    }
  }, []);

  const setLocale = (nextLocale: Locale) => {
    setLocaleState(nextLocale);
    window.localStorage.setItem(LOCALE_KEY, nextLocale);
  };

  const setTimeZone = (nextTimeZone: TimeZonePreference) => {
    setTimeZoneState(nextTimeZone);
    window.localStorage.setItem(TIMEZONE_KEY, nextTimeZone);
  };

  const value: LocaleContextValue = {
    locale,
    timeZone,
    setLocale,
    setTimeZone,
    t: (key) => messages[locale][key],
  };

  return (
    <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>
  );
}

export function useLocale() {
  const context = useContext(LocaleContext);
  if (!context) throw new Error("useLocale must be used within LocaleProvider");
  return {
    ...context,
    formatNumber: (value: number, options?: Intl.NumberFormatOptions) =>
      formatNumberValue(value, context.locale, options),
    formatDateTime: (
      value: string | number | Date,
      options?: Intl.DateTimeFormatOptions
    ) => formatDateTimeValue(value, context.locale, context.timeZone, options),
  };
}
