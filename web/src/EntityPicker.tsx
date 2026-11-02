import { useMemo, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { ApiError, type EntitiesResult } from "./api";
import { useI18n } from "./i18n";

const MAX_SELECTED = 200;
const MAX_ROWS = 200;
const CONSUMPTION = ["power", "energy"];

type Filter = "all" | "consumption" | "selected";

export function normalize(text: string) {
  return text
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase();
}

export default function EntityPicker({
  selected,
  onChange,
  load,
}: {
  selected: string[];
  onChange: (entities: string[]) => void;
  load: () => Promise<EntitiesResult> | null;
}) {
  const { t, errorText, collectError } = useI18n();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const list = useMutation({
    mutationFn: async () => {
      const pending = load();
      return pending ? await pending : null;
    },
  });
  const options = list.data?.entities;
  const chosen = useMemo(() => new Set(selected), [selected]);

  const matches = useMemo(() => {
    const terms = normalize(query).trim();
    return (options ?? []).filter((entity) => {
      if (filter === "consumption" && !CONSUMPTION.includes(entity.device_class ?? "")) return false;
      if (filter === "selected" && !chosen.has(entity.entity_id)) return false;
      return normalize(`${entity.name} ${entity.entity_id}`).includes(terms);
    });
  }, [options, query, filter, chosen]);

  const toggle = (id: string, on: boolean) =>
    onChange(on ? [...selected, id] : selected.filter((entity) => entity !== id));

  const failure =
    list.error instanceof ApiError ? errorText(list.error.code) : list.data && !list.data.ok ? collectError(list.data.error) : null;

  return (
    <fieldset className="picker">
      <legend>{t("form.entities")}</legend>
      <small>{t("form.entities.hint")}</small>
      <div className="picker-bar">
        <span role="status">{t("form.entities.count", { n: selected.length })}</span>
        <button type="button" className="secondary" disabled={list.isPending} onClick={() => list.mutate()}>
          {t(list.isPending ? "form.entities.loading" : "form.entities.load")}
        </button>
      </div>
      {failure && <p role="alert" className="error">{failure}</p>}
      {options && (
        <>
          <label>
            {t("form.entities.search")}
            <input type="search" value={query} onChange={(e) => setQuery(e.target.value)} />
          </label>
          <div className="picker-filter" role="radiogroup" aria-label={t("form.entities")}>
            {(["all", "consumption", "selected"] as const).map((value) => (
              <label key={value}>
                <input type="radio" name="entity-filter" checked={filter === value} onChange={() => setFilter(value)} />
                {t(`form.entities.${value}`)}
              </label>
            ))}
          </div>
          {matches.length === 0 ? (
            <p>{t("form.entities.none")}</p>
          ) : (
            <ul className="picker-list">
              {matches.slice(0, MAX_ROWS).map((entity) => {
                const checked = chosen.has(entity.entity_id);
                return (
                  <li key={entity.entity_id}>
                    <label>
                      <input
                        type="checkbox"
                        checked={checked}
                        disabled={!checked && selected.length >= MAX_SELECTED}
                        onChange={(e) => toggle(entity.entity_id, e.target.checked)}
                      />
                      <span>
                        {entity.name}
                        <small>
                          {entity.entity_id}: {entity.state}
                          {entity.unit ? ` ${entity.unit}` : ""}
                        </small>
                      </span>
                    </label>
                  </li>
                );
              })}
            </ul>
          )}
          {matches.length > MAX_ROWS && <small>{t("form.entities.more", { shown: MAX_ROWS, total: matches.length })}</small>}
          {selected.length >= MAX_SELECTED && <small>{t("form.entities.limit", { n: MAX_SELECTED })}</small>}
        </>
      )}
    </fieldset>
  );
}
