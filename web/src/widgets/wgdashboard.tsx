import type { WGDashboardData, WGDashboardTunnel } from "../api";
import { useI18n } from "../i18n";
import { formatBytes } from "./homeassistant";
import type { Widget } from "./types";

function TunnelRow({ tunnel }: { tunnel: WGDashboardTunnel }) {
  const { t } = useI18n();
  const details = [t(tunnel.up ? "wgdashboard.up" : "wgdashboard.stopped")];
  if (tunnel.up) {
    details.push(t("wgdashboard.connected", { connected: tunnel.connected_peers, total: tunnel.total_peers }));
  }
  return (
    <li className={tunnel.up ? undefined : "no-data"}>
      <span>{tunnel.name}</span>
      <span>{details.join(", ")}</span>
    </li>
  );
}

function WGDashboard({ data }: { data: WGDashboardData }) {
  const { t, lang } = useI18n();
  const decimal = new Intl.NumberFormat(lang, { maximumFractionDigits: 1 });
  const up = data.tunnels.filter((tunnel) => tunnel.up);
  const sum = (key: "connected_peers" | "total_peers" | "received_bytes" | "sent_bytes", tunnels = data.tunnels) =>
    tunnels.reduce((total, tunnel) => total + tunnel[key], 0);
  const received = sum("received_bytes");
  const sent = sum("sent_bytes");
  return (
    <>
      <dl className="figures compact">
        <div>
          <dt>{t("wgdashboard.tunnels")}</dt>
          <dd>
            {up.length}/{data.tunnels.length}
          </dd>
        </div>
        <div>
          <dt>{t("wgdashboard.peers")}</dt>
          <dd>
            {sum("connected_peers", up)}/{sum("total_peers")}
          </dd>
        </div>
        <div>
          <dt>{t("wgdashboard.traffic")}</dt>
          <dd>{formatBytes(received + sent, decimal)}</dd>
        </div>
      </dl>
      {data.tunnels.length > 0 && (
        <div className="ha-section">
          <h3>{t("wgdashboard.tunnels")}</h3>
          <ul className="ha-entities gateways">
            {data.tunnels.map((tunnel) => (
              <TunnelRow key={tunnel.name} tunnel={tunnel} />
            ))}
          </ul>
        </div>
      )}
      {received + sent > 0 && (
        <p className="service-foot">
          {t("wgdashboard.trafficFoot", { received: formatBytes(received, decimal), sent: formatBytes(sent, decimal) })}
        </p>
      )}
    </>
  );
}

export const wgdashboardWidget: Widget<WGDashboardData> = {
  state: (data, t) => {
    if (data.tunnels.length === 0) return { tone: "muted", label: t("wgdashboard.none") };
    if (data.tunnels.some((tunnel) => !tunnel.up)) return { tone: "warn", label: t("wgdashboard.down") };
    return { tone: "good", label: t("wgdashboard.allUp") };
  },
  Body: WGDashboard,
  footer: (data) => `WGDashboard ${data.version}`,
};
