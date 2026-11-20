import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type AppSettings, type Channel, type ChannelInput, type ChannelType, type LogEntry } from "./api";
import { useI18n } from "./i18n";

const CHANNEL_TYPES: ChannelType[] = ["telegram", "ntfy", "webhook"];
const DELAYS = [0, 60, 120, 300, 600, 900] as const;

const code = (error: unknown) => (error instanceof ApiError ? error.code : "internal");

/** Sección de avisos: idioma, retardo, canales y últimos envíos. */
export default function Notifications() {
  const { t } = useI18n();
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.settings });
  return (
    <section className="stack settings-section notifications" aria-labelledby="notifications-title">
      <h2 id="notifications-title">{t("notify.title")}</h2>
      <p>{t("notify.lead")}</p>
      {settings.data && <NotifySettings initial={settings.data} />}
      <Channels />
      <SendLog />
    </section>
  );
}

function NotifySettings({ initial }: { initial: AppSettings }) {
  const { t, lang, errorText } = useI18n();
  const client = useQueryClient();
  const [language, setLanguage] = useState(initial.notify_language || lang);
  const [delay, setDelay] = useState(initial.notify_delay_seconds);
  // El primer canal elige el idioma en el servidor: el formulario lo refleja.
  useEffect(() => {
    if (initial.notify_language) setLanguage(initial.notify_language);
  }, [initial.notify_language]);
  const save = useMutation({
    // Se parte de los ajustes actuales para no pisar los de otra sección.
    mutationFn: async () =>
      api.saveSettings({ ...(await api.settings()), notify_language: language, notify_delay_seconds: delay }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["settings"] }),
  });
  return (
    <form
      className="stack"
      onSubmit={(event) => {
        event.preventDefault();
        save.mutate();
      }}
    >
      <div className="field-row">
        <label>
          {t("notify.language")}
          <select value={language} onChange={(e) => setLanguage(e.target.value as "es" | "en")}>
            <option value="es">Español</option>
            <option value="en">English</option>
          </select>
        </label>
        <label>
          {t("notify.delay")}
          <select value={delay} onChange={(e) => setDelay(Number(e.target.value))}>
            {DELAYS.map((seconds) => (
              <option key={seconds} value={seconds}>
                {t(`notify.delay.${seconds}`)}
              </option>
            ))}
          </select>
          <small>{t("notify.delayHint")}</small>
        </label>
      </div>
      {save.isError && <p role="alert" className="error">{errorText(code(save.error))}</p>}
      {save.isSuccess && <p role="status" className="test-ok">{t("settings.saved")}</p>}
      <div>
        <button className="secondary" disabled={save.isPending}>
          {t("notify.saveSettings")}
        </button>
      </div>
    </form>
  );
}

function TestOutcome({ entry }: { entry: LogEntry }) {
  const { t } = useI18n();
  return (
    <p role="status" className={entry.result === "ok" ? "test-ok" : "error"}>
      {entry.result === "ok" ? t("notify.testOk") : t("notify.testFailed", { error: errorLabel(t, entry.code) })}
    </p>
  );
}

function errorLabel(t: ReturnType<typeof useI18n>["t"], code?: string) {
  const known = ["unauthorized", "rejected", "unreachable", "timeout", "tls", "bad_response", "secret_unreadable"];
  return t(`notify.error.${known.includes(code ?? "") ? code : "internal"}` as Parameters<typeof t>[0]);
}

