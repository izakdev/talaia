import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type LinkGroup, type SavedLink } from "./api";
import { useI18n } from "./i18n";
import { IconPicker } from "./IconPicker";

const MAX_GROUPS = 20;
const MAX_LINKS = 100;

function randomId() {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

function swap<T>(list: T[], from: number, delta: number): T[] {
  const to = from + delta;
  if (from < 0 || to < 0 || to >= list.length) return list;
  const next = [...list];
  [next[from], next[to]] = [next[to], next[from]];
  return next;
}

export default function LinksEditor({
  navigate,
}: {
  navigate: (path: string) => void;
}) {
  const { t } = useI18n();
  const links = useQuery({ queryKey: ["links"], queryFn: api.links });
  if (!links.data) return <p className="loading">{t("app.loading")}</p>;
  return <Editor initial={links.data.groups} navigate={navigate} />;
}

function Editor({
  initial,
  navigate,
}: {
  initial: LinkGroup[];
  navigate: (path: string) => void;
}) {
  const { t, errorText } = useI18n();
  const client = useQueryClient();
  const [groups, setGroups] = useState(initial);
  const [saved, setSaved] = useState(initial);
  const [confirming, setConfirming] = useState<string | null>(null);
  const dirty = useMemo(
    () => JSON.stringify(groups) !== JSON.stringify(saved),
    [groups, saved],
  );
  const total = groups.reduce((n, g) => n + g.links.length, 0);

  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  const save = useMutation({
    mutationFn: () => api.saveLinks(groups),
    onSuccess: (result) => {
      setGroups(result.groups);
      setSaved(result.groups);
      client.setQueryData(["links"], result);
      client.removeQueries({ queryKey: ["dashboard"] });
    },
  });

  // Un error de guardado deja de valer en cuanto se cambia algo (no mientras se guarda).
  const { reset, isPending } = save;
  const shown = useRef(groups);
  useEffect(() => {
    if (shown.current === groups) return;
    shown.current = groups;
    if (!isPending) reset();
  }, [groups, reset, isPending]);

  const updateGroup = (id: string, change: (g: LinkGroup) => LinkGroup) =>
    setGroups((all) => all.map((g) => (g.id === id ? change(g) : g)));
  const updateLink = (
    groupId: string,
    linkId: string,
    change: Partial<SavedLink>,
  ) =>
    updateGroup(groupId, (g) => ({
      ...g,
      links: g.links.map((l) => (l.id === linkId ? { ...l, ...change } : l)),
    }));
  const addGroup = () =>
    setGroups((all) => [
      ...all,
      {
        id: randomId(),
        name: t("links.group.default", { n: all.length + 1 }),
        links: [],
      },
    ]);
  const deleteGroup = (id: string) => {
    setGroups((all) => all.filter((g) => g.id !== id));
    setConfirming(null);
  };
  const addLink = (groupId: string) =>
    updateGroup(groupId, (g) => ({
      ...g,
      links: [...g.links, { id: randomId(), name: "", url: "", icon: "" }],
    }));
  const moveLinkTo = (from: string, linkId: string, to: string) =>
    setGroups((all) => {
      const link = all
        .find((g) => g.id === from)
        ?.links.find((l) => l.id === linkId);
      if (!link || from === to) return all;
      return all.map((g) =>
        g.id === from
          ? { ...g, links: g.links.filter((l) => l.id !== linkId) }
          : g.id === to
            ? { ...g, links: [...g.links, link] }
            : g,
      );
    });

  return (
    <main className="panel panel-wide">
      <div className="panel-head">
        <div>
          <h1>{t("links.title")}</h1>
          <p>{t("links.lead")}</p>
        </div>
        <div className="editor-actions">
          <span role="status" className={dirty ? "warn" : undefined}>
            {t(dirty ? "panel.unsaved" : "panel.saved")}
          </span>
          <button
            type="button"
            className="quiet"
            disabled={!dirty}
            onClick={() => setGroups(saved)}
          >
            {t("panel.discard")}
          </button>
          <button
            type="button"
            className="secondary"
            onClick={() => navigate("/")}
          >
            {t("admin.dashboard")}
          </button>
          <button
            type="button"
            className="primary"
            disabled={!dirty || save.isPending}
            onClick={() => save.mutate()}
          >
            {t("links.save")}
          </button>
        </div>
      </div>
      {save.isError && (
        <p role="alert" className="error">
          {errorText(
            save.error instanceof ApiError ? save.error.code : "internal",
          )}
        </p>
      )}

      {/* Mientras se guarda no se edita: lo que llegue del servidor reemplaza la lista. */}
      <fieldset className="links-fields" disabled={isPending}>
        {groups.length === 0 && (
          <p className="links-empty">{t("links.empty")}</p>
        )}

        {groups.map((group, gi) => (
          <section
            key={group.id}
            className="link-group"
            aria-label={group.name || t("links.group.unnamed")}
          >
            <div className="link-group-head">
              <label>
                {t("links.group.name")}
                <input
                  value={group.name}
                  maxLength={40}
                  onChange={(e) =>
                    updateGroup(group.id, (g) => ({
                      ...g,
                      name: e.target.value,
                    }))
                  }
                />
              </label>
              <button
                type="button"
                className="quiet"
                disabled={gi === 0}
                onClick={() => setGroups((all) => swap(all, gi, -1))}
              >
                {t("links.group.up")}
              </button>
              <button
                type="button"
                className="quiet"
                disabled={gi === groups.length - 1}
                onClick={() => setGroups((all) => swap(all, gi, 1))}
              >
                {t("links.group.down")}
              </button>
              {confirming === group.id ? (
                <>
                  <span>
                    {t("links.group.confirmDelete", {
                      name: group.name,
                      count: group.links.length,
                    })}
                  </span>
                  <button
                    type="button"
                    className="danger"
                    onClick={() => deleteGroup(group.id)}
                  >
                    {t("links.group.delete")}
                  </button>
                  <button
                    type="button"
                    className="quiet"
                    onClick={() => setConfirming(null)}
                  >
                    {t("form.cancel")}
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  className="quiet"
                  onClick={() =>
                    group.links.length > 0
                      ? setConfirming(group.id)
                      : deleteGroup(group.id)
                  }
                >
                  {t("links.group.delete")}
                </button>
              )}
            </div>

            {group.links.length > 0 && (
              <ul className="link-rows">
                {group.links.map((link, li) => {
                  const label = link.name || t("links.link.unnamed");
                  return (
                    <li key={link.id} className="link-row" aria-label={label}>
                      <label>
                        {t("links.link.name")}
                        <input
                          value={link.name}
                          maxLength={60}
                          onChange={(e) =>
                            updateLink(group.id, link.id, {
                              name: e.target.value,
                            })
                          }
                        />
                      </label>
                      <label>
                        {t("links.link.url")}
                        <input
                          value={link.url}
                          inputMode="url"
                          autoComplete="off"
                          spellCheck={false}
                          placeholder="https://"
                          onChange={(e) =>
                            updateLink(group.id, link.id, {
                              url: e.target.value,
                            })
                          }
                        />
                      </label>
                      <IconPicker
                        value={link.icon}
                        onChange={(icon) =>
                          updateLink(group.id, link.id, { icon })
                        }
                      />
                      {groups.length > 1 && (
                        <label>
                          {t("links.link.group")}
                          <select
                            value={group.id}
                            onChange={(e) =>
                              moveLinkTo(group.id, link.id, e.target.value)
                            }
                          >
                            {groups.map((g) => (
                              <option key={g.id} value={g.id}>
                                {g.name || t("links.group.unnamed")}
                              </option>
                            ))}
                          </select>
                        </label>
                      )}
                      <div className="link-row-actions">
                        <button
                          type="button"
                          className="quiet"
                          aria-label={t("links.link.up", { name: label })}
                          disabled={li === 0}
                          onClick={() =>
                            updateGroup(group.id, (g) => ({
                              ...g,
                              links: swap(g.links, li, -1),
                            }))
                          }
                        >
                          {t("links.up")}
                        </button>
                        <button
                          type="button"
                          className="quiet"
                          aria-label={t("links.link.down", { name: label })}
                          disabled={li === group.links.length - 1}
                          onClick={() =>
                            updateGroup(group.id, (g) => ({
                              ...g,
                              links: swap(g.links, li, 1),
                            }))
                          }
                        >
                          {t("links.down")}
                        </button>
                        <button
                          type="button"
                          className="quiet"
                          aria-label={t("links.link.delete", { name: label })}
                          onClick={() =>
                            updateGroup(group.id, (g) => ({
                              ...g,
                              links: g.links.filter((l) => l.id !== link.id),
                            }))
                          }
                        >
                          {t("links.delete")}
                        </button>
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
            <button
              type="button"
              className="secondary"
              disabled={total >= MAX_LINKS}
              onClick={() => addLink(group.id)}
            >
              {t("links.link.add")}
            </button>
          </section>
        ))}

        <button
          type="button"
          className="secondary"
          disabled={groups.length >= MAX_GROUPS}
          onClick={addGroup}
        >
          {t("links.group.add")}
        </button>
      </fieldset>
    </main>
  );
}
