# Notificaciones (hito 16) — plan de implementación

> **Para agentes:** sub-skill obligatoria: superpowers:subagent-driven-development (recomendada) o superpowers:executing-plans. Los pasos usan casillas (`- [ ]`).

**Objetivo:** Talaia avisa por Telegram, ntfy o webhook cuando un servicio del panel cambia de color y cuando vuelve a verde, sin navegador abierto.

**Arquitectura:**
- El color y la etiqueta de cada servicio pasan del navegador al servidor (`integrations.Stater`) y se publican en cada `collector.Snapshot`.
- Un paquete nuevo, `internal/notify`, se suscribe al recolector. Aplica el retardo y la agrupación, guarda el último color avisado en SQLite y envía a los canales activos.
- El panel solo traduce la clave de estado. Los textos de estado viven en un JSON que comparten Vite y Go.

**Tecnología:** Go 1.x (chi, modernc SQLite, `secrets.Box`), React 19 + TanStack Query y E2E con Playwright y servidores simulados en Node.

**Especificación:** [docs/superpowers/specs/2026-11-19-notificaciones-design.md](../specs/2026-11-19-notificaciones-design.md). Léela entera antes de cada tarea.

## Restricciones globales

- **Sin commits, push ni PR** sin que el usuario lo pida. Cada tarea termina en un punto de control con las pruebas en verde, no en un commit.
- **Mensajes, registro, logs y API:** nunca llevan URL, IP, secretos ni nombres internos. La API de administración nunca devuelve un secreto.
- **Idioma:** comentarios y documentación en español. `README.md` y `docs/user/user-manual.md` en inglés. Cada cambio de manual se hace en las dos versiones.
- **Numeración nueva:** RF-106…RF-113, DEC-48…DEC-51 y E2E-43…E2E-47. El hito 15 (SAI) ya está en `main` (RF-105, DEC-47, E2E-42); su regla `state()` de `nut.tsx` se porta también.
- **Nada del laboratorio** del usuario en el repositorio (los hooks lo bloquean).
- **Valores fijos:**
  - Retardo: 0, 60, 120, 300, 600 o 900 s; por defecto 120.
  - Reintentos: 0 s, 30 s y 2 min.
  - Registro: las 50 entradas más recientes.
  - Tiempo límite HTTP: 10 s.
  - ntfy: servidor por defecto `https://ntfy.sh`.
  - Telegram: dirección base de la API con `TALAIA_TELEGRAM_API` (por defecto `https://api.telegram.org`).
- **Texto del mensaje** exactamente como en la sección 3 de la especificación.
- **Antes de lanzar `make e2e`**, contrasta los valores esperados con los simuladores (memoria del proyecto).

## Puntos de revisión

Fallos que la especificación implica, que ninguna prueba ejercita por sí sola y que más afectarían al usuario:

1. **Un canal colgado** (servidor que acepta la conexión y no responde) no debe retrasar a los demás canales ni el siguiente ciclo. Prueba: `TestSlowChannelDoesNotBlockOthers` (tarea 6).
2. **Un nombre de conexión con saltos de línea o muy largo** no debe romper el formato. Se sustituyen los saltos por espacios y se recorta a 80 caracteres. Prueba: `TestComposeSanitizesNames` (tarea 4).
3. **El error de un canal no debe contener la URL ni el token.** Por ejemplo, `*url.Error` lleva la URL del webhook en su texto. Solo se guarda el código. Prueba: `TestSendErrorsHideURL` (tarea 5).
4. **Un secreto ilegible** (clave de cifrado cambiada) debe dejar el error `secret_unreadable` en el registro, sin bloquear el resto de canales. Prueba: `TestUnreadableChannelSecret` (tarea 6).
5. **Borrar un canal con envíos en curso o reintentos pendientes** no debe provocar un pánico ni el envío de un canal ya borrado. Antes de cada reintento se comprueba que el canal sigue existiendo y activo. Prueba: `TestRetrySkipsDeletedChannel` (tarea 6).

---

### Tarea 1: estado en el servidor

**Archivos:**
- Crear: `internal/integrations/state.go`, `web/src/i18n/states.json`, `web/states.go`, `web/states_test.go`.
- Modificar: el `Data` de cada integración (`adguard.go`, `proxmox.go`, `homeassistant.go`, `opnsense.go`, `portainer.go`, `wgdashboard.go`, `qnap.go`, `nut.go`) y `internal/collector/collector.go` (`Snapshot` y `record`).
- Pruebas: un `state_test.go` en cada paquete de integración y `internal/collector/collector_test.go`.

