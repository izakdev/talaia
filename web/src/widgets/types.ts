import type { FC } from "react";

export type { Tone } from "../api";

// Cada integración decide su contenido y su pie; su estado lo calcula el servidor.
export interface Widget<T = never> {
  Body: FC<{ data: T }>;
  footer: (data: T) => string;
}
