import { lazy, Suspense, useEffect, useRef, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type Connection, type ConnectionInput, type ConnectionType, type ConnectionState } from "./api";
import { CATALOG, catalogEntry } from "./catalog";
import { Horizon, IntegrationIcon, LanguageSwitch } from "./chrome";
import EntityPicker, { normalize } from "./EntityPicker";
import Dashboard from "./Dashboard";
import { useI18n } from "./i18n";
import Settings from "./Settings";
import { applyTheme } from "./theme";

// El editor y la rejilla arrastrable solo se cargan al abrirlos.
const PanelEditor = lazy(() => import("./PanelEditor"));

function usePath(): [string, (path: string) => void] {
  const [path, setPath] = useState(window.location.pathname);
  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  return [
    path,
    (next) => {
      window.history.pushState(null, "", next);
      setPath(next);
    },
  ];
}

const code = (error: unknown) => (error instanceof ApiError ? error.code : "internal");

export default function App() {
  const [path, navigate] = usePath();
  const { t } = useI18n();
  const state = useQuery({ queryKey: ["state"], queryFn: api.state });
  useEffect(() => {
    if (state.data) applyTheme(state.data.theme, state.data.accent);
  }, [state.data]);

  if (state.isPending) return <p className="loading">{t("app.loading")}</p>;
  if (state.isError) {
    return (
      <Horizon>
        <h1>Talaia</h1>
        <p role="alert">{t("app.unreachable")}</p>
        <button className="primary" onClick={() => state.refetch()}>
          {t("app.retry")}
        </button>
      </Horizon>
    );
  }
  if (state.data.setup_required) return <Credentials mode="setup" />;
  if (path.startsWith("/admin")) {
    return state.data.user ? (
      <Admin user={state.data.user} version={state.data.version} path={path} navigate={navigate} />
    ) : (
      <Credentials mode="login" />
    );
  }
  if (state.data.require_login && !state.data.user) return <Credentials mode="login" title={t("login.dashboard")} />;
  return <Dashboard path={path} navigate={navigate} />;
}

function Credentials({ mode, title }: { mode: "setup" | "login"; title?: string }) {
  const { t, errorText } = useI18n();
  const client = useQueryClient();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const submit = useMutation({
    mutationFn: () => (mode === "setup" ? api.setup(username, password) : api.login(username, password)),
    onSuccess: () => {
      setPassword("");
      return client.invalidateQueries({ queryKey: ["state"] });
    },
  });
  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    submit.mutate();
  };
  return (
    <Horizon>
      <h1>{title ?? t(mode === "setup" ? "setup.title" : "login.title")}</h1>
      {mode === "setup" && <p>{t("setup.lead")}</p>}
      <form className="stack" onSubmit={onSubmit}>
        <label>
          {t("field.username")}
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            autoFocus
            required
          />
        </label>
        <label>
          {t("field.password")}
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={mode === "setup" ? "new-password" : "current-password"}
            minLength={mode === "setup" ? 12 : undefined}
            required
          />
          {mode === "setup" && <small>{t("field.passwordHint")}</small>}
        </label>
        {submit.isError && <p role="alert" className="error">{errorText(code(submit.error))}</p>}
        <button className="primary" disabled={submit.isPending}>
          {t(mode === "setup" ? "setup.submit" : "login.submit")}
        </button>
      </form>
    </Horizon>
  );
}