**Interfaces:**
- Produce:

  ```go
  // internal/integrations
  type Tone string
  const (ToneGood Tone = "good"; ToneWarn Tone = "warn"; ToneBad Tone = "bad"; ToneMuted Tone = "muted")
  type State struct { Tone Tone `json:"tone"`; Key string `json:"key"`; Params map[string]any `json:"params,omitempty"` }
  type Stater interface { State() State }
  func Severity(t Tone) int // muted y good 0, warn 1, bad 2

  // web
  func StateLabels() map[string]map[string]string // idioma → clave → texto, desde src/i18n/states.json
  ```

- `collector.Snapshot` gana `State integrations.State \`json:"state"\``.

Pasos:

- [ ] **Paso 1: crear `web/src/i18n/states.json`** con `{"es": {...}, "en": {...}}`. Mueve allí de `web/src/i18n.tsx`, sin cambiar el texto, todas las claves que devuelven las `state()` de `web/src/widgets/*.tsx`, más `status.noResponse` y `status.checking`. Bórralas de `i18n.tsx`; la tarea 2 las vuelve a importar.
- [ ] **Paso 2: escribir `web/states.go`** con `//go:embed src/i18n/states.json` y `StateLabels()`. La prueba `TestStateLabelsBothLanguages` comprueba que `es` y `en` tienen exactamente las mismas claves.
- [ ] **Paso 3: escribir las pruebas que fallan.** Una tabla por integración, `TestState`, con un caso por rama de su `state()` de TypeScript, en el mismo orden. Ejemplos de aserciones obligatorias:
  - AdGuard: `{running:false}` → `{bad, "adguard.dnsStopped"}`.
  - Proxmox: `{nodes_online:1, nodes_total:2}` → `{warn, "proxmox.nodesDown", {"online":1,"total":2}}`.
  - Home Assistant: 2 entidades sin datos → `{warn, "homeassistant.unavailable", {"n":2}}`.
  - OPNsense: sin gateways monitorizados → `{muted, "opnsense.unmonitored"}`.
  - QNAP: un volumen al 90 % → `{warn, "qnap.state.full"}`; un disco `error` junto a ese volumen → `{bad, "qnap.state.diskError"}`.

  Cada prueba comprueba además que `Key` existe en `web.StateLabels()["es"]`.
- [ ] **Paso 4:** `go test ./internal/integrations/...`. Debe fallar porque `State` no está definido.
- [ ] **Paso 5: implementar `State()`** en cada `Data`, portando la lógica de su `state()` (incluidos `hasNoData` de Home Assistant y `FULL_PERCENT = 90` de QNAP) y `integrations.State`.
- [ ] **Paso 6: prueba del colector.** `TestSnapshotState` en `collector_test.go` cubre tres casos: un error da `{bad, "status.noResponse"}` aunque haya datos anteriores; los datos que implementan `Stater` dan su `State()`; pendiente da `{muted, "status.checking"}`.
- [ ] **Paso 7: implementar** el cálculo en `record` y al crear el snapshot en `start`.
- [ ] **Paso 8:** `make test` en verde.

### Tarea 2: el panel usa el estado del servidor

**Archivos:**
- Modificar: `web/src/api.ts` (`ConnectionState.state`), `web/src/Dashboard.tsx` (`serviceState`), `web/src/widgets/types.ts` (se quita `state`), los ocho `web/src/widgets/*.tsx` (se quitan sus `state`) y `web/src/i18n.tsx` (importa `states.json`).
- Pruebas: las E2E existentes, sin tocar sus valores esperados.

**Interfaces:**
- Consume: `snapshot.state` de la tarea 1.
- Produce: `export interface ServiceState { tone: Tone; key: string; params?: Record<string, string | number> }` en `api.ts`.

Pasos:

