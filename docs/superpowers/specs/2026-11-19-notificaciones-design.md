# Diseño: notificaciones de cambios de estado (hito 16)

## Intención

Hoy Talaia solo avisa si alguien está mirando el panel. La cabecera resume todos los servicios («Los 7 servicios están en orden» o «Necesita atención: VPN») y cada bloque se pinta en verde, ámbar o rojo, pero un problema que aparece de madrugada pasa desapercibido hasta que alguien abre la página. El cliente quiere que, cuando un servicio cambie de color, Talaia le mande un mensaje al móvil, y otro cuando vuelva a estar en orden. En la revisión de propuestas, esta fue la mejora que más cambia el uso diario.

**Causa actual.** El servidor solo distingue entre «responde» y «no responde» (`collector.Snapshot.Status`). El color y el texto de cada bloque («Protección desactivada», «Túnel parado»…) los calcula el navegador, con una función `state()` por integración en `web/src/widgets/*.tsx`. La cabecera los agrupa en `web/src/Dashboard.tsx` (`serviceState` y `watch`). Si nadie tiene el panel abierto, nadie calcula el estado.

## Objetivos

- Avisar sin ningún navegador abierto cuando un servicio del panel cambia de color y cuando vuelve a verde.
- Que el aviso diga lo mismo que el bloque: el nombre de la conexión y su etiqueta de estado, en el idioma elegido.
- Enviar por uno o varios canales a la vez: Telegram (bot), ntfy (servidor propio o ntfy.sh) y webhook JSON genérico.
- No notificar parpadeos: el estado nuevo debe mantenerse un tiempo configurable antes de avisar.
- No repetir avisos al reiniciar Talaia.
- Configurarlo todo en la administración: canales con su secreto cifrado, un botón «Enviar prueba», silenciar por servicio y un registro de los últimos envíos con sus errores.

## Fuera de alcance

- Recordatorios periódicos mientras un servicio sigue mal. Hay un aviso al cambiar y otro al volver a verde (decisión del cliente).
- Avisos por cambios dentro del mismo color, por ejemplo de un ámbar a otro ámbar con otra etiqueta.
- Reglas por canal (qué servicios van a qué canal), horarios de silencio, correo electrónico, Gotify, Pushover y similares. El webhook sirve de puente para estos.
- Histórico de estados: sigue valiendo RNF-01. Solo se guarda el último estado avisado y un registro de envíos acotado.

## Lo que lo haría fracasar

- Que el aviso no coincida con lo que se ve al abrir el panel (un aviso de ámbar con el bloque en verde, o al revés). Por eso las reglas de color tienen una sola fuente.
- Que un mensaje, el registro o la API de administración dejen escapar un secreto, una URL, una IP o un nombre interno.
- Ráfagas de avisos: por parpadeos, al reiniciar o al añadir una conexión.
- Que un canal caído bloquee la recolección, el panel o el resto de canales.

## Enfoques considerados

1. **Reglas de color en el servidor como única fuente (elegido por el cliente).** Cada integración de Go calcula su estado: el color, la clave del texto y los parámetros. La API lo publica y el panel solo lo traduce. Los textos de estado viven en un JSON compartido por el panel y el binario. Toca los 8 bloques, pero el aviso y el bloque no pueden desajustarse.
2. **Duplicar las reglas en Go** y dejar el panel como está. Es menos cambio, pero hay dos copias de cada regla y acabarían divergiendo sin que ninguna prueba lo impidiera del todo. Descartado por el primer punto de «Lo que lo haría fracasar».
3. **Calcular el estado en un navegador sin cabeza o con un motor JS embebido.** Descartado: depende de un navegador o de un intérprete en el LXC (RNF-05, RNF-06).

## Diseño

### 1. Estado de cada servicio en el servidor

- Nuevo tipo en `internal/integrations`:

  ```go
  type Tone string // "good" | "warn" | "bad" | "muted"
  type State struct {
      Tone   Tone           `json:"tone"`
      Key    string         `json:"key"`              // clave de texto, p. ej. "adguard.protectionOff"
      Params map[string]any `json:"params,omitempty"` // p. ej. {"n": 2}
  }
  type Stater interface { State() State }
  ```

- El `Data` de cada integración implementa `Stater` y porta tal cual la lógica de su `state()` de TypeScript. Las claves y el orden de gravedad no cambian: AdGuard, Proxmox, Home Assistant (incluido `hasNoData` de sus entidades), OPNsense, Portainer, WGDashboard y QNAP (con `FULL_PERCENT = 90` y las comprobaciones en el mismo orden).
- `collector.Snapshot` gana `State integrations.State` (`json:"state"`). Se calcula en `record`, igual que hoy `serviceState` en el navegador:
  - `Status == "error"` → `{bad, "status.noResponse"}`.
  - Con datos que implementan `Stater` → `data.State()`.
  - En otro caso (pendiente) → `{muted, "status.checking"}`.
