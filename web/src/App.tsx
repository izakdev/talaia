import { useEffect, useRef, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type Connection, type ConnectionInput, type ConnectionType, type ConnectionState } from "./api";
import { Horizon, LanguageSwitch } from "./chrome";
import Dashboard from "./Dashboard";
import { useI18n } from "./i18n";

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
      <Admin user={state.data.user} version={state.data.version} navigate={navigate} />
    ) : (
      <Credentials mode="login" />
    );
  }
  return <Dashboard navigate={navigate} />;
}

function Credentials({ mode }: { mode: "setup" | "login" }) {
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
      <h1>{t(mode === "setup" ? "setup.title" : "login.title")}</h1>
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

function Admin({ user, version, navigate }: { user: string; version: string; navigate: (path: string) => void }) {
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
      <Connections />
      <footer>Talaia {version}</footer>
    </div>
  );
}

function ConnectionHealth({ type, state }: { type: ConnectionType; state?: ConnectionState }) {
  const { t, collectError } = useI18n();
  if (!TESTABLE.includes(type)) return <span>{t("connections.notAvailable")}</span>;
  if (!state || state.status === "pending") return <span>{t("status.checking")}</span>;
  if (state.status === "error") return <span className="warn">{collectError(state.error)}</span>;
  return <span>{t("connections.working")}</span>;
}

type Editing = { kind: "new" } | { kind: "edit"; connection: Connection } | null;

function Connections() {
  const { t, errorText } = useI18n();
  const client = useQueryClient();
  const [editing, setEditing] = useState<Editing>(null);
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
  const states = useQuery({ queryKey: ["dashboard"], queryFn: api.dashboard, refetchInterval: 10000 });
  const remove = useMutation({
    mutationFn: api.deleteConnection,
    onSuccess: () => {
      setConfirming(null);
      client.invalidateQueries({ queryKey: ["dashboard"] });
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
        <button className="primary" onClick={() => setEditing({ kind: "new" })}>
          {t("connections.add")}
        </button>
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
                <ConnectionHealth
                  type={connection.type}
                  state={states.data?.connections.find((state) => state.id === connection.id)}
                />
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
                    <button className="quiet" onClick={() => setEditing({ kind: "edit", connection })}>
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
          key={editing.kind === "edit" ? editing.connection.id : "new"}
          connection={editing.kind === "edit" ? editing.connection : null}
          onClose={() => setEditing(null)}
        />
      )}
    </main>
  );
}

const TYPES: ConnectionType[] = ["proxmox", "homeassistant", "adguard"];
// Integraciones ya disponibles. Las demás se pueden guardar, pero aún no se consultan.
const TESTABLE: ConnectionType[] = ["adguard"];
const PLACEHOLDERS: Record<ConnectionType, string> = {
  proxmox: "https://proxmox.example:8006",
  homeassistant: "http://homeassistant.local:8123",
  adguard: "http://adguard.local:3000",
};

function ConnectionDialog({ connection, onClose }: { connection: Connection | null; onClose: () => void }) {
  const { t, errorText, collectError } = useI18n();
  const client = useQueryClient();
  const form = useRef<HTMLFormElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const [type, setType] = useState<ConnectionType>(connection?.type ?? "proxmox");
  const [name, setName] = useState(connection?.name ?? "");
  const [url, setUrl] = useState(connection?.base_url ?? "");
  const [username, setUsername] = useState(connection?.username ?? "");
  const [secret, setSecret] = useState("");
  const [interval, setIntervalValue] = useState(String(connection?.interval_seconds ?? 30));
  const [skipTls, setSkipTls] = useState(connection?.tls_skip_verify ?? false);

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
    ...(secret ? { secret } : {}),
  });
  const save = useMutation({
    mutationFn: () => (connection ? api.updateConnection(connection.id, input()) : api.createConnection(input())),
    onSuccess: async () => {
      setSecret("");
      client.invalidateQueries({ queryKey: ["dashboard"] });
      await client.invalidateQueries({ queryKey: ["connections"] });
      onClose();
    },
  });
  const test = useMutation({
    mutationFn: () => api.testConnection({ ...input(), ...(connection ? { id: connection.id } : {}) }),
  });
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
        <h2>{connection ? t("form.editTitle", { name: connection.name }) : t("form.addTitle")}</h2>
        {!connection && (
          <fieldset className="types">
            <legend>{t("form.type")}</legend>
            {TYPES.map((value) => (
              <label key={value}>
                <input
                  type="radio"
                  name="type"
                  value={value}
                  checked={type === value}
                  onChange={() => setType(value)}
                />
                {t(`type.${value}`)}
              </label>
            ))}
          </fieldset>
        )}
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
            placeholder={PLACEHOLDERS[type]}
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
        <div className="dialog-actions">
          {TESTABLE.includes(type) && (
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
          )}
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