- [ ] **Paso 1: unir los diccionarios.** `i18n.tsx` importa `./i18n/states.json` y lo une a `es` y `en` (`{...states.es, ...}`), de modo que el tipo `Key` siga incluyendo esas claves.
- [ ] **Paso 2: `serviceState`** devuelve `{ tone: connection.state.tone, label: t(connection.state.key as Key, connection.state.params) }`.
- [ ] **Paso 3: quitar `state`** de `Widget` y de los ocho widgets. Las funciones auxiliares que siguen coloreando filas (por ejemplo, `diskTone` y `volumeTone` de QNAP) se quedan.
- [ ] **Paso 4:** `npm --prefix web run build` sin errores de tipos.
- [ ] **Paso 5:** `make e2e`. Todas las E2E existentes pasan sin cambios, en especial E2E-37 y las de color de cada bloque.

### Tarea 3: almacén de notificaciones

**Archivos:**
- Modificar: `internal/store/store.go` (migración nueva al final de `migrations`; `Connection.Notify`) y `internal/store/panel.go` (`Settings`).
- Crear: `internal/store/notify.go`, `internal/store/notify_test.go`.

**Interfaces:**
- Produce:

  ```go
  // Connection gana Notify bool (columna notify INTEGER NOT NULL DEFAULT 1); Create y Update lo guardan.
  // Settings gana NotifyLanguage string `json:"notify_language"` ("" = sin elegir, "es", "en")
  //   y NotifyDelaySeconds int `json:"notify_delay_seconds"` (por defecto 120).
  type Channel struct {
      ID, Name, Type string // Type: "telegram" | "ntfy" | "webhook"
      Enabled, TLSSkipVerify bool
      Config map[string]string // telegram: chat_id; ntfy: server
      CreatedAt, UpdatedAt time.Time
  }
  func (s *Store) ListChannels(ctx) ([]Channel, error)
  func (s *Store) GetChannel(ctx, id string) (Channel, error)
  func (s *Store) CreateChannel(ctx, c Channel, secret string) (Channel, error)  // secret: JSON con los campos secretos
  func (s *Store) UpdateChannel(ctx, c Channel, secret *string) (Channel, error) // nil conserva
  func (s *Store) DeleteChannel(ctx, id string) error
  func (s *Store) ChannelSecret(ctx, id string) (string, error)

  func (s *Store) NotifiedTones(ctx) (map[string]string, error) // connection_id → tone
  func (s *Store) SetNotifiedTone(ctx, connectionID, tone string) error
  func (s *Store) DeleteNotifiedTone(ctx, connectionID string) error

  type LogService struct { Name string `json:"name"`; Tone string `json:"tone"`; Previous string `json:"previous"` }
  type LogEntry struct {
      ID int64 `json:"id"`; At time.Time `json:"at"`; Channel string `json:"channel"`; ChannelType string `json:"channel_type"`
      Event string `json:"event"`; Services []LogService `json:"services"`; Result string `json:"result"`
      Code string `json:"code,omitempty"`; Attempts int `json:"attempts"`
  }
  func (s *Store) AddLog(ctx, e LogEntry) error    // deja solo las 50 más recientes
  func (s *Store) ListLog(ctx) ([]LogEntry, error) // de la más reciente a la más antigua
  ```

- **Tablas:**
  - `notification_channels(id, name, type, enabled, config, secret BLOB, tls_skip_verify, created_at, updated_at)`. El secreto se sella con el `id` como AAD, igual que las conexiones.
  - `notify_state(connection_id PRIMARY KEY REFERENCES connections(id) ON DELETE CASCADE, tone)`.
  - `notification_log(id INTEGER PRIMARY KEY, at, channel, channel_type, event, services, result, code, attempts)`.

Pasos:

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `TestChannelSecretEncrypted`: la columna `secret` no contiene el texto plano y `ChannelSecret` lo recupera.
  - `TestUpdateChannelKeepsSecret`.
  - `TestNotifiedToneCascade`: al borrar la conexión desaparece su color.
  - `TestLogKeepsLatest50`: tras 55 entradas quedan 50 y la primera es la última añadida.
  - `TestSettingsNotifyDefaults`: `""` y `120`.
  - `TestConnectionNotifyDefaultTrue`: la migración deja `notify=1` en las conexiones existentes.
- [ ] **Paso 2:** `go test ./internal/store/`. Debe fallar.
- [ ] **Paso 3: implementar** la migración y las funciones.
- [ ] **Paso 4:** `go test ./internal/store/` en verde.

### Tarea 4: composición del mensaje

**Archivos:**
- Crear: `internal/notify/message.go`, `internal/notify/message_test.go`.

