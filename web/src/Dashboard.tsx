import { useEffect, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type AdGuardData, type ConnectionState, type DashboardState, type DomainCount } from "./api";
import { Horizon, LanguageSwitch, Tower } from "./chrome";
import { useI18n, type Language } from "./i18n";

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

type Tone = "good" | "warn" | "bad" | "muted";

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
  const data = connection.data as AdGuardData | null;
  const failed = connection.status === "error";

  let state = <StateLabel tone="muted">{t("status.checking")}</StateLabel>;
  if (failed) state = <StateLabel tone="bad">{t("status.noResponse")}</StateLabel>;
  else if (data) {
    state = !data.running ? (
      <StateLabel tone="bad">{t("adguard.dnsStopped")}</StateLabel>
    ) : data.protection_enabled ? (
      <StateLabel tone="good">{t("adguard.protectionOn")}</StateLabel>
    ) : (
      <StateLabel tone="warn">{t("adguard.protectionOff")}</StateLabel>
    );
  }

  return (
    <section className={failed ? "service stale" : "service"}>
      <div className="service-head">
        <h2>{connection.name}</h2>
        {state}
      </div>
      {failed && <p role="status" className="service-notice">{collectError(connection.error)}</p>}
      {data && <AdGuard data={data} />}
      <p className="service-foot">
        {failed
          ? connection.last_success_at
            ? t("age.last", { age: relativeTime(connection.last_success_at, now, lang) })
            : t("age.never")
          : connection.last_success_at && t("age.checked", { age: relativeTime(connection.last_success_at, now, lang) })}
        {data?.version && ` (AdGuard Home ${data.version})`}
      </p>
    </section>
  );
}

function AdGuard({ data }: { data: AdGuardData }) {
  const { t, lang } = useI18n();
  const integer = new Intl.NumberFormat(lang);
  const compact = new Intl.NumberFormat(lang, { notation: "compact", maximumFractionDigits: 1 });
  const percent = new Intl.NumberFormat(lang, { maximumFractionDigits: 1 });
  const period =
    data.period_length > 0 && data.period_unit
      ? t(`adguard.period.${data.period_unit}`, { n: data.period_length })
      : null;
  return (
    <>
      <dl className="figures">
        <div>
          <dt>{t("adguard.queries")}</dt>
          <dd title={integer.format(data.queries)}>{compact.format(data.queries)}</dd>
        </div>
        <div>
          <dt>{t("adguard.blocked")}</dt>
          <dd title={integer.format(data.blocked)}>{percent.format(data.blocked_percent)} %</dd>
        </div>
        <div>
          <dt>{t("adguard.avg")}</dt>
          <dd>{data.avg_processing_ms < 1 ? "< 1" : percent.format(data.avg_processing_ms)} ms</dd>
        </div>
      </dl>
      {data.top_queried && data.top_blocked && (
        <div className="tops">
          <TopDomains title={t("adguard.topQueried")} domains={data.top_queried} />
          <TopDomains title={t("adguard.topBlocked")} domains={data.top_blocked} />
        </div>
      )}
      {period && <p className="service-foot">{period}</p>}
    </>
  );
}

function TopDomains({ title, domains }: { title: string; domains: DomainCount[] }) {
  const { t, lang } = useI18n();
  const integer = new Intl.NumberFormat(lang);
  return (
    <div>
      <h3>{title}</h3>
      {domains.length === 0 ? (
        <p>{t("adguard.noDomains")}</p>
      ) : (
        <ol>
          {domains.map((entry) => (
            <li key={entry.domain}>
              <span>{entry.domain}</span>
              <span>{integer.format(entry.count)}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
