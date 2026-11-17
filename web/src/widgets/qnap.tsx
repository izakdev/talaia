import type { QNAPData, QNAPDisk, QNAPTemperature, QNAPVolume } from "../api";
import { useI18n, type Translate } from "../i18n";
import { formatBytes } from "./homeassistant";
import type { Tone, Widget } from "./types";

// Un volumen por encima de este uso se marca en ámbar.
const FULL_PERCENT = 90;

const used = (volume: QNAPVolume) => volume.total_bytes - volume.free_bytes;
const fill = (volume: QNAPVolume) => (volume.total_bytes > 0 ? (used(volume) / volume.total_bytes) * 100 : 0);

function tempTone(c: number | null | undefined, warn: number, err: number): Tone {
  if (c == null) return "good";
  if (c >= err) return "bad";
  if (c >= warn) return "warn";
  return "good";
}

const temperatureTone = (temp?: QNAPTemperature): Tone => (temp ? tempTone(temp.c, temp.warn_c, temp.err_c) : "good");
const diskTempTone = (data: QNAPData, disk: QNAPDisk) => tempTone(disk.temp_c, data.disk_temp_warn_c, data.disk_temp_err_c);

function diskTone(data: QNAPData, disk: QNAPDisk): Tone {
  if (disk.health === "error" || diskTempTone(data, disk) === "bad") return "bad";
  if (disk.health === "warning" || diskTempTone(data, disk) === "warn") return "warn";
  return "good";
}

function volumeTone(volume: QNAPVolume): Tone {
  if (volume.status === "degraded" || volume.status === "error") return "bad";
  if (volume.status !== "ready" || fill(volume) >= FULL_PERCENT) return "warn";
  return "good";
}

type Key = Parameters<Translate>[0];

// El primer problema de la lista da la etiqueta; el orden es el de gravedad.
function state(data: QNAPData, t: Translate): { tone: Tone; label: string } {
  const temps = [temperatureTone(data.cpu_temperature), temperatureTone(data.system_temperature)];
  const diskTemps = data.disks.map((disk) => diskTempTone(data, disk));
  const checks: [boolean, Tone, Key][] = [
    [data.disks.some((disk) => disk.health === "error"), "bad", "qnap.state.diskError"],
    [data.storage === "degraded" || data.volumes.some((v) => v.status === "degraded"), "bad", "qnap.state.degraded"],
    [data.storage === "error" || data.volumes.some((v) => v.status === "error"), "bad", "qnap.state.storageError"],
    [[...temps, ...diskTemps].includes("bad"), "bad", "qnap.state.hot"],
    [data.disks.some((disk) => disk.health === "warning"), "warn", "qnap.state.smartWarning"],
    [data.storage === "syncing" || data.volumes.some((v) => v.status === "syncing"), "warn", "qnap.state.syncing"],
    [data.storage === "warning" || data.volumes.some((v) => v.status === "warning"), "warn", "qnap.state.storageWarning"],
    [data.volumes.some((v) => fill(v) >= FULL_PERCENT), "warn", "qnap.state.full"],
    [[...temps, ...diskTemps].includes("warn"), "warn", "qnap.state.warm"],
    [data.fan_failed, "warn", "qnap.state.fan"],
  ];
  const problem = checks.find(([hit]) => hit);
  if (problem) return { tone: problem[1], label: t(problem[2]) };
  if (data.disks.length === 0) return { tone: "muted", label: t("qnap.state.noDisks") };
  return { tone: "good", label: t("qnap.state.ok") };
}

const rowClass = (tone: Tone) => (tone === "bad" ? "alarm" : tone === "warn" ? "no-data" : undefined);

function QNAP({ data }: { data: QNAPData }) {
  const { t, lang } = useI18n();
  const percent = new Intl.NumberFormat(lang, { maximumFractionDigits: 0 });
  const decimal = new Intl.NumberFormat(lang, { maximumFractionDigits: 1 });
  const usage = (part: number, total: number) => (
    <>
      {percent.format((part / total) * 100)} %
      <small>{t("qnap.of", { used: formatBytes(part, decimal), total: formatBytes(total, decimal) })}</small>
    </>
  );
  const storageTotal = data.volumes.reduce((sum, volume) => sum + volume.total_bytes, 0);
  const storageUsed = data.volumes.reduce((sum, volume) => sum + used(volume), 0);
  const temps = [
    data.cpu_temperature && t("qnap.cpuTemp", { c: data.cpu_temperature.c }),
    data.system_temperature && t("qnap.systemTemp", { c: data.system_temperature.c }),
    data.fan_failed && t("qnap.fanFailed"),
  ].filter(Boolean);
  return (
    <>
      <dl className="figures compact">
        <div>
          <dt>{t("qnap.cpu")}</dt>
          <dd>{percent.format(data.cpu_percent)} %</dd>
        </div>
        {data.memory_total_bytes > 0 && (
          <div>
            <dt>{t("qnap.memory")}</dt>
            <dd>{usage(data.memory_used_bytes, data.memory_total_bytes)}</dd>
          </div>
        )}
        {storageTotal > 0 && (
          <div>
            <dt>{t("qnap.storage")}</dt>
            <dd>{usage(storageUsed, storageTotal)}</dd>
          </div>
        )}
      </dl>
      {data.disks.length > 0 && (
        <div className="ha-section">
          <h3>{t("qnap.disks")}</h3>
          <ul className="ha-entities gateways">
            {data.disks.map((disk) => {
              const details = [t(`qnap.health.${disk.health}`)];
              if (disk.temp_c != null) details.push(`${disk.temp_c} °C`);
              return (
                <li
                  key={`${disk.external}-${disk.number}`}
                  // Sin datos SMART no es un problema: la fila va en gris.
                  className={disk.health === "unknown" ? "unknown" : rowClass(diskTone(data, disk))}
                >
                  <span>{t(disk.external ? "qnap.usbDisk" : "qnap.disk", { n: disk.number })}</span>
                  <span>{details.join(" · ")}</span>
                </li>
              );
            })}
          </ul>
        </div>
      )}
      {data.volumes.length > 0 && (
        <div className="ha-section">
          <h3>{t("qnap.volumes")}</h3>
          <ul className="ha-entities gateways">
            {data.volumes.map((volume) => (
              <li key={volume.number} className={rowClass(volumeTone(volume))}>
                <span>{t("qnap.volume", { n: volume.number })}</span>
                <span>
                  {[
                    t(`qnap.status.${volume.status}`),
                    t("qnap.volumeUse", {
                      percent: percent.format(fill(volume)),
                      total: formatBytes(volume.total_bytes, decimal),
                    }),
                  ].join(" · ")}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
      {temps.length > 0 && <p className="service-foot">{temps.join(" · ")}</p>}
    </>
  );
}

export const qnapWidget: Widget<QNAPData> = {
  state,
  Body: QNAP,
  footer: (data) => (data.version ? `QNAP ${data.version}` : "QNAP"),
};
