# Plan de pruebas

## Niveles de prueba

| Nivel | Herramienta | Comando | Se ejecuta |
|---|---|---|---|
| Unitarias y de integración | `go test` con detector de carreras | `make test` | CI, en cada cambio |
| Extremo a extremo (E2E) | Playwright y Chromium contra el binario real y un AdGuard simulado | `make e2e` | CI, en cada cambio |
| Compilación | `tsc`, Vite y `go build` | `make build` | CI, en cada cambio |
| Documentación | `scripts/check_docs.py` | `make docs-check` | CI, en cada cambio |
| Contra servicio real | Manual, con el servicio real en local | Ver [verificaciones manuales](#verificaciones-manuales) | Al cerrar cada integración |

Las pruebas E2E comparten un único recorrido: cada escenario parte del estado que deja el anterior, y el primer fallo detiene los siguientes.

## Pruebas de Go

### `internal/api`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestSetupOnlyOnceAndLogin` | El asistente funciona una sola vez, inicia sesión y rechaza contraseñas débiles; login y logout | RF-02, RF-03, RF-04 |
| `TestLogoutRevokesSessionOnServer` | Una cookie copiada deja de valer tras cerrar sesión | RF-04 |
| `TestLoginLimiter` | Bloquea tras 10 fallos, incluso con la contraseña correcta | RF-04 |
| `TestConnectionsNeverExposeSecret` | Ninguna respuesta incluye la credencial; se conserva si no se cambia; cambiar la URL obliga a escribirla | RF-10, RF-14 |
| `TestConnectionValidation` | Rechaza tipo, URL, usuario, intervalo, credencial y opciones no válidos | RF-10 |
| `TestConnectionOptionsRoundTrip` | Las opciones se guardan y se devuelven | RF-32 |
| `TestWritesRequireJSONAndSameOrigin` | Rechaza formularios y peticiones de otro origen | RF-02 |
| `TestWebFallback` | Las rutas de la web sirven la aplicación; archivos y API inexistentes dan 404 | RF-01 |
| `TestDashboardIsPublicAndHidesConfiguration` | El dashboard es público, con enlace y sin usuarios ni credenciales | RF-01, RF-25, RNF-03 |
| `TestConnectionLink` | Rechaza enlaces con otro esquema o con credenciales; uno propio sustituye a la URL de conexión en la API pública | RF-25 |
| `TestConnectionTestReportsNotes` | La prueba de conexión devuelve los avisos de la integración | RF-53 |
| `TestMultipleConnectionsOfSameService` | Admite varias conexiones del mismo servicio, ordenadas por nombre | RF-11 |
| `TestDashboardFollowsConnectionChanges` | Editar y eliminar se refleja; un fallo conserva el último dato | RF-20, RF-21 |
| `TestDashboardEvents` | El flujo SSE envía el estado inicial y los cambios | RF-20 |
| `TestListEntitiesEndpoint` | Exige sesión, usa la credencial guardada solo con la misma URL y rechaza tipos sin entidades | RF-51, RF-14 |
| `TestEntitySelectionValidation` | Rechaza formatos no válidos, repetidas, más de 200 y selección en otro tipo de servicio | RF-51 |
| `TestPanelSaveAndValidation` | Exige sesión; rechaza páginas y bloques no válidos, fuera de la rejilla, solapados o repetidos | RF-22, RF-23 |
| `TestUnplacedConnectionIsHidden` | Una conexión fuera del panel no sale por la API pública, pero sí en la de administración | RF-22 |
| `TestDashboardCanRequireLogin` | Con sesión obligatoria, la API da 401, los flujos abiertos se cierran y el estado público informa | RF-07, RF-24 |
| `TestSettingsValidation` | Exige sesión y rechaza temas y acentos desconocidos | RF-24 |
| `TestChangePassword` | Exige sesión y la contraseña actual; revoca otras sesiones | RF-08 |
| `TestConnectionTest` | Exige sesión, no guarda nada, reutiliza la credencial solo con la misma URL y rechaza tipos sin integración | RF-13, RF-14 |

### `internal/auth`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestPassword` | El hash Argon2id verifica la contraseña correcta y rechaza la incorrecta y los hashes no válidos | RF-04 |

### `internal/secrets`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestSealOpen` | Cifra y descifra; un valor no se abre con el ID de otra conexión | RNF-04 |
| `TestKeyIsStableAndProtected` | La clave se conserva y se rechaza con permisos abiertos | RNF-04 |

### `internal/store`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestFirstUserOnlyOnce` | Solo se crea el primer administrador | RF-03 |
| `TestExpiredSessionIsRejected` | Una sesión caducada no es válida | RF-04 |
| `TestConnectionSecretIsEncryptedAndKept` | La credencial no está en claro en la base de datos y se actualiza solo si se pide | RNF-04, RF-14 |
| `TestReopenKeepsData` | Los datos persisten al reabrir | RNF-05 |
| `TestOptionsArePersisted` | Las opciones se guardan | RF-32 |
| `TestLinkFallsBackToConnectionURL` | Sin enlace propio se usa la URL de conexión | RF-25 |
| `TestEntitiesArePersisted` | La selección de entidades se guarda y se puede vaciar | RF-51 |
| `TestNewConnectionsFillTwoColumns` | Una conexión nueva se coloca sola en la primera página, sin solapes | RF-22 |
| `TestPanelSaveAndCascade` | Guarda páginas en orden; eliminar una conexión quita sus bloques | RF-22, RF-23 |
| `TestSettingsDefaultsAndSave` | Valores por defecto y guardado de los ajustes | RF-07, RF-24 |
| `TestPasswordChangeAndReset` | El cambio revoca las demás sesiones y la recuperación, todas | RF-08 |
| `TestMigratesExistingDatabase` | Una base de datos anterior migra, conserva sus conexiones y las coloca en el panel | RNF-05, RF-22 |

### `internal/collector`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollectsAndPassesDecryptedSecret` | La integración recibe la credencial y el estado no filtra datos de configuración | RNF-03, RNF-04 |
| `TestErrorKeepsLastDataAndRecovers` | Un fallo conserva el último dato y su fecha; luego se recupera | RF-21 |
| `TestChangingTargetDropsOldData` | Cambiar de destino descarta los datos anteriores | RF-21 |
| `TestChangingOptionsDropsOldData` | Cambiar opciones descarta los datos anteriores | RF-32 |
| `TestChangingLinkKeepsData` | Cambiar solo el enlace conserva los datos y lo actualiza | RF-25 |
| `TestChangingEntitiesDropsOldData` | Cambiar la selección descarta los datos anteriores | RF-51 |
| `TestRemoveAndUnsupported` | Eliminar detiene la conexión; un tipo sin integración no aparece | RF-10, RNF-07 |
| `TestSubscribersAreNotified` | Los suscriptores reciben los cambios | RF-20 |

### `internal/integrations/adguard`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollect` | Interpreta estado, contadores, porcentaje, tiempo medio, periodo y top de 5 | RF-30, RF-31, RF-32, RNF-02 |
| `TestTopDomainsAreOptional` | Sin la opción no hay dominios; con ella y sin datos, la lista va vacía | RF-32 |
| `TestErrors` | Clasifica credenciales erróneas y respuestas ajenas a AdGuard | RF-21 |
| `TestConnectionFailures` | Clasifica servicio caído y TLS no válido; admite la excepción de TLS | RF-21 |
| `TestRedirectIsNotFollowed` | No sigue redirecciones, para no enviar credenciales a otro destino | RNF-03 |

### `internal/integrations/proxmox`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestClusterAggregates` | Quórum, nodos, VM, LXC sin plantillas, CPU ponderada y RAM; ignora los nodos caídos | RF-40, RNF-02 |
| `TestNodesAndGuests` | Nodos ordenados con consumos y sus invitados; nodo caído sin consumos | RF-41 |
| `TestGuestsAreOptional` | Sin la opción no hay invitados ni IP, y se conservan los contadores | RF-42 |
| `TestStandaloneNode` | Un nodo independiente no tiene datos de clúster | RF-40 |
| `TestLostQuorum` | Detecta la pérdida de quórum | RF-40 |
| `TestErrors` | Token incorrecto, token sin permisos y respuestas ajenas a Proxmox. También en AdGuard | RF-21 |
| `TestSelfSignedCertificate` | Clasifica un certificado autofirmado como TLS | RF-21 |
| `TestHTTPSAgainstPlainHTTP` | Clasifica una URL `https` contra un servidor HTTP como TLS | RF-21 |

### `internal/integrations/homeassistant`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestStatusWithoutEntitiesSkipsStates` | Sin selección solo consulta el estado | RF-50 |
| `TestOnlySelectedEntitiesArePublished` | Publica solo lo elegido, sin ubicación, atributos ni otras entidades | RF-51, RF-52, RNF-02 |
| `TestMissingEntityIsReported` | Una entidad eliminada se publica como no encontrada | RF-51 |
| `TestStartingAndDegradedModes` | Detecta arranque, modo seguro y modo de recuperación | RF-50 |
| `TestListEntities` | Lista las entidades con nombre, unidad y clase | RF-51 |
| `TestSystemFromSupervisor` | Lee CPU, memoria, disco y arranque por WebSocket, sin publicar el nombre del equipo | RF-53 |
| `TestSystemIsOptional` | Sin Supervisor o sin permisos, el servicio sigue funcionando y lo indica | RF-53 |
| `TestSummary` | Cuenta por dominio, excluye grupos, puertas y garajes, y no publica personas, cerraduras ni alarmas | RF-54 |
| `TestErrors` | Token incorrecto y respuestas ajenas a Home Assistant. También en AdGuard y Proxmox | RF-21 |

## Pruebas E2E

| ID | Escenario | Requisitos |
|---|---|---|
| E2E-01 | Primer arranque y creación del administrador | RF-03 |
| E2E-02 | Cierre de sesión y acceso protegido a la administración | RF-01, RF-02, RF-04 |
| E2E-03 | Inicio de sesión incorrecto y correcto | RF-04 |
| E2E-04 | Catálogo de servicios: búsqueda y navegación | RF-12 |
| E2E-05 | Probar conexión sin guardar | RF-13 |
| E2E-06 | Alta de AdGuard y datos en el panel | RF-10, RF-30, RF-31 |
| E2E-07 | Top de dominios opcional y excluido de la API pública | RF-32 |
| E2E-08 | Cambio de estado visible en vivo y sin recargar | RF-20, RF-30 |
| E2E-09 | Caída y recuperación del servicio | RF-21 |
| E2E-10 | Conexión guardada con un servicio inaccesible | RF-10, RF-21 |
| E2E-11 | Editar y eliminar conexiones | RF-10, RF-14 |
| E2E-12 | Cambio de idioma recordado | RF-05 |
| E2E-24 | Editor: arrastrar, controles de teclado, descartar y guardar | RF-22, RNF-08 |
| E2E-25 | Varias páginas y servicios fuera del panel | RF-22, RF-23 |
| E2E-26 | Tema y color de acento | RF-24 |
| E2E-27 | Inicio de sesión opcional para ver el panel | RF-07 |
| E2E-28 | Cambio y recuperación de contraseña | RF-08 |
| E2E-13 | Móvil y modo oscuro sin desbordamiento | RF-06 |
| E2E-14 | La API pública no expone configuración ni credenciales | RNF-03 |
| E2E-23 | Enlace a cada servicio desde su bloque | RF-25 |
| E2E-15 | Sin errores de JavaScript durante el recorrido | RF-01 |
| E2E-16 | Alta de Proxmox con validación del token | RF-10, RF-13 |
| E2E-17 | Clúster en el panel: agregados, nodos y detalle | RF-40, RF-41 |
| E2E-18 | Nodo caído en el clúster | RF-21, RF-41 |
| E2E-19 | Detalle de VM y LXC opcional y excluido de la API pública | RF-42 |
| E2E-20 | Alta de Home Assistant con selección de entidades | RF-13, RF-51 |
| E2E-21 | Home Assistant en el panel: consumos, entidades y cambios en vivo | RF-20, RF-50, RF-51, RF-52, RF-53, RF-54 |
| E2E-22 | Solo las entidades elegidas salen por la API pública | RF-51 |

Los servicios simulados reproducen las rutas y la forma de respuesta usadas. Las discrepancias con el servicio real las cubren las verificaciones manuales.

## Verificaciones manuales

| Fecha | Versión de Talaia | Servicio | Resultado |
|---|---|---|---|
| 2026-11-02 | Hito 2 | AdGuard Home v0.107.79 real en local, con DNS y reglas de bloqueo reales | Correcto: credenciales erróneas, datos, protección desactivada, caída y recuperación, y actualización en vivo durante más de 35 s |
| 2026-11-02 | Hito 2 | Validación del cliente | Correcto: catálogo de servicios y opción de ocultar el top de dominios |
| 2026-11-02 | Hito 3 (`a33fbed`) | Proxmox VE 9.2.20 y AdGuard Home v0.107.77 del cliente, con el clúster en marcha | Correcto: validado por el cliente. Revisión adicional del panel y de la API pública: datos coherentes (totales de CPU, RAM, VM y LXC iguales a la suma por nodo), actualización en vivo, sin errores en la consola y sin URL, usuarios ni credenciales en la respuesta |
| 2026-11-02 | Hito 4 | Home Assistant 2026.2.3 real en local, con 117 entidades, sensores de plantilla de potencia y energía, y un interruptor | Correcto: token erróneo, listado y filtro de entidades, datos en el panel, entidad no disponible, cambio real visto en vivo y sin ubicación ni token en la API pública |
| 2026-11-02 | Hito 4 (`73d815b`) | Home Assistant 2026.10.1 del cliente, junto a sus conexiones de AdGuard y Proxmox | Correcto en la revisión del panel y de la API pública: estado en marcha, consumos reales de potencia, entidades elegidas sin datos caídos, sin errores en la consola y sin URL, ubicación ni credenciales en la respuesta. No se han comparado los valores con la interfaz de Home Assistant |
| 2026-11-02 | Hito 4, recursos y resumen | Home Assistant 2026.2.3 real en local, sin Supervisor | Correcto: el resumen coincide con los estados reales (117 entidades, 1 no disponible, 5 de 6 luces), la falta de Supervisor se indica sin afectar al resto y el enlace propio se publica |
| 2026-11-02 | Hito 4 (`dd3706c`) | Home Assistant 2026.10.1 del cliente, con Supervisor y token de administrador | Correcto: validado por el cliente. Revisión adicional: CPU, memoria, disco y arranque reales y coherentes (porcentajes iguales a usado sobre total), resumen por dominio, enlaces en los tres servicios con `noopener` y sin usuarios, credenciales, nombre del equipo, personas ni cerraduras en la API pública. Reconexión de la actualización en vivo tras reiniciar Talaia. No se han comparado los recuentos con la interfaz de Home Assistant |
| 2026-11-02 | Hito 5 | Base de datos creada por la versión del hito 4, con conexiones | Correcto: conserva el administrador y las conexiones, y las coloca en la página «Inicio» por orden alfabético y sin solapes |

**Contraste con fuentes oficiales:**
- AdGuard Home: especificación OpenAPI.
- Home Assistant: documentación de la API de Supervisor y código de `hassio/http.py` y del comando `supervisor/api`.
- Proxmox VE: fuente de la API en `PVE/API2/Cluster.pm`, de `pve-manager`, y documentación de API tokens.

**Aún sin verificar:**
- **Hito 5 con la instalación del cliente:** migración de sus datos y uso real del editor.
- **Arrastrar en pantallas táctiles.** Solo se ha probado con ratón; los controles de teclado funcionan también con toque.
- **Recursos con un token sin permisos de administrador.** La falta de Supervisor se ha probado en real; el rechazo por permisos, solo por el código de Home Assistant.
- **Home Assistant real, casos no probados:** instalaciones con miles de entidades, sensores de energía físicos, modo de recuperación y acceso por HTTPS.
- **Proxmox VE real, casos no probados:** nodo caído, pérdida de quórum y nodo independiente. Solo se han probado con el simulador.
- La instalación y el consumo en un LXC real de Proxmox.
- Otros navegadores además de Chromium.
- Compatibilidad con otras versiones de AdGuard Home.
- Accesibilidad con lector de pantalla.

## Trazabilidad

Los requisitos pendientes se probarán en su hito.

| Requisito | Cobertura |
|---|---|
| RF-01 | `TestWebFallback`, `TestDashboardIsPublicAndHidesConfiguration`, E2E-02 |
| RF-02 | `TestSetupOnlyOnceAndLogin`, `TestWritesRequireJSONAndSameOrigin`, E2E-02 |
| RF-03 | `TestSetupOnlyOnceAndLogin`, `TestFirstUserOnlyOnce`, E2E-01 |
| RF-04 | `TestLoginLimiter`, `TestLogoutRevokesSessionOnServer`, `TestPassword`, `TestExpiredSessionIsRejected`, E2E-03 |
| RF-05 | E2E-12 |
| RF-06 | E2E-13 |
| RF-10 | `TestConnectionValidation`, `TestConnectionsNeverExposeSecret`, E2E-06, E2E-11 |
| RF-11 | `TestMultipleConnectionsOfSameService` |
| RF-12 | E2E-04 |
| RF-13 | `TestConnectionTest`, E2E-05 |
| RF-14 | `TestConnectionsNeverExposeSecret`, `TestConnectionTest`, `TestConnectionSecretIsEncryptedAndKept`, E2E-11 |
| RF-20 | `TestDashboardEvents`, `TestDashboardFollowsConnectionChanges`, `TestSubscribersAreNotified`, E2E-08 |
| RF-21 | `TestErrorKeepsLastDataAndRecovers`, `TestChangingTargetDropsOldData`, `TestErrors`, `TestConnectionFailures`, E2E-09 |
| RF-30 | `TestCollect`, E2E-06, E2E-08 |
| RF-31 | `TestCollect`, E2E-06 |
| RF-32 | `TestTopDomainsAreOptional`, `TestChangingOptionsDropsOldData`, `TestOptionsArePersisted`, `TestConnectionOptionsRoundTrip`, E2E-07 |
| RF-40 | `TestClusterAggregates`, `TestStandaloneNode`, `TestLostQuorum`, E2E-17 |
| RF-41 | `TestNodesAndGuests`, E2E-17, E2E-18 |
| RF-42 | `TestGuestsAreOptional`, E2E-19 |
| RF-50 | `TestStatusWithoutEntitiesSkipsStates`, `TestStartingAndDegradedModes`, E2E-21 |
| RF-51 | `TestOnlySelectedEntitiesArePublished`, `TestMissingEntityIsReported`, `TestListEntities`, `TestListEntitiesEndpoint`, `TestEntitySelectionValidation`, `TestEntitiesArePersisted`, `TestChangingEntitiesDropsOldData`, E2E-20, E2E-21, E2E-22 |
| RF-52 | `TestOnlySelectedEntitiesArePublished`, E2E-21 |
| RF-25 | `TestConnectionLink`, `TestLinkFallsBackToConnectionURL`, `TestChangingLinkKeepsData`, E2E-23 |
| RF-53 | `TestSystemFromSupervisor`, `TestSystemIsOptional`, `TestConnectionTestReportsNotes`, E2E-21 |
| RF-54 | `TestSummary`, E2E-21 |
| RF-07 | `TestDashboardCanRequireLogin`, `TestSettingsDefaultsAndSave`, E2E-27 |
| RF-08 | `TestChangePassword`, `TestPasswordChangeAndReset`, E2E-28 |
| RF-22 | `TestPanelSaveAndValidation`, `TestUnplacedConnectionIsHidden`, `TestNewConnectionsFillTwoColumns`, `TestPanelSaveAndCascade`, E2E-24, E2E-25 |
| RF-23 | `TestPanelSaveAndValidation`, `TestPanelSaveAndCascade`, E2E-25 |
| RF-24 | `TestSettingsValidation`, `TestSettingsDefaultsAndSave`, E2E-26 |
| RNF-01 | Por diseño: el modelo de datos no tiene tablas de métricas |
| RNF-02 | `TestCollect` y `TestClusterAggregates` usan solo lecturas; por diseño, el cliente solo ofrece `GetJSON` |
| RNF-03 | `TestDashboardIsPublicAndHidesConfiguration`, `TestCollectsAndPassesDecryptedSecret`, `TestRedirectIsNotFollowed`, E2E-14 |
| RNF-04 | `TestSealOpen`, `TestKeyIsStableAndProtected`, `TestConnectionSecretIsEncryptedAndKept` |
| RNF-05 | `TestReopenKeepsData`, `TestMigratesExistingDatabase`; el binario se compila con `CGO_ENABLED=0` |
| RNF-06 | Medición manual en reposo, ver [Estado y cambios](06-estado-y-cambios.md#consumo-medido) |
| RNF-07 | `TestRemoveAndUnsupported`; el recolector solo depende de `integrations.Registry` |
| RNF-08 | E2E-24 para los controles sin arrastrar; revisión de código y de capturas; sin auditoría formal |
| RNF-09 | Compilación para arm64 en local; publicación definida en `release.yml`, aún sin ejecutar |
| RNF-10 | Revisión del repositorio |
| RNF-11 | `make docs-check` |
