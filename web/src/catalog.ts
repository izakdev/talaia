import { siAdguard, siHomeassistant, siOpnsense, siPortainer, siProxmox, siQnap, siWireguard } from "simple-icons";
import type { ConnectionType } from "./api";

export type ConnectionOption = "show_top_domains" | "show_guests" | "show_summary" | "show_wireguard";

export interface CatalogEntry {
  type: ConnectionType;
  icon: { path: string; hex: string; title: string };
  // Nombre del mismo icono en /icons/, el que llevan sus bloques si no se elige otro.
  iconSlug: string;
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

// Un pulso, con el mismo formato (24 × 24): cualquier equipo que responda.
const availabilityIcon = {
  title: "Disponibilidad",
  hex: "0EA5E9",
  path: "M1 11h5.4l2.7-6.3a1 1 0 0 1 1.85.06L15 16.3l1.6-4.7a1 1 0 0 1 .95-.6H23v2h-4.73l-2.32 6.82a1 1 0 0 1-1.89.03L10 7.9l-2.08 4.9A1 1 0 0 1 7 13.4H1v-2.4Z",
};

// Una impresora con su hoja, con el mismo formato (24 × 24): cualquier marca con IPP.
const printerIcon = {
  title: "Impresora",
  hex: "6366F1",
  path: "M6 2h12v5H6V2Zm-3 6h18a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2h-2v-4H5v4H3a2 2 0 0 1-2-2v-6a2 2 0 0 1 2-2Zm15 2.5a1 1 0 1 0 0 2 1 1 0 1 0 0-2ZM7 16h10v6H7v-6Z",
};

export const CATALOG: CatalogEntry[] = [
  { type: "adguard", iconSlug: "adguard", icon: siAdguard, urlPlaceholder: "http://adguard.local:3000", hasEntities: false, options: ["show_top_domains"], defaultOptions: {} },
  { type: "homeassistant", iconSlug: "homeassistant", icon: siHomeassistant, urlPlaceholder: "http://homeassistant.local:8123", hasEntities: true, options: ["show_summary"], defaultOptions: { show_summary: true } },
  { type: "proxmox", iconSlug: "proxmox", icon: siProxmox, urlPlaceholder: "https://proxmox.example:8006", hasEntities: false, options: ["show_guests"], defaultOptions: { show_guests: true } },
  { type: "opnsense", iconSlug: "opnsense", icon: siOpnsense, urlPlaceholder: "https://opnsense.local", hasEntities: false, options: ["show_wireguard"], defaultOptions: {} },
  { type: "portainer", iconSlug: "portainer", icon: siPortainer, urlPlaceholder: "https://portainer.local:9443", hasEntities: false, options: [], defaultOptions: {} },
  { type: "qnap", iconSlug: "qnap", icon: siQnap, urlPlaceholder: "snmp://qnap.local", hasEntities: false, options: [], defaultOptions: {} },
  { type: "nut", iconSlug: "talaia-nut", icon: nutIcon, urlPlaceholder: "nut://sai.local", hasEntities: false, options: [], defaultOptions: {} },
  { type: "wgdashboard", iconSlug: "wireguard", icon: siWireguard, urlPlaceholder: "http://wgdashboard.local:10086", hasEntities: false, options: [], defaultOptions: {} },
  { type: "printer", iconSlug: "talaia-printer", icon: printerIcon, urlPlaceholder: "ipp://impresora.local", hasEntities: false, options: [], defaultOptions: {} },
  { type: "availability", iconSlug: "talaia-availability", icon: availabilityIcon, urlPlaceholder: "tcp://switch.local:80", hasEntities: false, options: [], defaultOptions: {} },
];

export function catalogEntry(type: ConnectionType): CatalogEntry {
  return CATALOG.find((entry) => entry.type === type)!;
}
