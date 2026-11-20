# Iconos en los bloques — plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Objetivo:** cada bloque del panel muestra el icono de su servicio junto al nombre. En cada conexión se puede elegir cualquier logotipo de `simple-icons` o «Sin icono».

**Arquitectura:** la compilación web genera `web/dist/icons/<slug>.svg` (uno por icono) y `web/dist/icons/index.json`. Go los embebe con el resto de `dist`, carga el índice en un paquete `internal/icons`, valida y resuelve el icono de cada conexión, y lo publica en la instantánea (`icon: {slug, title, hex}` o `null`). El frontend pinta el icono como máscara CSS con la URL del SVG, y el selector usa el índice.

**Tecnología:** Go (chi, SQLite), React + TypeScript, Vite, `simple-icons` 16, Playwright (E2E en `e2e/run.mjs`).

**Diseño:** `iconos-en-bloques-diseno.md`, en la carpeta de notas compartida de la sesión.

## Restricciones globales

- Sin peticiones a terceros: todo sale del binario.
- `connections.icon` puede valer `''` (el icono del tipo, que sigue al catálogo), `'none'` (sin icono) o un slug del índice. Cualquier otro valor da 400 `invalid_icon`.
- La API pública solo expone `slug`, `title` y `hex`.
- La atalaya de la cabecera no lleva iconos.
- Documentación en español; los manuales y `README` cambian en los dos idiomas. Mensajes de commit en español.
- La decisión nueva es DEC-54; si el hito 17 se fusiona antes, el número que siga al suyo.
- Iconos propios: `talaia-nut` (la batería de `catalog.ts`, `16A34A`). Con el hito 17, también `talaia-availability`.

## Puntos de revisión

1. Un slug guardado que desaparece al actualizar `simple-icons`: el bloque se queda sin icono, sin error, y el formulario muestra «Sin icono» hasta que se elija otro. Prueba: `TestResolveUnknownStoredSlug`.
2. Marca negra en modo oscuro (por ejemplo, `github`, `000000`): se pinta con el color del texto. Prueba: `lowContrast` en E2E con el tema oscuro.
3. Nombre largo en el móvil: el icono no se encoge (`flex: none`) y el texto se recorta con elipsis. Prueba: E2E a 375 px.
4. Tildes y mayúsculas en la búsqueda: «pihole», «Pi-hole» y «PI HOLE» encuentran Pi-hole. Prueba: `searchIcons` en E2E.
5. Conexión creada antes de la migración: `icon = ''`, y muestra el icono de su tipo. Prueba: `TestMigrationKeepsDefaultIcon`.

---

### Tarea 1: generar los iconos en la compilación web

**Archivos:**
- Crear: `web/scripts/icons.mjs`, `web/icons/talaia-nut.svg`
- Modificar: `web/package.json` (`"build": "tsc --noEmit && vite build && node scripts/icons.mjs"`)

**Interfaces:**
- Produce:
  - `web/dist/icons/<slug>.svg`: SVG de 24×24 con un único `<path>` y sin `fill`.
  - `web/dist/icons/index.json`: `[{"slug":"pihole","title":"Pi-hole","hex":"96060C"}, …]`, ordenado por `title`, con los iconos propios incluidos.

- [ ] **Paso 1:** `icons.mjs` lee `simple-icons/data/simple-icons.json` y los SVG de `simple-icons/icons/`. Copia cada uno sin `<title>` y añade los SVG de `web/icons/`. El título y el color de los propios van en el mismo script (`talaia-nut`: «Network UPS Tools», `16A34A`).
- [ ] **Paso 2:** ejecuta `npm --prefix web run build`. Debe dar 3.468 SVG en `web/dist/icons/`, `index.json` por debajo de 200 KB y `jq '.[]|select(.slug=="pihole")'` con un resultado.
- [ ] **Paso 3:** haz commit: «Genera los iconos de simple-icons en la compilación».

### Tarea 2: paquete `internal/icons`

**Archivos:**
- Crear: `internal/icons/icons.go`, `internal/icons/icons_test.go`

**Interfaces:**
- Produce:
  - `type Icon struct { Slug string \`json:"slug"\`; Title string \`json:"title"\`; Hex string \`json:"hex"\` }`
  - `type Set struct{ … }`
  - `func Load(fsys fs.FS) Set`: lee `icons/index.json`; si no está o está mal formado, devuelve un conjunto vacío.
  - `func (Set) Has(slug string) bool`
  - `func (Set) Resolve(connType, stored string) *Icon`: `''` da el icono por defecto del tipo, `'none'` da `nil`, y un slug que no está en el índice también da `nil`.
  - `var Defaults = map[string]string{"adguard":"adguard","homeassistant":"homeassistant","proxmox":"proxmox","opnsense":"opnsense","portainer":"portainer","qnap":"qnap","nut":"talaia-nut","wgdashboard":"wireguard"}`
  - `const None = "none"`

