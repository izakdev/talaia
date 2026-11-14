// Colocación de los bloques del panel: cada uno queda justo debajo del último
// bloque que ocupe alguna de sus columnas, según la altura real de su contenido.

export const COLUMNS = 12;
const SINGLE_COLUMN_BELOW = 900;
const HALF_WIDTH_BELOW = 1400;

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

/** Anchos efectivos según el ancho de la ventana, en px. */
export function responsiveItems(items: StackItem[], viewport: number): StackItem[] {
  const ordered = [...items].sort(byOrder);
  if (viewport < SINGLE_COLUMN_BELOW) return ordered.map((item, i) => ({ ...item, x: 0, y: i, w: COLUMNS }));
  if (viewport >= HALF_WIDTH_BELOW) return ordered;
  // Los bloques estrechos pasan a media fila y ya no caben en sus columnas:
  // se recolocan por orden, de izquierda a derecha.
  let x = 0;
  let row = 0;
  return ordered.map((item) => {
    const w = Math.max(item.w, COLUMNS / 2);
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
