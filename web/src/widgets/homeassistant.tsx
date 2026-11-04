import type { EntityOption, HomeAssistantData, HomeAssistantSummary, HomeAssistantSystem } from "../api";
import { useI18n, type Translate } from "../i18n";
import type { Widget } from "./types";

const CONSUMPTION = ["power", "energy"];
const NO_DATA = ["unavailable", "unknown", "missing"];
const DOMAINS = ["light", "switch", "fan", "climate", "cover", "media_player", "automation"] as const;
type Domain = (typeof DOMAINS)[number];

const hasNoData = (entity: EntityOption) => NO_DATA.includes(entity.state);

function useEntityValue() {
  const { t, lang } = useI18n();
  const decimal = new Intl.NumberFormat(lang, { maximumFractionDigits: 2 });
  return (entity: EntityOption) => {
    const known = stateLabel(entity.state, t);
    if (known) return known;
    const value = entity.state.trim() === "" ? NaN : Number(entity.state);
    const text = Number.isFinite(value) ? decimal.format(value) : entity.state;
    return entity.unit ? `${text} ${entity.unit}` : text;
  };
}

function stateLabel(state: string, t: Translate) {
  switch (state) {
    case "on":
    case "off":
    case "unavailable":
    case "unknown":
    case "missing":
      return t(`homeassistant.state.${state}`);
    default:
      return null;
  }
}

export function formatBytes(value: number, format: Intl.NumberFormat) {
  const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
  let size = value;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit++;
  }
  return `${format.format(size)} ${units[unit]}`;
}

function System({ system }: { system: HomeAssistantSystem }) {
  const { t, lang } = useI18n();
  const percent = new Intl.NumberFormat(lang, { maximumFractionDigits: 0 });
  const decimal = new Intl.NumberFormat(lang, { maximumFractionDigits: 1 });
  const disk = system.disk_total > 0 ? (system.disk_used / system.disk_total) * 100 : 0;
  const days = system.booted_at ? Math.round((system.booted_at - Date.now() / 1000) / 86400) : null;
  const age = days === null ? null : new Intl.RelativeTimeFormat(lang, { numeric: "auto" }).format(days, "day");
  return (
    <div className="ha-section">
      <h3>{t("homeassistant.system")}</h3>
      <dl className="figures compact">
        <div>
          <dt>{t("homeassistant.cpu")}</dt>
          <dd>{percent.format(system.cpu_percent)} %</dd>
        </div>
        <div>
          <dt>{t("homeassistant.memory")}</dt>
          <dd>
            {percent.format(system.memory_percent)} %
            <small>
              {t("homeassistant.of", {
                used: formatBytes(system.memory_used, decimal),
                total: formatBytes(system.memory_limit, decimal),
              })}
            </small>
          </dd>
        </div>
        {system.disk_total > 0 && (
          <div>
            <dt>{t("homeassistant.disk")}</dt>
            <dd>
              {percent.format(disk)} %
              <small>
                {t("homeassistant.of", {
                  used: formatBytes(system.disk_used, decimal),
                  total: formatBytes(system.disk_total, decimal),
                })}
              </small>
            </dd>
          </div>
        )}
      </dl>
      {age && <p className="service-foot">{t("homeassistant.booted", { age })}</p>}
    </div>
  );
}

function Summary({ summary }: { summary: HomeAssistantSummary }) {
  const { t, lang } = useI18n();
  const integer = new Intl.NumberFormat(lang);
  return (
    <div className="ha-section">
      <h3>{t("homeassistant.summary")}</h3>
      <ul className="ha-entities">
        {summary.domains.map(({ domain, active, total }) => (
          <li key={domain}>
            <span>{(DOMAINS as readonly string[]).includes(domain) ? t(`homeassistant.domain.${domain as Domain}`) : domain}</span>
            <span>
              {integer.format(active)}/{integer.format(total)}
            </span>
          </li>
        ))}
        <li>
          <span>{t("homeassistant.total.entities")}</span>
          <span>{integer.format(summary.entities)}</span>
        </li>
        <li className={summary.unavailable > 0 ? "no-data" : undefined}>
          <span>{t("homeassistant.total.unavailable")}</span>
          <span>{integer.format(summary.unavailable)}</span>
        </li>
        <li className={summary.updates > 0 ? "no-data" : undefined}>
          <span>{t("homeassistant.total.updates")}</span>
          <span>{integer.format(summary.updates)}</span>
        </li>
        <li>
          <span>{t("homeassistant.total.integrations")}</span>
          <span>{integer.format(summary.integrations)}</span>
        </li>
      </ul>
    </div>
  );
}

function HomeAssistant({ data }: { data: HomeAssistantData }) {
  const { t } = useI18n();
  const value = useEntityValue();
  const consumption = data.entities.filter((entity) => CONSUMPTION.includes(entity.device_class ?? ""));
  const others = data.entities.filter((entity) => !CONSUMPTION.includes(entity.device_class ?? ""));
  return (
    <>
      {data.system && <System system={data.system} />}
      {data.summary && <Summary summary={data.summary} />}
      {consumption.length > 0 && (
        <div className="ha-section">
          <h3>{t("homeassistant.consumption")}</h3>
          <dl className="figures compact">
            {consumption.map((entity) => (
              <div key={entity.entity_id}>
                <dt>{entity.name}</dt>
                <dd className={hasNoData(entity) ? "no-data" : undefined}>{value(entity)}</dd>
              </div>
            ))}
          </dl>
        </div>
      )}
      {others.length > 0 && (
        <div className="ha-section">
          <h3>{t("homeassistant.entities")}</h3>
          <ul className="ha-entities">
            {others.map((entity) => (
              <li key={entity.entity_id} className={hasNoData(entity) ? "no-data" : undefined}>
                <span>{entity.name}</span>
                <span>{value(entity)}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </>
  );
}

export const homeAssistantWidget: Widget<HomeAssistantData> = {
  state: (data, t) => {
    if (data.state !== "RUNNING") return { tone: "warn", label: t("homeassistant.starting") };
    if (data.recovery_mode) return { tone: "bad", label: t("homeassistant.recovery") };
    if (data.safe_mode) return { tone: "warn", label: t("homeassistant.safeMode") };
    const missing = data.entities.filter(hasNoData).length;
    if (missing > 0) return { tone: "warn", label: t("homeassistant.unavailable", { n: missing }) };
    return { tone: "good", label: t("homeassistant.running") };
  },
  Body: HomeAssistant,
  footer: (data) => (data.version ? `Home Assistant ${data.version}` : ""),
};
