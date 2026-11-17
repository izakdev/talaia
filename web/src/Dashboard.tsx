import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type ConnectionState, type ConnectionType, type DashboardState } from "./api";
import { Horizon, LanguageSwitch, Tower } from "./chrome";
import { useI18n, type I18n, type Language } from "./i18n";
import { adguardWidget } from "./widgets/adguard";
import { opnsenseWidget } from "./widgets/opnsense";
import { portainerWidget } from "./widgets/portainer";
import { homeAssistantWidget } from "./widgets/homeassistant";
import { proxmoxWidget } from "./widgets/proxmox";
import { qnapWidget } from "./widgets/qnap";
import { wgdashboardWidget } from "./widgets/wgdashboard";
import type { Tone, Widget } from "./widgets/types";
import { responsiveItems, stackLayout, COLUMNS } from "./stack";
import type { PlacedWidget } from "./api";

const RETRY_MS = 5000;

const WIDGETS: Partial<Record<ConnectionType, unknown>> = {
  adguard: adguardWidget,
  homeassistant: homeAssistantWidget,
  opnsense: opnsenseWidget,
  portainer: portainerWidget,
  proxmox: proxmoxWidget,
  qnap: qnapWidget,
  wgdashboard: wgdashboardWidget,
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

const SEVERITY: Record<Tone, number> = { muted: 0, good: 0, warn: 1, bad: 2 };

/** Estado de un servicio: el mismo para su bloque y para el resumen de la cabecera. */
function serviceState(connection: ConnectionState, t: I18n["t"]): { tone: Tone; label: string } {
  const widget = WIDGETS[connection.type] as Widget<unknown> | undefined;
  if (connection.status === "error") return { tone: "bad", label: t("status.noResponse") };
  if (widget && connection.data) return widget.state(connection.data, t);
  return { tone: "muted", label: t("status.checking") };
}

export function serviceAnchor(connectionId: string) {
  return `servicio-${connectionId}`;
}

interface Watched {
  connection: ConnectionState;
  pageId: string;
  tone: Tone;
}

/** Peor estado de todos los servicios del panel, de cualquier página. */
function watch(dashboard: DashboardState | undefined, t: I18n["t"]) {
  const states = new Map(dashboard?.connections.map((c) => [c.id, c]));
  const seen = new Set<string>();
  const watched: Watched[] = [];
  for (const page of dashboard?.pages ?? []) {
    for (const widget of page.widgets) {
      const connection = states.get(widget.connection_id);
      if (!connection || seen.has(connection.id) || !WIDGETS[connection.type]) continue;
      seen.add(connection.id);
      watched.push({ connection, pageId: page.id, tone: serviceState(connection, t).tone });
    }
  }
  const worst = watched.reduce<Tone>((tone, item) => (SEVERITY[item.tone] > SEVERITY[tone] ? item.tone : tone), "good");
  const checking = watched.length > 0 && watched.every((item) => item.tone === "muted" && !item.connection.data);
  const updated = watched.reduce<string | null>((latest, item) => {
    const at = item.connection.last_success_at;
    return at && (!latest || at > latest) ? at : latest;
  }, null);
  return {
    updated,
    tone: checking ? ("muted" as Tone) : worst,
    total: watched.length,
    attention: watched
      .filter((item) => SEVERITY[item.tone] > 0)
      .sort((a, b) => SEVERITY[b.tone] - SEVERITY[a.tone]),
  };
}

function WatchSummary({
  summary,
  goTo,
}: {
  summary: ReturnType<typeof watch>;
  goTo: (item: Watched) => void;
}) {
  const { t, lang } = useI18n();
  if (summary.total === 0) return null;
  if (summary.tone === "muted") return <p className="watch">{t("watch.checking")}</p>;
  if (summary.attention.length === 0) {
    return (
      <p className="watch">
        <StateLabel tone="good">
          {summary.total === 1 ? t("watch.ok.one") : t("watch.ok.other", { count: summary.total })}
        </StateLabel>
      </p>
    );
  }
  const count = summary.attention.length;
  // Intl.ListFormat da «A, B y C» o «A, B and C»; los nombres se intercalan como enlaces.
  const parts = new Intl.ListFormat(lang, { type: "conjunction" }).formatToParts(
    summary.attention.map((item) => item.connection.id),
  );
  const byId = new Map(summary.attention.map((item) => [item.connection.id, item]));
  return (
    <p className="watch">
      <StateLabel tone={summary.tone}>
        {count === 1 ? t("watch.attention.one") : t("watch.attention.other", { count })}
      </StateLabel>{" "}
      {parts.map((part, i) => {
        const item = part.type === "element" ? byId.get(part.value) : undefined;
        if (!item) return <span key={i}>{part.value}</span>;
        return (
          <a
            key={i}
            className={`watch-item ${item.tone}`}
            href={`/p/${encodeURIComponent(item.pageId)}#${serviceAnchor(item.connection.id)}`}
            onClick={(event) => {
              event.preventDefault();
              goTo(item);
            }}
          >
            {item.connection.name}
          </a>
        );
      })}
    </p>
  );
}

export default function Dashboard({
  path,
  navigate,
  version,
}: {
  path: string;
  navigate: (path: string) => void;
  version: string;
}) {
  const { t, lang } = useI18n();
  const now = useNow(10000);
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
  const summary = watch(dashboard.data, t);
  const goTo = (item: Watched) => {
    if (item.pageId !== page?.id) navigate(`/p/${encodeURIComponent(item.pageId)}`);
    // Tras el cambio de página, el bloque ya está en el documento.
    requestAnimationFrame(() => {
      const block = document.getElementById(serviceAnchor(item.connection.id));
      block?.scrollIntoView({ block: "start" });
      block?.focus({ preventScroll: true });
    });
  };
  return (
    <div className="lookout">
      <header className="lookout-sky" data-watch={summary.tone}>
        {/* La clave repite el barrido de la luz cada vez que cambia el peor estado. */}
        <Tower key={summary.tone} />
        <div className="lookout-title">
          <span className="wordmark">Talaia</span>
          <WatchSummary summary={summary} goTo={goTo} />
          {summary.updated && summary.tone !== "muted" && (
            <span className="watch-age">{t("age.checked", { age: relativeTime(summary.updated, now, lang) })}</span>
          )}
        </div>
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
        <StackedPanel widgets={page?.widgets ?? []} states={states} />
      </main>
      <footer>Talaia {version}</footer>
    </div>
  );
}

// Espacio entre bloques, en px: 1.5rem.
const GAP = 24;

function useViewportWidth() {
  const [width, setWidth] = useState(() => window.innerWidth);
  useEffect(() => {
    const onResize = () => setWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);
  return width;
}

/**
 * Coloca cada bloque justo debajo del último bloque de sus columnas, según la
 * altura real de su contenido. Cada bloque se mide ya con su ancho definitivo.
 */
function StackedPanel({ widgets, states }: { widgets: PlacedWidget[]; states: Map<string, ConnectionState> }) {
  const viewport = useViewportWidth();
  const [heights, setHeights] = useState<Record<string, number>>({});
  const observer = useRef<ResizeObserver | null>(null);
  if (!observer.current && typeof ResizeObserver !== "undefined") {
    observer.current = new ResizeObserver((entries) => {
      setHeights((current) => {
        let next = current;
        for (const entry of entries) {
          // Un bloque que se desmonta avisa con altura 0: no es una medida.
          if (!entry.target.isConnected) continue;
          const id = (entry.target as HTMLElement).dataset.id;
          const height = Math.round(entry.borderBoxSize?.[0]?.blockSize ?? entry.target.getBoundingClientRect().height);
          if (id && current[id] !== height) next = { ...next, [id]: height };
        }
        return next;
      });
    });
  }
  useEffect(() => () => observer.current?.disconnect(), []);
  // Al desmontarse, el bloque deja de observarse y olvida su altura: si vuelve, se mide de nuevo.
  const measure = useCallback((element: HTMLDivElement | null) => {
    if (!element) return;
    const id = element.dataset.id;
    observer.current?.observe(element);
    return () => {
      observer.current?.unobserve(element);
      if (id) setHeights(({ [id]: _, ...rest }) => rest);
    };
  }, []);

  const visible = widgets.filter((widget) => states.has(widget.connection_id));
  const items = responsiveItems(visible, viewport);
  const ready = items.length > 0 && items.every((item) => heights[item.id] !== undefined);
  const { placed, height } = stackLayout(items, heights, GAP);
  const byId = new Map(placed.map((p) => [p.id, p]));
  const column = (n: number) => `calc((100% + ${GAP}px) * ${n} / ${COLUMNS})`;
  return (
    <div className={ready ? "panel-grid stacked measured" : "panel-grid stacked"} style={{ height }}>
      {items.map((item) => {
        const widget = visible.find((w) => w.id === item.id)!;
        const spot = byId.get(item.id)!;
        return (
          <div
            key={item.id}
            ref={measure}
            data-id={item.id}
            data-x={spot.x}
            data-w={spot.w}
            className="panel-cell"
            style={{
              left: column(spot.x),
              width: `calc(${column(spot.w)} - ${GAP}px)`,
              top: spot.top,
              // Un bloque sin medir ya tiene su ancho definitivo, pero no se ve hasta saber su altura.
              visibility: heights[item.id] === undefined ? "hidden" : undefined,
            }}
          >
            <Service connection={states.get(widget.connection_id)!} />
          </div>
        );
      })}
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
  const state = serviceState(connection, t);

  const age = connection.last_success_at ? relativeTime(connection.last_success_at, now, lang) : null;
  // Cuando todo va bien, la antigüedad está en la cabecera; el bloque solo la indica si su dato es viejo.
  const freshness = failed ? (age ? t("age.last", { age }) : t("age.never")) : null;
  const version = data ? widget.footer(data) : "";

  return (
    <section
      id={serviceAnchor(connection.id)}
      tabIndex={-1}
      className={["service", `tone-${state.tone}`, failed && "stale"].filter(Boolean).join(" ")}
    >
      <div className="service-head">
        <h2>
          {/* Sin enlace propio, una conexión snmp:// no tiene página que abrir. */}
          {connection.link ? (
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
          ) : (
            connection.name
          )}
        </h2>
        <StateLabel tone={state.tone}>{state.label}</StateLabel>
      </div>
      {failed && <p role="status" className="service-notice">{collectError(connection.error)}</p>}
      {data ? <widget.Body data={data} /> : null}
      <p className="service-foot">
        {freshness && version ? `${freshness} (${version})` : freshness || version}
      </p>
    </section>
  );
}
