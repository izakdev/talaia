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
  "form.addTitle": "Añadir {name}",
  "catalog.title": "Elige un servicio",
  "catalog.lead": "Selecciona el servicio que quieres añadir.",
  "catalog.back": "Volver a conexiones",
  "catalog.search": "Buscar servicio",
  "catalog.empty": "Ningún servicio coincide con «{query}».",
  "catalog.soon": "Sin datos todavía",
  "catalog.desc.proxmox": "Clúster, nodos, máquinas y contenedores",
  "catalog.desc.homeassistant": "Estado, entidades y consumos",
  "catalog.desc.adguard": "Protección, consultas y bloqueos",
  "form.display": "Información en el panel",
  "form.option.show_top_domains": "Mostrar los dominios más consultados y bloqueados",
  "form.option.show_top_domains.hint": "Cualquiera que pueda abrir el panel podrá verlos.",
  "form.editTitle": "Editar {name}",
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
  "connections.working": "Funciona",
  "connections.notAvailable": "Lectura de datos aún no disponible",
  "form.test": "Probar conexión",
  "form.testing": "Probando…",
  "form.testOk": "Conexión correcta.",
  "status.checking": "Consultando…",
  "status.noResponse": "Con fallos",
  "dashboard.admin.short": "Administración",
  "dashboard.live.lost": "Sin actualización en vivo. Reintentando; los datos pueden estar desactualizados.",
  "dashboard.unavailable": "No se han podido cargar los servicios.",
  "age.checked": "Actualizado {age}",
  "age.last": "Último dato {age}",
  "age.never": "Todavía sin datos.",
  "adguard.protectionOn": "Protección activa",
  "adguard.protectionOff": "Protección desactivada",
  "adguard.dnsStopped": "Servidor DNS parado",
  "adguard.queries": "Consultas",
  "adguard.blocked": "Bloqueadas",
  "adguard.avg": "Tiempo medio",
  "adguard.topQueried": "Más consultados",
  "adguard.topBlocked": "Más bloqueados",
  "adguard.noDomains": "Sin datos en este periodo.",
  "adguard.period.hours": "Últimas {n} horas",
  "adguard.period.days": "Últimos {n} días",
  "collect.unauthorized": "Credenciales rechazadas por el servicio. Revisa la conexión.",
  "collect.timeout": "El servicio no ha respondido a tiempo.",
  "collect.unreachable": "No se puede conectar con el servicio. Revisa la URL y la red.",
  "collect.tls": "Certificado TLS no válido. Si es autofirmado, desactiva la verificación en la conexión.",
  "collect.bad_response": "La respuesta no es la esperada. Revisa que la URL sea la de este servicio.",
  "collect.secret_unreadable": "No se puede descifrar la credencial. Vuelve a escribirla en la conexión.",
  "collect.internal": "Error inesperado al consultar el servicio. Revisa los registros de Talaia.",
  "error.not_supported": "Esta integración aún no está disponible.",
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
  "form.addTitle": "Add {name}",
  "catalog.title": "Choose a service",
  "catalog.lead": "Select the service you want to add.",
  "catalog.back": "Back to connections",
  "catalog.search": "Search services",
  "catalog.empty": "No service matches “{query}”.",
  "catalog.soon": "No data yet",
  "catalog.desc.proxmox": "Cluster, nodes, machines and containers",
  "catalog.desc.homeassistant": "Status, entities and consumption",
  "catalog.desc.adguard": "Protection, queries and blocks",
  "form.display": "Information on the dashboard",
  "form.option.show_top_domains": "Show the most queried and blocked domains",
  "form.option.show_top_domains.hint": "Anyone who can open the dashboard will be able to see them.",
  "form.editTitle": "Edit {name}",
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
  "connections.working": "Working",
  "connections.notAvailable": "Data reading not available yet",
  "form.test": "Test connection",
  "form.testing": "Testing…",
  "form.testOk": "Connection works.",
  "status.checking": "Checking…",
  "status.noResponse": "Failing",
  "dashboard.admin.short": "Administration",
  "dashboard.live.lost": "Live updates lost. Retrying; data may be out of date.",
  "dashboard.unavailable": "Services could not be loaded.",
  "age.checked": "Updated {age}",
  "age.last": "Last data {age}",
  "age.never": "No data yet.",
  "adguard.protectionOn": "Protection on",
  "adguard.protectionOff": "Protection off",
  "adguard.dnsStopped": "DNS server stopped",
  "adguard.queries": "Queries",
  "adguard.blocked": "Blocked",
  "adguard.avg": "Average time",
  "adguard.topQueried": "Most queried",
  "adguard.topBlocked": "Most blocked",
  "adguard.noDomains": "No data in this period.",
  "adguard.period.hours": "Last {n} hours",
  "adguard.period.days": "Last {n} days",
  "collect.unauthorized": "The service rejected the credentials. Check the connection.",
  "collect.timeout": "The service did not respond in time.",
  "collect.unreachable": "Can't connect to the service. Check the URL and the network.",
  "collect.tls": "Invalid TLS certificate. If it's self-signed, turn off verification in the connection.",
  "collect.bad_response": "Unexpected response. Check that the URL belongs to this service.",
  "collect.secret_unreadable": "The credential can't be decrypted. Enter it again in the connection.",
  "collect.internal": "Unexpected error reading the service. Check Talaia's logs.",
  "error.not_supported": "This integration isn't available yet.",
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
  collectError: (code?: string) => string;
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
      collectError: (code) => (dict as Record<string, string>)[`collect.${code}`] ?? dict["collect.internal"],
    };
  }, [lang]);
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useI18n(): I18n {
  const value = useContext(Context);
  if (!value) throw new Error("I18nProvider ausente");
  return value;
}