function Channels() {
  const { t, errorText } = useI18n();
  const client = useQueryClient();
  const channels = useQuery({ queryKey: ["channels"], queryFn: api.channels });
  const [editing, setEditing] = useState<Channel | null>(null);
  const [adding, setAdding] = useState(false);
  const [confirming, setConfirming] = useState<string | null>(null);
  const [tested, setTested] = useState<Record<string, LogEntry>>({});
  const refresh = () => Promise.all([client.invalidateQueries({ queryKey: ["channels"] }), client.invalidateQueries({ queryKey: ["notification-log"] })]);
  const remove = useMutation({
    mutationFn: api.deleteChannel,
    onSuccess: async () => {
      setConfirming(null);
      await refresh();
    },
  });
  const test = useMutation({
    mutationFn: api.testSavedChannel,
    onSuccess: async (entry, id) => {
      setTested((current) => ({ ...current, [id]: entry }));
      await refresh();
    },
  });
  return (
    <div className="stack">
      <div className="panel-head">
        <h3>{t("notify.channels")}</h3>
        <button type="button" className="primary" onClick={() => setAdding(true)}>
          {t("notify.add")}
        </button>
      </div>
      {channels.isError && <p role="alert" className="error">{errorText(code(channels.error))}</p>}
      {remove.isError && <p role="alert" className="error">{errorText(code(remove.error))}</p>}
      {test.isError && <p role="alert" className="error">{errorText(code(test.error))}</p>}
      {channels.data?.length === 0 && <p className="muted-text">{t("notify.empty")}</p>}
      {!!channels.data?.length && (
        <ul className="connections channels">
          {channels.data.map((channel) => (
            <li key={channel.id}>
              <div className="connection-main">
                <strong>{channel.name}</strong>
                <span>{t(`notify.type.${channel.type}`)}</span>
              </div>
              <div className="connection-meta">
                <span className={channel.enabled ? "ok" : "warn"}>
                  {t(channel.enabled ? "notify.enabled" : "notify.disabled")}
                </span>
                {tested[channel.id] && <TestOutcome entry={tested[channel.id]} />}
              </div>
              <div className="connection-actions">
                {confirming === channel.id ? (
                  <>
                    <span>{t("notify.confirmDelete", { name: channel.name })}</span>
                    <button className="danger" disabled={remove.isPending} onClick={() => remove.mutate(channel.id)}>
                      {t("connections.delete")}
                    </button>
                    <button className="quiet" onClick={() => setConfirming(null)}>
                      {t("form.cancel")}
                    </button>
                  </>
                ) : (
                  <>
                    <button
                      className="quiet"
                      disabled={test.isPending && test.variables === channel.id}
                      onClick={() => test.mutate(channel.id)}
                    >
                      {t(test.isPending && test.variables === channel.id ? "notify.sending" : "notify.test")}
                    </button>
                    <button className="quiet" onClick={() => setEditing(channel)}>
                      {t("connections.edit")}
                    </button>
                    <button className="quiet" onClick={() => setConfirming(channel.id)}>
                      {t("connections.delete")}
                    </button>
                  </>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
      {(adding || editing) && (
        <ChannelDialog
          key={editing?.id ?? "nuevo"}
          channel={editing}
          onClose={() => {
            setAdding(false);
            setEditing(null);
          }}
          onSaved={async () => {
            setAdding(false);
            setEditing(null);
            await Promise.all([refresh(), client.invalidateQueries({ queryKey: ["settings"] })]);
          }}
        />
      )}
    </div>
  );
}

function ChannelDialog({ channel, onClose, onSaved }: { channel: Channel | null; onClose: () => void; onSaved: () => void }) {
  const { t, lang, errorText } = useI18n();
  const client = useQueryClient();
  const dialog = useRef<HTMLDialogElement>(null);
  const form = useRef<HTMLFormElement>(null);
  const [type, setType] = useState<ChannelType>(channel?.type ?? "telegram");
  const [name, setName] = useState(channel?.name ?? "");
  const [enabled, setEnabled] = useState(channel?.enabled ?? true);
  const [skipTls, setSkipTls] = useState(channel?.tls_skip_verify ?? false);
  const [chatId, setChatId] = useState(channel?.chat_id ?? "");
  const [server, setServer] = useState(channel?.server ?? "");
  const [token, setToken] = useState("");
  const [topic, setTopic] = useState("");
  const [url, setUrl] = useState("");
  const editing = channel !== null;
  // Otro servidor de ntfy exige volver a escribir el tópico y el token: los guardados no se le envían.
  const ntfyServer = (value: string) => value.trim().replace(/\/+$/, "") || "https://ntfy.sh";
  const serverMoved = editing && type === "ntfy" && ntfyServer(server) !== ntfyServer(channel.server ?? "");

  useEffect(() => {
    dialog.current?.showModal();
  }, []);

  const input = (): ChannelInput => ({
    ...(channel ? { id: channel.id } : { language: lang }),
    name,
    type,
    enabled,
    tls_skip_verify: type !== "telegram" && skipTls,
    ...(type === "telegram" ? { chat_id: chatId, token } : {}),
    ...(type === "ntfy" ? { server, topic, token } : {}),
    ...(type === "webhook" ? { url } : {}),
  });
  const save = useMutation({
    mutationFn: () => (channel ? api.updateChannel(channel.id, input()) : api.createChannel(input())),
    onSuccess: onSaved,
  });
  const test = useMutation({
    mutationFn: () => api.testChannel(input()),
    onSettled: () => client.invalidateQueries({ queryKey: ["notification-log"] }),
  });
  const { reset: resetTest } = test;
  // Un resultado solo vale para los datos con los que se probó.
  useEffect(() => resetTest(), [type, chatId, server, token, topic, url, skipTls, resetTest]);

  const keep = editing ? t("notify.secretKeep") : undefined;
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
          <h2>{editing ? t("form.editTitle", { name: channel.name }) : t("notify.addTitle")}</h2>
        </div>
        {!editing && (
          <fieldset className="options">
            <legend>{t("notify.type")}</legend>
            <div className="choice-row">
              {CHANNEL_TYPES.map((value) => (
                <label key={value}>
                  <input type="radio" name="channel-type" checked={type === value} onChange={() => setType(value)} />
                  {t(`notify.type.${value}`)}
                </label>
              ))}
            </div>
          </fieldset>
        )}
        <label>
          {t("form.name")}
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={60} required autoFocus />
        </label>
        {type === "telegram" && (
          <>
            <label>
              {t("notify.telegram.token")}
              <input
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                autoComplete="off"
                required={!editing}
                placeholder={keep}
              />
              <small>{t("notify.telegram.tokenHint")}</small>
            </label>
            <label>
              {t("notify.telegram.chatId")}
              <input value={chatId} onChange={(e) => setChatId(e.target.value)} autoComplete="off" required />
              <small>{t("notify.telegram.chatIdHint")}</small>
            </label>
          </>
        )}
        {type === "ntfy" && (
          <>
            <label>
              {t("notify.ntfy.server")}
              <input type="url" value={server} onChange={(e) => setServer(e.target.value)} placeholder="https://ntfy.sh" />
              <small>{t("notify.ntfy.serverHint")}</small>
            </label>
            <label>
              {t("notify.ntfy.topic")}
              <input
                type="password"
                value={topic}
                onChange={(e) => setTopic(e.target.value)}
                autoComplete="off"
                required={!editing || serverMoved}
                placeholder={serverMoved ? undefined : keep}
              />
              <small>{t(serverMoved ? "form.secretUrlChanged" : "notify.ntfy.topicHint")}</small>
            </label>
            <label>
              {t("notify.ntfy.token")}
              <input type="password" value={token} onChange={(e) => setToken(e.target.value)} autoComplete="off" placeholder={keep} />
              <small>{t("notify.ntfy.tokenHint")}</small>
            </label>
          </>
        )}
        {type === "webhook" && (
          <label>
            {t("notify.webhook.url")}
            <input
              type="password"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              autoComplete="off"
              required={!editing}
              placeholder={keep}
            />
            <small>{t("notify.webhook.urlHint")}</small>
          </label>
        )}
        <label className="check">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          <span>{t("notify.enabledLabel")}</span>
        </label>
        {type !== "telegram" && (
          <label className="check">
            <input type="checkbox" checked={skipTls} onChange={(e) => setSkipTls(e.target.checked)} />
            <span>
              {t("form.tls")}
              <small>{t("form.tlsHint")}</small>
            </span>
          </label>
        )}
        {save.isError && <p role="alert" className="error">{errorText(code(save.error))}</p>}
        {test.isError && <p role="alert" className="error">{errorText(code(test.error))}</p>}
        {test.data && <TestOutcome entry={test.data} />}
        <div className="dialog-actions">
          <button
            type="button"
            className="secondary"
            disabled={test.isPending}
            onClick={() => {
              if (form.current?.reportValidity()) test.mutate();
            }}
          >
            {t(test.isPending ? "notify.sending" : "notify.test")}
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

function SendLog() {
  const { t, lang } = useI18n();
  const log = useQuery({ queryKey: ["notification-log"], queryFn: api.notificationLog, refetchInterval: 15000 });
  const date = new Intl.DateTimeFormat(lang, { dateStyle: "short", timeStyle: "short" });
  return (
    <div className="stack">
      <h3>{t("notify.log")}</h3>
      {log.data?.length === 0 && <p className="muted-text">{t("notify.logEmpty")}</p>}
      {!!log.data?.length && (
        <div className="table-scroll">
          <table className="send-log">
            <thead>
              <tr>
                <th scope="col">{t("notify.log.when")}</th>
                <th scope="col">{t("notify.log.channel")}</th>
                <th scope="col">{t("notify.log.message")}</th>
                <th scope="col">{t("notify.log.result")}</th>
              </tr>
            </thead>
            <tbody>
              {log.data.map((entry) => (
                <tr key={entry.id}>
                  <td>{date.format(new Date(entry.at))}</td>
                  <td>{entry.channel}</td>
                  <td>
                    {entry.event === "test"
                      ? t("notify.log.test")
                      : entry.services.map((s) => `${s.name}: ${t(`notify.tone.${s.tone}` as Parameters<typeof t>[0])}`).join(", ")}
                  </td>
                  <td className={entry.result === "ok" ? "test-ok" : "error"}>
                    {entry.result === "ok" ? t("notify.log.ok") : errorLabel(t, entry.code)}
                    {entry.attempts > 1 && ` · ${t("notify.log.attempts", { n: entry.attempts })}`}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
