import { useEffect, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type ConnectionState, type ConnectionType, type DashboardState } from "./api";
import { Horizon, LanguageSwitch, Tower } from "./chrome";
import { useI18n, type Language } from "./i18n";
import { adguardWidget } from "./widgets/adguard";
import { opnsenseWidget } from "./widgets/opnsense";
import { homeAssistantWidget } from "./widgets/homeassistant";
import { proxmoxWidget } from "./widgets/proxmox";
import type { Tone, Widget } from "./widgets/types";

const RETRY_MS = 5000;

const WIDGETS: Partial<Record<ConnectionType, unknown>> = {
  adguard: adguardWidget,
  homeassistant: homeAssistantWidget,
  opnsense: opnsenseWidget,
  proxmox: proxmoxWidget,
};

function useLiveDashboard() {
  const client = useQueryClient();
  const [live, setLive] = useState(true);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let retry: ReturnType<typeof setTimeout> | undefined;
    const events = new EventSource("/api/dashboard/events");
    events.addEventListener("snapshot", (event) => {
      setLive(true);
      client.setQueryData<DashboardState>(["dashboard"], JSON.parse((event as MessageEvent<string>).data));
    });
    events.onerror = () => {
      setLive(false);
      // Puede que ahora el panel exija iniciar sesión.
      client.invalidateQueries({ queryKey: ["state"] });
      // EventSource solo reintenta por sí solo ante errores de red, no de HTTP.
      if (events.readyState === EventSource.CLOSED) retry = setTimeout(() => setAttempt((n) => n + 1), RETRY_MS);
    };
    return () => {
      clearTimeout(retry);
      events.close();
    };
  }, [client, attempt]);
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

export default function Dashboard({ path, navigate }: { path: string; navigate: (path: string) => void }) {
  const { t } = useI18n();
  const live = useLiveDashboard();
  const dashboard = useQuery({ queryKey: ["dashboard"], queryFn: api.dashboard });
  const link = (to: string, label: string, className: string, current?: boolean) => (
    <a
      key={to}
      className={className}
      href={to}
      aria-current={current ? "page" : undefined}
      onClick={(event) => {
        event.preventDefault();
        navigate(to);
      }}
    >
      {label}
    </a>
  );

  const pages = dashboard.data?.pages ?? [];
  if (dashboard.data && pages.every((p) => p.widgets.length === 0)) {
    return (
      <Horizon>
        <h1>{t("dashboard.title")}</h1>
        <p>{t("dashboard.lead")}</p>
        {link("/admin", t("dashboard.admin"), "button primary")}
      </Horizon>
    );
  }
  const requested = path.startsWith("/p/") ? decodeURIComponent(path.slice(3)) : null;
  const page = pages.find((p) => p.id === requested) ?? pages[0];
  const states = new Map(dashboard.data?.connections.map((c) => [c.id, c]));
  return (
    <div className="lookout">
      <header className="lookout-sky">
        <Tower />
        <span className="wordmark">Talaia</span>
        {link("/admin", t("dashboard.admin.short"), "quiet")}
        <LanguageSwitch />
      </header>
      <main className="lookout-main">
        {pages.length > 1 ? (
          <nav className="page-tabs" aria-label={t("panel.pages")}>
            {pages.map((p) =>
              link(`/p/${encodeURIComponent(p.id)}`, p.name, p.id === page.id ? "page-tab current" : "page-tab", p.id === page.id),
            )}
          </nav>
        ) : null}
        <h1 className="visually-hidden">{page?.name ?? "Talaia"}</h1>
        {!live && <p role="status" className="live-lost">{t("dashboard.live.lost")}</p>}
        {dashboard.isPending && <p>{t("app.loading")}</p>}
        {dashboard.isError && <p role="alert" className="error">{t("dashboard.unavailable")}</p>}
        {page && page.widgets.length === 0 && <p className="lead">{t("dashboard.page.empty")}</p>}
        <div className="panel-grid">
          {page?.widgets.map((widget) => {
            const state = states.get(widget.connection_id);
            return state ? (
              <div
                key={widget.id}
                style={{
                  gridColumn: `${widget.x + 1} / span ${widget.w}`,
                  gridRow: `${widget.y + 1} / span ${widget.h}`,
                }}
              >
                <Service connection={state} />
              </div>
            ) : null;
          })}
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

export function Service({ connection }: { connection: ConnectionState }) {
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
