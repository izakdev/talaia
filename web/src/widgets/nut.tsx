import type { NUTData } from "../api";
import { useI18n, type Translate } from "../i18n";
import type { Widget } from "./types";

// La autonomía viene en segundos: «40 min» o «1 h 5 min».
export function formatRuntime(seconds: number, t: Translate): string {
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return t("nut.minutes", { m: minutes });
  return t("nut.hours", { h: Math.floor(minutes / 60), m: minutes % 60 });
}

function NUT({ data }: { data: NUTData }) {
  const { t, lang } = useI18n();
  const percent = new Intl.NumberFormat(lang, { maximumFractionDigits: 0 });
  if (data.charge_percent == null && data.runtime_seconds == null) return null;
  return (
    <dl className="figures compact">
      {data.charge_percent != null && (
        <div>
          <dt>{t("nut.charge")}</dt>
          <dd>
            {percent.format(data.charge_percent)} %
            {data.status.includes("CHRG") && <small>{t("nut.charging")}</small>}
          </dd>
        </div>
      )}
      {data.runtime_seconds != null && (
        <div>
          <dt>{t("nut.runtime")}</dt>
          <dd>{formatRuntime(data.runtime_seconds, t)}</dd>
        </div>
      )}
    </dl>
  );
}

export const nutWidget: Widget<NUTData> = {
  Body: NUT,
  footer: (data) => [data.model, data.version ? `NUT ${data.version}` : "NUT"].filter(Boolean).join(" · "),
};
