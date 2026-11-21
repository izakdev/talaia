import { useEffect, useState, type ReactNode } from "react";
import type { BrandIconData, ConnectionState } from "./api";
import type { Language } from "./i18n";
import type { Tone } from "./widgets/types";

/** Un servicio vigilado: su conexión, la página donde está su bloque y su estado. */
export interface Watched {
  connection: ConnectionState;
  pageId: string;
  tone: Tone;
  label: string;
}

/** Resumen del panel: el peor estado, la última actualización y lo que necesita atención. */
export interface WatchState {
  updated: string | null;
  tone: Tone;
  total: number;
  attention: Watched[];
}

/** Un acceso de la columna: un servicio del panel con enlace. */
export interface AccessItem {
  id: string;
  name: string;
  link: string;
  icon: BrandIconData | null;
  tone: Tone;
}

export function useNow(intervalMs: number) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs]);
  return now;
}

export function relativeTime(iso: string, now: number, lang: Language) {
  const seconds = Math.round((new Date(iso).getTime() - now) / 1000);
  const format = new Intl.RelativeTimeFormat(lang, { numeric: "auto" });
  if (Math.abs(seconds) < 60) return format.format(Math.min(seconds, 0), "second");
  const minutes = Math.round(seconds / 60);
  if (Math.abs(minutes) < 60) return format.format(minutes, "minute");
  const hours = Math.round(minutes / 60);
  if (Math.abs(hours) < 24) return format.format(hours, "hour");
  return format.format(Math.round(hours / 24), "day");
}

/** La forma de cada tono, para que el estado no dependa solo del color. */
export function ToneMark({ tone }: { tone: Tone }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true">
      {tone === "good" && <circle cx="8" cy="8" r="6" fill="currentColor" />}
      {tone === "warn" && <path d="M8 1.5 15 14H1Z" fill="currentColor" />}
      {tone === "bad" && <rect x="2.5" y="2.5" width="11" height="11" fill="currentColor" />}
      {tone === "muted" && <circle cx="8" cy="8" r="5" fill="none" stroke="currentColor" strokeWidth="2" />}
    </svg>
  );
}

export function StateLabel({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <span className={`service-state ${tone}`}>
      <ToneMark tone={tone} />
      {children}
    </span>
  );
}