- [ ] **Paso 1:** escribe las pruebas con un `fstest.MapFS` y un índice de tres iconos:
  - `TestResolveDefault`: `Resolve("proxmox","")` devuelve `{proxmox, Proxmox, E57000}`.
  - `TestResolveNone`: devuelve `nil`.
  - `TestResolveCustom`: `Resolve("adguard","pihole")` devuelve `Slug=="pihole"`.
  - `TestResolveUnknownStoredSlug`: devuelve `nil`.
  - `TestLoadMissingIndex`: un conjunto vacío, sin pánico.
- [ ] **Paso 2:** ejecuta `go test ./internal/icons/`. Debe FALLAR.
- [ ] **Paso 3:** implementa el paquete.
- [ ] **Paso 4:** ejecuta `go test ./internal/icons/`. Debe PASAR.
- [ ] **Paso 5:** haz commit.

### Tarea 3: columna `icon` en el almacén

**Archivos:**
- Modificar: `internal/store/store.go` (una migración nueva al final de `migrations`: `ALTER TABLE connections ADD COLUMN icon TEXT NOT NULL DEFAULT '';`, más `Connection.Icon`, `connectionColumns`, `scanConnection`, el insert y el update)
- Probar: `internal/store/store_test.go`

- [ ] **Paso 1:** escribe las pruebas:
  - `TestConnectionIconRoundTrip`: crea con `Icon:"pihole"`, la lee igual, la actualiza a `"none"` y la relee.
  - `TestMigrationKeepsDefaultIcon`: una conexión creada sin el campo lee `Icon==""`.
- [ ] **Paso 2:** ejecuta las pruebas: deben FALLAR. Implementa y deben PASAR (`go test ./internal/store/`).
- [ ] **Paso 3:** haz commit.

### Tarea 4: API, instantánea y `/icons/`

**Archivos:**
- Modificar:
  - `internal/api/api.go`: `connectionIn.Icon` y `connectionOut.Icon` (`json:"icon"`, el valor guardado tal cual); en `validateConnection`, `invalid_icon`; `Server.icons icons.Set`, cargado en `New` con `icons.Load(web)`; en `serveWeb`, `Cache-Control: public, max-age=604800` para `icons/`.
  - `internal/collector/collector.go`: `Snapshot.Icon *icons.Icon \`json:"icon"\``; `Manager.SetIcons(icons.Set)`; resolver con `Resolve(c.Type, c.Icon)` donde se construyen las instantáneas.
  - `cmd/talaia/main.go`: `manager.SetIcons(icons.Load(web.Dist()))`.
- Probar: `internal/api/api_test.go`, con el `MapFS` de las pruebas y un `icons/index.json` mínimo.

**Interfaces:**
- Consume: `icons.Set`, `icons.Icon` y `icons.None` (tarea 2), y `store.Connection.Icon` (tarea 3).
- Produce, para el frontend:
  - `ConnectionState.icon: {slug, title, hex} | null`
  - `Connection.icon: string`
  - el error `invalid_icon`

- [ ] **Paso 1:** escribe las pruebas:
  - `TestDashboardDefaultIcon`: `/api/dashboard` incluye `"icon":{"slug":"adguard"`.
  - `TestConnectionIconChange`: un PUT con `icon:"pihole"` hace que el panel diga `"slug":"pihole"`; con `"none"`, dice `"icon":null`.
  - `TestConnectionInvalidIcon`: con `icon:"no-existe"` responde 400 `invalid_icon`.
  - `TestEveryTypeHasDefaultIcon`: cada clave de `connectionTypes` está en `icons.Defaults`.
  - `TestIconsCached`: `GET /icons/adguard.svg` lleva `max-age=604800`.
- [ ] **Paso 2:** ejecuta las pruebas: deben FALLAR. Implementa y deben PASAR (`go test -race ./...`).
- [ ] **Paso 3:** haz commit.

### Tarea 5: el icono en el bloque

**Archivos:**
- Modificar:
  - `web/src/api.ts`: `ConnectionState.icon` y `Connection.icon`.
  - `web/src/chrome.tsx`: añadir `BrandIcon`.
  - `web/src/Dashboard.tsx`: en `Service`, el icono va antes del nombre, dentro del `<h2>` y del enlace.
  - `web/src/styles.css`.

**Interfaces:**
- Produce:
  - `function BrandIcon({ icon, className }: { icon: { slug: string; title: string; hex: string } | null; className?: string })`: devuelve `null` sin icono; si no, un `<span class="brand-icon" aria-hidden="true">` con `--icon-url: url(/icons/<slug>.svg)` y `--icon-color: #<hex>`.
  - `function lowContrast(hex: string): { dark: boolean; light: boolean }`: luminancia relativa por debajo de 0,08 da `dark`; por encima de 0,85, `light`. Añade las clases `low-on-dark` y `low-on-light`.