function Admin({
  user,
  version,
  path,
  navigate,
}: {
  user: string;
  version: string;
  path: string;
  navigate: (path: string) => void;
}) {
  const { t } = useI18n();
  const client = useQueryClient();
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: () => {
      client.clear();
      navigate("/");
      return client.invalidateQueries({ queryKey: ["state"] });
    },
  });
  return (
    <div className="admin">
      <header>
        <a
          className="wordmark"
          href="/"
          onClick={(event) => {
            event.preventDefault();
            navigate("/");
          }}
        >
          Talaia
        </a>
        <nav>
          <a
            className="quiet"
            href="/"
            onClick={(event) => {
              event.preventDefault();
              navigate("/");
            }}
          >
            {t("admin.dashboard")}
          </a>
          <LanguageSwitch />
          <span className="user">{user}</span>
          <button className="quiet" onClick={() => logout.mutate()} disabled={logout.isPending}>
            {t("admin.logout")}
          </button>
        </nav>
      </header>
      <nav className="admin-tabs" aria-label={t("admin.sections")}>
        {ADMIN_SECTIONS.map((section) => (
          <a
            key={section.path}
            href={section.path}
            aria-current={section.active(path) ? "page" : undefined}
            onClick={(event) => {
              event.preventDefault();
              navigate(section.path);
            }}
          >
            {t(section.label)}
          </a>
        ))}
      </nav>
      {path === NEW_CONNECTION_PATH ? (
        <NewConnection navigate={navigate} />
      ) : path === PANEL_PATH ? (
        <Suspense fallback={<p className="loading">{t("app.loading")}</p>}>
          <PanelEditor navigate={navigate} />
        </Suspense>
      ) : path === SETTINGS_PATH ? (
        <Settings />
      ) : (
        <Connections navigate={navigate} />
      )}
      <footer>Talaia {version}</footer>
    </div>
  );
}

function ConnectionHealth({ state }: { state?: ConnectionState }) {
  const { t, collectError } = useI18n();
  if (!state || state.status === "pending") return <span>{t("status.checking")}</span>;
  if (state.status === "error") return <span className="warn">{collectError(state.error)}</span>;
  return <span>{t("connections.working")}</span>;
}

const NEW_CONNECTION_PATH = "/admin/connections/new";
const PANEL_PATH = "/admin/panel";
const SETTINGS_PATH = "/admin/settings";
const ADMIN_SECTIONS = [
  { path: "/admin", label: "connections.title", active: (p: string) => p !== PANEL_PATH && p !== SETTINGS_PATH },
  { path: PANEL_PATH, label: "panel.title", active: (p: string) => p === PANEL_PATH },
  { path: SETTINGS_PATH, label: "settings.title", active: (p: string) => p === SETTINGS_PATH },
] as const;

function NewConnection({ navigate }: { navigate: (path: string) => void }) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [chosen, setChosen] = useState<ConnectionType | null>(null);
  const terms = normalize(query).trim();
  const matches = CATALOG.filter((entry) =>
    normalize(`${t(`type.${entry.type}`)} ${t(`catalog.desc.${entry.type}`)}`).includes(terms),
  );
  return (
    <main className="panel">
      <a
        className="back"
        href="/admin"
        onClick={(event) => {
          event.preventDefault();
          navigate("/admin");
        }}
      >
        {t("catalog.back")}
      </a>
      <div>
        <h1>{t("catalog.title")}</h1>
        <p className="lead">{t("catalog.lead")}</p>
      </div>
      <label className="catalog-search">
        {t("catalog.search")}
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          // En pantallas táctiles abriría el teclado y taparía el catálogo.
          autoFocus={window.matchMedia("(pointer: fine)").matches}
        />
      </label>
      {matches.length === 0 && <p className="lead">{t("catalog.empty", { query })}</p>}
      <ul className="catalog">
        {matches.map((entry) => (
          <li key={entry.type}>
            <button onClick={() => setChosen(entry.type)}>
              <IntegrationIcon icon={entry.icon} />
              <strong>{t(`type.${entry.type}`)}</strong>
              <span>{t(`catalog.desc.${entry.type}`)}</span>
            </button>
          </li>
        ))}
      </ul>
      {chosen && (
        <ConnectionDialog
          key={chosen}
          connection={null}
          initialType={chosen}
          onClose={() => setChosen(null)}
          onSaved={() => navigate("/admin")}
        />
      )}
    </main>
  );
}

