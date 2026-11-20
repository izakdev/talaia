import { useId, useMemo, useState, type KeyboardEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import type { BrandIconData, ConnectionType } from "./api";
import { CATALOG, catalogEntry } from "./catalog";
import { BrandIcon } from "./chrome";
import { useI18n } from "./i18n";

const MAX_RESULTS = 48;
const NONE = "none";

function fold(text: string) {
  return text.normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase().replace(/[^a-z0-9]/g, "");
}

/** Busca por nombre sin distinguir mayúsculas, tildes ni signos; vacía, da los iconos de los tipos de Talaia. */
export function searchIcons(index: BrandIconData[], query: string, type: ConnectionType): BrandIconData[] {
  const q = fold(query);
  if (!q) {
    const slugs = [catalogEntry(type).iconSlug, ...CATALOG.map((e) => e.iconSlug).filter((s) => s !== catalogEntry(type).iconSlug)];
    return slugs.flatMap((slug) => index.filter((icon) => icon.slug === slug));
  }
  const starts: BrandIconData[] = [];
  const contains: BrandIconData[] = [];
  for (const icon of index) {
    const title = fold(icon.title);
    if (title.startsWith(q) || icon.slug.startsWith(q)) starts.push(icon);
    else if (title.includes(q) || icon.slug.includes(q)) contains.push(icon);
    if (starts.length >= MAX_RESULTS) break;
  }
  return [...starts, ...contains].slice(0, MAX_RESULTS);
}

/** Icono del tipo, sin esperar al índice: el catálogo ya lo conoce. */
export function defaultIcon(type: ConnectionType): BrandIconData {
  const entry = catalogEntry(type);
  return { slug: entry.iconSlug, title: entry.icon.title, hex: entry.icon.hex };
}

export function IconPicker({ type, value, onChange }: { type: ConnectionType; value: string; onChange: (value: string) => void }) {
  const { t } = useI18n();
  const id = useId();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const custom = value !== "" && value !== NONE;
  const index = useQuery({
    queryKey: ["icons"],
    queryFn: async (): Promise<BrandIconData[]> => {
      const res = await fetch("/icons/index.json");
      if (!res.ok) throw new Error(String(res.status));
      return res.json();
    },
    enabled: open || custom,
    staleTime: Infinity,
  });
  const current = value === "" ? defaultIcon(type) : custom ? (index.data?.find((i) => i.slug === value) ?? null) : null;
  const results = useMemo(() => (index.data ? searchIcons(index.data, query, type) : []), [index.data, query, type]);
  // La primera opción es siempre «Sin icono».
  const options: (BrandIconData | null)[] = [null, ...results];

  // Los iconos propios (talaia-<tipo>) se nombran como su tipo, en el idioma de la página.
  const label = (icon: BrandIconData) =>
    icon.slug.startsWith("talaia-") ? t(`type.${icon.slug.slice("talaia-".length) as ConnectionType}`) : icon.title;
  const choose = (icon: BrandIconData | null) => {
    onChange(icon ? (icon.slug === defaultIcon(type).slug ? "" : icon.slug) : NONE);
    setOpen(false);
    setQuery("");
  };
  const onKey = (event: KeyboardEvent) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive((a) => (a + step + options.length) % options.length);
    } else if (event.key === "Enter") {
      event.preventDefault();
      choose(options[active] ?? null);
    } else if (event.key === "Escape") {
      // Cierra el buscador, no el diálogo de la conexión.
      event.preventDefault();
      event.stopPropagation();
      setOpen(false);
    }
  };

  return (
    <div className="icon-field">
      <span id={`${id}-label`}>{t("form.icon")}</span>
      <div className="icon-current">
        <button
          type="button"
          className="secondary icon-choose"
          aria-labelledby={`${id}-label ${id}-value`}
          aria-expanded={open}
          onClick={() => {
            setOpen(!open);
            setActive(0);
          }}
        >
          <BrandIcon icon={current} />
          <span id={`${id}-value`}>{current ? label(current) : custom && index.isPending ? "…" : t("form.icon.none")}</span>
        </button>
        {value !== "" && (
          <button type="button" className="quiet" onClick={() => onChange("")}>
            {t("form.icon.reset")}
          </button>
        )}
      </div>
      {open && (
        <div className="icon-search">
          <input
            type="search"
            aria-label={t("form.icon.search")}
            placeholder={t("form.icon.search")}
            value={query}
            autoFocus
            role="combobox"
            aria-expanded="true"
            aria-controls={`${id}-list`}
            aria-activedescendant={`${id}-opt-${active}`}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            onKeyDown={onKey}
          />
          {index.isPending && <small>{t("form.icon.loading")}</small>}
          <ul id={`${id}-list`} role="listbox" aria-label={t("form.icon")} className="icon-options">
            {options.map((icon, i) => (
              <li
                key={icon?.slug ?? NONE}
                id={`${id}-opt-${i}`}
                role="option"
                aria-selected={i === active}
                className={i === active ? "active" : undefined}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => choose(icon)}
              >
                {icon ? <BrandIcon icon={icon} /> : <span className="brand-icon icon-blank" aria-hidden="true" />}
                <span>{icon ? label(icon) : t("form.icon.none")}</span>
              </li>
            ))}
          </ul>
          {index.data && results.length === 0 && <small>{t("form.icon.empty")}</small>}
        </div>
      )}
    </div>
  );
}
