import { Fragment, useEffect, useState, type ReactNode } from "react";
import type { BrandIconData, DashboardLinkGroup } from "./api";
import { BrandIcon, LanguageSwitch, Tower } from "./chrome";
import { useI18n } from "./i18n";
import { relativeTime, StateLabel, ToneMark, useNow, type AccessItem, type Watched, type WatchState } from "./status";

// Desde este ancho de ventana, la torre va en una columna a la izquierda; por debajo, en una barra arriba.
export const WIDE_FROM_PX = 1100;

export function useViewportWidth() {
  const [width, setWidth] = useState(() => window.innerWidth);
  useEffect(() => {
    const onResize = () => setWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);
  return width;
}

interface PageLink {
  id: string;
  name: string;
}

interface LookoutProps {
  summary: WatchState;
  goTo: (item: Watched) => void;
  pages: PageLink[];
  currentPageId?: string;
  access: AccessItem[];
  linkGroups: DashboardLinkGroup[];
  navigate: (path: string) => void;
  version: string;
  children: ReactNode;
}

/**
 * Carcasa del panel. La columna y la barra son dos maquetaciones del mismo contenido:
 * cada una en el orden en que se ve, para que el foco siga el mismo orden.
 */
export function Lookout({ summary, goTo, pages, currentPageId, access, linkGroups, navigate, version, children }: LookoutProps) {
  const { t } = useI18n();
  const wide = useViewportWidth() >= WIDE_FROM_PX;
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
  const skip = (
    <a className="skip-link" href="#servicios">
      {t("panel.skip")}
    </a>
  );
  const brand = (
    <div className="lookout-brand">
      {/* La clave repite el barrido de la luz cada vez que cambia el peor estado. */}
      <Tower key={summary.tone} />
      <span className="wordmark">Talaia</span>
    </div>
  );
  const pageLinks =
    pages.length > 1 ? (
      <nav className="page-tabs" aria-label={t("panel.pages")}>
        {pages.map((p) =>
          link(`/p/${encodeURIComponent(p.id)}`, p.name, p.id === currentPageId ? "page-tab current" : "page-tab", p.id === currentPageId),
        )}
      </nav>
    ) : null;
  const admin = link("/admin", t("dashboard.admin.short"), "quiet");
  const main = (
    <main id="servicios" className="lookout-main" tabIndex={-1}>
      {children}
    </main>
  );

  if (wide) {
    return (
      <div className="lookout wide">
        {skip}
        <nav className="lookout-side" aria-label={t("panel.nav")} data-watch={summary.tone}>
          {brand}
          <WatchSummary summary={summary} goTo={goTo} />
          {pageLinks && (
            <section className="side-section">
              <h2>{t("panel.pages")}</h2>
              {pageLinks}
            </section>
          )}
          {access.length > 0 && (
            <section className="side-section">
              <h2>{t("panel.services")}</h2>
              <ul className="access-list">
                {access.map((item) => (
                  <li key={item.id}>
                    <AccessLink name={item.name} url={item.link} icon={item.icon} tone={item.tone} />
                  </li>
                ))}
              </ul>
            </section>
          )}
          {linkGroups.map((group, i) => (
            <section key={i} className="side-section">
              <h2>{group.name}</h2>
              <ul className="access-list">
                {group.links.map((link, j) => (
                  <li key={j}>
                    <AccessLink name={link.name} url={link.url} icon={link.icon} />
                  </li>
                ))}
              </ul>
            </section>
          ))}
          <div className="side-foot">
            {admin}
            <LanguageSwitch />
            <span className="version">Talaia {version}</span>
          </div>
        </nav>
        {main}
      </div>
    );
  }
  return (
    <div className="lookout narrow">
      {skip}
      <header className="lookout-bar" data-watch={summary.tone}>
        <div className="bar-row">
          {brand}
          <span className={`bar-mark ${summary.tone}`}>
            <ToneMark tone={summary.tone} />
          </span>
          {admin}
          <LanguageSwitch />
        </div>
        <WatchSummary summary={summary} goTo={goTo} />
        {(access.length > 0 || linkGroups.length > 0) && (
          <nav className="access-row" aria-label={t("panel.access")}>
            <ul className="access-list">
              {access.map((item) => (
                <li key={item.id}>
                  <AccessLink name={item.name} url={item.link} icon={item.icon} tone={item.tone} />
                </li>
              ))}
              {linkGroups.map((group, i) => (
                <Fragment key={i}>
                  <li className="access-group">{group.name}</li>
                  {group.links.map((link, j) => (
                    <li key={j}>
                      <AccessLink name={link.name} url={link.url} icon={link.icon} />
                    </li>
                  ))}
                </Fragment>
              ))}
            </ul>
          </nav>
        )}
      </header>
      {pageLinks}
      {main}
      <footer className="version">Talaia {version}</footer>
    </div>
  );
}

/** Un acceso: un servicio vigilado (con su estado) o un enlace guardado (sin estado). */
function AccessLink({ name, url, icon, tone }: { name: string; url: string; icon: BrandIconData | null; tone?: AccessItem["tone"] }) {
  const { t } = useI18n();
  return (
    <a href={url} target="_blank" rel="noopener noreferrer" aria-label={t("service.open", { name })} title={url}>
      {icon ? <BrandIcon icon={icon} /> : <span className="brand-icon placeholder" aria-hidden="true" />}
      <span className="access-name">{name}</span>
      {tone && (
        <span className={`access-tone ${tone}`}>
          <ToneMark tone={tone} />
        </span>
      )}
    </a>
  );
}

function WatchSummary({ summary, goTo }: { summary: WatchState; goTo: (item: Watched) => void }) {
  const { t, lang } = useI18n();
  const now = useNow(10000);
  if (summary.total === 0) return null;
  if (summary.tone === "muted") {
    return (
      <div className="watch">
        <p>{t("watch.checking")}</p>
      </div>
    );
  }
  const age = summary.updated ? (
    <p className="watch-age">{t("age.checked", { age: relativeTime(summary.updated, now, lang) })}</p>
  ) : null;
  if (summary.attention.length === 0) {
    return (
      <div className="watch good">
        <p>
          <StateLabel tone="good">
            {summary.total === 1 ? t("watch.ok.one") : t("watch.ok.other", { count: summary.total })}
          </StateLabel>
        </p>
        {age}
      </div>
    );
  }
  const count = summary.attention.length;
  const others = summary.total - count;
  return (
    <div className={`watch ${summary.tone}`}>
      <p>
        <StateLabel tone={summary.tone}>
          {count === 1 ? t("watch.attention.one") : t("watch.attention.other", { count })}
        </StateLabel>
      </p>
      <ul className="watch-list">
        {summary.attention.map((item) => (
          <li key={item.connection.id}>
            <a
              className={`watch-item ${item.tone}`}
              href={`/p/${encodeURIComponent(item.pageId)}#servicio-${item.connection.id}`}
              onClick={(event) => {
                event.preventDefault();
                goTo(item);
              }}
            >
              {item.connection.name}
            </a>
            : {item.label}
          </li>
        ))}
      </ul>
      {others > 0 && <p className="watch-others">{others === 1 ? t("watch.others.one") : t("watch.others.other", { count: others })}</p>}
      {age}
    </div>
  );
}
