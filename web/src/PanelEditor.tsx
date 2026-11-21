import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import ReactGridLayout, { useContainerWidth, type Layout } from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import { api, ApiError, type ConnectionState, type Page, type PlacedWidget } from "./api";
import { catalogEntry } from "./catalog";
import { IntegrationIcon } from "./chrome";
import { Service } from "./Dashboard";
import { useI18n } from "./i18n";
import { COLUMNS, WIDTHS } from "./stack";

// react-draggable inserta un <style> la primera vez que se arrastra, y la política de
// seguridad lo bloquea. Solo lo hace si no existe este elemento; su regla está en styles.css.
if (!document.getElementById("react-draggable-style-el")) {
  const marker = document.createElement("meta");
  marker.id = "react-draggable-style-el";
  document.head.appendChild(marker);
}

const MIN_WIDTH = WIDTHS[0];
const ROW_HEIGHT = 28;
const MARGIN = 16;
const MIN_HEIGHT = 2;
const MAX_HEIGHT = 60;
const NEW_WIDTH = 4;
const NEW_HEIGHT = 10;
const MAX_PAGES = 20;

function randomId() {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

const byPosition = (a: PlacedWidget, b: PlacedWidget) => a.y - b.y || a.x - b.x;

const overlaps = (a: PlacedWidget, b: PlacedWidget) => a.x < b.x + b.w && b.x < a.x + a.w;

// Lo que decide el usuario: páginas, columnas, ancho y qué bloques van encima de
// cada uno en sus columnas. La altura la mide el editor y la «y» la recoloca la
// rejilla, que puede cambiar el orden entre bloques sin columnas en común sin que
// cambie el panel: nada de eso cuenta como cambio.
const layoutKey = (pages: Page[]) =>
  JSON.stringify(
    pages.map((p) => {
      const ordered = [...p.widgets].sort(byPosition);
      return {
        ...p,
        widgets: ordered
          .map((widget, i) => ({
            id: widget.id,
            connection_id: widget.connection_id,
            x: widget.x,
            w: widget.w,
            above: ordered.slice(0, i).filter((other) => overlaps(other, widget)).map((other) => other.id).sort(),
          }))
          .sort((a, b) => a.id.localeCompare(b.id)),
      };
    }),
  );

function withHeights(pages: Page[], rows: Record<string, number>): Page[] {
  return pages.map((p) =>
    p.widgets.some((w) => rows[w.id] !== undefined && rows[w.id] !== w.h)
      ? { ...p, widgets: p.widgets.map((w) => (rows[w.id] !== undefined ? { ...w, h: rows[w.id] } : w)) }
      : p,
  );
}

// Alto en filas de la rejilla para un contenido de `px` píxeles.
const rowsFor = (px: number) => Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, Math.ceil((px + MARGIN) / (ROW_HEIGHT + MARGIN))));

function nextWidth(current: number, delta: number) {
  if (delta > 0) return WIDTHS.find((w) => w > current) ?? COLUMNS;
  return [...WIDTHS].reverse().find((w) => w < current) ?? MIN_WIDTH;
}

// Coloca los widgets en orden, por filas, sin solapes. Es la base de los controles
// de teclado, que no dependen de arrastrar.
function reflow(widgets: PlacedWidget[]): PlacedWidget[] {
  let x = 0;
  let y = 0;
  let rowHeight = 0;
  return widgets.map((widget) => {
    if (x + widget.w > COLUMNS) {
      x = 0;
      y += rowHeight;
      rowHeight = 0;
    }
    const placed = { ...widget, x, y };
    x += widget.w;
    rowHeight = Math.max(rowHeight, widget.h);
    return placed;
  });
}

export default function PanelEditor({ navigate }: { navigate: (path: string) => void }) {
  const { t } = useI18n();
  const panel = useQuery({ queryKey: ["panel"], queryFn: api.panel, refetchOnWindowFocus: false, staleTime: Infinity, gcTime: 0 });
  const connections = useQuery({ queryKey: ["connections"], queryFn: api.connections });
  const status = useQuery({ queryKey: ["status"], queryFn: api.status, refetchInterval: 10000 });
  if (!panel.data || !connections.data) return <p className="loading">{t("app.loading")}</p>;
  return (
    <Editor
      initial={panel.data.pages}
      connectionNames={connections.data.map((c) => ({ id: c.id, name: c.name, type: c.type }))}
      states={status.data?.connections ?? []}
      navigate={navigate}
    />
  );
}