- CSS:
  - `.brand-icon`: `inline-block`, `1.1em` de lado, `flex: none`, `background: var(--icon-color)`, `mask: var(--icon-url) center / contain no-repeat`.
  - Las clases `low-on-*` usan `currentColor` bajo los mismos selectores de tema oscuro y claro que ya usa `styles.css`.
  - El `<h2>` del bloque usa `display: flex`, `gap`, `min-width: 0`, y el texto se recorta con `text-overflow: ellipsis`.

- [ ] **Paso 1:** implementa y ejecuta `npm --prefix web run typecheck`, que debe PASAR.
- [ ] **Paso 2:** haz commit.

### Tarea 6: el selector en el formulario

**Archivos:**
- Crear: `web/src/IconPicker.tsx`
- Modificar:
  - `web/src/App.tsx`: estado `icon`, inicializado con `connection?.icon ?? ""` y enviado en `input()`. El campo va justo después del nombre.
  - `web/src/i18n.tsx`: textos en es y en.
  - `web/src/styles.css`.

**Interfaces:**
- Produce:
  - `function IconPicker({ type, value, onChange }: { type: ConnectionType; value: string; onChange: (v: string) => void })`
  - `function searchIcons(index: Icon[], query: string, type: ConnectionType): Icon[]`, exportada para la E2E con `window`. Normaliza con NFD, quita los diacríticos y todo lo que no sea letra o número, y devuelve 48 resultados como máximo. Con la búsqueda vacía, primero van los iconos por defecto de los tipos, empezando por el del tipo actual.
- Comportamiento:
  - Un botón muestra el icono actual y su nombre, o «Sin icono».
  - Al pulsarlo se abre un panel con un `<input type="search">` y un `role="listbox"`. La primera opción es siempre «Sin icono» (`none`).
  - «Restablecer» guarda `''` y solo aparece si el valor no es `''`.
  - El índice se pide con `useQuery(["icons"], () => fetch("/icons/index.json"))`, y solo al abrir el panel.
  - Teclado: flechas, Intro elige y Esc cierra sin cerrar el `<dialog>`.
  - Las opciones miden 44 px de alto como mínimo.
  - Copia en español:
    - «Icono»
    - «Buscar un icono»
    - «Sin icono»
    - «Restablecer»
    - «El icono no existe.» (para `invalid_icon`)
  - Copia en inglés:
    - "Icon"
    - "Search for an icon"
    - "No icon"
    - "Reset"
    - "That icon does not exist."

- [ ] **Paso 1:** implementa y ejecuta `npm --prefix web run typecheck`, que debe PASAR.
- [ ] **Paso 2:** haz commit.

### Tarea 7: E2E y capturas

**Archivos:**
- Modificar: `e2e/run.mjs`

- [ ] **Paso 1:** añade las pruebas:
  - **Icono por defecto:** el bloque de AdGuard tiene un `.brand-icon` con `--icon-url` `/icons/adguard.svg`, y `GET /icons/adguard.svg` responde 200.
  - **Cambio de icono:** al editar la conexión, se busca «PI HOLE», se elige Pi-hole y se guarda; el bloque usa `pihole.svg`. Con «Sin icono» el bloque no tiene `.brand-icon` y, con «Restablecer», vuelve a `adguard.svg`.
  - **Modo oscuro:** con `github` y el tema oscuro, el icono tiene `low-on-dark`.
  - **Móvil:** a 375 px, con un nombre de 80 caracteres, el icono mide más de 0 px y el `<h2>` no desborda el bloque.
  - **Fugas:** la API pública no incluye `/icons/` ni nada que no sea `slug`, `title` o `hex`. La lista de fugas sigue igual.
- [ ] **Paso 2:** ejecuta `make e2e` y debe PASAR.
- [ ] **Paso 3:** ejecuta `make screenshots`; las capturas de los bloques muestran los iconos.
- [ ] **Paso 4:** haz commit.

### Tarea 8: documentación

**Archivos:** `docs/proyecto/01-requisitos.md`, `02-arquitectura.md` (la columna, `internal/icons`, `/icons/` y el campo `icon` en la API pública), `03-diseno-funcional.md`, `04-plan-de-pruebas.md` (con la trazabilidad), `05-decisiones.md` (DEC-54: los iconos como SVG servidos uno a uno; las alternativas B y C, descartadas), `06-estado-y-cambios.md`, `docs/usuario/manual-de-usuario.md` y `docs/user/user-manual.md`.

- [ ] **Paso 1:** ejecuta `make docs-check`, que debe PASAR.
- [ ] **Paso 2:** haz commit.

### Tarea 9: cierre

- [ ] Ejecuta `make test`, `make e2e` y `make docs-check`, todo en verde.
- [ ] Revisa la rama con el subagente `revisor-de-hito`.
- [ ] Ejecuta `make dist` y prepara los comandos de la prueba real para el usuario (`scp` y `ssh -t` con `--binary`).
- [ ] Si el hito 17 se ha fusionado: actualiza la rama desde `main`, añade `talaia-availability` a `Defaults` y `web/icons/`, y renumera la DEC.
