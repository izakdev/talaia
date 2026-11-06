import { siAdguard, siHomeassistant, siOpnsense, siPortainer, siProxmox, siWireguard } from "simple-icons";
import type { ConnectionType } from "./api";

export type ConnectionOption = "show_top_domains" | "show_guests" | "show_summary";

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

export const CATALOG: CatalogEntry[] = [
  { type: "adguard", icon: siAdguard, urlPlaceholder: "http://adguard.local:3000", hasEntities: false, options: ["show_top_domains"], defaultOptions: {} },
  { type: "homeassistant", icon: siHomeassistant, urlPlaceholder: "http://homeassistant.local:8123", hasEntities: true, options: ["show_summary"], defaultOptions: { show_summary: true } },
  { type: "proxmox", icon: siProxmox, urlPlaceholder: "https://proxmox.example:8006", hasEntities: false, options: ["show_guests"], defaultOptions: { show_guests: true } },
  { type: "opnsense", icon: siOpnsense, urlPlaceholder: "https://opnsense.local", hasEntities: false, options: [], defaultOptions: {} },
  { type: "portainer", icon: siPortainer, urlPlaceholder: "https://portainer.local:9443", hasEntities: false, options: [], defaultOptions: {} },
  { type: "wgdashboard", icon: siWireguard, urlPlaceholder: "http://wgdashboard.local:10086", hasEntities: false, options: [], defaultOptions: {} },
];

export function catalogEntry(type: ConnectionType): CatalogEntry {
  return CATALOG.find((entry) => entry.type === type)!;
}