- El panel deja de calcular: `serviceState` usa `connection.state` y traduce `key` con `params`. La propiedad `state` de la interfaz `Widget` desaparece; cada widget conserva `Body` y `footer`. Las funciones de color de filas internas (por ejemplo, `diskTone` y `volumeTone` de QNAP, que pintan filas) siguen en el navegador porque solo decoran el bloque. El color del bloque y de la cabecera sale del servidor.
- **Textos compartidos.** Las claves de estado (las que devuelven las `state()` actuales, más `status.noResponse` y `status.checking`) salen de `i18n.tsx` a un JSON con `es` y `en` que importa el panel y que el binario incrusta con `go:embed`. Dónde vive el JSON y cómo lo ven Vite y Go es decisión del constructor. Una opción es `web/src/i18n/states.json`, incrustado desde el paquete `web`, que ya existe. La interpolación `{n}` sigue la misma sintaxis que `i18n.tsx`.
- Una prueba de Go recorre todas las claves que pueden devolver los `State()` y comprueba que existen en ambos idiomas del JSON.

### 2. Qué servicios se vigilan y qué cambios avisan

- Solo se vigilan las conexiones que están en alguna página del panel, como en la cabecera (`watch` en `Dashboard.tsx`). Una conexión que no está en el panel no avisa.
- Cuenta el **color**, no la etiqueta. Los colores con aviso son `good`, `warn` y `bad`. `muted` («Comprobando», «Sin túneles», «Sin gateways monitorizados»…) no avisa ni cambia nada: se ignora y se conserva el último color avisado. Una lectura `muted` durante el retardo tampoco reinicia al candidato: se salta como si no existiera, y el tiempo del candidato sigue corriendo (por ejemplo, rojo → «Comprobando» tras reiniciar → rojo). *Decisión del constructor.*
- Al quitar una conexión de todas las páginas del panel se borra su color avisado, igual que al borrar la conexión. Si se vuelve a poner, empieza como un servicio nuevo. *Decisión del constructor.*
- Hay aviso cuando el color estable es distinto del último avisado: verde→ámbar, verde→rojo, ámbar→rojo, rojo→ámbar, ámbar o rojo→verde.
- **Estabilidad (retardo).** Cada servicio guarda en memoria su color candidato y desde cuándo lo tiene. Si el color actual difiere del último avisado y se mantiene durante el retardo configurado, el cambio madura. Si en ese tiempo vuelve al color avisado o pasa a otro, el candidato se reinicia con su nueva hora. Retardo por defecto: 2 minutos. Valores elegibles: sin espera, 1, 2, 5, 10 y 15 minutos.
- **Agrupación.** Un ciclo del notificador revisa todos los servicios (al recibir un cambio del recolector y con un temporizador de unos 5 s mientras haya candidatos pendientes). Todos los cambios que maduran en el mismo ciclo van en un solo mensaje. Como cada servicio se consulta a su ritmo (30 s por defecto), dos problemas simultáneos se ven con segundos de diferencia: cuando uno madura, se adelantan los candidatos a los que les falte menos de la mitad del retardo, con un máximo de 30 s, para que vayan en el mismo aviso. *Decisión tomada al implementar.*
- **Primer estado de un servicio** (sin color avisado guardado, porque es nuevo o se acaba de añadir al panel): se toma verde como punto de partida. Un servicio que entra ya en rojo avisa una vez pasado el retardo, y uno que entra en verde no avisa. *Decisión del constructor anotada en el diseño.*
- **Silenciar por servicio.** Hay una casilla «Avisar de sus cambios» en el formulario de la conexión, activada por defecto. Un servicio silenciado no avisa, pero su color avisado se sigue actualizando en silencio. Así, al reactivar la casilla no llega un aviso atrasado de algo que ya pasó.
- **Sin canales activos.** El notificador sigue el estado y actualiza el color avisado, pero no envía nada ni escribe en el registro. Al dar de alta el primer canal no llega una ráfaga de problemas antiguos.
- **Persistencia.** El último color avisado de cada conexión se guarda en SQLite y se borra en cascada con la conexión. Tras reiniciar se carga, así que si el servicio sigue igual no hay aviso. Los candidatos pendientes no se guardan: tras reiniciar, el retardo empieza de nuevo.

### 3. Mensaje

Contenido aprobado por el cliente. Va en el idioma de los avisos, con el nombre de la conexión y su etiqueta traducida. No lleva URL, IP, versión ni datos internos. Un mensaje con problemas y recuperaciones a la vez los separa en dos bloques: primero los problemas, del peor al menos grave; luego los que vuelven a verde.