function Connections({ navigate }: { navigate: (path: string) => void }) {
  const { t, errorText } = useI18n();
  const client = useQueryClient();
  const [editing, setEditing] = useState<Connection | null>(null);
  const [confirming, setConfirming] = useState<string | null>(null);
  const connections = useQuery({
    queryKey: ["connections"],
    queryFn: api.connections,
    retry: (count, error) => !(error instanceof ApiError && error.status === 401) && count < 2,
  });
  useEffect(() => {
    if (connections.error instanceof ApiError && connections.error.status === 401) {
      client.invalidateQueries({ queryKey: ["state"] });
    }
  }, [connections.error, client]);
  const states = useQuery({ queryKey: ["status"], queryFn: api.status, refetchInterval: 10000 });
  const remove = useMutation({
    mutationFn: api.deleteConnection,
    onSuccess: () => {
      setConfirming(null);
      client.removeQueries({ queryKey: ["dashboard"] });
      client.invalidateQueries({ queryKey: ["status"] });
      return client.invalidateQueries({ queryKey: ["connections"] });
    },
  });

  return (
    <main className="panel">
      <div className="panel-head">
        <div>
          <h1>{t("connections.title")}</h1>
          <p>{t("connections.lead")}</p>
        </div>
        <a
          className="button primary"
          href={NEW_CONNECTION_PATH}
          onClick={(event) => {
            event.preventDefault();
            navigate(NEW_CONNECTION_PATH);
          }}
        >
          {t("connections.add")}
        </a>
      </div>

      {connections.isError && <p role="alert" className="error">{t("connections.loadError")}</p>}
      {remove.isError && <p role="alert" className="error">{errorText(code(remove.error))}</p>}

      {connections.data?.length === 0 && (
        <div className="empty">
          <h2>{t("connections.emptyTitle")}</h2>
          <p>{t("connections.emptyLead")}</p>
        </div>
      )}

      {!!connections.data?.length && (
        <ul className="connections">
          {connections.data.map((connection) => (
            <li key={connection.id}>
              <div className="connection-main">
                <strong>{connection.name}</strong>
                <span>{t(`type.${connection.type}`)}</span>
              </div>
              <div className="connection-meta">
                <span className="url">{connection.base_url}</span>
                <span>{t("connections.every", { n: connection.interval_seconds })}</span>
                <ConnectionHealth state={states.data?.connections.find((state) => state.id === connection.id)} />
                {connection.tls_skip_verify && <span className="warn">{t("connections.tlsOff")}</span>}
              </div>
              <div className="connection-actions">
                {confirming === connection.id ? (
                  <>
                    <span>{t("connections.confirmDelete", { name: connection.name })}</span>
                    <button
                      className="danger"
                      disabled={remove.isPending}
                      onClick={() => remove.mutate(connection.id)}
                    >
                      {t("connections.delete")}
                    </button>
                    <button className="quiet" onClick={() => setConfirming(null)}>
                      {t("form.cancel")}
                    </button>
                  </>
                ) : (
                  <>
                    <button className="quiet" onClick={() => setEditing(connection)}>
                      {t("connections.edit")}
                    </button>
                    <button className="quiet" onClick={() => setConfirming(connection.id)}>
                      {t("connections.delete")}
                    </button>
                  </>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      {editing && (
        <ConnectionDialog
          key={editing.id}
          connection={editing}
          initialType={editing.type}
          onClose={() => setEditing(null)}
          onSaved={() => setEditing(null)}
        />
      )}
    </main>
  );
}

function ConnectionDialog({
  connection,
  initialType,
  onClose,
  onSaved,
}: {
  connection: Connection | null;
  initialType: ConnectionType;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t, errorText, collectError } = useI18n();
  const client = useQueryClient();
  const form = useRef<HTMLFormElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const type = initialType;
  const entry = catalogEntry(type);
  const [name, setName] = useState(connection?.name ?? "");
  const [url, setUrl] = useState(connection?.base_url ?? "");
  const [username, setUsername] = useState(connection?.username ?? "");
  const [secret, setSecret] = useState("");
  const [interval, setIntervalValue] = useState(String(connection?.interval_seconds ?? 30));
  const [skipTls, setSkipTls] = useState(connection?.tls_skip_verify ?? false);
  const [options, setOptions] = useState<Record<string, boolean>>(connection?.options ?? entry.defaultOptions);
  const [entities, setEntities] = useState<string[]>(connection?.entities ?? []);
  const [link, setLink] = useState(connection?.link_url ?? "");

  useEffect(() => {
    dialog.current?.showModal();
  }, []);

  const urlChanged = connection !== null && url.trim().replace(/\/+$/, "") !== connection.base_url;
  const secretRequired = connection === null || urlChanged;
  const hint = secretRequired
    ? urlChanged
      ? t("form.secretUrlChanged")
      : t(`form.secretHint.${type}`)
    : t("form.secretKeep");

  const input = (): ConnectionInput => ({
    name,
    type,
    base_url: url,
    username,
    interval_seconds: Number(interval),
    tls_skip_verify: skipTls,
    options,
    entities,
    link_url: link,
    ...(secret ? { secret } : {}),
  });
  const save = useMutation({
    mutationFn: () => (connection ? api.updateConnection(connection.id, input()) : api.createConnection(input())),
    onSuccess: async () => {
      setSecret("");
      client.removeQueries({ queryKey: ["dashboard"] });
      client.invalidateQueries({ queryKey: ["status"] });
      await client.invalidateQueries({ queryKey: ["connections"] });
      onSaved();
    },
  });
  const withId = () => ({ ...input(), ...(connection ? { id: connection.id } : {}) });
  const test = useMutation({ mutationFn: () => api.testConnection(withId()) });
  const { reset: resetTest } = test;
  // Un resultado solo vale para los datos con los que se probó.
  useEffect(() => resetTest(), [type, url, username, secret, skipTls, resetTest]);

  return (
    <dialog ref={dialog} onClose={onClose}>
      <form
        ref={form}
        className="stack"
        onSubmit={(event) => {
          event.preventDefault();
          save.mutate();
        }}
      >
        <div className="dialog-title">
          <IntegrationIcon icon={entry.icon} />
          <h2>
            {connection
              ? t("form.editTitle", { name: connection.name })
              : t("form.addTitle", { name: t(`type.${type}`) })}
          </h2>
        </div>
        <label>
          {t("form.name")}
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={80} required autoFocus />
        </label>
        <label>
          {t("form.url")}
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={entry.urlPlaceholder}
            required
          />
        </label>
        {type === "adguard" && (
          <label>
            {t("form.username")}
            <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="off" required />
          </label>
        )}
        <label>
          {t(`form.secret.${type}`)}
          <input
            type="password"
            value={secret}
            onChange={(e) => setSecret(e.target.value)}
            autoComplete="new-password"
            required={secretRequired}
          />
          {hint && <small>{hint}</small>}
        </label>
        <label>
          {t("form.link")}
          <input type="url" value={link} onChange={(e) => setLink(e.target.value)} />
          <small>{t("form.link.hint")}</small>
        </label>
        <label>
          {t("form.interval")}
          <input
            type="number"
            min={5}
            max={3600}
            value={interval}
            onChange={(e) => setIntervalValue(e.target.value)}
            required
          />
        </label>
        {entry.options.length > 0 && (
          <fieldset className="options">
            <legend>{t("form.display")}</legend>
            {entry.options.map((option) => (
              <label key={option} className="check">
                <input
                  type="checkbox"
                  checked={!!options[option]}
                  onChange={(e) => setOptions({ ...options, [option]: e.target.checked })}
                />
                <span>
                  {t(`form.option.${option}`)}
                  <small>{t(`form.option.${option}.hint`)}</small>
                </span>
              </label>
            ))}
          </fieldset>
        )}
        {entry.hasEntities && (
          <EntityPicker
            selected={entities}
            onChange={setEntities}
            load={() => {
              if (!form.current?.reportValidity()) return null;
              return api.listEntities(withId());
            }}
          />
        )}
        <label className="check">
          <input type="checkbox" checked={skipTls} onChange={(e) => setSkipTls(e.target.checked)} />
          <span>
            {t("form.tls")}
            <small>{t("form.tlsHint")}</small>
          </span>
        </label>
        {save.isError && <p role="alert" className="error">{errorText(code(save.error))}</p>}
        {test.isError && <p role="alert" className="error">{errorText(code(test.error))}</p>}
        {test.data && (
          <p role="status" className={test.data.ok ? "test-ok" : "error"}>
            {test.data.ok ? t("form.testOk") : collectError(test.data.error)}
          </p>
        )}
        {test.data?.notes?.includes("system_unavailable") && <small>{t("form.note.system_unavailable")}</small>}
        <div className="dialog-actions">
          <button
            type="button"
            className="secondary"
            disabled={test.isPending}
            onClick={() => {
              if (form.current?.reportValidity()) test.mutate();
            }}
          >
            {t(test.isPending ? "form.testing" : "form.test")}
          </button>
          <span className="spacer" />
          <button type="button" className="quiet" onClick={onClose}>
            {t("form.cancel")}
          </button>
          <button className="primary" disabled={save.isPending}>
            {t("form.save")}
          </button>
        </div>
      </form>
    </dialog>
  );
}