function Editor({
  initial,
  connectionNames,
  states,
  navigate,
}: {
  initial: Page[];
  connectionNames: { id: string; name: string; type: ConnectionState["type"] }[];
  states: ConnectionState[];
  navigate: (path: string) => void;
}) {
  const { t, errorText } = useI18n();
  const client = useQueryClient();
  const [pages, setPages] = useState(initial);
  const [saved, setSaved] = useState(initial);
  const [selected, setSelected] = useState(initial[0]?.id ?? "");
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const dirty = useMemo(() => layoutKey(pages) !== layoutKey(saved), [pages, saved]);
  const page = pages.find((p) => p.id === selected) ?? pages[0];

  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  const save = useMutation({
    mutationFn: () => api.savePanel(pages),
    onSuccess: async (result) => {
      setPages(result.pages);
      setSaved(result.pages);
      client.removeQueries({ queryKey: ["dashboard"] });
    },
  });

  const updatePage = (change: (page: Page) => Page) =>
    setPages((all) => all.map((p) => (p.id === page.id ? change(p) : p)));
  const setWidgets = (widgets: PlacedWidget[]) => updatePage((p) => ({ ...p, widgets }));
  // Ajusta la altura de cada bloque a la medida de su vista previa.
  // Las alturas medidas se recuerdan para aplicarlas también a lo que vuelve al descartar,
  // porque descartar no cambia el tamaño del contenido y no se vuelve a medir.
  const measured = useRef<Record<string, number>>({});
  const setHeights = useCallback((rows: Record<string, number>) => {
    measured.current = { ...measured.current, ...rows };
    setPages((all) => withHeights(all, rows));
  }, []);

  const addPage = () => {
    const id = randomId();
    setPages((all) => [...all, { id, name: t("panel.page.default", { n: all.length + 1 }), widgets: [] }]);
    setSelected(id);
    setConfirmingDelete(false);
  };
  const movePage = (delta: number) =>
    setPages((all) => {
      const from = all.findIndex((p) => p.id === page.id);
      const to = from + delta;
      if (to < 0 || to >= all.length) return all;
      const next = [...all];
      [next[from], next[to]] = [next[to], next[from]];
      return next;
    });
  const deletePage = () => {
    setPages((all) => all.filter((p) => p.id !== page.id));
    setSelected(pages.find((p) => p.id !== page.id)?.id ?? "");
    setConfirmingDelete(false);
  };

  const available = connectionNames.filter((c) => !page.widgets.some((w) => w.connection_id === c.id));
  const addWidget = (connectionId: string) => {
    const y = page.widgets.reduce((bottom, w) => Math.max(bottom, w.y + w.h), 0);
    setWidgets([...page.widgets, { id: randomId(), connection_id: connectionId, x: 0, y, w: NEW_WIDTH, h: NEW_HEIGHT }]);
  };
  const moveWidget = (id: string, delta: number) => {
    const ordered = [...page.widgets].sort(byPosition);
    const from = ordered.findIndex((w) => w.id === id);
    const to = from + delta;
    if (to < 0 || to >= ordered.length) return;
    [ordered[from], ordered[to]] = [ordered[to], ordered[from]];
    setWidgets(reflow(ordered));
  };
  const resizeWidget = (id: string, delta: number) => {
    const ordered = [...page.widgets].sort(byPosition).map((w) =>
      w.id === id ? { ...w, w: nextWidth(w.w, delta) } : w,
    );
    setWidgets(reflow(ordered));
  };
  const removeWidget = (id: string) => setWidgets(page.widgets.filter((w) => w.id !== id));

  if (!page) return null;
  return (
    <main className="panel panel-wide">
      <div className="panel-head">
        <div>
          <h1>{t("panel.title")}</h1>
          <p>{t("panel.lead")}</p>
        </div>
        <div className="editor-actions">
          <span role="status" className={dirty ? "warn" : undefined}>
            {t(dirty ? "panel.unsaved" : "panel.saved")}
          </span>
          <button type="button" className="quiet" disabled={!dirty} onClick={() => setPages(withHeights(saved, measured.current))}>
            {t("panel.discard")}
          </button>
          <button
            type="button"
            className="secondary"
            onClick={() => navigate("/")}
          >
            {t("admin.dashboard")}
          </button>
          <button type="button" className="primary" disabled={!dirty || save.isPending} onClick={() => save.mutate()}>
            {t("panel.save")}
          </button>
        </div>
      </div>
      {save.isError && (
        <p role="alert" className="error">
          {errorText(save.error instanceof ApiError ? save.error.code : "internal")}
        </p>
      )}

      <div className="editor-pages" role="group" aria-label={t("panel.pages")}>
        {pages.map((p) => (
          <button
            type="button"
            key={p.id}
            className={p.id === page.id ? "page-tab current" : "page-tab"}
            aria-pressed={p.id === page.id}
            onClick={() => {
              setSelected(p.id);
              setConfirmingDelete(false);
            }}
          >
            {p.name || t("panel.page.unnamed")}
          </button>
        ))}
        <button type="button" className="quiet" disabled={pages.length >= MAX_PAGES} onClick={addPage}>
          {t("panel.page.add")}
        </button>
      </div>

      <div className="editor-page-settings">
        <label>
          {t("panel.page.name")}
          <input
            value={page.name}
            maxLength={40}
            onChange={(e) => updatePage((p) => ({ ...p, name: e.target.value }))}
          />
        </label>
        <button type="button" className="quiet" disabled={pages[0].id === page.id} onClick={() => movePage(-1)}>
          {t("panel.page.left")}
        </button>
        <button type="button" className="quiet" disabled={pages[pages.length - 1].id === page.id} onClick={() => movePage(1)}>
          {t("panel.page.right")}
        </button>
        {confirmingDelete ? (
          <>
            <span>{t("panel.page.confirmDelete", { name: page.name })}</span>
            <button type="button" className="danger" onClick={deletePage}>
              {t("panel.page.delete")}
            </button>
            <button type="button" className="quiet" onClick={() => setConfirmingDelete(false)}>
              {t("form.cancel")}
            </button>
          </>
        ) : (
          <button type="button" className="quiet" disabled={pages.length === 1} onClick={() => setConfirmingDelete(true)}>
            {t("panel.page.delete")}
          </button>
        )}
      </div>

      {available.length > 0 && (
        <div className="editor-add">
          <h2>{t("panel.add")}</h2>
          <ul>
            {available.map((c) => (
              <li key={c.id}>
                <button type="button" className="secondary" onClick={() => addWidget(c.id)}>
                  <IntegrationIcon icon={catalogEntry(c.type).icon} />
                  {c.name}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {page.widgets.length === 0 ? (
        <div className="empty">
          <h2>{t("panel.empty.title")}</h2>
          <p>{t(connectionNames.length ? "panel.empty.lead" : "panel.empty.noConnections")}</p>
        </div>
      ) : (
        <Grid
          key={page.id}
          widgets={page.widgets}
          names={connectionNames}
          states={states}
          onChange={setWidgets}
          onHeights={setHeights}
          onMove={moveWidget}
          onResize={resizeWidget}
          onRemove={removeWidget}
        />
      )}
      <p className="service-foot">{t("panel.hint")}</p>
    </main>
  );
}

function Grid({
  widgets,
  names,
  states,
  onChange,
  onHeights,
  onMove,
  onResize,
  onRemove,
}: {
  widgets: PlacedWidget[];
  names: { id: string; name: string; type: ConnectionState["type"] }[];
  states: ConnectionState[];
  onChange: (widgets: PlacedWidget[]) => void;
  onHeights: (rows: Record<string, number>) => void;
  onMove: (id: string, delta: number) => void;
  onResize: (id: string, delta: number) => void;
  onRemove: (id: string) => void;
}) {
  const { t } = useI18n();
  const { width, containerRef, mounted } = useContainerWidth();
  // Mide la barra y la vista previa de cada bloque para darle la altura de su contenido.
  const observer = useRef<ResizeObserver | null>(null);
  useEffect(() => {
    observer.current = new ResizeObserver((entries) => {
      const rows: Record<string, number> = {};
      for (const entry of entries) {
        const box = entry.target as HTMLElement;
        const card = box.closest(".editor-widget");
        const bar = card?.querySelector(".editor-widget-bar") as HTMLElement | null;
        const preview = card?.querySelector(".editor-widget-preview") as HTMLElement | null;
        if (!box.dataset.id || !bar || !preview) continue;
        const padding = parseFloat(getComputedStyle(preview).paddingTop) + parseFloat(getComputedStyle(preview).paddingBottom);
        rows[box.dataset.id] = rowsFor(bar.offsetHeight + box.offsetHeight + padding + 2);
      }
      onHeights(rows);
    });
    document.querySelectorAll<HTMLElement>(".editor-widget-content").forEach((el) => observer.current?.observe(el));
    return () => observer.current?.disconnect();
  }, [onHeights]);
  const measure = useCallback((element: HTMLDivElement | null) => {
    if (element) observer.current?.observe(element);
  }, []);
  const layout = widgets.map((w) => ({
    i: w.id, x: w.x, y: w.y, w: w.w, h: w.h,
    minW: MIN_WIDTH, minH: w.h, maxH: w.h,
  }));
  const handleLayout = (next: Layout) => {
    const changed = widgets.some((w) => {
      const item = next.find((n) => n.i === w.id);
      return item && (item.x !== w.x || item.y !== w.y || item.w !== w.w || item.h !== w.h);
    });
    if (!changed) return;
    onChange(
      widgets.map((w) => {
        const item = next.find((n) => n.i === w.id);
        return item ? { ...w, x: item.x, y: item.y, w: item.w, h: item.h } : w;
      }),
    );
  };
  return (
    <div ref={containerRef} className="editor-grid">
      {mounted && (
        <ReactGridLayout
          width={width}
          layout={layout}
          gridConfig={{ cols: COLUMNS, rowHeight: ROW_HEIGHT, margin: [MARGIN, MARGIN] }}
          dragConfig={{ enabled: true, handle: ".widget-handle" }}
          resizeConfig={{ enabled: true, handles: ["e"] }}
          onLayoutChange={handleLayout}
        >
          {widgets.map((widget) => {
            const connection = names.find((c) => c.id === widget.connection_id);
            const state = states.find((s) => s.id === widget.connection_id);
            return (
              <div key={widget.id} className="editor-widget" data-testid={`widget-${connection?.name}`}>
                <div className="editor-widget-bar">
                  <span className="widget-handle" title={t("panel.widget.drag")}>
                    <svg viewBox="0 0 16 16" aria-hidden="true">
                      <path d="M5 3h2v2H5zM9 3h2v2H9zM5 7h2v2H5zM9 7h2v2H9zM5 11h2v2H5zM9 11h2v2H9z" fill="currentColor" />
                    </svg>
                    {connection?.name}
                  </span>
                  <span className="editor-widget-actions">
                    <button type="button" onClick={() => onMove(widget.id, -1)}>{t("panel.widget.before")}</button>
                    <button type="button" onClick={() => onMove(widget.id, 1)}>{t("panel.widget.after")}</button>
                    <button type="button" onClick={() => onResize(widget.id, -2)}>{t("panel.widget.narrower")}</button>
                    <button type="button" onClick={() => onResize(widget.id, 2)}>{t("panel.widget.wider")}</button>
                    <button type="button" onClick={() => onRemove(widget.id)}>{t("panel.widget.remove")}</button>
                  </span>
                </div>
                <div className="editor-widget-preview" inert>
                  <div className="editor-widget-content" data-id={widget.id} ref={measure}>
                    {state ? <Service connection={state} /> : <p>{t("connections.notInPanel")}</p>}
                  </div>
                </div>
              </div>
            );
          })}
        </ReactGridLayout>
      )}
    </div>
  );
}
