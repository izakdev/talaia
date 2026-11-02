import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type AppSettings } from "./api";
import { useI18n } from "./i18n";

const THEMES = ["auto", "light", "dark"] as const;
const ACCENTS = ["amber", "teal", "lime", "coral", "pink"] as const;

const code = (error: unknown) => (error instanceof ApiError ? error.code : "internal");

export default function Settings() {
  const { t } = useI18n();
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.settings });
  return (
    <main className="panel settings">
      <div>
        <h1>{t("settings.title")}</h1>
      </div>
      {settings.data && <GeneralSettings initial={settings.data} />}
      <PasswordForm />
    </main>
  );
}

function GeneralSettings({ initial }: { initial: AppSettings }) {
  const { t, errorText } = useI18n();
  const client = useQueryClient();
  const [form, setForm] = useState(initial);
  const save = useMutation({
    mutationFn: () => api.saveSettings(form),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["state"] });
      await client.invalidateQueries({ queryKey: ["settings"] });
    },
  });
  useEffect(() => save.reset(), [form]); // eslint-disable-line react-hooks/exhaustive-deps
  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    save.mutate();
  };
  return (
    <form className="stack settings-section" onSubmit={onSubmit}>
      <h2>{t("settings.appearance")}</h2>
      <fieldset className="options">
        <legend>{t("settings.theme")}</legend>
        <div className="choice-row">
          {THEMES.map((theme) => (
            <label key={theme}>
              <input type="radio" name="theme" checked={form.theme === theme} onChange={() => setForm({ ...form, theme })} />
              {t(`settings.theme.${theme}`)}
            </label>
          ))}
        </div>
      </fieldset>
      <fieldset className="options">
        <legend>{t("settings.accent")}</legend>
        <div className="choice-row">
          {ACCENTS.map((accent) => (
            <label key={accent}>
              <input
                type="radio"
                name="accent"
                checked={form.accent === accent}
                onChange={() => setForm({ ...form, accent })}
              />
              <span className={`swatch swatch-${accent}`} aria-hidden="true" />
              {t(`settings.accent.${accent}`)}
            </label>
          ))}
        </div>
      </fieldset>
      <h2>{t("settings.access")}</h2>
      <label className="check">
        <input
          type="checkbox"
          checked={form.require_login}
          onChange={(e) => setForm({ ...form, require_login: e.target.checked })}
        />
        <span>
          {t("settings.requireLogin")}
          <small>{t("settings.requireLogin.hint")}</small>
        </span>
      </label>
      {save.isError && <p role="alert" className="error">{errorText(code(save.error))}</p>}
      {save.isSuccess && <p role="status" className="test-ok">{t("settings.saved")}</p>}
      <div>
        <button className="primary" disabled={save.isPending}>
          {t("settings.save")}
        </button>
      </div>
    </form>
  );
}

function PasswordForm() {
  const { t, errorText } = useI18n();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [repeat, setRepeat] = useState("");
  const change = useMutation({
    mutationFn: () => api.changePassword(current, next),
    onSuccess: () => {
      setCurrent("");
      setNext("");
      setRepeat("");
    },
  });
  const mismatch = repeat !== "" && next !== repeat;
  return (
    <form
      className="stack settings-section"
      onSubmit={(event) => {
        event.preventDefault();
        if (!mismatch) change.mutate();
      }}
    >
      <h2>{t("settings.password")}</h2>
      <label>
        {t("settings.password.current")}
        <input type="password" value={current} onChange={(e) => setCurrent(e.target.value)} autoComplete="current-password" required />
      </label>
      <label>
        {t("settings.password.new")}
        <input type="password" value={next} onChange={(e) => setNext(e.target.value)} autoComplete="new-password" minLength={12} required />
        <small>{t("field.passwordHint")}</small>
      </label>
      <label>
        {t("settings.password.repeat")}
        <input type="password" value={repeat} onChange={(e) => setRepeat(e.target.value)} autoComplete="new-password" required />
      </label>
      {mismatch && <p role="alert" className="error">{t("settings.password.mismatch")}</p>}
      {change.isError && <p role="alert" className="error">{errorText(code(change.error))}</p>}
      {change.isSuccess && <p role="status" className="test-ok">{t("settings.password.changed")}</p>}
      <div>
        <button className="primary" disabled={change.isPending || mismatch}>
          {t("settings.password.submit")}
        </button>
      </div>
    </form>
  );
}
