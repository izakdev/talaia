import { useId, type CSSProperties, type ReactNode } from "react";
import type { BrandIconData } from "./api";
import { useI18n } from "./i18n";

export function IntegrationIcon({ icon }: { icon: { path: string; hex: string; title: string } }) {
  return (
    <svg className="integration-icon" viewBox="0 0 24 24" aria-hidden="true" fill={`#${icon.hex}`}>
      <path d={icon.path} />
    </svg>
  );
}

/** Luminancia relativa de un color #RRGGBB, según WCAG. */
function luminance(hex: string) {
  const [r, g, b] = [0, 2, 4].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/**
 * Indica si el color de la marca apenas se distingue del fondo de un bloque
 * (contraste por debajo de 1,5 con --surface en cada tema): entonces se pinta con el color del texto.
 */
export function lowContrast(hex: string): { dark: boolean; light: boolean } {
  const l = luminance(hex);
  return { dark: l < 0.08, light: l > 0.65 };
}

/** Icono de /icons/, pintado como máscara con el color de su marca. */
export function BrandIcon({ icon, className }: { icon: BrandIconData | null; className?: string }) {
  if (!icon) return null;
  const low = lowContrast(icon.hex);
  const style = { "--icon-url": `url(/icons/${icon.slug}.svg)`, "--icon-color": `#${icon.hex}` } as CSSProperties;
  return (
    <span
      className={["brand-icon", low.dark && "low-on-dark", low.light && "low-on-light", className].filter(Boolean).join(" ")}
      style={style}
      data-icon={icon.slug}
      aria-hidden="true"
    />
  );
}

export function LanguageSwitch() {
  const { lang, setLang } = useI18n();
  const next = lang === "es" ? "en" : "es";
  return (
    <button className="quiet language" lang={next} onClick={() => setLang(next)}>
      {next === "en" ? "English" : "Español"}
    </button>
  );
}

export function Tower() {
  // El haz se desvanece hacia su extremo en vez de acabar en un corte, sea cual sea el ancho de la pantalla.
  const fade = `beam-${useId().replace(/[^a-zA-Z0-9-]/g, "")}`;
  return (
    <svg className="tower" viewBox="0 0 120 200" aria-hidden="true">
      <defs>
        <linearGradient id={fade} gradientUnits="userSpaceOnUse" x1="64" y1="30" x2="3000" y2="30">
          <stop offset="0" className="beam-stop" />
          <stop offset="0.45" className="beam-stop" stopOpacity="0.6" />
          <stop offset="1" className="beam-stop" stopOpacity="0" />
        </linearGradient>
      </defs>
      <path className="beam" fill={`url(#${fade})`} d="M64 30 L3000 -110 L3000 170 Z" />
      <path className="masonry" d="M36 200 46 62h28l10 138Z" />
      <path className="masonry" d="M40 62V44h6v6h7v-6h6v6h7v-6h6v6h7v12Z" transform="translate(-1 0)" />
      <path className="door" d="M54 200v-22a6 6 0 0 1 12 0v22Z" />
      <circle className="beacon" cx="60" cy="30" r="7" />
    </svg>
  );
}

export function Horizon({ children }: { children: ReactNode }) {
  return (
    <main className="horizon">
      <div className="sky">
        <span className="wordmark">Talaia</span>
        <LanguageSwitch />
        <Tower />
      </div>
      <div className="sea">
        <div className="sea-content">{children}</div>
      </div>
    </main>
  );
}
