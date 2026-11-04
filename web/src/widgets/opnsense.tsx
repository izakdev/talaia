import type { OPNsenseData, OPNsenseGateway } from "../api";
import { useI18n } from "../i18n";
import { formatBytes } from "./homeassistant";
import type { Widget } from "./types";

function formatBits(value: number, format: Intl.NumberFormat) {
  const units = ["bit/s", "kbit/s", "Mbit/s", "Gbit/s"];
  let size = value;
  let unit = 0;
  while (size >= 1000 && unit < units.length - 1) {
    size /= 1000;
    unit++;
  }
  return `${format.format(size)} ${units[unit]}`;
}

function GatewayRow({ gateway }: { gateway: OPNsenseGateway }) {
  const { t, lang } = useI18n();
  const decimal = new Intl.NumberFormat(lang, { maximumFractionDigits: 1 });
  const details = [t(`opnsense.gateway.${gateway.state}`)];
  if (gateway.delay_ms !== undefined) details.push(t("opnsense.gateway.delay", { delay: decimal.format(gateway.delay_ms) }));
  if (gateway.loss_percent) details.push(t("opnsense.gateway.loss", { loss: decimal.format(gateway.loss_percent) }));
  return (
    <li className={gateway.state === "unmonitored" ? "no-data" : undefined}>
      <span>{gateway.name}</span>
      <span>{details.join(", ")}</span>
    </li>
  );
}

function OPNsense({ data }: { data: OPNsenseData }) {
  const { t, lang } = useI18n();
  const percent = new Intl.NumberFormat(lang, { maximumFractionDigits: 0 });
  const decimal = new Intl.NumberFormat(lang, { maximumFractionDigits: 1 });
  const age = new Intl.RelativeTimeFormat(lang, { numeric: "auto" }).format(-Math.floor(data.uptime_seconds / 86400), "day");
  const speed = (bits?: number) => (bits === undefined ? t("opnsense.measuring") : formatBits(bits, decimal));
  const usage = (used: number, total: number) => (
    <>
      {percent.format((used / total) * 100)} %
      <small>{t("opnsense.of", { used: formatBytes(used, decimal), total: formatBytes(total, decimal) })}</small>
    </>
  );
  return (
    <>
      <dl className="figures compact">
        <div>
          <dt>{t("opnsense.cpu")}</dt>
          <dd>{percent.format(data.cpu_percent)} %</dd>
        </div>
        {!!data.memory_total && !!data.memory_used && (
          <div>
            <dt>{t("opnsense.memory")}</dt>
            <dd>{usage(data.memory_used, data.memory_total)}</dd>
          </div>
        )}
        {!!data.disk_total && !!data.disk_used && (
          <div>
            <dt>{t("opnsense.disk")}</dt>
            <dd>{usage(data.disk_used, data.disk_total)}</dd>
          </div>
        )}
      </dl>
      {data.uptime_seconds > 0 && <p className="service-foot">{t("opnsense.booted", { age })}</p>}
      {data.gateways.length > 0 && (
        <div className="ha-section">
          <h3>{t("opnsense.gateways")}</h3>
          <ul className="ha-entities gateways">
            {data.gateways.map((gateway) => (
              <GatewayRow key={gateway.name} gateway={gateway} />
            ))}
          </ul>
        </div>
      )}
      {data.wan && (
        <div className="ha-section">
          <h3>{t("opnsense.wan")}</h3>
          <dl className="figures compact">
            <div>
              <dt>{t("opnsense.download")}</dt>
              <dd>
                {speed(data.wan.receive_bits_per_second)}
                <small>{t("opnsense.total", { total: formatBytes(data.wan.received_bytes, decimal) })}</small>
              </dd>
            </div>
            <div>
              <dt>{t("opnsense.upload")}</dt>
              <dd>
                {speed(data.wan.transmit_bits_per_second)}
                <small>{t("opnsense.total", { total: formatBytes(data.wan.sent_bytes, decimal) })}</small>
              </dd>
            </div>
          </dl>
        </div>
      )}
    </>
  );
}

export const opnsenseWidget: Widget<OPNsenseData> = {
  state: (data, t) => {
    const states = data.gateways.map((gateway) => gateway.state);
    if (states.length === 0) return { tone: "muted", label: t("opnsense.noGateways") };
    if (states.includes("offline")) return { tone: "bad", label: t("opnsense.offline") };
    if (states.includes("degraded")) return { tone: "warn", label: t("opnsense.degraded") };
    // Un gateway sin monitorizar siempre figura como correcto: no demuestra que haya conexión.
    if (!states.includes("online")) return { tone: "muted", label: t("opnsense.unmonitored") };
    return { tone: "good", label: t("opnsense.online") };
  },
  Body: OPNsense,
  footer: (data) => (data.version ? `OPNsense ${data.version}` : ""),
};
