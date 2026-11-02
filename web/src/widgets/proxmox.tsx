import type { ProxmoxData, ProxmoxGuest, ProxmoxNode } from "../api";
import { useI18n } from "../i18n";
import type { Widget } from "./types";

const HIGH_USAGE = 0.9;

function useFormats() {
  const { lang } = useI18n();
  const percent = new Intl.NumberFormat(lang, { style: "percent", maximumFractionDigits: 0 });
  const decimal = new Intl.NumberFormat(lang, { maximumFractionDigits: 1 });
  const bytes = (value: number) => {
    const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
    let size = value;
    let unit = 0;
    while (size >= 1024 && unit < units.length - 1) {
      size /= 1024;
      unit++;
    }
    return `${decimal.format(size)} ${units[unit]}`;
  };
  return { percent, bytes };
}

const ratio = (used: number, total: number) => (total > 0 ? Math.min(used / total, 1) : 0);

function Meter({ label, value, detail }: { label: string; value: number; detail: string }) {
  const { percent } = useFormats();
  return (
    <div className={value >= HIGH_USAGE ? "meter high" : "meter"} title={detail}>
      <span className="meter-label">
        {label}
        <b>{percent.format(value)}</b>
      </span>
      <span className="meter-track" aria-hidden="true">
        <span className="meter-fill" style={{ width: `${value * 100}%` }} />
      </span>
    </div>
  );
}

function Proxmox({ data }: { data: ProxmoxData }) {
  const { t } = useI18n();
  const { percent, bytes } = useFormats();
  return (
    <>
      <dl className="figures compact">
        <div>
          <dt>{t("proxmox.nodes")}</dt>
          <dd>
            {data.nodes_online}/{data.nodes_total}
          </dd>
        </div>
        <div>
          <dt>{t("proxmox.vms")}</dt>
          <dd>
            {data.vms.running}/{data.vms.total}
          </dd>
        </div>
        <div>
          <dt>{t("proxmox.containers")}</dt>
          <dd>
            {data.containers.running}/{data.containers.total}
          </dd>
        </div>
        <div>
          <dt>{t("proxmox.cpu")}</dt>
          <dd>
            {percent.format(data.cpu)}
            <small>{t("proxmox.cores", { n: data.max_cpu })}</small>
          </dd>
        </div>
        <div>
          <dt>{t("proxmox.ram")}</dt>
          <dd>
            {percent.format(ratio(data.mem_used, data.mem_total))}
            <small>{t("proxmox.of", { used: bytes(data.mem_used), total: bytes(data.mem_total) })}</small>
          </dd>
        </div>
      </dl>
      <ul className="nodes">
        {data.nodes.map((node) => (
          <li key={node.name}>
            <NodeBlock node={node} />
          </li>
        ))}
      </ul>
    </>
  );
}

function NodeBlock({ node }: { node: ProxmoxNode }) {
  const { t } = useI18n();
  const { bytes } = useFormats();
  const row = (
    <span className="node-row">
      <span className="node-name">{node.name}</span>
      {node.online ? (
        <>
          <Meter label={t("proxmox.cpu")} value={node.cpu} detail={t("proxmox.cores", { n: node.max_cpu })} />
          <Meter
            label={t("proxmox.ram")}
            value={ratio(node.mem_used, node.mem_total)}
            detail={t("proxmox.of", { used: bytes(node.mem_used), total: bytes(node.mem_total) })}
          />
          <Meter
            label={t("proxmox.disk")}
            value={ratio(node.disk_used, node.disk_total)}
            detail={t("proxmox.of", { used: bytes(node.disk_used), total: bytes(node.disk_total) })}
          />
        </>
      ) : (
        <span className="node-offline">{t("proxmox.offline")}</span>
      )}
      <span className="node-summary">
        {t("proxmox.summary", {
          vms: node.vms.total,
          containers: node.containers.total,
          running: node.vms.running + node.containers.running,
        })}
      </span>
    </span>
  );
  if (!node.guests) return row;
  return (
    <details>
      <summary>{row}</summary>
      {node.guests.length === 0 ? <p className="no-guests">{t("proxmox.noGuests")}</p> : <Guests guests={node.guests} />}
    </details>
  );
}

function Guests({ guests }: { guests: ProxmoxGuest[] }) {
  const { t } = useI18n();
  const { percent, bytes } = useFormats();
  const statusLabel = (status: string) =>
    status === "running" || status === "stopped" || status === "paused"
      ? t(`proxmox.status.${status}`)
      : t("proxmox.status.unknown");
  return (
    <div className="guests-wrap">
      <table className="guests">
        <thead>
          <tr>
            <th scope="col">{t("proxmox.guest")}</th>
            <th scope="col">{t("proxmox.type")}</th>
            <th scope="col">{t("proxmox.status")}</th>
            <th scope="col">{t("proxmox.cpu")}</th>
            <th scope="col">{t("proxmox.ram")}</th>
          </tr>
        </thead>
        <tbody>
          {guests.map((guest) => {
            const running = guest.status === "running";
            return (
              <tr key={guest.vmid} className={running ? undefined : "stopped"}>
                <td>
                  {guest.name || guest.vmid} <span className="guest-id">{guest.name ? guest.vmid : ""}</span>
                </td>
                <td>{guest.type === "qemu" ? "VM" : "LXC"}</td>
                <td>{statusLabel(guest.status)}</td>
                <td>{running ? percent.format(guest.cpu) : ""}</td>
                <td title={running ? t("proxmox.of", { used: bytes(guest.mem_used), total: bytes(guest.mem_total) }) : undefined}>
                  {running ? percent.format(ratio(guest.mem_used, guest.mem_total)) : ""}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

export const proxmoxWidget: Widget<ProxmoxData> = {
  state: (data, t) => {
    if (data.quorate === false) return { tone: "bad", label: t("proxmox.noQuorum") };
    if (data.nodes_online < data.nodes_total) {
      return { tone: "warn", label: t("proxmox.nodesDown", { online: data.nodes_online, total: data.nodes_total }) };
    }
    return { tone: "good", label: t(data.cluster_name ? "proxmox.clusterOk" : "proxmox.nodeOk") };
  },
  Body: Proxmox,
  footer: (data) => (data.version ? `Proxmox VE ${data.version}` : ""),
};
