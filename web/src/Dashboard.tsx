import { useEffect, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type ConnectionState, type ConnectionType, type DashboardState } from "./api";
import { Horizon, LanguageSwitch, Tower } from "./chrome";
import { useI18n, type Language } from "./i18n";
import { adguardWidget } from "./widgets/adguard";
import { homeAssistantWidget } from "./widgets/homeassistant";
import { proxmoxWidget } from "./widgets/proxmox";
import type { Tone, Widget } from "./widgets/types";

const WIDGETS: Partial<Record<ConnectionType, unknown>> = {
  adguard: adguardWidget,
  homeassistant: homeAssistantWidget,
  proxmox: proxmoxWidget,
};

function useLiveDashboard() {
  const client = useQueryClient();
  const [live, setLive] = useState(true);
  useEffect(() => {
    const events = new EventSource("/api/dashboard/events");
    events.addEventListener("snapshot", (event) => {
      setLive(true);
      client.setQueryData<DashboardState>(["dashboard"], JSON.parse((event as MessageEvent<string>).data));
    });
    // EventSource reintenta por sí solo mientras no se cierre.
    events.onerror = () => setLive(false);
    return () => events.close();
  }, [client]);
  return live;
}

function useNow(intervalMs: number) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs]);
  return now;
}

function relativeTime(iso: string, now: number, lang: Language) {
  const seconds = Math.round((new Date(iso).getTime() - now) / 1000);
  const format = new Intl.RelativeTimeFormat(lang, { numeric: "auto" });
  if (Math.abs(seconds) < 60) return format.format(Math.min(seconds, 0), "second");
  const minutes = Math.round(seconds / 60);
  if (Math.abs(minutes) < 60) return format.format(minutes, "minute");
  const hours = Math.round(minutes / 60);
  if (Math.abs(hours) < 24) return format.format(hours, "hour");
  return format.format(Math.round(hours / 24), "day");
}

export default function Dashboard({ navigate }: { navigate: (path: string) => void }) {
  const { t } = useI18n();
  const live = useLiveDashboard();
  const dashboard = useQuery({ queryKey: ["dashboard"], queryFn: api.dashboard });
  const adminLink = (label: string, className: string) => (
    <a
      className={className}
      href="/admin"
      onClick={(event) => {
        event.preventDefault();
        navigate("/admin");
      }}
    >
      {label}
    </a>
  );

  if (dashboard.data?.connections.length === 0) {
    return (
      <Horizon>
        <h1>{t("dashboard.title")}</h1>
        <p>{t("dashboard.lead")}</p>
        {adminLink(t("dashboard.admin"), "button primary")}
      </Horizon>
    );
  }
  return (
    <div className="lookout">
      <header className="lookout-sky">
        <Tower />
        <span className="wordmark">Talaia</span>
        {adminLink(t("dashboard.admin.short"), "quiet")}
        <LanguageSwitch />
      </header>
      <main className="lookout-main">
        <h1 className="visually-hidden">Talaia</h1>
        {!live && <p role="status" className="live-lost">{t("dashboard.live.lost")}</p>}
        {dashboard.isPending && <p>{t("app.loading")}</p>}
        {dashboard.isError && <p role="alert" className="error">{t("dashboard.unavailable")}</p>}
        <div className="services">
          {dashboard.data?.connections.map((connection) => <Service key={connection.id} connection={connection} />)}
        </div>
      </main>
    </div>
  );
}

function StateLabel({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <span className={`service-state ${tone}`}>
      <svg viewBox="0 0 16 16" aria-hidden="true">
        {tone === "good" && <circle cx="8" cy="8" r="6" fill="currentColor" />}
        {tone === "warn" && <path d="M8 1.5 15 14H1Z" fill="currentColor" />}
        {tone === "bad" && <rect x="2.5" y="2.5" width="11" height="11" fill="currentColor" />}
        {tone === "muted" && <circle cx="8" cy="8" r="5" fill="none" stroke="currentColor" strokeWidth="2" />}
      </svg>
      {children}
    </span>
  );
}

function Service({ connection }: { connection: ConnectionState }) {
  const { t, lang, collectError } = useI18n();
  const now = useNow(10000);
  const widget = WIDGETS[connection.type] as Widget<unknown> | undefined;
  const data = connection.data;
  const failed = connection.status === "error";
  if (!widget) return null;

  let state = { tone: "muted" as Tone, label: t("status.checking") };
  if (failed) state = { tone: "bad", label: t("status.noResponse") };
  else if (data) state = widget.state(data, t);

  const age = connection.last_success_at ? relativeTime(connection.last_success_at, now, lang) : null;
  const freshness = failed ? (age ? t("age.last", { age }) : t("age.never")) : age && t("age.checked", { age });
  const version = data ? widget.footer(data) : "";

  return (
    <section className={failed ? "service stale" : "service"}>
      <div className="service-head">
        <h2>
          <a
            className="service-link"
            href={connection.link}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={t("service.open", { name: connection.name })}
            title={connection.link}
          >
            {connection.name}
            <svg viewBox="0 0 16 16" aria-hidden="true">
              <path d="M6 3H3v10h10v-3M9 3h4v4M13 3 7.5 8.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          </a>
        </h2>
        <StateLabel tone={state.tone}>{state.label}</StateLabel>
      </div>
      {failed && <p role="status" className="service-notice">{collectError(connection.error)}</p>}
      {data ? <widget.Body data={data} /> : null}
      <p className="service-foot">
        {freshness}
        {version && ` (${version})`}
      </p>
    </section>
  );
}