```
🔴 Talaia · Necesita atención
Portainer: Entorno caído
VPN: Túnel parado

🟢 Talaia · Vuelve a estar en orden
AdGuard: Protección activada
```

- El título y el emoji de cada bloque salen del peor color del bloque: 🔴 rojo, 🟠 ámbar, 🟢 verde.
- Mensaje de prueba: «🔔 Talaia · Mensaje de prueba. Si lo ves, este canal funciona.», traducido.
- **Telegram:** `sendMessage` con texto plano (sin `parse_mode`) y el texto completo.
- **ntfy:** `POST {servidor}/{tópico}` con el cuerpo en texto. `Title` es la primera línea, `Priority` es `high` si hay rojo, `default` si hay ámbar y `low` si solo hay recuperaciones, y `Tags` lleva el emoji correspondiente. Cuando hay token: `Authorization: Bearer`.
- **Webhook:** `POST` con `Content-Type: application/json` y un cuerpo estable y documentado:

  ```json
  {
    "event": "change" | "test",
    "language": "es",
    "text": "…el mismo texto que en Telegram…",
    "services": [
      {"name": "Portainer", "type": "portainer", "tone": "bad", "previous": "good", "label": "Entorno caído"}
    ],
    "sent_at": "2026-11-19T10:00:00Z"
  }
  ```

  En el webhook no va el `id` interno de la conexión.

### 4. Canales y su configuración

- Tabla nueva `notification_channels`: `id`, `name` (nombre visible elegido por el usuario), `type` (`telegram` | `ntfy` | `webhook`), `enabled`, `config` (JSON sin secretos), `secret` (cifrado con `secrets.Box` y el `id` como AAD, igual que las conexiones), `tls_skip_verify`, `created_at` y `updated_at`.
- Campos por tipo:

  | Tipo | `config` (no secreto) | Secreto cifrado |
  |---|---|---|
  | Telegram | `chat_id` | token del bot |
  | ntfy | `server` (por defecto `https://ntfy.sh`) | tópico y token de acceso opcional (JSON) |
  | Webhook | ninguno | URL completa |

- El tópico de ntfy cuenta como secreto: en ntfy.sh quien lo conozca puede leer los mensajes.
- La API de administración nunca devuelve secretos. Igual que las conexiones, devuelve si hay uno guardado y al editar se deja en blanco para conservarlo.
- `tls_skip_verify` solo aparece en ntfy y webhook, para servidores propios con certificado autofirmado. Telegram siempre verifica TLS.
- Cliente HTTP de los canales: el mismo criterio que `integrations.NewHTTPClient` (sin proxy, sin seguir redirecciones, tiempo límite de 10 s). Las respuestas se leen como mucho hasta un tamaño pequeño y nunca se guardan.
- **Para las pruebas.** La dirección base de la API de Telegram se puede cambiar con una variable de entorno (`TALAIA_TELEGRAM_API`, sin documentar en el manual y sí en arquitectura), para que la E2E apunte al receptor simulado. ntfy y el webhook ya admiten cualquier dirección.
- **Ajustes generales** (en `settings`): `notify_language` (`es` | `en`) y `notify_delay_seconds` (0, 60, 120, 300, 600 o 900; por defecto 120). Si `notify_language` no está guardado, al crear el primer canal se rellena con el idioma de la administración.

### 5. Envío, errores y registro

- Cada mensaje maduro se envía a todos los canales activos de forma independiente y en paralelo. Un canal lento o caído no retrasa a los demás ni bloquea el recolector: el notificador corre en su propia goroutine y lee del recolector con `Subscribe`.
- **Reintentos:** hasta 3 intentos por canal (inmediato, a los 30 s y a los 2 min) ante errores de red, tiempo agotado o respuestas 5xx o 429. Un 4xx distinto de 429 no se reintenta. El color avisado se actualiza al decidir el envío, no al confirmarlo, así que un canal caído no provoca repeticiones.
- **Registro de envíos:** tabla `notification_log`, acotada a las 50 entradas más recientes. Guarda la fecha, el nombre del canal (copiado, para que sobreviva al borrado del canal), el tipo de evento (`change` | `test`), el resumen (los nombres de los servicios con su color nuevo y su color anterior), el resultado (`ok` | `error`), el código de error y el número de intentos.
- Los códigos de error son públicos y sin detalles: los existentes (`unauthorized`, `timeout`, `unreachable`, `tls`, `bad_response`, `internal`) más `rejected` (otro 4xx, por ejemplo un chat de Telegram que no existe). No se guarda el texto de la respuesta ni la URL.
- Los registros del servicio (slog) siguen el mismo criterio: canal, código y número de intento; nunca el secreto ni la URL.

### 6. Administración

