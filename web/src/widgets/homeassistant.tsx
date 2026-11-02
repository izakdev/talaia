import type { EntityOption, HomeAssistantData } from "../api";
import { useI18n, type Translate } from "../i18n";
import type { Widget } from "./types";

const CONSUMPTION = ["power", "energy"];
const NO_DATA = ["unavailable", "unknown", "missing"];

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

function HomeAssistant({ data }: { data: HomeAssistantData }) {
  const { t } = useI18n();
  const value = useEntityValue();
  const consumption = data.entities.filter((entity) => CONSUMPTION.includes(entity.device_class ?? ""));
  const others = data.entities.filter((entity) => !CONSUMPTION.includes(entity.device_class ?? ""));
  return (
    <>
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
