import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type ConnectionState, type ConnectionType, type DashboardState } from "./api";
import { BrandIcon, Horizon } from "./chrome";
import { Lookout } from "./Lookout";
import { relativeTime, StateLabel, useNow, type AccessItem, type Watched, type WatchState } from "./status";
import { useI18n, type I18n } from "./i18n";
import { adguardWidget } from "./widgets/adguard";
import { availabilityWidget } from "./widgets/availability";
import { nutWidget } from "./widgets/nut";
import { printerWidget } from "./widgets/printer";
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
  availability: availabilityWidget,
  homeassistant: homeAssistantWidget,
  nut: nutWidget,
  printer: printerWidget,
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

const SEVERITY: Record<Tone, number> = { muted: 0, good: 0, warn: 1, bad: 2 };

/** Estado de un servicio: el mismo para su bloque y para el resumen de la cabecera. */
function serviceState(connection: ConnectionState, t: I18n["t"]): { tone: Tone; label: string } {
  const { tone, key, params } = connection.state;
  return { tone, label: t(key as Parameters<I18n["t"]>[0], params) };
}

export function serviceAnchor(connectionId: string) {
  return `servicio-${connectionId}`;
}

/** Peor estado de todos los servicios del panel, de cualquier página. */
function watch(dashboard: DashboardState | undefined, t: I18n["t"]): WatchState {
  const states = new Map(dashboard?.connections.map((c) => [c.id, c]));
  const seen = new Set<string>();
  const watched: Watched[] = [];
  for (const page of dashboard?.pages ?? []) {
    for (const widget of page.widgets) {
      const connection = states.get(widget.connection_id);
      if (!connection || seen.has(connection.id) || !WIDGETS[connection.type]) continue;
      seen.add(connection.id);
      watched.push({ connection, pageId: page.id, ...serviceState(connection, t) });
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
    tone: checking ? "muted" : worst,
    total: watched.length,
    attention: watched
      .filter((item) => SEVERITY[item.tone] > 0)
      .sort((a, b) => SEVERITY[b.tone] - SEVERITY[a.tone]),
  };
}

/** Los servicios del panel con enlace, de todas las páginas, sin repetir y en el orden del panel. */
function accessItems(dashboard: DashboardState | undefined, t: I18n["t"]): AccessItem[] {
  const states = new Map(dashboard?.connections.map((c) => [c.id, c]));
  const seen = new Set<string>();
  const items: AccessItem[] = [];
  for (const page of dashboard?.pages ?? []) {
    for (const widget of [...page.widgets].sort((a, b) => a.y - b.y || a.x - b.x)) {
      const connection = states.get(widget.connection_id);
      if (!connection?.link || !connection.show_access || seen.has(connection.id) || !WIDGETS[connection.type]) continue;
      seen.add(connection.id);
      items.push({ id: connection.id, name: connection.name, link: connection.link, icon: connection.icon, tone: serviceState(connection, t).tone });
    }
  }
  return items;
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
  const summary = watch(dashboard.data, t);
  const goTo = (item: Watched) => {
    if (item.pageId !== page?.id) navigate(`/p/${encodeURIComponent(item.pageId)}`);
    // Tras el cambio de página, el bloque se mide antes de verse: hasta entonces no puede recibir el foco.
    const until = performance.now() + 2000;
    const focusBlock = () => {
      const block = document.getElementById(serviceAnchor(item.connection.id));
      if (!block || block.checkVisibility?.({ visibilityProperty: true }) === false) {
        if (performance.now() < until) requestAnimationFrame(focusBlock);
        return;
      }
      block.scrollIntoView({ block: "start" });
      block.focus({ preventScroll: true });
    };
    requestAnimationFrame(focusBlock);
  };
  return (
    <Lookout
      summary={summary}
      goTo={goTo}
      pages={pages}
      currentPageId={page?.id}
      access={accessItems(dashboard.data, t)}
      linkGroups={dashboard.data?.link_groups ?? []}
      navigate={navigate}
      version={version}
    >
      <h1 className="visually-hidden">{page?.name ?? "Talaia"}</h1>
      {!live && <p role="status" className="live-lost">{t("dashboard.live.lost")}</p>}
      {dashboard.isPending && <p>{t("app.loading")}</p>}
      {dashboard.isError && <p role="alert" className="error">{t("dashboard.unavailable")}</p>}
      {page && page.widgets.length === 0 && <p className="lead">{t("dashboard.page.empty")}</p>}
      <StackedPanel widgets={page?.widgets ?? []} states={states} />
    </Lookout>
  );
}

// Espacio entre bloques, en px: 1rem.
const GAP = 16;

/** Ancho real de un elemento, en px, al día con sus cambios de tamaño. */
function useElementWidth() {
  const [element, setElement] = useState<HTMLDivElement | null>(null);
  const [width, setWidth] = useState(0);
  useLayoutEffect(() => {
    if (!element) return;
    // Primera medida antes de pintar, para no colocar los bloques con un ancho falso.
    setWidth(element.getBoundingClientRect().width);
    const observer = new ResizeObserver(([entry]) => setWidth(entry.contentRect.width));
    observer.observe(element);
    return () => observer.disconnect();
  }, [element]);
  return [setElement, width] as const;
}

/**
 * Coloca cada bloque justo debajo del último bloque de sus columnas, según la
 * altura real de su contenido. Cada bloque se mide ya con su ancho definitivo.
 */
function StackedPanel({ widgets, states }: { widgets: PlacedWidget[]; states: Map<string, ConnectionState> }) {
  const [gridRef, area] = useElementWidth();
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
  const items = area > 0 ? responsiveItems(visible, area, GAP) : [];
  const ready = items.length > 0 && items.every((item) => heights[item.id] !== undefined);
  const { placed, height } = stackLayout(items, heights, GAP);
  const byId = new Map(placed.map((p) => [p.id, p]));
  const column = (n: number) => `calc((100% + ${GAP}px) * ${n} / ${COLUMNS})`;
  return (
    <div ref={gridRef} className={ready ? "panel-grid stacked measured" : "panel-grid stacked"} style={{ height }}>
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
              <BrandIcon icon={connection.icon} />
              {connection.name}
              <svg viewBox="0 0 16 16" aria-hidden="true">
                <path d="M6 3H3v10h10v-3M9 3h4v4M13 3 7.5 8.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
              </svg>
            </a>
          ) : (
            <span className="service-name">
              <BrandIcon icon={connection.icon} />
              {connection.name}
            </span>
          )}
        </h2>
        <StateLabel tone={state.tone}>{state.label}</StateLabel>
      </div>
      <div className="service-body">
        {failed && <p role="status" className="service-notice">{collectError(connection.error)}</p>}
        {data ? <widget.Body data={data} /> : null}
        <p className="service-foot">
          {freshness && version ? `${freshness} (${version})` : freshness || version}
        </p>
      </div>
    </section>
  );
}