- **Sección nueva «Notificaciones»** en la página de ajustes, después de «Acceso» (la colocación exacta es decisión del constructor, siguiendo el estilo de las secciones actuales):
  - Ajustes generales: idioma de los avisos y «Avisar si el cambio dura», con las opciones del retardo.
  - Lista de canales: nombre, tipo, activado o desactivado y el resultado del último envío. Botones para añadir, editar, borrar y «Enviar prueba».
  - Diálogo de canal con los campos de su tipo, cada uno con su ayuda (por ejemplo, cómo obtener el `chat_id` hablando antes con el bot). Incluye «Enviar prueba» con los valores del formulario sin guardar, como «Probar conexión» en las conexiones. Si el secreto está en blanco al editar, usa el guardado.
  - «Últimos envíos»: la tabla del registro, con el error traducido.
- **Formulario de conexión:** casilla «Avisar de sus cambios» (`notify`, por defecto activada), guardada como columna nueva de `connections`.
- API nueva bajo `/api/admin` (los nombres exactos son decisión del constructor): listar, crear, editar y borrar canales; probar un canal guardado o sin guardar; leer el registro; y los ajustes de notificación dentro de `/settings`.

### 7. Pruebas

- **Go:**
  - Tabla de casos por integración para `State()`, que cubra cada rama de las `state()` actuales.
  - Claves del JSON en ambos idiomas.
  - Motor del notificador con reloj inyectable: retardo, parpadeo que no avisa, agrupación, `muted` ignorado, silencio, servicio fuera del panel, primer estado, persistencia tras reiniciar y sin canales activos.
  - Formato del mensaje en ambos idiomas.
  - Cada emisor contra `httptest`: cabeceras, cuerpo, reintentos y códigos de error.
  - Almacén: cifrado del secreto, cascada y registro acotado a 50.
  - API: los secretos no salen y la validación funciona.
- **Receptor simulado** `e2e/fake-notify.mjs`: un servidor que acepta `POST /bot{token}/sendMessage` (estilo Telegram), `POST /{tópico}` (estilo ntfy) y `POST /hook` (webhook). Guarda cada mensaje con sus cabeceras y su cuerpo, y lo expone en `GET /messages` para las aserciones. Puede forzar un fallo (por ejemplo, el token `malo` → 401) para ver el error en el registro.
- **E2E** (numeración a partir de E2E-43):
  - Alta de un canal de cada tipo con prueba recibida.
  - Un servicio pasa a ámbar y a rojo, y llega un solo mensaje agrupado con las etiquetas del panel.
  - Vuelta a verde.
  - Servicio silenciado.
  - Idioma inglés.
  - Canal con error visible en «Últimos envíos».
  - Los secretos no aparecen en ninguna respuesta de la API de administración.
  - El panel sigue mostrando los mismos colores y etiquetas que antes del cambio de fuente. Las E2E existentes ya lo cubren y deben pasar sin tocar sus valores esperados.

  La E2E usa el retardo «sin espera».

### 8. Documentación y verificación

- Documentación obligatoria del `CLAUDE.md`:
  - Requisitos nuevos a partir de RF-106, más una nota en RNF-05: los avisos a Telegram o ntfy.sh necesitan salida a Internet, pero son opcionales.
  - Arquitectura: el notificador, las tablas, la API, la fuente única del estado y `TALAIA_TELEGRAM_API`.
  - Diseño funcional y los dos manuales con capturas (`make screenshots`).
  - Plan de pruebas y trazabilidad.
  - Decisiones a partir de DEC-48: el estado en el servidor como única fuente, el retardo medido en tiempo, el primer estado verde y el color avisado actualizado al decidir.
  - Estado y cambios.
- **Coordinación con el hito 15 (SAI por NUT):** se trabaja en paralelo en otro worktree y es probable que use los mismos números (RF, DEC y E2E). Su bloque trae una `state()` en TypeScript. Si se fusiona antes que este hito, aquí se porta su regla a Go y se renumera. Si se fusiona después, esa rama debe hacerlo al actualizarse desde `main`.
- **Terminado** es:
  - `make test`, `make e2e` y `make docs-check` en verde.
  - Revisión con `revisor-de-hito`.
  - Un aviso real de Telegram recibido por el cliente desde su Talaia en Proxmox (prueba y cambio de estado real o provocado, por ejemplo desactivando la protección de AdGuard), registrado en `04-plan-de-pruebas.md` antes del PR.

## Lo que queda al constructor

- La ubicación del JSON de textos compartidos y su carga en Vite y Go.
- Los nombres de rutas, tipos y archivos nuevos (paquete sugerido: `internal/notify`).
- La disposición exacta de la sección de notificaciones, dentro del estilo actual.
- El intervalo exacto del ciclo del notificador, siempre que la agrupación funcione como se describe.
