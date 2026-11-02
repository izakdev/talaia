import type { ReactNode } from "react";
import { useI18n } from "./i18n";

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
  return (
    <svg className="tower" viewBox="0 0 120 200" aria-hidden="true">
      <path className="beam" d="M64 30 L3000 -110 L3000 170 Z" />
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
