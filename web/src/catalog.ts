import { siAdguard, siHomeassistant, siProxmox } from "simple-icons";
import type { ConnectionType } from "./api";

export type ConnectionOption = "show_top_domains" | "show_guests";

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
  { type: "homeassistant", icon: siHomeassistant, urlPlaceholder: "http://homeassistant.local:8123", hasEntities: true, options: [], defaultOptions: {} },
  { type: "proxmox", icon: siProxmox, urlPlaceholder: "https://proxmox.example:8006", hasEntities: false, options: ["show_guests"], defaultOptions: { show_guests: true } },
];

export function catalogEntry(type: ConnectionType): CatalogEntry {
  return CATALOG.find((entry) => entry.type === type)!;
}
