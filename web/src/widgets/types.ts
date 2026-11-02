import type { FC } from "react";
import type { Translate } from "../i18n";

export type Tone = "good" | "warn" | "bad" | "muted";

// Cada integración decide su estado, su contenido y su pie.
export interface Widget<T = never> {
  state: (data: T, t: Translate) => { tone: Tone; label: string };
  Body: FC<{ data: T }>;
  footer: (data: T) => string;
}