**Interfaces:**
- Consume: `integrations.State`, `integrations.Severity` y `web.StateLabels()`.
- Produce:

  ```go
  type Change struct { Name, Type string; Tone, Previous integrations.Tone; State integrations.State }
  type Message struct {
      Event string // "change" | "test"
      Language string; Title string; Text string
      Worst integrations.Tone; Changes []Change; SentAt time.Time
  }
  func Compose(lang string, changes []Change, now time.Time) Message
  func ComposeTest(lang string, now time.Time) Message
  func Label(lang string, s integrations.State) string // interpola {param} como i18n.tsx
  ```

- **Textos:**
  - Las cabeceras («Talaia · Necesita atención», «Talaia · Vuelve a estar en orden» y el mensaje de prueba) y sus traducciones al inglés («Talaia · Needs attention», «Talaia · Back to normal», «🔔 Talaia · Test message. If you can see it, this channel works.») son constantes del paquete.
  - Las etiquetas de estado salen de `StateLabels`.

Pasos:

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `TestComposeGroupsProblemsAndRecoveries`: Portainer rojo, VPN ámbar y AdGuard verde dan exactamente el texto de la sección 3 de la especificación. Los problemas van ordenados por gravedad y `Worst == bad`.
  - `TestComposeEnglish`.
  - `TestComposeOnlyRecoveries`: un solo bloque 🟢.
  - `TestComposeSanitizesNames`: `"a\nb"` da `"a b"` y un nombre de 200 caracteres se recorta a 80.
  - `TestLabelParams`: `homeassistant.unavailable` con `n=2`.
- [ ] **Paso 2:** `go test ./internal/notify/`. Debe fallar.
- [ ] **Paso 3: implementar.** `Title` es la primera línea de `Text`.
- [ ] **Paso 4:** en verde.

### Tarea 5: emisores por canal

**Archivos:**
- Crear: `internal/notify/send.go` y `internal/notify/send_test.go`.

**Interfaces:**
- Consume: `store.Channel` y `Message`.
- Produce:

  ```go
  const CodeRejected = "rejected"
  type Sender interface { Send(ctx context.Context, m Message) error }
  // secret es el JSON guardado: telegram {"token"}, ntfy {"topic","token"?}, webhook {"url"}.
  func NewSender(ch store.Channel, secret string) (Sender, error)
  func Retryable(err error) bool // red, tiempo agotado, 5xx y 429
  ```

- Los errores son `*integrations.Error` con un código público (`unauthorized` para 401 y 403, `rejected` para otros 4xx, `bad_response` para 5xx, y los de red mediante `integrations.Code`). El `Err` envuelto nunca lleva la URL: se envuelve un error propio y no el `*url.Error`.
- El cliente es como `integrations.NewHTTPClient` (TLS según `TLSSkipVerify`, que Telegram ignora) y la respuesta se lee como mucho hasta 64 KiB.
- **Peticiones:**
  - Telegram: `POST {TALAIA_TELEGRAM_API}/bot{token}/sendMessage` con el JSON `{"chat_id", "text"}`.
  - ntfy: `POST {server}/{topic}`, cuerpo en texto, con las cabeceras `Title`, `Priority` (`high` si hay rojo, `default` si hay ámbar, `low` en los demás casos) y `Tags` (`red_circle`, `orange_circle`, `green_circle` o `bell`). Si hay token, `Authorization: Bearer`.
  - Webhook: `POST` del JSON de la sección 3 de la especificación.

Pasos:

- [ ] **Paso 1: escribir las pruebas que fallan** contra `httptest.Server`:
  - `TestTelegramRequest` (ruta, JSON y sin `parse_mode`).
  - `TestNtfyHeaders` (prioridad según el peor color y token opcional).
  - `TestWebhookBody` (campos exactos y sin `id`).
  - `TestSendErrorCodes` (401 → `unauthorized`, 400 → `rejected`, 500 → `bad_response`, servidor cerrado → `unreachable`).
  - `TestSendErrorsHideURL`: `err.Error()` no contiene ni la URL ni el token.
  - `TestRetryable`.
- [ ] **Paso 2:** debe fallar.
- [ ] **Paso 3: implementar.**
- [ ] **Paso 4:** en verde.

### Tarea 6: motor del notificador

**Archivos:**
- Crear: `internal/notify/notifier.go` y `internal/notify/notifier_test.go`.
- Modificar: `cmd/talaia/main.go`, que crea el notificador después del recolector, lanza `Run(ctx)` y lo espera al cerrar.

