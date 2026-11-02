import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

const es = {
  "app.loading": "Cargando…",
  "app.unreachable": "No se puede conectar con Talaia. Comprueba que el servicio está en marcha y recarga la página.",
  "app.retry": "Reintentar",
  "setup.title": "Crea el administrador",
  "setup.lead": "Esta cuenta configura lo que se muestra en el panel.",
  "setup.submit": "Crear administrador",
  "login.title": "Entra en la administración",
  "login.submit": "Entrar",
  "field.username": "Usuario",
  "field.password": "Contraseña",
  "field.passwordHint": "Mínimo 12 caracteres.",
  "dashboard.title": "Aún no hay servicios en este panel",
  "dashboard.lead": "Empieza por añadir las conexiones con tus servicios.",
  "dashboard.admin": "Ir a la administración",
  "admin.dashboard": "Ver el panel",
  "admin.logout": "Cerrar sesión",
  "connections.title": "Conexiones",
  "connections.lead": "Los servicios de los que Talaia leerá su estado. Las credenciales se guardan cifradas.",
  "connections.add": "Añadir conexión",
  "connections.emptyTitle": "Todavía no hay conexiones",
  "connections.emptyLead": "Añade Proxmox, Home Assistant o AdGuard para empezar.",
  "connections.every": "cada {n} s",
  "connections.tlsOff": "TLS sin verificar",
  "connections.loadError": "No se han podido cargar las conexiones.",
  "connections.edit": "Editar",
  "connections.delete": "Eliminar",
  "connections.confirmDelete": "¿Eliminar {name}?",
  "form.addTitle": "Añadir conexión",
  "form.editTitle": "Editar {name}",
  "form.type": "Servicio",
  "form.name": "Nombre",
  "form.url": "URL",
  "form.username": "Usuario de AdGuard",
  "form.secret.proxmox": "API token de Proxmox",
  "form.secret.homeassistant": "Token de larga duración",
  "form.secret.adguard": "Contraseña de AdGuard",
  "form.secretHint.proxmox": "Formato: usuario@pam!nombre=valor. Basta con el rol PVEAuditor.",
  "form.secretHint.homeassistant": "Se crea en tu perfil de Home Assistant, en Seguridad.",
  "form.secretHint.adguard": "",
  "form.secretKeep": "Déjalo vacío para conservar el actual.",
  "form.secretUrlChanged": "Al cambiar la URL hay que volver a escribirlo.",
  "form.interval": "Refresco, en segundos",
  "form.tls": "No verificar el certificado TLS",
  "form.tlsHint": "Solo para certificados autofirmados en una red de confianza.",
  "form.save": "Guardar",
  "form.cancel": "Cancelar",
  "type.proxmox": "Proxmox",
  "type.homeassistant": "Home Assistant",
  "type.adguard": "AdGuard",
  "error.network": "No se puede conectar con Talaia.",
  "error.internal": "Talaia ha fallado al procesar la petición. Revisa sus registros.",
  "error.invalid_credentials": "Usuario o contraseña incorrectos.",
  "error.too_many_attempts": "Demasiados intentos fallidos. Espera 15 minutos.",
  "error.weak_password": "La contraseña debe tener al menos 12 caracteres.",
  "error.invalid_username": "Escribe un usuario.",
  "error.already_setup": "Ya existe un administrador. Recarga la página para entrar.",
  "error.invalid_name": "Escribe un nombre de hasta 80 caracteres.",
  "error.invalid_url": "Escribe una URL con http:// o https://, sin usuario ni parámetros.",
  "error.invalid_interval": "El refresco debe estar entre 5 y 3600 segundos.",
  "error.invalid_secret": "Escribe la credencial.",
  "error.invalid_proxmox_token": "El token debe tener el formato usuario@pam!nombre=valor.",
  "error.secret_required_for_url_change": "Al cambiar la URL hay que volver a escribir la credencial.",
  "error.unauthorized": "La sesión ha caducado. Vuelve a entrar.",
  "error.not_found": "Esta conexión ya no existe.",
};

type Key = keyof typeof es;

