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
    "nav.invocations": "Invocations",
    "nav.compare": "Compare",
    "nav.watchlist": "Watchlist",
    "nav.watchdog": "Watchdog",
    "nav.groups": "Groups",
    "nav.labels": "Labels",
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
    "empty.trackContract": "Track a contract",
    "empty.clearFilters": "Clear filters",
    "empty.browseContracts": "Browse contracts",
    "contracts.error.title": "Couldn't load contracts.",
    "contracts.empty.firstRun.title": "No contracts tracked yet",
    "contracts.empty.firstRun.body":
      "Use the CLI or API to start tracking a Soroban contract, or click",
    "contracts.empty.firstRun.action": "run the tracking wizard",
    "contracts.empty.firstRun.bodySuffix": "to add one from here.",
    "contracts.empty.noResults.title": "No contracts match your search",
    "contracts.empty.noResults.body": "No contracts match your search.",
    "events.error.title": "Couldn't load events.",
    "events.empty.firstRun.title": "No events indexed yet.",
    "events.empty.firstRun.suffix": "to start seeing events here.",
    "events.empty.noResults.title": "No events match these filters.",
    "invocations.error.title": "Couldn't load invocations.",
    "invocations.empty.firstRun.title": "No invocations indexed yet",
    "invocations.empty.firstRun.body":
      "Invocations appear here once the indexer has traced activity for tracked contracts.",
    "invocations.empty.noResults.title": "No invocations match your filters",
    "invocations.empty.noResults.body":
      "Try widening the date range or clearing the contract and function filters.",
    "watchlist.error.title": "Couldn't load your watchlist.",
    "watchlist.empty.title": "No bookmarks yet",
    "watchlist.empty.body":
      "Star a contract on the Contracts page to add it to your watchlist.",
    "groups.error.title": "Couldn't load your groups.",
    "groups.empty.title": "No groups yet",
    "groups.empty.bodyPrefix":
      "Create a group above, then add contracts from the",
    "groups.empty.bodyLink": "contracts page",
    "groups.empty.bodySuffix": ".",
    "rules.error.title": "Couldn't load your rules.",
    "rules.empty.title": "No rules yet",
    "rules.empty.body":
      "Name a rule in the editor above, or start from a library example, then save it to see it here.",
    "rules.empty.action": "Write a rule",
    "watchdog.contracts.error.title": "Couldn't load monitored contracts.",
    "watchdog.contracts.empty.title": "No monitored contracts yet",
    "watchdog.contracts.empty.body":
      "Register a contract with the on-chain watchdog to get started. It will appear here on the indexer's next tick.",
    "watchdog.alerts.error.title": "Couldn't load alerts.",
    "watchdog.alerts.empty.title": "No alerts yet",
    "watchdog.alerts.empty.groupedBody":
      "Alerts pushed via report_alert on the watchdog contract are deduplicated and appear here as groups.",
    "watchdog.alerts.empty.body":
      "Alerts pushed via report_alert on the watchdog contract will appear here.",
    "watchdog.alerts.noResults.title": "No alerts match these filters",
    "watchdog.alerts.noResults.body":
      "Try a different severity or contract, or clear the filters.",
  },
  es: {
    "nav.live": "En vivo",
    "nav.contracts": "Contratos",
    "nav.events": "Eventos",
    "nav.invocations": "Invocaciones",
    "nav.compare": "Comparar",
    "nav.watchlist": "Lista de seguimiento",
    "nav.watchdog": "Vigilancia",
    "nav.groups": "Grupos",
    "nav.labels": "Etiquetas",
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
    "empty.trackContract": "Seguir un contrato",
    "empty.clearFilters": "Borrar filtros",
    "empty.browseContracts": "Ver contratos",
    "contracts.error.title": "No se pudieron cargar los contratos.",
    "contracts.empty.firstRun.title": "Aún no sigues ningún contrato",
    "contracts.empty.firstRun.body":
      "Usa la CLI o la API para empezar a seguir un contrato Soroban, o haz clic en",
    "contracts.empty.firstRun.action": "el asistente de seguimiento",
    "contracts.empty.firstRun.bodySuffix": "para añadir uno desde aquí.",
    "contracts.empty.noResults.title":
      "Ningún contrato coincide con tu búsqueda",
    "contracts.empty.noResults.body":
      "Ningún contrato coincide con tu búsqueda.",
    "events.error.title": "No se pudieron cargar los eventos.",
    "events.empty.firstRun.title": "Aún no hay eventos indexados.",
    "events.empty.firstRun.suffix": "para empezar a ver eventos aquí.",
    "events.empty.noResults.title": "Ningún evento coincide con estos filtros.",
    "invocations.error.title": "No se pudieron cargar las invocaciones.",
    "invocations.empty.firstRun.title": "Aún no hay invocaciones indexadas",
    "invocations.empty.firstRun.body":
      "Las invocaciones aparecen aquí cuando el indexador ha rastreado actividad de contratos seguidos.",
    "invocations.empty.noResults.title":
      "Ninguna invocación coincide con tus filtros",
    "invocations.empty.noResults.body":
      "Prueba a ampliar el rango de fechas o a borrar los filtros de contrato y función.",
    "watchlist.error.title": "No se pudo cargar tu lista de seguimiento.",
    "watchlist.empty.title": "Aún no hay marcadores",
    "watchlist.empty.body":
      "Marca con estrella un contrato en la página de Contratos para añadirlo a tu lista de seguimiento.",
    "groups.error.title": "No se pudieron cargar tus grupos.",
    "groups.empty.title": "Aún no hay grupos",
    "groups.empty.bodyPrefix":
      "Crea un grupo arriba y luego añade contratos desde la",
    "groups.empty.bodyLink": "página de contratos",
    "groups.empty.bodySuffix": ".",
    "rules.error.title": "No se pudieron cargar tus reglas.",
    "rules.empty.title": "Aún no hay reglas",
    "rules.empty.body":
      "Nombra una regla en el editor de arriba, o empieza con un ejemplo de la biblioteca, y guárdala para verla aquí.",
    "rules.empty.action": "Escribir una regla",
    "watchdog.contracts.error.title":
      "No se pudieron cargar los contratos vigilados.",
    "watchdog.contracts.empty.title": "Aún no hay contratos vigilados",
    "watchdog.contracts.empty.body":
      "Registra un contrato en el watchdog on-chain para empezar. Aparecerá aquí en el siguiente ciclo del indexador.",
    "watchdog.alerts.error.title": "No se pudieron cargar las alertas.",
    "watchdog.alerts.empty.title": "Aún no hay alertas",
    "watchdog.alerts.empty.groupedBody":
      "Las alertas enviadas con report_alert en el contrato watchdog se deduplican y aparecen aquí agrupadas.",
    "watchdog.alerts.empty.body":
      "Las alertas enviadas con report_alert en el contrato watchdog aparecerán aquí.",
    "watchdog.alerts.noResults.title":
      "Ninguna alerta coincide con estos filtros",
    "watchdog.alerts.noResults.body":
      "Prueba otra severidad u otro contrato, o borra los filtros.",
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

// Rendering outside a LocaleProvider (a unit test of a single page, for
// example) still gets the default English catalog instead of throwing, so
// shared components can rely on useLocale().
const defaultLocaleContext: LocaleContextValue = {
  locale: "en",
  timeZone: "UTC",
  setLocale: () => {},
  setTimeZone: () => {},
  t: (key) => messages.en[key],
};

export function useLocale() {
  const context = useContext(LocaleContext) ?? defaultLocaleContext;
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