**Interfaces:**
- Consume: las tareas 3, 4 y 5, y `collector.Snapshot` con su `State`.
- Produce:

  ```go
  type Source interface { Snapshots() []collector.Snapshot; Subscribe() (<-chan struct{}, func(), bool) }
  type Notifier struct { /* now func() time.Time; newSender func(store.Channel, string) (Sender, error); sleep func(ctx, d) error */ }
  func New(st *store.Store, src Source, log *slog.Logger) *Notifier
  func (n *Notifier) Run(ctx context.Context)
  func (n *Notifier) Wait()
  func (n *Notifier) SendTest(ctx context.Context, ch store.Channel, secret string) store.LogEntry // envía, registra y devuelve la entrada
  ```

- **Algoritmo de `evaluate(ctx, now)`.** Se ejecuta en cada aviso del recolector y cada 5 s mientras haya candidatos.

  ```
  placed := conexiones en alguna página (store.Panel)
  borra los tonos avisados de las conexiones que no están en placed
  para cada snapshot en placed:
      tone := snapshot.State.Tone; si es muted, sigue (no toca al candidato)
      notified := tono avisado, o "good" si no hay
      si tone == notified: borra el candidato; sigue
      si el candidato no existe o su tono != tone: candidato = {tone, since: now}; sigue
      si now - since < retardo: sigue
      madura: guarda SetNotifiedTone(tone) y borra el candidato
              si la conexión tiene Notify y hay canales activos: añade un Change{previous: notified}
  si hay cambios: Compose(idioma, cambios) y envía a cada canal activo en su goroutine con deliver()
  ```

- **`deliver`:** intentos a los 0 s, 30 s y 2 min mientras `Retryable(err)`. Antes de cada reintento relee el canal y se detiene si ya no existe o está desactivado. Al acabar escribe una sola entrada con `AddLog`, con el número de intentos. Un secreto ilegible da `secret_unreadable`.
- **Idioma:** si `NotifyLanguage` es `""`, se usa `es`.

Pasos:

- [ ] **Paso 1: escribir las pruebas que fallan.** Usan un `Source` falso, reloj y `sleep` inyectados y un `Sender` falso que registra:
  - `TestNotifiesAfterDelay`: ámbar con 119 s no envía; con 120 s envía uno.
  - `TestFlapDoesNotNotify`: ámbar 60 s, verde y ámbar de nuevo reinician el plazo.
  - `TestMutedSkipped`: rojo, `muted` y rojo no reinician el plazo.
  - `TestGroupsSimultaneous`: dos servicios que maduran en el mismo ciclo dan un mensaje con dos cambios.
  - `TestRecovery`: rojo avisado y luego verde estable dan un mensaje 🟢.
  - `TestSilencedUpdatesTone`: con `Notify=false` no envía, pero guarda el tono; al reactivarlo no hay aviso.
  - `TestNoChannelsUpdatesTone`: lo mismo sin canales activos, y sin entrada en el registro.
  - `TestNotOnPanelIgnoredAndForgotten`.
  - `TestFirstStateAssumesGood`.
  - `TestRestartDoesNotRepeat`: un notificador nuevo sobre el mismo almacén no reenvía.
  - `TestRetryThenLog`: dos 500 y luego éxito dan una entrada `ok` con 3 intentos.
  - `TestRetrySkipsDeletedChannel`.
  - `TestUnreadableChannelSecret`.
  - `TestSlowChannelDoesNotBlockOthers`.
- [ ] **Paso 2:** `go test -race ./internal/notify/`. Debe fallar.
- [ ] **Paso 3: implementar** y conectarlo en `main.go`.
- [ ] **Paso 4:** `make test` en verde con `-race`.

### Tarea 7: API de administración

**Archivos:**
- Crear: `internal/api/notify.go` y `internal/api/notify_test.go`.
- Modificar:
  - `internal/api/api.go`: rutas; `Server` recibe `*notify.Notifier` en `New` (actualiza las llamadas de las pruebas y de `main.go`); `notify` en `connectionIn` como `*bool`, por defecto `true`, y en `connectionOut`.
  - `internal/api/panel.go`: validación de los ajustes nuevos (idioma `es`, `en` o `""`, y un retardo de la lista).

