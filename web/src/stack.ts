// Colocación de los bloques del panel: cada uno queda justo debajo del último
// bloque que ocupe alguna de sus columnas, según la altura real de su contenido.

export const COLUMNS = 12;
// Anchos que ofrece el editor: un cuarto, un tercio, media fila, dos tercios y la fila.
export const WIDTHS = [3, 4, 6, 8, 12];
// Un bloque más estrecho que esto se ensancha al siguiente ancho del editor.
export const MIN_BLOCK_PX = 340;
// Con un área de bloques más estrecha, todos van en una sola columna.
export const SINGLE_COLUMN_BELOW_PX = 700;

export interface StackItem {
  id: string;
  x: number;
  y: number;
  w: number;
}

export interface Placed {
  id: string;
  x: number;
  w: number;
  top: number;
}

const byOrder = (a: StackItem, b: StackItem) => a.y - b.y || a.x - b.x;

/** Anchos efectivos según el ancho real del área de bloques, en px. */
export function responsiveItems(items: StackItem[], areaWidth: number, gap: number): StackItem[] {
  const ordered = [...items].sort(byOrder);
  if (areaWidth < SINGLE_COLUMN_BELOW_PX) return ordered.map((item, i) => ({ ...item, x: 0, y: i, w: COLUMNS }));
  const pixels = (w: number) => ((areaWidth + gap) / COLUMNS) * w - gap;
  // Un ancho que llega se respeta aunque no sea de la escala del editor (el tirador deja elegir cualquiera).
  const widen = (w: number) =>
    pixels(w) >= MIN_BLOCK_PX ? w : (WIDTHS.find((option) => option > w && pixels(option) >= MIN_BLOCK_PX) ?? COLUMNS);
  if (ordered.every((item) => widen(item.w) === item.w)) return ordered;
  // Los bloques ensanchados ya no caben en sus columnas: se recolocan por orden, de izquierda a derecha.
  let x = 0;
  let row = 0;
  return ordered.map((item) => {
    const w = widen(item.w);
    if (x + w > COLUMNS) {
      x = 0;
      row++;
    }
    const placed = { ...item, x, y: row, w };
    x += w;
    return placed;
  });
}

/** Apila por orden (y, x): el top de cada bloque es el fondo más bajo ocupado en sus columnas. */
export function stackLayout(
  items: StackItem[],
  heights: Record<string, number>,
  gap: number,
): { placed: Placed[]; height: number } {
  const bottoms = new Array<number>(COLUMNS).fill(0);
  const placed = [...items].sort(byOrder).map((item) => {
    const columns = bottoms.slice(item.x, item.x + item.w);
    const top = Math.max(0, ...columns);
    const bottom = top + (heights[item.id] ?? 0) + gap;
    for (let c = item.x; c < item.x + item.w; c++) bottoms[c] = bottom;
    return { id: item.id, x: item.x, w: item.w, top };
  });
  return { placed, height: Math.max(0, Math.max(...bottoms) - gap) };
}
