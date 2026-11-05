import type { PortainerData, PortainerEnvironment } from "../api";
import { useI18n } from "../i18n";
import type { Widget } from "./types";

function EnvironmentRow({ environment }: { environment: PortainerEnvironment }) {
  const { t } = useI18n();
  const { containers, online } = environment;
  const details = [t(online ? "portainer.online" : "portainer.offline")];
  if (online && containers) {
    details.push(t("portainer.running", { running: containers.running, total: containers.total }));
    if (containers.unhealthy > 0) details.push(t("portainer.unhealthy", { count: containers.unhealthy }));
  }
  return (
    <li className={!online || containers?.unhealthy ? "no-data" : undefined}>
      <span>{environment.name}</span>
      <span>{details.join(", ")}</span>
    </li>
  );
}

function Portainer({ data }: { data: PortainerData }) {
  const { t, lang } = useI18n();
  const live = data.environments.filter((e) => e.online);
  const counts = live.flatMap((e) => (e.containers ? [e.containers] : []));
  const sum = (key: "running" | "total" | "stacks") => counts.reduce((total, c) => total + c[key], 0);
  const oldest = counts.length ? Math.min(...counts.map((c) => c.snapshot_at)) : 0;
  const minutes = Math.max(0, Math.round((Date.now() / 1000 - oldest) / 60));
  const age = new Intl.RelativeTimeFormat(lang, { numeric: "auto" }).format(-minutes, "minute");
  return (
    <>
      <dl className="figures compact">
        <div>
          <dt>{t("portainer.environments")}</dt>
          <dd>
            {live.length}/{data.environments.length}
          </dd>
        </div>
        <div>
          <dt>{t("portainer.containers")}</dt>
          <dd>
            {sum("running")}/{sum("total")}
          </dd>
        </div>
        <div>
          <dt>{t("portainer.stacks")}</dt>
          <dd>{sum("stacks")}</dd>
        </div>
      </dl>
      {data.environments.length > 0 && (
        <div className="ha-section">
          <h3>{t("portainer.environments")}</h3>
          <ul className="ha-entities gateways">
            {data.environments.map((environment) => (
              <EnvironmentRow key={environment.name} environment={environment} />
            ))}
          </ul>
        </div>
      )}
      {oldest > 0 && <p className="service-foot">{t("portainer.snapshot", { age })}</p>}
    </>
  );
}

export const portainerWidget: Widget<PortainerData> = {
  state: (data, t) => {
    if (data.environments.length === 0) return { tone: "muted", label: t("portainer.none") };
    if (data.environments.some((e) => !e.online)) return { tone: "bad", label: t("portainer.down") };
    if (data.environments.some((e) => e.containers?.unhealthy)) return { tone: "warn", label: t("portainer.problems") };
    return { tone: "good", label: t("portainer.allOnline") };
  },
  Body: Portainer,
  footer: (data) => `Portainer ${data.version}`,
};
