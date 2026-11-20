import type { AvailabilityData } from "../api";
import { useI18n } from "../i18n";
import type { Widget } from "./types";

function Availability({ data }: { data: AvailabilityData }) {
  const { t, lang } = useI18n();
  const ms = new Intl.NumberFormat(lang, { maximumFractionDigits: 0 });
  return (
    <dl className="figures compact">
      <div>
        <dt>{t("availability.latency")}</dt>
        <dd>{t("availability.ms", { ms: ms.format(data.latency_ms) })}</dd>
      </div>
    </dl>
  );
}

// El pie dice cómo se comprueba: «TCP» o «HTTP 200», nunca la dirección.
export const availabilityWidget: Widget<AvailabilityData> = {
  Body: Availability,
  footer: (data) => (data.check === "http" ? `HTTP ${data.http_status ?? ""}`.trim() : "TCP"),
};
