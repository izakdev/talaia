import { siAdguard, siHomeassistant, siOpnsense, siPortainer, siProxmox, siQnap, siWireguard } from "simple-icons";
import type { ConnectionType } from "./api";

export type ConnectionOption = "show_top_domains" | "show_guests" | "show_summary" | "show_wireguard";

export interface CatalogEntry {
  type: ConnectionType;
  icon: { path: string; hex: string; title: string };
  urlPlaceholder: string;
  // Indica si se eligen entidades concretas para mostrarlas en el panel.
  hasEntities: boolean;
  options: ConnectionOption[];
  // Opciones activas al crear una conexión.
  defaultOptions: Partial<Record<ConnectionOption, boolean>>;
}

// simple-icons no tiene NUT: una batería con el mismo formato (24 × 24).
const nutIcon = {
  title: "Network UPS Tools",
  hex: "16A34A",
  path: "M2 7h16a1 1 0 0 1 1 1v2h2v4h-2v2a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V8a1 1 0 0 1 1-1Zm1 2v6h14V9H3Zm1 1h7v4H4v-4Z",
};

export const CATALOG: CatalogEntry[] = [
  { type: "adguard", icon: siAdguard, urlPlaceholder: "http://adguard.local:3000", hasEntities: false, options: ["show_top_domains"], defaultOptions: {} },
  { type: "homeassistant", icon: siHomeassistant, urlPlaceholder: "http://homeassistant.local:8123", hasEntities: true, options: ["show_summary"], defaultOptions: { show_summary: true } },
  { type: "proxmox", icon: siProxmox, urlPlaceholder: "https://proxmox.example:8006", hasEntities: false, options: ["show_guests"], defaultOptions: { show_guests: true } },
  { type: "opnsense", icon: siOpnsense, urlPlaceholder: "https://opnsense.local", hasEntities: false, options: ["show_wireguard"], defaultOptions: {} },
  { type: "portainer", icon: siPortainer, urlPlaceholder: "https://portainer.local:9443", hasEntities: false, options: [], defaultOptions: {} },
  { type: "qnap", icon: siQnap, urlPlaceholder: "snmp://qnap.local", hasEntities: false, options: [], defaultOptions: {} },
  { type: "nut", icon: nutIcon, urlPlaceholder: "nut://sai.local", hasEntities: false, options: [], defaultOptions: {} },
  { type: "wgdashboard", icon: siWireguard, urlPlaceholder: "http://wgdashboard.local:10086", hasEntities: false, options: [], defaultOptions: {} },
];

export function catalogEntry(type: ConnectionType): CatalogEntry {
  return CATALOG.find((entry) => entry.type === type)!;
}
