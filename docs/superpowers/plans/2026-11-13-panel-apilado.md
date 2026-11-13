# Bloques que se apilan solos y panel a todo el ancho: plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** En el panel, cada bloque se coloca justo debajo del último bloque de sus columnas según su altura real. El panel ocupa todo el ancho de la ventana, y el editor muestra lo mismo.

**Architecture:** Una función pura `stackLayout` en `web/src/stack.ts` calcula la posición de cada bloque a partir de sus columnas, su orden y las alturas medidas. El panel pasa del CSS grid por filas a un contenedor relativo con bloques absolutos, medidos con `ResizeObserver`. El editor calcula `h` a partir de la vista previa, solo permite cambiar el ancho, y su estado «sin guardar» ignora `h`.

**Tech Stack:** React 19 + TypeScript, react-grid-layout 2, Go (API y store), Playwright (E2E).

**Spec:** `docs/superpowers/specs/2026-11-13-panel-apilado-design.md`

## Global Constraints

- Sin cambios de esquema ni de formato en la API: se siguen guardando `x`, `y`, `w` y `h` en 12 columnas.
- La API mantiene `minWidgetWidth = 2`. El editor usa `minW = 3`.
- Anchos de los botones: 3, 4, 6, 8 y 12. El tirador lateral (`e`) permite cualquier ancho de 3 a 12.
- Ancho por defecto de un bloque nuevo: 4, en el editor (`NEW_WIDTH`) y en el store (`DefaultWidgetWidth`).
- Umbrales del panel: por debajo de 900 px, una columna; de 900 a 1400 px, anchos menores de 6 pasan a 6 y se recolocan en dos mitades por orden; por encima de 1400 px, la disposición del editor.
- Sin animaciones de posición en el panel.
- Rama `hito-13-panel-apilado` desde `main`. Commits en español con la línea `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Documentación en el mismo cambio: 01, 03, 04, 05, 06 y los dos manuales. Capturas en los dos idiomas con `make screenshots`.

## Review Focus

1. **Un bloque cambia de altura con datos en vivo** (aparece una sección, crece una lista): los de debajo se recolocan sin solaparse. Se prueba en la tarea 2, con un estado del simulador que alarga un bloque.
2. **Abrir el editor no marca «Cambios sin guardar»**, ni tampoco cuando llegan datos nuevos. Se prueba en la tarea 3 con la aserción de «Sin cambios» tras esperar más de un ciclo de refresco.
3. **Pantalla mediana (1100 px) con bloques de ancho 3 y 4:** ningún bloque se sale de la ventana ni se solapa. Se prueba en la tarea 2 con un viewport de 1100.
4. **Panel guardado antes, con `h = 10` y `y` antiguos:** se pinta en el mismo orden y columnas, sin huecos. Lo cubren los escenarios E2E existentes, que guardan con el formato antiguo antes de la tarea 3.
5. **Móvil (375 px):** una columna en orden y sin desbordamiento horizontal. Lo cubre E2E-13, que debe seguir pasando.

---

### Task 1: Apilado puro y ancho por defecto en el store

**Files:**
- Create: `web/src/stack.ts`
- Modify: `internal/store/panel.go` (`DefaultWidgetWidth` = 4; `placeNewConnection` alterna tres columnas: `x = (count % 3) * 4`)
- Test: `internal/store/store_test.go` (`TestNewConnectionsFillTwoColumns` → `TestNewConnectionsFillThreeColumns`)

**Interfaces:**
- Produces:
  ```ts
  export interface StackItem { id: string; x: number; y: number; w: number }
  export interface Placed { id: string; x: number; w: number; top: number }
  export const COLUMNS = 12;
  /** Anchos efectivos según el ancho de la ventana (px). */
  export function responsiveItems(items: StackItem[], viewport: number): StackItem[];
  /** Apila por orden (y, x): top = máximo fondo ocupado en sus columnas. */
  export function stackLayout(items: StackItem[], heights: Record<string, number>, gap: number): { placed: Placed[]; height: number };
  ```
- `responsiveItems`: viewport < 900 → todos `x = 0, w = 12`, en orden; 900 ≤ viewport < 1400 → `w = max(w, 6)`, después se recolocan por orden de izquierda a derecha (`x` acumulado, salto de línea si `x + w > 12`), con una `y` creciente para conservar el orden; ≥ 1400 → sin cambios.

- [ ] **Step 1: Write the failing test** `TestNewConnectionsFillThreeColumns`: con cuatro conexiones creadas, los widgets tienen `X` 0, 4, 8 y 0, `Y` 0, 0, 0 y `DefaultWidgetHeight`, y `W` igual a 4.
- [ ] **Step 2: Run** `go test ./internal/store -run TestNewConnectionsFill`. Expected: FAIL.
- [ ] **Step 3: Implement** the store change and `web/src/stack.ts` (pure functions, no React).
- [ ] **Step 4: Run** `go test ./internal/store` and `npm --prefix web run typecheck`. Expected: PASS.
- [ ] **Step 5: Update** the test name in `docs/proyecto/04-plan-de-pruebas.md`, including its traceability row, and run `make docs-check`.
- [ ] **Step 6: Commit** «Añade el apilado de bloques y el ancho por defecto de un tercio».

### Task 2: Panel apilado y a todo el ancho

**Files:**
- Modify: `web/src/Dashboard.tsx` (bloque `.panel-grid`)
- Modify: `web/src/styles.css` (`.lookout-main` sin `max-width`; `.panel-grid` relativo; hijos absolutos; quitar la regla `grid-column … !important` de 900 px)
- Modify: `e2e/fake-homeassistant.mjs` si hace falta un estado que alargue el bloque (Review Focus 1)
- Test: `e2e/run.mjs` (E2E-38 nuevo; E2E-24 deja de leer `style.gridColumn`)

**Interfaces:**
- Consumes: `stackLayout` and `responsiveItems` from Task 1.
- Produces: each block wrapper is `<div class="panel-cell" data-x data-w style="left; width; top">`. The `data-x` and `data-w` values are the effective columns. Tests use them, not styles.

- [ ] **Step 1: Write the failing E2E** `["E2E-38", "Bloques apilados por su altura real y panel a todo el ancho"]`:
  - En el editor, coloca dos bloques cortos uno encima del otro en las columnas 0-5 y uno largo en 6-11, y guarda.
  - A 1600 px: el `top` del segundo bloque corto es igual al fondo del primero más el espacio entre bloques (±2 px), y menor que el fondo del largo.
  - `.lookout-main` mide más de 1472 px de ancho.
  - Si el simulador alarga el bloque de arriba, el de debajo baja y no se solapan.
  - A 1100 px: ningún `.panel-cell` se sale de la ventana ni se solapa con otro.
- [ ] **Step 2: Run** `make e2e`. Expected: FAIL at E2E-38.
- [ ] **Step 3: Implement** the panel:
  - Measure each `.service` with one `ResizeObserver`.
  - Calculate `responsiveItems(widgets, window.innerWidth)` and then `stackLayout(…, heights, 24)`.
  - Position with `left = x / 12`, `width = w / 12` minus the gap, and `top` in px. The container gets the total height.
  - Recalculate on window resize.
  - Until every height is measured, render in normal flow.
- [ ] **Step 4: Adapt E2E-24** to read `data-w` and run `make e2e`. Expected: PASS, including E2E-13 at 375 px.
- [ ] **Step 5: Commit** «Apila los bloques del panel por su altura real y usa todo el ancho».

### Task 3: Editor con alturas reales y anchos por botones

**Files:**
- Modify: `web/src/PanelEditor.tsx`:
  - `NEW_WIDTH = 4`, `MIN_WIDTH = 3`.
  - `resizeConfig.handles = ["e"]`.
  - `resizeWidget` recorre `[3, 4, 6, 8, 12]`.
  - `dirty` compara sin `h`.
  - Medición de la vista previa con `ResizeObserver` → `h = ceil((px + 16) / (28 + 16))`, limitada a `[2, 60]`.
- Modify: `web/src/styles.css`: `.panel-wide` sin `max-width`, y `.editor-widget-preview` sin recorte vertical, para medir su altura real.
- Test: `e2e/run.mjs` (E2E-24)

**Interfaces:**
- Consumes: nothing from earlier tasks. The editor keeps sending `PlacedWidget` to `api.savePanel` unchanged.

- [ ] **Step 1: Extend E2E-24** (failing):
  - On opening the editor, wait 6 s (more than the 5 s refresh of the simulators) and check that it still says «Sin cambios».
  - «Más ancho» on a block of width 4 leaves it at 6, and again at 8.
  - The corner handle no longer changes the height: dragging down keeps the bounding box height within ±4 px.
- [ ] **Step 2: Run** `make e2e`. Expected: FAIL at E2E-24.
- [ ] **Step 3: Implement** the editor changes listed above.
- [ ] **Step 4: Run** `make test`, `make e2e` and `make docs-check`. Expected: PASS.
- [ ] **Step 5: Commit** «Ajusta el editor a las alturas reales y a anchos de cuarto a fila entera».

### Task 4: Documentación, capturas y verificación

**Files:**
- Modify:
  - `docs/proyecto/01-requisitos.md`: RF-22 o un RF nuevo, hito 13.
  - `docs/proyecto/03-diseno-funcional.md`: Altura, Recuadro, anchos y umbrales.
  - `docs/proyecto/04-plan-de-pruebas.md`: E2E-38, E2E-24 y trazabilidad.
  - `docs/proyecto/05-decisiones.md`: DEC-41, que sustituye en parte a DEC-38 en la altura de los bloques.
  - `docs/proyecto/06-estado-y-cambios.md`: fila e historial del hito 13.
  - Los dos manuales: «Organizar el panel».
  - Capturas de `docs/usuario/img`, `docs/user/img` y `docs/img`.
- Commit the spec and this plan as well.

- [ ] **Step 1: Write** the documentation above.
- [ ] **Step 2: Run** `make screenshots` and keep only the images that change for real reasons. Look at 07, 11, 15, 16, 18 and the cover images.
- [ ] **Step 3: Run** `make test`, `make e2e` and `make docs-check`. Expected: PASS.
- [ ] **Step 4: Run** the `revisor-de-hito` subagent and fix what it reports.
- [ ] **Step 5: Commit** «Documenta el panel apilado (hito 13)».