const en: Record<Key, string> = {
  "app.loading": "Loading…",
  "app.unreachable": "Can't reach Talaia. Check that the service is running and reload the page.",
  "app.retry": "Try again",
  "setup.title": "Create the administrator",
  "setup.lead": "This account configures what the dashboard shows.",
  "setup.submit": "Create administrator",
  "login.title": "Sign in to administration",
  "login.submit": "Sign in",
  "field.username": "Username",
  "field.password": "Password",
  "field.passwordHint": "At least 12 characters.",
  "dashboard.title": "No services on this dashboard yet",
  "dashboard.lead": "Start by adding the connections to your services.",
  "dashboard.admin": "Go to administration",
  "admin.dashboard": "View dashboard",
  "admin.logout": "Sign out",
  "connections.title": "Connections",
  "connections.lead": "The services Talaia reads status from. Credentials are stored encrypted.",
  "connections.add": "Add connection",
  "connections.emptyTitle": "No connections yet",
  "connections.emptyLead": "Add Proxmox, Home Assistant or AdGuard to get started.",
  "connections.every": "every {n} s",
  "connections.tlsOff": "TLS not verified",
  "connections.loadError": "Connections could not be loaded.",
  "connections.edit": "Edit",
  "connections.delete": "Delete",
  "connections.confirmDelete": "Delete {name}?",
  "form.addTitle": "Add connection",
  "form.editTitle": "Edit {name}",
  "form.type": "Service",
  "form.name": "Name",
  "form.url": "URL",
  "form.username": "AdGuard username",
  "form.secret.proxmox": "Proxmox API token",
  "form.secret.homeassistant": "Long-lived access token",
  "form.secret.adguard": "AdGuard password",
  "form.secretHint.proxmox": "Format: user@pam!name=value. The PVEAuditor role is enough.",
  "form.secretHint.homeassistant": "Create it in your Home Assistant profile, under Security.",
  "form.secretHint.adguard": "",
  "form.secretKeep": "Leave empty to keep the current one.",
  "form.secretUrlChanged": "Changing the URL requires entering it again.",
  "form.interval": "Refresh, in seconds",
  "form.tls": "Don't verify the TLS certificate",
  "form.tlsHint": "Only for self-signed certificates on a trusted network.",
  "form.save": "Save",
  "form.cancel": "Cancel",
  "type.proxmox": "Proxmox",
  "type.homeassistant": "Home Assistant",
  "type.adguard": "AdGuard",
  "error.network": "Can't reach Talaia.",
  "error.internal": "Talaia failed to process the request. Check its logs.",
  "error.invalid_credentials": "Wrong username or password.",
  "error.too_many_attempts": "Too many failed attempts. Wait 15 minutes.",
  "error.weak_password": "The password must have at least 12 characters.",
  "error.invalid_username": "Enter a username.",
  "error.already_setup": "An administrator already exists. Reload the page to sign in.",
  "error.invalid_name": "Enter a name of up to 80 characters.",
  "error.invalid_url": "Enter a URL with http:// or https://, without credentials or parameters.",
  "error.invalid_interval": "Refresh must be between 5 and 3600 seconds.",
  "error.invalid_secret": "Enter the credential.",
  "error.invalid_proxmox_token": "The token must have the format user@pam!name=value.",
  "error.secret_required_for_url_change": "Changing the URL requires entering the credential again.",
  "error.unauthorized": "Your session has expired. Sign in again.",
  "error.not_found": "This connection no longer exists.",
};

const dictionaries = { es, en };
export type Language = keyof typeof dictionaries;

function initialLanguage(): Language {
  try {
    const saved = localStorage.getItem("talaia.lang");
    if (saved === "es" || saved === "en") return saved;
  } catch {
    // El almacenamiento puede no estar disponible.
  }
  return navigator.language.toLowerCase().startsWith("es") ? "es" : "en";
}

interface I18n {
  lang: Language;
  setLang: (lang: Language) => void;
  t: (key: Key, vars?: Record<string, string | number>) => string;
  errorText: (code: string) => string;
}

const Context = createContext<I18n | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLang] = useState<Language>(initialLanguage);
  useEffect(() => {
    document.documentElement.lang = lang;
    try {
      localStorage.setItem("talaia.lang", lang);
    } catch {
      // El idioma solo se recuerda si hay almacenamiento.
    }
  }, [lang]);
  const value = useMemo<I18n>(() => {
    const dict = dictionaries[lang];
    const t: I18n["t"] = (key, vars) =>
      Object.entries(vars ?? {}).reduce((text, [k, v]) => text.replace(`{${k}}`, String(v)), dict[key]);
    return {
      lang,
      setLang,
      t,
      errorText: (code) => (dict as Record<string, string>)[`error.${code}`] ?? dict["error.internal"],
    };
  }, [lang]);
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useI18n(): I18n {
  const value = useContext(Context);
  if (!value) throw new Error("I18nProvider ausente");
  return value;
}