**Interfaces:**
- **Rutas** bajo `/api/admin`:
  - `GET /notifications/channels` y `POST /notifications/channels`.
  - `PUT /notifications/channels/{id}` y `DELETE /notifications/channels/{id}`.
  - `POST /notifications/channels/{id}/test`, que prueba un canal guardado.
  - `POST /notifications/test`, que prueba un canal sin guardar con el cuerpo `channelIn`. Si trae `id`, completa con el secreto guardado los campos secretos que lleguen en blanco.
  - `GET /notifications/log`.
- **`channelIn`:** `{id?, name, type, enabled, tls_skip_verify, chat_id, server, token *string, topic *string, url *string, language?}`.
  - `language` solo se usa al crear el primer canal si `notify_language` es `""`.
  - Al editar, cada campo secreto `nil` o vacío conserva el valor guardado.
- **`channelOut`:** `{id, name, type, enabled, tls_skip_verify, chat_id, server, last?: {at, result, code}}`. Nunca lleva `token`, `topic` ni `url`.
- **Validación**, que devuelve `400 invalid_channel`:
  - Nombre de 1 a 60 caracteres.
  - Telegram: token `^\d+:[A-Za-z0-9_-]+$` y `chat_id` `^(-?\d+|@[A-Za-z0-9_]{5,})$`.
  - ntfy: `server` con `http(s)://` y tópico `^[A-Za-z0-9_-]{1,64}$`.
  - Webhook: URL con `http(s)://`.
- La respuesta de las dos rutas de prueba es la `store.LogEntry` resultante.

Pasos:

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `TestChannelsNeverExposeSecrets`: crea los tres tipos y ni las respuestas JSON de lista, alta, edición, prueba y registro ni el cuerpo crudo contienen el token, el tópico o la URL.
  - `TestChannelValidation`.
  - `TestTestChannelUsesStoredSecret`.
  - `TestFirstChannelSetsLanguage`.
  - `TestConnectionNotifyRoundTrip`.
  - `TestNotifyRoutesRequireAuth`.
- [ ] **Paso 2:** debe fallar.
- [ ] **Paso 3: implementar.**
- [ ] **Paso 4:** `make test` en verde.

### Tarea 8: administración web

**Archivos:**
- Crear: `web/src/Notifications.tsx`.
- Modificar: `web/src/Settings.tsx` (sección después de «Acceso»), `web/src/api.ts` (tipos y llamadas de la tarea 7), `web/src/App.tsx` (casilla en el formulario de conexión) y `web/src/i18n.tsx` (claves `notify.*` en `es` y `en`).

**Interfaces:**
- Consume: las rutas de la tarea 7 y `AppSettings.notify_language` y `notify_delay_seconds`.

**Textos fijos en español** (y su traducción):

| Elemento | Texto |
|---|---|
| Título de la sección | «Notificaciones» |
| Entradilla | «Talaia avisa cuando un servicio del panel cambia de color y cuando vuelve a estar en orden.» |
| Idioma | «Idioma de los avisos» |
| Retardo | «Avisar si el cambio dura», con las opciones «Sin espera», «1 minuto», «2 minutos», «5 minutos», «10 minutos» y «15 minutos» |
| Botones | «Añadir canal», «Enviar prueba» |
| Registro | «Últimos envíos» |
| Casilla de la conexión | «Avisar de sus cambios» |

Errores del registro: `notify.error.{code}`, con un texto genérico de respaldo.

Pasos:

- [ ] **Paso 1: implementar** la sección, el diálogo de canal con los campos de cada tipo y sus ayudas (Telegram: «Escribe primero al bot; el chat ID lo da @userinfobot o `getUpdates`»), la tabla del registro y la casilla de la conexión.
- [ ] **Paso 2:** `npm --prefix web run build` sin errores de tipos.
- [ ] **Paso 3: comprobación a mano** con `make run` en el navegador integrado: crea un canal webhook hacia un receptor local, envía una prueba y la ves en «Últimos envíos».

### Tarea 9: receptor simulado y E2E

**Archivos:**
- Crear: `e2e/fake-notify.mjs`.
- Modificar: `e2e/run.mjs`, que arranca el receptor en el puerto 13010 y lanza Talaia con `TALAIA_TELEGRAM_API=http://localhost:13010`.

**Interfaces:**
- `fakeNotify(port)` devuelve `{url, messages, clear(), start(), stop()}`.
  - Acepta `POST /bot{token}/sendMessage`, `POST /{topic}` y `POST /hook`.
  - Guarda `{kind, path, headers, body}`.
  - El token `0:malo` responde 401.

