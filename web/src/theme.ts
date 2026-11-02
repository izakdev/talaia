const KEY = "talaia.theme";

export function applyTheme(theme: string, accent: string) {
  const root = document.documentElement;
  root.dataset.theme = theme;
  root.dataset.accent = accent;
  try {
    localStorage.setItem(KEY, JSON.stringify({ theme, accent }));
  } catch {
    // El tema se vuelve a recibir del servidor en cada carga.
  }
}

// Evita un parpadeo con los colores por defecto mientras llega el estado.
export function restoreTheme() {
  try {
    const saved = JSON.parse(localStorage.getItem(KEY) ?? "null");
    if (saved?.theme && saved?.accent) applyTheme(saved.theme, saved.accent);
  } catch {
    // Sin almacenamiento se usan los valores por defecto.
  }
}
