import type { PrinterData, PrinterMarker, PrinterReason } from "../api";
import { useI18n, type Key, type Translate } from "../i18n";
import type { Widget } from "./types";

// Los cuatro colores de proceso se nombran; cualquier otro consumible usa su nombre.
const COLOR_NAMES: Record<string, Key> = {
  "#000000": "printer.color.black",
  "#00FFFF": "printer.color.cyan",
  "#FF00FF": "printer.color.magenta",
  "#FFFF00": "printer.color.yellow",
};

// Misma agrupación que las reglas del servidor (internal/integrations/printer/state.go).
type StateKey = Extract<Key, `printer.state.${string}`>;
const REASON_KEYS: Record<string, StateKey> = {
  "media-jam": "printer.state.jam",
  "door-open": "printer.state.doorOpen",
  "cover-open": "printer.state.doorOpen",
  "interlock-open": "printer.state.doorOpen",
  "media-empty": "printer.state.mediaEmpty",
  "media-needed": "printer.state.mediaEmpty",
  "toner-empty": "printer.state.tonerEmpty",
  "marker-supply-empty": "printer.state.tonerEmpty",
  "developer-empty": "printer.state.tonerEmpty",
  "marker-waste-full": "printer.state.wasteFull",
  "input-tray-missing": "printer.state.trayMissing",
  "output-tray-missing": "printer.state.trayMissing",
  "output-area-full": "printer.state.outputFull",
  "opc-life-over": "printer.state.drumEnd",
  "toner-low": "printer.state.tonerLow",
  "marker-supply-low": "printer.state.tonerLow",
  "developer-low": "printer.state.tonerLow",
  "marker-waste-almost-full": "printer.state.wasteAlmostFull",
  "opc-near-eol": "printer.state.drumNearEnd",
  "media-low": "printer.state.mediaLow",
  "output-area-almost-full": "printer.state.outputAlmostFull",
  "fuser-over-temp": "printer.state.fuser",
  paused: "printer.state.stopped",
  shutdown: "printer.state.stopped",
  "stopped-partly": "printer.state.stopped",
};

export function markerLabel(marker: PrinterMarker, t: Translate): string {
  const key = COLOR_NAMES[marker.color];
  return key ? t(key) : marker.name;
}

export function reasonLabel(reason: PrinterReason, t: Translate): string {
  return t(REASON_KEYS[reason.reason] ?? "printer.state.warning");
}

// El negro se pinta con el color del texto, para que se vea también en el tema oscuro.
const fillColor = (color: string) => (color === "" ? undefined : color === "#000000" ? "var(--ink)" : color);

function Printer({ data }: { data: PrinterData }) {
  const { t, lang } = useI18n();
  const percent = new Intl.NumberFormat(lang, { maximumFractionDigits: 0 });
  const reasons = [...new Set(data.reasons.map((r) => reasonLabel(r, t)))];
  return (
    <>
      {data.markers.length > 0 && (
        <div className="printer-markers">
          {data.markers.map((marker, i) => {
            const low = marker.level != null && marker.level <= marker.low;
            return (
              <div key={i} className={low ? "meter high" : "meter"}>
                <span className="meter-label">
                  {markerLabel(marker, t)}
                  <b>{marker.level == null ? t("printer.unknownLevel") : `${percent.format(marker.level)} %`}</b>
                </span>
                <span className="meter-track" aria-hidden="true">
                  <span className="meter-fill printer-fill" style={{ width: `${marker.level ?? 0}%`, background: fillColor(marker.color) }} />
                </span>
              </div>
            );
          })}
        </div>
      )}
      {(reasons.length > 0 || !data.accepting_jobs) && (
        <div className="ha-section">
          <h3>{t("printer.alerts")}</h3>
          <ul className="printer-alerts">
            {reasons.map((label) => (
              <li key={label}>{label}</li>
            ))}
            {!data.accepting_jobs && <li>{t("printer.notAccepting")}</li>}
          </ul>
        </div>
      )}
    </>
  );
}

// El pie dice el modelo y el firmware, nunca el nombre ni la dirección.
export const printerWidget: Widget<PrinterData> = {
  Body: Printer,
  footer: (data) => [data.model, data.firmware && `${data.firmware}`].filter(Boolean).join(" · "),
};