**Escenarios:**
- **E2E-43:** alta de un canal de cada tipo con «Enviar prueba» recibida por el simulador. Ninguna respuesta de la API de administración lleva los secretos.
- **E2E-44:** con «Sin espera», AdGuard sin protección y WGDashboard con el túnel parado llegan en un solo mensaje por canal, con las etiquetas del panel. Al restaurarlos llega un mensaje «Vuelve a estar en orden».
- **E2E-45:** una conexión con «Avisar de sus cambios» desmarcada cambia de color y no llega nada.
- **E2E-46:** con el idioma de los avisos en inglés, el texto llega en inglés.
- **E2E-47:** un canal Telegram con el token `0:malo` aparece en «Últimos envíos» con el error traducido.

Pasos:

- [ ] **Paso 1: escribir el simulador** y los escenarios. Antes, contrasta las etiquetas esperadas con `states.json` y con el estado de cada simulador.
- [ ] **Paso 2:** `make e2e` en verde, con las E2E anteriores incluidas.

### Tarea 10: documentación obligatoria y capturas

**Archivos:**
- Modificar:
  - `docs/proyecto/01-requisitos.md`: sección «Notificaciones» con RF-106…RF-113 y una nota en RNF-05.
  - `docs/proyecto/02-arquitectura.md`.
  - `docs/proyecto/03-diseno-funcional.md`.
  - `docs/proyecto/04-plan-de-pruebas.md`: pruebas de Go, E2E-43…47 y trazabilidad.
  - `docs/proyecto/05-decisiones.md`: DEC-48…51.
  - `docs/proyecto/06-estado-y-cambios.md`.
  - `docs/usuario/manual-de-usuario.md` y `docs/user/user-manual.md`.
  - Si los README enumeran funciones, `README.md` y `README.es.md`.
  - Capturas con `make screenshots`, más una nueva de la sección de notificaciones en los dos manuales.

**Requisitos:**
- **RF-106:** avisa por los canales activos cuando un servicio del panel pasa a ámbar o a rojo y cuando vuelve a verde, sin ningún navegador abierto.
- **RF-107:** solo avisa si el cambio dura el retardo configurado, y los cambios que coinciden van en un mensaje.
- **RF-108:** canales Telegram, ntfy y webhook JSON, uno o varios, con el secreto cifrado que nunca sale del servidor y el botón «Enviar prueba».
- **RF-109:** silenciar los avisos de una conexión.
- **RF-110:** el mensaje lleva el nombre y la etiqueta del panel, en el idioma elegido, sin URL, IP, credenciales ni nombres internos.
- **RF-111:** registro de los 50 últimos envíos con su resultado y su error.
- **RF-112:** no repite avisos tras reiniciar.
- **RF-113:** el servidor calcula el color de cada servicio, una sola fuente para el panel y los avisos.

**Decisiones:**
- **DEC-48:** el estado en el servidor como única fuente.
- **DEC-49:** el retardo medido en tiempo y `muted` ignorado.
- **DEC-50:** el primer estado se toma como verde y el color avisado se actualiza al decidir, no al confirmar.
- **DEC-51:** el tópico de ntfy es secreto y `TALAIA_TELEGRAM_API` existe para las pruebas.

Pasos:

- [ ] **Paso 1: redactar.**
- [ ] **Paso 2:** `make screenshots`, y revisa las capturas.
- [ ] **Paso 3:** `make docs-check` en verde.

### Tarea 11: revisión y verificación real

- [ ] **Paso 1:** `make test`, `make e2e` y `make docs-check` en verde.
- [ ] **Paso 2: revisión.** El subagente `revisor-de-hito` revisa la rama. Corrige lo que encuentre.
- [ ] **Paso 3: prueba real.** Pide al usuario que instale la rama con `talaia-proxmox.sh update --ctid ID --binary` y que dé de alta su bot de Telegram tecleando él el token. Después:
  - «Enviar prueba».
  - Desactivar la protección de AdGuard más de 2 minutos y reactivarla.

  Pídele las capturas de los mensajes y de «Últimos envíos».
- [ ] **Paso 4:** registra esa verificación en `04-plan-de-pruebas.md` y en los datos sin verificar.
- [ ] **Paso 5:** pide al usuario permiso para el commit y el PR.
