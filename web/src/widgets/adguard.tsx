import type { AdGuardData, DomainCount } from "../api";
import { useI18n } from "../i18n";
import type { Widget } from "./types";

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

export const adguardWidget: Widget<AdGuardData> = {
  state: (data, t) => {
    if (!data.running) return { tone: "bad", label: t("adguard.dnsStopped") };
    return data.protection_enabled
      ? { tone: "good", label: t("adguard.protectionOn") }
      : { tone: "warn", label: t("adguard.protectionOff") };
  },
  Body: AdGuard,
  footer: (data) => (data.version ? `AdGuard Home ${data.version}` : ""),
};
