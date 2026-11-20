# Plan de pruebas

## Niveles de prueba

| Nivel | Herramienta | Comando | Se ejecuta |
|---|---|---|---|
| Unitarias y de integración | `go test` con detector de carreras | `make test` | CI, en cada cambio de código |
| Extremo a extremo (E2E) | Playwright y Chromium contra el binario real, los servicios simulados y un receptor de avisos simulado | `make e2e` | CI, en cada cambio de código |
| Compilación | `tsc`, Vite y `go build` | `make build` | CI, en cada cambio de código |
| Instalador | `shellcheck` y Bash con systemd real en Docker | `make test-installer`, como root | CI, en cada cambio de código |
| Documentación | `scripts/check_docs.py` | `make docs-check` | CI, en cada cambio |
| Clasificación de cambios de la CI | Bash | `make test-ci` | CI, en cada cambio |
| Contra servicio real | Manual, con el servicio real en local | Ver [verificaciones manuales](#verificaciones-manuales) | Al cerrar cada integración |

Un cambio es «de código» si toca algún archivo que no sea Markdown ni esté en `docs/` o `.claude/`. Si solo cambia documentación, la CI se salta `test` e `installer` y ejecuta solo el job `docs`; ver DEC-52 y las [pruebas de la clasificación](#clasificación-de-cambios-de-la-ci).

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
| `TestOPNsenseCredentialsAreCleaned` | Limpia las líneas `key=` y `secret=` pegadas de OPNsense y rechaza una clave con espacios | RF-73 |
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
| `TestSetupRequiresTokenWhenConfigured` | Con archivo de código, el asistente lo exige, no lo publica y lo elimina al usarlo | RF-62 |
| `TestSetupTokenAttemptsAreLimited` | Los intentos con un código erróneo se bloquean | RF-62 |
| `TestEmptySetupTokenNeverMatches` | Un archivo vacío no deja crear el administrador | RF-62 |
| `TestPastedTokenIsTrimmed` | Un token pegado con espacios o saltos de línea se limpia; con saltos en medio se rechaza | RF-14 |
| `TestConnectionTest` | Exige sesión, no guarda nada, reutiliza la credencial solo con la misma URL y rechaza tipos sin integración | RF-13, RF-14 |
| `TestQNAPConnectionRules` | QNAP exige `snmp://`, las dos claves a la vez y de al menos 8 caracteres; las guarda juntas y sin devolverlas, las conserva si llegan vacías, desactiva TLS y no publica la dirección como enlace | RF-102, RF-14, RF-25 |
| `TestNUTConnectionRules` | SAI (NUT) exige `nut://` y un nombre de SAI válido; se crea sin credencial ni usuario, no guarda un secreto aunque llegue, cambia de dirección sin pedirlo, desactiva TLS y no publica la dirección ni el SAI | RF-105, RF-14 |
| `TestAvailabilityConnectionRules` | Disponibilidad exige `tcp://host:puerto` o `http(s)://` sin credenciales ni consulta; se crea sin credencial ni usuario, guarda «No verificar TLS» solo con `https://` y no publica la dirección | RF-114, RF-116 |
| `TestChannelsNeverExposeSecrets` | Ninguna respuesta de canales, pruebas, registro ni ajustes incluye el token, el tópico ni la URL del webhook; una edición con secretos en blanco los conserva y la prueba los usa; el primer canal fija el idioma | RF-108, RF-111 |
| `TestChannelValidation` | Rechaza nombres vacíos o largos, tipos desconocidos, URL sin `http(s)`, tokens y chat ID de Telegram mal formados y tópicos con espacios; acepta ntfy sin servidor y un `@canal` | RF-108 |
| `TestTestChannelUsesStoredSecret` | La prueba sin guardar usa el secreto guardado si llega en blanco, informa del error del canal y queda en el registro | RF-108, RF-111 |
| `TestNotifySettingsValidation` | Rechaza idiomas y retardos fuera de la lista | RF-107, RF-110 |
| `TestConnectionNotifyRoundTrip` | Una conexión avisa por defecto y se puede silenciar | RF-109 |
| `TestNotifyRoutesRequireAuth` | Las rutas de notificaciones exigen sesión | RF-108 |
| `TestNtfyServerChangeRequiresSecrets` | Cambiar el servidor de ntfy, al editar o al probar, exige volver a escribir el tópico y el token guardado; los guardados no llegan al servidor nuevo, y el mismo servidor con barra final no cuenta como cambio | RF-108 |

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
| `TestAvailabilityLinkHidesAddress` | La dirección de una conexión de disponibilidad, también la `http(s)://`, nunca es su enlace; el enlace propio, sí | RF-116, RF-25 |
| `TestEntitiesArePersisted` | La selección de entidades se guarda y se puede vaciar | RF-51 |
| `TestNewConnectionsFillThreeColumns` | Una conexión nueva se coloca sola en la primera página, sin solapes | RF-22 |
| `TestPanelSaveAndCascade` | Guarda páginas en orden; eliminar una conexión quita sus bloques | RF-22, RF-23 |
| `TestSettingsDefaultsAndSave` | Valores por defecto y guardado de los ajustes, también los de avisos | RF-07, RF-24, RF-107 |
| `TestPasswordChangeAndReset` | El cambio revoca las demás sesiones y la recuperación, todas | RF-08 |
| `TestMigratesExistingDatabase` | Una base de datos anterior migra, conserva sus conexiones y las coloca en el panel | RNF-05, RF-22 |
| `TestChannelSecretEncrypted` | El secreto de un canal no está en claro y se recupera | RF-108, RNF-04 |
| `TestUpdateChannelKeepsSecret` | Editar sin secreto lo conserva; con secreto, lo cambia; borrar elimina el canal | RF-108 |
| `TestNotifiedToneCascade` | El color avisado se guarda y se borra con la conexión | RF-112 |
| `TestDeleteNotifiedTone` | El color avisado se puede olvidar | RF-106 |
| `TestLogKeepsLatest50` | El registro guarda los 50 envíos más recientes, del último al primero | RF-111 |
| `TestSettingsNotifyDefaults` | Sin idioma de avisos y 2 minutos por defecto; el primer canal fija el idioma una sola vez | RF-107, RF-110 |
| `TestConnectionNotifyDefaultTrue` | Una conexión, también las anteriores a la migración, avisa salvo que se silencie | RF-109 |

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
| `TestSnapshotState` | Con error, el estado es rojo «Con fallos» aunque haya un dato anterior; con datos, el de la integración | RF-113, RF-21 |
| `TestPendingSnapshotState` | Sin datos, el estado es neutro «Consultando…» | RF-113 |

### `internal/integrations/adguard`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollect` | Interpreta estado, contadores, porcentaje, tiempo medio, periodo y top de 5 | RF-30, RF-31, RF-32, RNF-02 |
| `TestTopDomainsAreOptional` | Sin la opción no hay dominios; con ella y sin datos, la lista va vacía | RF-32 |
| `TestErrors` | Clasifica credenciales erróneas y respuestas ajenas a AdGuard | RF-21 |
| `TestConnectionFailures` | Clasifica servicio caído y TLS no válido; admite la excepción de TLS | RF-21 |
| `TestRedirectIsNotFollowed` | No sigue redirecciones, para no enviar credenciales a otro destino | RNF-03 |
| `TestState` (adguard) | Mismo color y etiqueta que el bloque: DNS parado, protección activa y desactivada; cada clave está en los textos de ambos idiomas | RF-113 |

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
| `TestState` (proxmox) | Mismo color y etiqueta que el bloque: sin quórum, nodos caídos con sus cifras, clúster y nodo suelto; cada clave está en los textos de ambos idiomas | RF-113 |

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
| `TestState` (homeassistant) | Mismo color y etiqueta que el bloque: arrancando, recuperación, modo seguro, entidades sin datos con su número y en marcha; cada clave está en los textos de ambos idiomas | RF-113 |

### `internal/integrations/opnsense`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollect` | Interpreta CPU, tiempo encendido, memoria, disco, versión y WAN con la forma de un OPNsense real; solo usa GET | RF-70, RF-72, RF-73 |
| `TestGatewayStates` | Distingue gateways en línea, degradados, caídos y sin monitorizar, y lee sus medidas | RF-71 |
| `TestWANSpeedNeedsTwoReadings` | Calcula la velocidad entre lecturas y la omite en la primera y si los contadores bajan | RF-72 |
| `TestMissingPrivilegesLimitData` | Sin privilegios opcionales, omite esos datos y avisa | RF-73 |
| `TestPublicDataHidesInfrastructure` | No publica equipo, direcciones, procesos, dispositivos ni credenciales | RF-73, RNF-02 |
| `TestErrors` | Clasifica credenciales, privilegios imprescindibles, respuestas ajenas y errores del servicio | RF-21, RF-73 |
| `TestParseSize` | Interpreta los tamaños de `df -h` | RF-70 |
| `TestWireGuardIsOptional` | Sin la opción, no consulta WireGuard ni avisa por su privilegio | RF-74, RF-73 |
| `TestCollectWireGuard` | Agrupa los peers por instancia, cuenta solo los `online` como conectados, suma el tráfico y no publica claves, endpoints, IP, puertos ni nombres de peers | RF-74, RNF-02 |
| `TestWireGuardWithoutPrivilege` | Sin el privilegio, omite WireGuard y avisa | RF-74, RF-73 |
| `TestWireGuardEmptyAndForeign` | Sin instancias devuelve una lista vacía, y una respuesta ajena es un fallo | RF-74 |
| `TestState` (opnsense) | Mismo color y etiqueta que el bloque: sin gateways, caído, degradado, WireGuard caído, sin monitorizar y en línea; cada clave está en los textos de ambos idiomas | RF-113, RF-71 |

### `internal/integrations/portainer`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollectEnvironments` | Lee versión, entornos y recuentos con la forma de un Portainer real, y omite los que no son Docker; solo usa GET | RF-80, RF-81 |
| `TestEdgeAgentUsesHeartbeat` | Deduce el estado de un agente Edge de su latido | RF-80 |
| `TestPortainerDataHidesInfrastructure` | No publica URL, claves ni identificador de la instancia | RF-81, RNF-02 |
| `TestPortainerErrors` | Clasifica el token rechazado y las respuestas ajenas | RF-21, RF-81 |
| `TestNoEnvironments` | Sin entornos devuelve una lista vacía | RF-80 |
| `TestState` (portainer) | Mismo color y etiqueta que el bloque: sin entornos, entorno caído, contenedores con problemas y en línea; cada clave está en los textos de ambos idiomas | RF-113 |

### `internal/integrations/wgdashboard`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollectTunnels` | Lee versión, túneles, peers y tráfico con la forma de un WGDashboard real; pide la versión sin clave y solo usa GET | RF-90, RF-91 |
| `TestWGDashboardDataHidesInfrastructure` | No publica claves, direcciones, puerto, reglas ni ajustes de los peers | RF-91, RNF-02 |
| `TestWGDashboardErrors` | Clasifica la clave rechazada, las respuestas ajenas y las que WGDashboard marca sin éxito | RF-21, RF-91 |
| `TestNoTunnels` | Sin túneles devuelve una lista vacía | RF-90 |
| `TestState` (wgdashboard) | Mismo color y etiqueta que el bloque: sin túneles, túnel parado y activos; cada clave está en los textos de ambos idiomas | RF-113 |

### `internal/integrations/qnap`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollectQNAP` | Lee CPU, memoria (total menos disponible), temperaturas, firmware, discos por bahía y USB, y volúmenes en KiB, con la forma de un QuTS hero 5.2 real; una temperatura sin sensor no se publica, y un USB sin sensor que QNAP da como «Abnormal» queda sin datos SMART, mientras que uno con temperatura conserva su SMART | RF-100, RF-101 |
| `TestQNAPWithoutQTSMIB` | Sin QTS-MIB, usa la memoria libre y deja el firmware vacío | RF-100 |
| `TestQNAPStorageStates` | Traduce los estados de RAID, pool y volumen: reconstrucción, degradado, aviso, error, comprobación y texto desconocido | RF-101 |
| `TestQNAPFanFailure` | Detecta un ventilador averiado | RF-101 |
| `TestQNAPDataHidesInfrastructure` | No publica claves, usuario, dirección, nombre del equipo, modelos, números de serie ni nombres de volúmenes, ni recorre esas columnas | RF-102, RNF-02 |
| `TestQNAPErrors` | Clasifica la firma, el usuario y el cifrado rechazados, la falta de respuesta, la red inalcanzable y un agente que no es un QNAP | RF-21, RF-102 |
| `TestTarget` | Acepta `snmp://host[:puerto]` y rechaza otros esquemas, rutas, credenciales y puertos no válidos | RF-102 |
| `TestState` (qnap) | Mismo color y etiqueta que el bloque: cada problema en su orden de gravedad, temperaturas, volumen al 90 %, sin discos y sin temperatura; cada clave está en los textos de ambos idiomas | RF-113, RF-101 |

### `internal/integrations/nut`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollect` | Lee versión, modelo, banderas de estado, carga y autonomía con la forma de un upsd 2.8.1 real | RF-103 |
| `TestCollectPicksOnlyUPS` | Sin SAI en la dirección, usa el único del servidor | RF-105 |
| `TestCollectMinimal` | Sin `VER`, sin carga, sin autonomía y con `device.model`: deja esos datos vacíos, recorta un modelo largo a 128 caracteres y descarta banderas desconocidas | RF-103 |
| `TestNUTDataHidesPrivate` | No publica el nombre del SAI, su descripción, el número de serie, el driver ni la dirección | RF-105, RNF-02 |
| `TestNUTSendsOnlyReadCommands` | Solo envía `VER`, `LIST` y `LOGOUT` | RF-105, RNF-02 |
| `TestNUTErrors` | Clasifica SAI desconocido, ninguno o varios sin indicar, acceso denegado, driver desconectado, datos caducados, respuesta ajena, lista sin fin, falta de respuesta y servidor apagado | RF-21, RF-104, RF-105 |
| `TestTarget` (nut) | Acepta `nut://host[:puerto][/sai]` y rechaza otros esquemas, credenciales, puertos, rutas y nombres de SAI con espacios o saltos de línea | RF-105 |
| `TestParseLine` | Separa las palabras de una línea de NUT y desescapa `\"` y `\\` | RF-103 |
| `TestState` (nut) | Mismo color y etiqueta que el bloque: batería baja, con batería, cambiar batería, sobrecarga, con red y desconocido; cada clave está en los textos de ambos idiomas | RF-113, RF-104 |

### `internal/integrations/availability`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollectTCP` | Por TCP conecta, cierra y da la latencia, sin código HTTP | RF-114 |
| `TestCollectHTTP` | Por HTTP hace solo `GET`, da 200 y, ante un 301, no sigue la redirección y cuenta como respuesta | RF-114, RF-115, RNF-02 |
| `TestCollectHTTPError` | Un 503 devuelve datos con su código y el estado ámbar | RF-115 |
| `TestAvailabilityErrors` | Clasifica puerto cerrado (TCP y HTTP), plazo agotado, certificado autofirmado y su excepción de TLS, y una dirección no válida | RF-115, RF-21 |
| `TestAvailabilityDataHidesTarget` | Los datos no incluyen el host, el puerto, la ruta ni el esquema | RF-116 |
| `TestTarget` (availability) | Acepta `tcp://host:puerto` (también IPv6) y `http(s)://`; rechaza TCP sin puerto o con ruta, credenciales, consultas, fragmentos y otros esquemas | RF-114, RF-116 |
| `TestState` (availability) | Mismo color y etiqueta que el bloque: TCP, HTTP 200 y 302 en verde; 503 y 401 en ámbar con su código; cada clave está en los textos de ambos idiomas | RF-113, RF-115 |

### `internal/notify`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestComposeGroupsProblemsAndRecoveries` | El texto exacto del aviso: problemas del más grave al menos grave y luego las vueltas a verde, con el emoji del peor color | RF-110, RF-107 |
| `TestComposeWarnOnly` | Solo ámbar: 🟠 | RF-110 |
| `TestComposeEnglish` | El aviso en inglés | RF-110 |
| `TestComposeOnlyRecoveries` | Solo vueltas a verde: 🟢 | RF-106, RF-110 |
| `TestComposeTest` | El mensaje de prueba en los dos idiomas, con su título aparte | RF-108 |
| `TestComposeSanitizesNames` | Un nombre con saltos de línea o de más de 80 caracteres no rompe el mensaje | RF-110 |
| `TestLabelParams` | Las etiquetas con cifras; idioma o clave desconocidos | RF-110 |
| `TestTelegramRequest` | Telegram recibe `chat_id` y el texto plano, sin `parse_mode` | RF-108 |
| `TestNtfyJSON` | ntfy recibe tópico, título sin emoji, mensaje sin repetir el título, prioridad y etiqueta según el peor color, y el token solo si existe | RF-108 |
| `TestNtfyDefaultServer` | Sin servidor, ntfy.sh | RF-108 |
| `TestWebhookBody` | El JSON del webhook, sin el identificador interno de la conexión, y la lista vacía en la prueba | RF-108, RF-110 |
| `TestSendErrorCodes` | 401 y 403, otros 4xx, 429, 5xx, redirección y servidor caído dan su código | RF-111 |
| `TestSendErrorsHideURL` | El error no incluye la URL del webhook ni el token del bot | RF-108, RF-111 |
| `TestRetryable` | Se reintentan los fallos de red, de tiempo, 5xx y 429; no los rechazos ni los envíos sin confirmar | RF-111 |
| `TestSendTimeoutIsLonger` | Telegram, ntfy y webhook esperan 30 s, no los 10 s de las consultas al panel | RF-111 |
| `TestSlowReplyIsUnconfirmed` | Si el canal recibe el mensaje entero y no responde a tiempo, el resultado es «sin confirmar» y no se reintenta | RF-111 |
| `TestTimeoutBeforeSendingIsRetried` | Si el tiempo se agota antes de enviar (saludo TLS sin respuesta), el fallo es de tiempo y se reintenta | RF-111 |
| `TestNewSenderRejectsBadSecret` | Un secreto incompleto o un tipo desconocido no crea el emisor | RF-108 |
| `TestNotifiesAfterDelay` | Avisa al cumplirse el retardo y no antes, una sola vez, y lo anota | RF-106, RF-107, RF-111 |
| `TestFlapDoesNotNotify` | Un parpadeo reinicia el plazo | RF-107 |
| `TestColorChangeRestartsCandidate` | Pasar de ámbar a rojo durante el plazo lo reinicia | RF-107 |
| `TestMutedSkipped` | Un estado neutro ni avisa ni reinicia el plazo ni cambia el color avisado | RF-106 |
| `TestGroupsSimultaneous` | Dos cambios del mismo ciclo van en un mensaje, el más grave primero | RF-107 |
| `TestGroupsNearlySimultaneous` | Dos cambios vistos con 25 s de diferencia van en un mensaje y no se repiten | RF-107 |
| `TestGroupWindowIsLimited` | Un cambio que acaba de empezar no se adelanta más de lo permitido | RF-107 |
| `TestSendsToEveryEnabledChannel` | Cada canal activo recibe el aviso en el idioma elegido; uno desactivado, no | RF-108, RF-110 |
| `TestRecovery` | La vuelta a verde se avisa con el color anterior | RF-106 |
| `TestSilencedUpdatesTone` | Una conexión silenciada no avisa ni deja un aviso atrasado al reactivarla | RF-109 |
| `TestNoChannelsUpdatesTone` | Sin canales no se envía ni se anota nada, y el primer canal no recibe problemas antiguos | RF-106 |
| `TestNotOnPanelIgnoredAndForgotten` | Un servicio fuera del panel no avisa y se olvida su color | RF-106 |
| `TestFirstStateAssumesGood` | Un servicio nuevo en rojo avisa; en verde, no | RF-106 |
| `TestRestartDoesNotRepeat` | Un notificador nuevo sobre la misma base de datos no repite el aviso | RF-112 |
| `TestRetryThenLog` | Dos fallos y un acierto dan una entrada «Enviado» con 3 intentos | RF-111 |
| `TestRejectedIsNotRetried` | Un rechazo se anota sin reintentar | RF-111 |
| `TestUnconfirmedIsNotRetried` | Un envío sin confirmar se anota con un solo intento y no se repite | RF-111 |
| `TestRetrySkipsDeletedChannel` | Un canal borrado entre intentos no se reintenta | RF-108 |
| `TestUnreadableChannelSecret` | Un secreto ilegible da `secret_unreadable` en su canal y no frena a los demás | RF-111 |
| `TestSlowChannelDoesNotBlockOthers` | Un canal colgado no retrasa la revisión ni a los demás canales | RF-106 |
| `TestSendTest` | La prueba envía un solo intento en el idioma elegido y queda en el registro | RF-108, RF-111 |

### `web`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestStateLabelsBothLanguages` | Los textos de estado compartidos tienen las mismas claves en español e inglés | RF-113, RF-110 |

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
| E2E-12 | Cambio de idioma recordado y aplicado a las demás pestañas | RF-05 |
| E2E-24 | Editor: arrastrar, controles de teclado, alturas automáticas, descartar y guardar | RF-22, RNF-08 |
| E2E-25 | Varias páginas, servicios fuera del panel y aviso desde otra página | RF-22, RF-23, RF-26 |
| E2E-26 | Tema y color de acento | RF-24 |
| E2E-27 | Inicio de sesión opcional para ver el panel | RF-07 |
| E2E-28 | Cambio y recuperación de contraseña | RF-08 |
| E2E-29 | Primer arranque protegido con un código de instalación | RF-62 |
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
| E2E-30 | Alta de OPNsense y privilegios limitados | RF-10, RF-73 |
| E2E-31 | OPNsense en el panel: recursos, gateways y tráfico WAN | RF-20, RF-70, RF-71, RF-72 |
| E2E-32 | Alta de Portainer | RF-10, RF-81 |
| E2E-33 | Portainer en el panel: entornos, contenedores y estados | RF-20, RF-80 |
| E2E-34 | Alta de WGDashboard | RF-10, RF-91 |
| E2E-35 | WGDashboard en el panel: túneles, peers y estados | RF-20, RF-90 |
| E2E-36 | WireGuard en el bloque de OPNsense, opcional y sin nombres de peers; un nombre largo sin espacios no se parte en el móvil | RF-20, RF-73, RF-74 |
| E2E-37 | La atalaya resume el peor estado y la antigüedad, lleva al bloque, y el pie da la versión | RF-26 |
| E2E-38 | Bloques apilados por su altura real, panel a todo el ancho y editor sin cambios falsos | RF-27 |
| E2E-39 | Alta de QNAP por SNMPv3: dirección no válida, usuario rechazado, cifrado AES y sin casilla de TLS | RF-10, RF-102 |
| E2E-40 | QNAP en el panel: cifras, discos, volúmenes y temperaturas; aviso SMART, volumen casi lleno, almacenamiento degradado, caída y recuperación; sin claves, dirección, nombres ni modelos en la API pública | RF-20, RF-21, RF-100, RF-101, RF-102 |
| E2E-41 | Alta de un SAI por NUT: sin campo de clave ni de TLS, dirección no válida, varios SAI sin indicar cuál y conexión correcta con el nombre | RF-10, RF-105 |
| E2E-43 | Alta de canales de Telegram, ntfy y webhook con mensaje de prueba (en ntfy, título y texto por separado): el primero fija el idioma, y ni la administración ni el panel devuelven los secretos | RF-108, RF-111 |
| E2E-44 | Avisos agrupados con las etiquetas del panel (AdGuard sin protección y túnel parado en un solo mensaje por canal, con un minuto de retardo; en ntfy, el título aparte y sin emoji) y vuelta a la normalidad | RF-106, RF-107, RF-110, RF-113 |
| E2E-45 | Una conexión silenciada no avisa, ni al volver a activarla | RF-109 |
| E2E-46 | Avisos en inglés | RF-110 |
| E2E-47 | Un canal de Telegram con el token rechazado: error en la prueba y en «Últimos envíos», que lo conserva al borrar el canal | RF-111 |
| E2E-42 | SAI en el panel: carga, «Cargando», autonomía, modelo y versión; con batería, batería baja, cambiar batería, driver sin datos, caída y recuperación; sin nombre del SAI, serie ni dirección en la API pública | RF-20, RF-21, RF-103, RF-104, RF-105 |
| E2E-48 | Alta de dos equipos por disponibilidad, uno por TCP y otro por HTTP: sin campo de clave, TCP sin puerto rechazado y conexión correcta | RF-10, RF-114 |
| E2E-49 | Equipos en el panel: «Responde», latencia y pie «TCP» o «HTTP 200»; HTTP 503 en ámbar sin afectar al otro, caída en rojo de los dos y recuperación; sin dirección, puerto, esquema ni ruta en la API pública | RF-20, RF-21, RF-114, RF-115, RF-116 |

Los servicios simulados reproducen las rutas y la forma de respuesta usadas. El receptor de avisos simulado (`e2e/fake-notify.mjs`) hace de API de bots de Telegram, gracias a `TALAIA_TELEGRAM_API`, de servidor ntfy y de webhook, y guarda cada mensaje. El QNAP simulado es un agente SNMPv3 (`net-snmp`) con AES, porque el OpenSSL de Node no trae DES; ver DEC-43. Las discrepancias con el servicio real las cubren las verificaciones manuales.

## Pruebas del instalador

`scripts/test/proxmox-integration.sh` ejecuta el instalador real contra un contenedor con systemd. Docker sustituye a `pct`, y un servidor local sirve las versiones. Prueba el script y el servicio, pero **no** un Proxmox real ni un LXC no privilegiado.

| ID | Prueba | Requisitos |
|---|---|---|
| INS-01 | Instalación con servicio endurecido: plantilla de la arquitectura del nodo, opciones del contenedor, usuario, permisos y arranque automático | RF-60 |
| INS-02 | Código de instalación: rechaza sin él, acepta con él y lo elimina | RF-62 |
| INS-03 | Recuperación de contraseña dentro del contenedor, sin dejar archivos de root | RF-08 |
| INS-04 | Actualización con copia de seguridad, conservando datos, sesión y puerto | RF-61 |
| INS-05 | No hace nada si ya está en esa versión | RF-61 |
| INS-06 | Marcha atrás automática si la versión nueva no arranca | RF-61 |
| INS-07 | Una suma de comprobación incorrecta no crea nada | RF-60 |
| INS-08 | Una instalación fallida elimina el contenedor creado | RF-60 |
| INS-09 | No reutiliza ni borra un contenedor existente; por defecto instala la última versión | RF-60 |
| INS-10 | Rechaza opciones no válidas sin tocar nada | RF-60 |
| INS-11 | Si falla un paso con el servicio parado (`pct push`, configuración o copia), indica el comando y la línea, y deja en marcha la versión anterior con sus datos, sin conservar copias incompletas | RF-61 |
| INS-12 | La confirmación pregunta en la terminal aunque stderr vaya a un archivo, y sin terminal se niega sin parar el servicio | RF-61 |

## Clasificación de cambios de la CI

`scripts/test/ci-cambios-test.sh` comprueba `scripts/ci-cambios.sh`, que decide si la CI ejecuta las pruebas completas; ver DEC-52. No cubren requisitos del producto.

| ID | Archivos cambiados | Resultado esperado |
|---|---|---|
| CI-01 | Solo los README | Solo documentación |
| CI-02 | Archivos bajo `docs/`, también imágenes y HTML | Solo documentación |
| CI-03 | Archivos bajo `.claude/` y `CLAUDE.md` | Solo documentación |
| CI-04 | Documentación y un archivo Go | Código |
| CI-05 | Un workflow de GitHub Actions | Código |
| CI-06 | `scripts/check_docs.py` | Código |
| CI-07 | Lista vacía | Código, por prudencia |
| CI-08 | Un archivo sin salto de línea final | Solo documentación |

El workflow también prueba todo cuando no puede calcular la lista: en un push que crea la rama o reescribe su historia, o si falla `git diff`.

## Verificaciones manuales

| Fecha | Versión de Talaia | Servicio | Resultado |
|---|---|---|---|
| 2026-11-20 | Hito 17 (`v0.4.1-2-g30dd9b1-dirty`, binario de la rama) | Switch TP-Link TL-SG108PE v5 real del cliente, dado de alta como `tcp://` al puerto 80 de su web, con Talaia actualizado en su Proxmox VE con el instalador y `--binary` | Correcto según la captura del cliente: bloque en verde «Responde», latencia de 4 ms y «TCP» en el pie, junto al resto de servicios del panel. El nombre enlaza con el enlace propio que puso el cliente. Solo se ha probado un switch por TCP: no se han visto el modo HTTP, el ámbar por un código de error, el rojo de un switch desconectado ni su aviso, ni se ha revisado a mano la API pública, que cubren `TestAvailabilityDataHidesTarget`, `TestAvailabilityLinkHidesAddress`, E2E-14 y E2E-49 |
| 2026-11-20 | Hito 16 (`v0.3.1-dirty`, binario de la rama) | Bot de Telegram real del cliente en un chat privado, con Talaia actualizado en su Proxmox VE con el instalador y `--binary`, y su AdGuard Home real | Correcto según tres capturas del cliente: alta del canal con el token tecleado por el cliente; «Enviar prueba» recibida; con la protección de AdGuard desactivada, «🟠 Talaia · Necesita atención / Adguard: Protección desactivada» a las 10:05, unos 2 minutos después (retardo por defecto); al reactivarla, «🟢 Talaia · Vuelve a estar en orden / Adguard: Protección activa» a las 10:08; «Últimos envíos» con las cuatro entradas. La primera prueba dio «el servidor no responde a tiempo» (más de 10 s) pero Telegram la entregó igualmente: llegaron dos mensajes de prueba. En Safari, los desplegables de la sección se veían más bajos que los demás campos; corregido después. No se han probado ntfy ni webhook reales, un grupo de Telegram, un aviso en rojo ni varios servicios a la vez, que cubren las pruebas de `internal/notify` y E2E-43 a E2E-47 |
| 2026-11-19 | Hito 15 (`7d5a8d6`) | upsd 2.8.1 real del cliente, en otro nodo de su Proxmox, dado de alta como `nut://` sin indicar el SAI, con Talaia actualizado en su Proxmox VE con el instalador y `--binary` | Correcto según la captura del cliente: estado verde «Con red», batería 100 %, autonomía en minutos, modelo y «NUT 2.8.1» en el pie, y atalaya con los 7 servicios en orden. Toma el único SAI del servidor sin indicarlo. No se ha visto «Cargando» porque el SAI estaba cargado (`OL`), ni se han provocado un corte de luz u otros avisos, ni revisado a mano la API pública, que cubren `TestNUTDataHidesPrivate`, E2E-14 y E2E-42. La primera actualización falló sin cambiar nada porque `ssh` sin `-t` no deja confirmar; con `ssh -t` fue bien |
| 2026-11-20 | Hito 17, antes de programar | Ninguno: los switches TP-Link TL-SG108PE v5 del cliente (Easy Smart) no tienen API ni SNMP | No hubo exploración con `explorar-api`, porque no hay API que leer: la única fuente de datos sería la interfaz web con la contraseña de administrador por HTTP sin cifrar, que el cliente descarta (DEC-54). La prueba solo abre una conexión TCP o lee un código HTTP, y su comportamiento lo fijan RFC 9110 y Go, contrastados abajo. Que los switches respondan en el puerto 80 queda para la verificación real |
| 2026-11-19 | Hito 15, antes de programar | upsd 2.8.1 (NETVER 1.3) real del cliente, en otro nodo de su Proxmox, consultado por TCP desde el LXC de Talaia, con nombres, serie, puertos y direcciones ocultos | El host de Proxmox donde corre Talaia no tiene `upsd`: solo `upsmon`, cliente de otro nodo que tiene el SAI por USB. Desde el LXC, `VER`, `LIST UPS`, `LIST VAR` y `LIST CMD` responden sin `LOGIN`. Un SAI con `ups.status` «OL», `battery.charge` 100, `battery.runtime` en segundos, `battery.charge.low`, `ups.model` y `device.model`, `ups.load` y `ups.realpower.nominal`. Un SAI inexistente da `ERR UNKNOWN-UPS` y una variable inexistente, `ERR VAR-NOT-SUPPORTED`. Por eso no se piden credenciales; ver DEC-47 |
| 2026-11-02 | Hito 2 | AdGuard Home v0.107.79 real en local, con DNS y reglas de bloqueo reales | Correcto: credenciales erróneas, datos, protección desactivada, caída y recuperación, y actualización en vivo durante más de 35 s |
| 2026-11-02 | Hito 2 | Validación del cliente | Correcto: catálogo de servicios y opción de ocultar el top de dominios |
| 2026-11-02 | Hito 3 (`a33fbed`) | Proxmox VE 9.2.20 y AdGuard Home v0.107.77 del cliente, con el clúster en marcha | Correcto: validado por el cliente. Revisión adicional del panel y de la API pública: datos coherentes (totales de CPU, RAM, VM y LXC iguales a la suma por nodo), actualización en vivo, sin errores en la consola y sin URL, usuarios ni credenciales en la respuesta |
| 2026-11-02 | Hito 4 | Home Assistant 2026.2.3 real en local, con 117 entidades, sensores de plantilla de potencia y energía, y un interruptor | Correcto: token erróneo, listado y filtro de entidades, datos en el panel, entidad no disponible, cambio real visto en vivo y sin ubicación ni token en la API pública |
| 2026-11-02 | Hito 4 (`73d815b`) | Home Assistant 2026.10.1 del cliente, junto a sus conexiones de AdGuard y Proxmox | Correcto en la revisión del panel y de la API pública: estado en marcha, consumos reales de potencia, entidades elegidas sin datos caídos, sin errores en la consola y sin URL, ubicación ni credenciales en la respuesta. No se han comparado los valores con la interfaz de Home Assistant |
| 2026-11-02 | Hito 4, recursos y resumen | Home Assistant 2026.2.3 real en local, sin Supervisor | Correcto: el resumen coincide con los estados reales (117 entidades, 1 no disponible, 5 de 6 luces), la falta de Supervisor se indica sin afectar al resto y el enlace propio se publica |
| 2026-11-02 | Hito 4 (`dd3706c`) | Home Assistant 2026.10.1 del cliente, con Supervisor y token de administrador | Correcto: validado por el cliente. Revisión adicional: CPU, memoria, disco y arranque reales y coherentes (porcentajes iguales a usado sobre total), resumen por dominio, enlaces en los tres servicios con `noopener` y sin usuarios, credenciales, nombre del equipo, personas ni cerraduras en la API pública. Reconexión de la actualización en vivo tras reiniciar Talaia. No se han comparado los recuentos con la interfaz de Home Assistant |
| 2026-11-02 | Hito 5 | Base de datos creada por la versión del hito 4, con conexiones | Correcto: conserva el administrador y las conexiones, y las coloca en la página «Inicio» por orden alfabético y sin solapes |
| 2026-11-02 | Hito 5 | Validación del cliente en su instalación | Correcto: editor y recuadro de los bloques |
| 2026-11-04 | Hito 6 | Proxmox VE real del cliente, primer intento | Fallo: el instalador eligió la plantilla arm64 en un nodo amd64 y el contenedor no arrancó. Corregido, con su prueba en INS-01. El contenedor fallido se eliminó solo, como estaba previsto |
| 2026-11-04 | Hito 6 | Proxmox VE real del cliente, segundo intento, LXC no privilegiado | Correcto según el cliente y el registro del servicio: plantilla amd64, servicio endurecido activo, enlace con código, administrador creado y las tres conexiones correctas. 18,3 MB de memoria y pico de 140,3 MB |
| 2026-11-04 | Hito 6 | Proxmox VE real del cliente, actualización y reinicio | Correcto: reinstalación con `--force` y copia de seguridad, servicio activo al reiniciar el contenedor, y conexiones y panel conservados según el cliente, con captura del panel con datos reales |
| 2026-11-04 | `v0.1.0` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Sumas correctas tras la descarga y el binario amd64 indica `v0.1.0`. No se ha instalado esta versión en un Proxmox real |
| 2026-11-04 | `v0.1.0` | Repositorio público, sin credenciales | Correcto: se descargan el instalador, los binarios y las sumas desde `releases/latest/download`, y las sumas coinciden. Sin IP ni dominios del laboratorio en el historial de Git |
| 2026-11-04 | `v0.1.0` | Proxmox VE real del cliente, actualización de `v0.1.0-rc.1` a `v0.1.0` con `--binary` | Correcto según la salida del instalador: cambio de versión con copia de seguridad; repetir el comando no hace nada porque ya está en esa versión. No se han vuelto a comprobar los datos tras este cambio |
| 2026-11-04 | `v0.1.0` | Proxmox VE real del cliente, instalador sin `--binary` | Correcto según la salida del instalador: descarga desde GitHub, suma comprobada (si no coincide, no instala) y reinstalación con `--force` y copia de seguridad, dos veces. Después, captura del cliente con su panel y datos actuales de Home Assistant y Proxmox; el contenido del bloque de AdGuard no se ve en ella |
| 2026-11-06 | Hito 8 | Portainer 2.39.1 real del cliente, con Talaia en Proxmox | Correcto según la captura y la confirmación del cliente: dos entornos en línea, 11 de 12 contenedores en marcha, 7 stacks, filas por entorno, antigüedad del recuento y versión |
| 2026-11-16 | `v0.2.5` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary` | Correcto según una captura del panel: el pie indica `Talaia v0.2.5`, los seis servicios están en orden y, en una ventana de unos 1700 px, los tres bloques de un tercio de la derecha se apilan uno justo debajo del otro junto a los tres de dos tercios de la izquierda, sin huecos ni solapes. No se ha visto la salida del instalador. En la sección WireGuard de OPNsense, un nombre de instancia largo se parte a mitad de palabra |
| 2026-11-16 | `v0.2.5` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.2.5` e incluye el panel apilado, y el instalador es igual al del repositorio |
| 2026-11-14 | Hito 13 (`v0.2.4-8-g912160a`) | Panel real del cliente con seis servicios, instalado con `talaia-proxmox.sh update --binary`, en una ventana de unos 2000 px | Correcto según su captura: el panel ocupa todo el ancho; a la izquierda, tres bloques de dos tercios (Proxmox, OPNsense y Home Assistant) y, a la derecha, tres de un tercio (AdGuard, WGDashboard y Portainer) apilados uno justo debajo del otro sin esperar a los de la izquierda, y sin huecos ni solapes. No se han probado con el panel real las ventanas de 900 a 1400 px ni el móvil, que cubren E2E-38 y E2E-13 |
| 2026-11-12 | `v0.2.4` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary` | Correcto según la salida del instalador y una captura del panel: paso de `v0.2.3` a `v0.2.4` con copia de seguridad; el pie del panel indica `Talaia v0.2.4` y los bloques visibles (OPNsense con WireGuard, AdGuard, Proxmox, WGDashboard y parte de Home Assistant) muestran datos actuales en verde. No se ha comprobado con el cliente el cambio de idioma entre pestañas, que cubre E2E-12 |
| 2026-11-12 | `v0.2.4` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.2.4` e incluye la sincronización del idioma entre pestañas, y el instalador es igual al del repositorio. Aún no se ha instalado esta versión en un Proxmox real |
| 2026-11-11 | `v0.2.3` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary` | Correcto según la salida del instalador y dos capturas del cliente: paso de `v0.2.2` a `v0.2.3` con copia de seguridad; la administración indica `v0.2.3` y las direcciones de Conexiones ya no llevan espacios alrededor de la puntuación |
| 2026-11-11 | Hito 12 (`v0.2.3`) | Panel real del cliente | Correcto según su captura: «Actualizado ahora» en la cabecera y solo el servicio y su versión en el pie de cada bloque; una línea encima de cada sección (Recursos, Resumen, Consumos, Entidades); el bloque de Portainer acaba en su contenido aunque el de Home Assistant sea más alto. Además se ha visto por primera vez un fallo real: un entorno de Portainer caído pone la cabecera, la atalaya y el borde del bloque en rojo, con «Necesita atención: Portainer» |
| 2026-11-11 | `v0.2.3` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.2.3` e incluye la antigüedad en la cabecera, y el instalador es igual al del repositorio. La CI del PR de preparación necesitó cuatro intentos: los tres primeros fallaron al instalar el navegador porque el runner no resolvía el espejo de Ubuntu. Aún no se ha instalado esta versión en un Proxmox real |
| 2026-11-09 | `v0.2.2` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary` | Correcto según la salida del instalador y una captura del cliente: descarga de la última versión para amd64, paso de `v0.2.1-6-g933d621-dirty` a `v0.2.2`, los pasos nuevos «Parando Talaia y haciendo la copia de seguridad» e «Instalando la versión nueva», y copia de seguridad de la anterior. En la captura de la administración, el pie indica `v0.2.2` y las seis conexiones están en «Funciona». El panel no se ve en ella, y no se ha provocado un fallo para ver la marcha atrás |
| 2026-11-09 | `v0.2.2` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.2.2` e incluye el resumen del panel, y el instalador es igual al del repositorio. Aún no se ha instalado esta versión en un Proxmox real |
| 2026-11-09 | Hito 11 (sin confirmar, `v0.2.1-6-g933d621-dirty`) | Panel real del cliente con seis servicios, instalado con `talaia-proxmox.sh update --binary` | Correcto según tres capturas del cliente: «Los 6 servicios están en orden» con la atalaya del acento elegido; con la protección de AdGuard desactivada, «Necesita atención: Adguard» con el triángulo, haz y línea inferior en ámbar, y el bloque de AdGuard con su borde superior ámbar; cifras más pequeñas en todos los bloques. No se ha visto un fallo real (rojo) ni el salto a un bloque de otra página |
| 2026-11-09 | `v0.2.1` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary` | Correcto según la salida del instalador y una captura del cliente: descarga de la última versión para amd64, paso de `v0.2.0-4-g016a514` a `v0.2.1` y copia de seguridad de la anterior. En la captura, el pie indica `v0.2.1` y los bloques de Portainer, Proxmox y AdGuard muestran datos actuales en verde; el bloque de OPNsense no se ve en ella |
| 2026-11-07 | `v0.2.1` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.2.1` e incluye el WireGuard de OPNsense, y el instalador es igual al del repositorio. Aún no se ha instalado esta versión en un Proxmox real |
| 2026-11-07 | Hito 10 (`016a514`) | OPNsense 26.7.1_1 real del cliente, con una instancia y un peer conectado, y Talaia actualizado en su Proxmox VE con el instalador y `--binary` | Correcto según la salida del instalador y una captura del cliente: paso de `v0.2.0` a `v0.2.0-4-g016a514` con copia de seguridad, sección WireGuard con la instancia «Activa, 1/1 peers conectados», su tráfico (75,8 KiB recibidos y 85,3 KiB enviados) y estado general «Gateways en línea». No se ha visto una instancia caída ni un peer sin contacto reciente, ni se ha revisado la API pública con este servicio |
| 2026-11-06 | Hito 10, antes de programar | `/api/wireguard/service/show` en el OPNsense real del cliente, con el privilegio «VPN: WireGuard: Status», claves y direcciones ocultas | Responde a `GET` con `{rows, rowCount, total, current}`: una instancia `wg0` activa con su nombre en `name` e `ifname`, sin peers en `wg show`. `/api/wireguard/general/get` da `403` con ese privilegio |
| 2026-11-06 | `v0.2.0` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.2.0` e incluye OPNsense, Portainer y WGDashboard, y el instalador es igual al del repositorio. No se ha instalado esta versión en un Proxmox real |
| 2026-11-06 | `v0.2.0` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary` | Correcto según la salida del instalador y una captura del cliente: descarga de la última versión para amd64, paso de `v0.1.0-16-g3611893` a `v0.2.0` con copia de seguridad, pie con `v0.2.0` y las seis conexiones (AdGuard, Home Assistant, Portainer, Proxmox, OPNsense y WGDashboard) en «Funciona» |
| 2026-11-06 | Hito 9 (`3611893`) | WGDashboard 4.3.3 real del cliente, con un túnel y un peer, y Talaia en su Proxmox VE | Correcto según dos capturas del cliente. Con el túnel activo: «Túneles activos», 1 de 1 túneles, 0 de 1 peers conectados, 1,5 GiB de tráfico (215 MiB recibidos y 1,3 GiB enviados, igual que en la exploración) y versión. Con el túnel parado: «Túnel parado» en ámbar, 0 de 1 túneles y la fila «Parado». No se ha visto un peer conectado ni se ha revisado la API pública con este servicio |
| 2026-11-20 | `v0.4.1` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary`, desde el binario de prueba de `main` | Correcto según una captura del panel: el pie indica `Talaia v0.4.1` y los bloques visibles (Portainer, Home Assistant, OPNsense y WGDashboard) muestran datos actuales en verde. La velocidad WAN sale «Midiendo…» justo tras el reinicio, como prevé DEC-31. No se ha visto la salida del instalador ni se ha enviado un aviso con esta versión: los dos arreglos que trae se verificaron con el binario de prueba |
| 2026-11-20 | `v0.4.1` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, los dos indican `v0.4.1` e incluyen «Sin confirmar» y el nuevo formato de ntfy, y el instalador es igual al del repositorio. El PR de preparación pasó la CI en unos 10 s, con `test` e `installer` saltados (DEC-52) |
| 2026-11-20 | `main` con el formato de ntfy (`v0.4.0-7-g80f30a1-dirty`, binario de la rama) | SAI real del cliente por NUT, con un corte de luz provocado de unos 3 minutos, y avisos por su bot de Telegram y por ntfy.sh, con Talaia actualizado en su Proxmox VE con el instalador y `--binary` | Correcto según siete capturas del cliente. Panel: bloque del SAI en ámbar «Con batería», batería 100 % y autonomía en minutos, y la cabecera «Necesita atención: SAI». A las 15:05, un solo «🟠 Talaia · Necesita atención / SAI: Con batería» por cada canal, unos 2 minutos después del corte (retardo por defecto). Al volver la luz, a las 15:08, un solo «🟢 Talaia · Vuelve a estar en orden / SAI: Con red» por cada canal. En ntfy, una sola 🔔 o círculo de color y el texto bajo el título, también en «Enviar prueba» (13:54); antes del arreglo, a las 13:42, el emoji salía doble y el texto repetía el título. «Últimos envíos»: las seis entradas «Enviado», sin reintentos. No se han visto «Sin confirmar» (ningún canal tardó), un aviso en rojo, batería baja ni el webhook real |
| 2026-11-20 | `main` (`v0.4.0-7-g80f30a1`) | Red del LXC de Talaia hacia la API de Telegram, medida con `bash` y `getent` dentro del contenedor (sin `curl`) | DNS en 0,09 s y conexión IPv4 correcta en unos 0,03 s. El contenedor no tiene ruta IPv6 y la conexión IPv6 falla al instante («Network is unreachable»), así que no retrasa el envío. La espera de más de 10 s de la verificación del hito 16 fue, por tanto, de la propia API de Telegram, y la cubren el plazo de 30 s y «Sin confirmar» (DEC-53) |
| 2026-11-20 | `v0.4.0` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary`, desde el binario de la rama del hito 16 | Correcto según una captura del panel: el pie indica `Talaia v0.4.0` y los siete bloques muestran datos actuales en verde, igual que antes de mover el cálculo del color al servidor (DEC-48). No se ha visto la salida del instalador ni se ha provocado un aviso con esta versión: el envío real por Telegram está verificado con el binario de la rama |
| 2026-11-20 | `v0.4.0` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, los dos indican `v0.4.0` e incluyen las notificaciones (`internal/notify` y la API de Telegram), y el instalador es igual al del repositorio |
| 2026-11-20 | `v0.3.1` | Proxmox VE real del cliente, actualización desde `v0.3.0` con el instalador descargado de GitHub y sin `--binary` | Correcto según una captura del panel: el pie indica `Talaia v0.3.1` y los siete bloques muestran datos actuales en verde. El del SAI, «Con red», con batería 100 %, autonomía en minutos, el modelo y «NUT 2.8.1» en el pie. No se ha visto la salida del instalador |
| 2026-11-19 | `v0.3.1` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, los dos indican `v0.3.1` e incluyen el código de NUT (`nut://`), y el instalador es igual al del repositorio |
| 2026-11-18 | `v0.3.0` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary` | Correcto según una captura del panel: el pie indica `Talaia v0.3.0` y los siete bloques muestran datos actuales en verde. El de QNAP, «Discos en buen estado», con CPU, RAM y almacenamiento, cinco discos internos «Correcto» con su temperatura, los dos USB «Sin datos SMART», tres volúmenes «Listo» con su uso, temperaturas y firmware. En la sección WireGuard de OPNsense, el nombre largo de la instancia ya sale entero y se reparte el estado. No se ha visto la salida del instalador |
| 2026-11-18 | `v0.3.0` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.3.0` e incluye QNAP por SNMPv3 (gosnmp y los OID de QNAP), y el instalador es igual al del repositorio |
| 2026-11-18 | Hito 14 (binario de la rama, tras DEC-45) | QuTS hero 5.2.11 del cliente por SNMPv3 con SHA y DES, con Talaia en Proxmox | Correcto según la captura del cliente: estado verde «Discos en buen estado» y atalaya con los 7 servicios en orden; los dos discos USB como «Sin datos SMART»; cinco discos internos «Correcto» con su temperatura; tres volúmenes «Listo» con su uso; CPU, RAM, almacenamiento, temperaturas y firmware. No se han provocado avisos ni fallos en el NAS ni revisado a mano la API pública, que cubren `TestQNAPDataHidesInfrastructure`, E2E-14 y E2E-40 |
| 2026-11-18 | Hito 14 (binario de la rama, primera prueba) | QuTS hero 5.2.11 del cliente por SNMPv3 con SHA y DES, con Talaia en Proxmox | Correcto según la captura del cliente: alta con DES, CPU, RAM y almacenamiento, cinco discos internos «Correcto» con su temperatura, tres volúmenes «Listo» con su uso, temperaturas de CPU y sistema, y firmware. **Fallo:** los dos discos USB salían «Fallo SMART» y el bloque en rojo, porque QNAP los da como «Abnormal» (`-1` y `2`, temperatura `-1`) mientras QTS los muestra «Ready» sin datos de salud; corregido con DEC-45 y verificado en la fila anterior |
| 2026-11-17 | Hito 14, antes de programar | API web de un QuTS hero 5.2 real del cliente, con nombres, modelos, series y direcciones ocultos | El inicio de sesión (`POST authLogin.cgi`) funciona con una cuenta del grupo «everyone», pero todas las consultas de gestión responden `authPassed: 0`: hace falta el grupo «administrators». QTS responde siempre HTTP 200 y da el rechazo en el XML (`authPassed: 0`, `errorValue: -1`). Por eso se eligió SNMPv3; ver DEC-42 |
| 2026-11-17 | Hito 14, antes de programar | SNMPv3 del mismo NAS, con `snmpwalk` y `snmpget` (SHA y DES) y textos ocultos salvo cifras, unidades y estados | QTS solo ofrece CBC-DES como cifrado. Una clave mala da «Authentication failure». Responden NAS-MIB y QTS-MIB; los valores sueltos (CPU, memoria, temperaturas, firmware) solo con GET, no al recorrer. Discos con SMART «Good» y caja 0 o la de USB; volúmenes «Ready» con tamaños en KiB aunque el MIB diga bytes (comprobado con la API web); pools con estado `0`; RAID «Ready» de nivel «5» y «Single»; un ventilador con estado `0` |
| 2026-11-06 | Hito 9, antes de programar | Respuestas de un WGDashboard 4.3.3 real, con claves, direcciones y nombres ocultos | Sobre `{status, message, data}`. Sin clave da `401` «Unauthorized access.» y con una clave inexistente, `401` «API Key does not exist». La versión responde sin clave. Un túnel `wg` activo con un peer sin contacto (`stopped`, «No Handshake») y tráfico en GiB. La respuesta de túneles incluye la clave privada del servidor |
| 2026-11-05 | Hito 8, antes de programar | Respuestas de un Portainer 2.39.1 real con un usuario no administrador | `/api/endpoints` devuelve los recuentos por entorno; la lista de contenedores de Docker llega vacía, así que no se usa |
| 2026-11-04 | Hito 7, antes de programar | Respuestas de un OPNsense 26.7.1 real, con IP y MAC ocultas | Formatos confirmados: actividad, recursos, disco, información, tráfico y gateways. `core/firmware/running` da `403`. Los gateways del cliente devuelven `~` en las medidas |
| 2026-11-04 | Hito 7 | OPNsense 26.7.1 real del cliente, con Talaia en Proxmox | Correcto según la captura del cliente: estado en verde, recursos, tiempo encendido, gateway IPv4 con latencia, gateway IPv6 sin monitorizar, velocidad y total de la WAN, y versión. La monitorización desactivada devolvía `~`; al activarla sin indicar IP, el gateway PPPoE pasaba a `Offline`, y responde con una IP pública como Monitor IP. API pública revisada: sin nombre del equipo, direcciones, procesos ni dispositivos. No se han comparado las cifras con la interfaz de OPNsense |

**Contraste con fuentes oficiales:**
- AdGuard Home: especificación OpenAPI.
- Home Assistant: documentación de la API de Supervisor y código de `hassio/http.py` y del comando `supervisor/api`.
- Proxmox VE: fuente de la API en `PVE/API2/Cluster.pm`, de `pve-manager`, y documentación de API tokens.
- OPNsense: referencia de la API para rutas y métodos. El formato de las respuestas no está documentado: se ha tomado del servicio real.
- OPNsense WireGuard: código de `opnsense/core` 26.7, en `Wireguard/Api/ServiceController.php`, `scripts/wireguard/wg_show.py` y `Wireguard/ACL/ACL.xml`, para la forma de los registros, el estado de los peers y los privilegios.
- QNAP: MIB que descarga el propio NAS (QTS-MIB y NAS-MIB 2.0), para OID, tipos y estados de pools, discos y ventiladores. Para la API web descartada, `python-qnapstats` y la documentación de los plugins de QNAP de Linuxfabrik, que exigen el grupo «administrators».
- Telegram: documentación de la Bot API, `sendMessage`, para la ruta, `chat_id` y el texto sin `parse_mode`. ntfy: documentación de publicación, en JSON a la raíz del servidor (`topic`, `title`, `message`, `priority` y `tags`) y con `Authorization: Bearer`.
- NUT: guía de desarrollo de Network UPS Tools, «Network protocol information», para las órdenes, las respuestas y los errores; y las páginas de `upsd.conf` y `upsd.users`, que confirman que leer no exige usuario y que `LISTEN` decide en qué direcciones escucha.
- Disponibilidad: RFC 9110 (semántica de HTTP), apartado 15, para las clases de código 2xx y 3xx; y la documentación de `net.Dialer` y `net/http` de Go para los plazos y la clasificación de errores. Los TL-SG108PE v5 (TP-Link Easy Smart) no tienen API ni SNMP: solo su interfaz web por HTTP, que exige la contraseña de administrador y no se usa.
- WGDashboard: código de la v4.3.4, en `src/dashboard.py` y `src/modules/WireguardConfiguration.py`, para rutas, autenticación, registro de accesos y cálculo de los peers conectados.

**Aún sin verificar:**
- **Disponibilidad real, casos no probados:** un switch TP-Link TL-SG108PE v5 real responde por TCP (2026-11-20), pero no se han probado con equipos reales el modo HTTP o HTTPS, un código de error, un equipo caído y su aviso, IPv6 ni un equipo lento. Solo se han probado con el simulador y los servidores de prueba de Go.
- **Avisos reales por webhook:** solo se han probado con el receptor simulado. Telegram y ntfy.sh, solo con avisos en ámbar y verde de un servicio cada vez: no se han probado un grupo o un canal de Telegram, un servidor ntfy propio, un aviso en rojo real, varios servicios a la vez ni un envío «Sin confirmar».
- **SAI real por NUT, casos no probados:** batería baja durante un corte (el corte con batería sí, el 2026-11-20), batería cargándose («Cargando»), aviso de cambiar batería, sobrecarga, driver desconectado, varios SAI en un servidor y SAI de otros fabricantes. Solo se han probado con el simulador.
- **QNAP real, casos no probados:** aviso o fallo SMART, RAID degradado o reconstruyéndose, pool en aviso, ventilador averiado, volumen lleno y QTS clásico (no hero). Los textos de estado de RAID y volumen distintos de «Ready» no están documentados; solo se han probado con el simulador.
- **WireGuard de OPNsense real, casos no probados:** instancia caída, peers sin contacto reciente y varias instancias. Solo se han probado con el simulador. Tampoco se ha revisado la API pública con este servicio, aunque la cubren `TestCollectWireGuard` y E2E-36.
- **WGDashboard real, casos no probados:** peers conectados, varios túneles y túneles AmneziaWG. Solo se han probado con el simulador. Tampoco se ha revisado la API pública con este servicio, aunque la cubren `TestWGDashboardDataHidesInfrastructure` y E2E-14.
- **Agentes Edge y contenedores con problemas en un Portainer real.** Solo se han probado con el simulador.
- **OPNsense real, casos no probados:** gateway degradado o caído, comparación de las cifras con su interfaz y nombres exactos de los privilegios mínimos.
- **Marcha atrás automática en un Proxmox VE real.** Solo se ha probado en CI (INS-06 e INS-11).
- **Instalación nueva con descarga desde GitHub en un Proxmox VE real.** La descarga se ha probado al actualizar; las instalaciones reales usaron `--binary`.
- **Arrastrar en pantallas táctiles.** Solo se ha probado con ratón; los controles de teclado funcionan también con toque.
- **Recursos con un token sin permisos de administrador.** La falta de Supervisor se ha probado en real; el rechazo por permisos, solo por el código de Home Assistant.
- **Home Assistant real, casos no probados:** instalaciones con miles de entidades, sensores de energía físicos, modo de recuperación y acceso por HTTPS.
- **Proxmox VE real, casos no probados:** nodo caído, pérdida de quórum y nodo independiente. Solo se han probado con el simulador.
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
| RF-14 | `TestPastedTokenIsTrimmed`, `TestConnectionsNeverExposeSecret`, `TestConnectionTest`, `TestConnectionSecretIsEncryptedAndKept`, E2E-11 |
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
| RF-08 | `TestChangePassword`, `TestPasswordChangeAndReset`, E2E-28, INS-03 |
| RF-22 | `TestPanelSaveAndValidation`, `TestUnplacedConnectionIsHidden`, `TestNewConnectionsFillThreeColumns`, `TestPanelSaveAndCascade`, E2E-24, E2E-25 |
| RF-23 | `TestPanelSaveAndValidation`, `TestPanelSaveAndCascade`, E2E-25 |
| RF-26 | E2E-25, E2E-37 |
| RF-27 | `TestNewConnectionsFillThreeColumns`, E2E-24, E2E-38 |
| RF-24 | `TestSettingsValidation`, `TestSettingsDefaultsAndSave`, E2E-26 |
| RF-60 | INS-01, INS-07, INS-08, INS-09, INS-10 |
| RF-70 | `TestCollect`, `TestParseSize`, E2E-31 |
| RF-71 | `TestGatewayStates`, E2E-31 |
| RF-72 | `TestCollect`, `TestWANSpeedNeedsTwoReadings`, E2E-31 |
| RF-73 | `TestCollect`, `TestMissingPrivilegesLimitData`, `TestPublicDataHidesInfrastructure`, `TestErrors`, `TestOPNsenseCredentialsAreCleaned`, E2E-30 |
| RF-74 | `TestWireGuardIsOptional`, `TestCollectWireGuard`, `TestWireGuardWithoutPrivilege`, `TestWireGuardEmptyAndForeign`, E2E-36 |
| RF-80 | `TestCollectEnvironments`, `TestEdgeAgentUsesHeartbeat`, `TestNoEnvironments`, E2E-33 |
| RF-81 | `TestCollectEnvironments`, `TestPortainerDataHidesInfrastructure`, `TestPortainerErrors`, E2E-32 |
| RF-90 | `TestCollectTunnels`, `TestNoTunnels`, E2E-35 |
| RF-91 | `TestCollectTunnels`, `TestWGDashboardDataHidesInfrastructure`, `TestWGDashboardErrors`, E2E-34 |
| RF-100 | `TestCollectQNAP`, `TestQNAPWithoutQTSMIB`, E2E-40 |
| RF-101 | `TestQNAPStorageStates`, `TestQNAPFanFailure`, E2E-40 |
| RF-102 | `TestQNAPDataHidesInfrastructure`, `TestQNAPErrors`, `TestTarget`, `TestQNAPConnectionRules`, E2E-39, E2E-40 |
| RF-103 | `TestCollect`, `TestCollectMinimal`, `TestParseLine`, E2E-42 |
| RF-104 | `TestNUTErrors`, E2E-42 |
| RF-105 | `TestNUTDataHidesPrivate`, `TestNUTSendsOnlyReadCommands`, `TestCollectPicksOnlyUPS`, `TestNUTErrors`, `TestTarget` (nut), `TestNUTConnectionRules`, E2E-41, E2E-42 |
| RF-106 | `TestNotifiesAfterDelay`, `TestRecovery`, `TestMutedSkipped`, `TestNotOnPanelIgnoredAndForgotten`, `TestFirstStateAssumesGood`, `TestNoChannelsUpdatesTone`, `TestSlowChannelDoesNotBlockOthers`, E2E-44 |
| RF-107 | `TestNotifiesAfterDelay`, `TestFlapDoesNotNotify`, `TestColorChangeRestartsCandidate`, `TestGroupsSimultaneous`, `TestGroupsNearlySimultaneous`, `TestGroupWindowIsLimited`, `TestNotifySettingsValidation`, E2E-44 |
| RF-108 | `TestNtfyServerChangeRequiresSecrets`, `TestTelegramRequest`, `TestNtfyJSON`, `TestWebhookBody`, `TestChannelsNeverExposeSecrets`, `TestChannelValidation`, `TestChannelSecretEncrypted`, `TestSendErrorsHideURL`, `TestSendTest`, E2E-43 |
| RF-109 | `TestSilencedUpdatesTone`, `TestConnectionNotifyRoundTrip`, `TestConnectionNotifyDefaultTrue`, E2E-45 |
| RF-110 | `TestComposeGroupsProblemsAndRecoveries`, `TestComposeEnglish`, `TestComposeSanitizesNames`, `TestLabelParams`, `TestStateLabelsBothLanguages`, E2E-44, E2E-46 |
| RF-111 | `TestLogKeepsLatest50`, `TestSendErrorCodes`, `TestRetryThenLog`, `TestRejectedIsNotRetried`, `TestSendTimeoutIsLonger`, `TestSlowReplyIsUnconfirmed`, `TestTimeoutBeforeSendingIsRetried`, `TestUnconfirmedIsNotRetried`, `TestUnreadableChannelSecret`, E2E-47 |
| RF-112 | `TestRestartDoesNotRepeat`, `TestNotifiedToneCascade` |
| RF-113 | `TestState` (cada integración), `TestSnapshotState`, `TestPendingSnapshotState`, E2E-44 y todas las E2E de colores de bloque |
| RF-114 | `TestCollectTCP`, `TestCollectHTTP`, `TestTarget` (availability), `TestAvailabilityConnectionRules`, E2E-48, E2E-49 |
| RF-115 | `TestCollectHTTP`, `TestCollectHTTPError`, `TestAvailabilityErrors`, `TestState` (availability), E2E-49 |
| RF-116 | `TestAvailabilityDataHidesTarget`, `TestTarget` (availability), `TestAvailabilityConnectionRules`, `TestAvailabilityLinkHidesAddress`, E2E-49 |
| RF-61 | INS-04, INS-05, INS-06, INS-11, INS-12 |
| RF-62 | `TestSetupRequiresTokenWhenConfigured`, `TestSetupTokenAttemptsAreLimited`, `TestEmptySetupTokenNeverMatches`, E2E-29, INS-02 |
| RNF-01 | Por diseño: el modelo de datos no tiene tablas de métricas |
| RNF-02 | `TestCollect` y `TestClusterAggregates` usan solo lecturas; por diseño, el cliente HTTP solo ofrece `GetJSON` y el de QNAP solo GET y walk de SNMP |
| RNF-03 | `TestDashboardIsPublicAndHidesConfiguration`, `TestCollectsAndPassesDecryptedSecret`, `TestRedirectIsNotFollowed`, E2E-14 |
| RNF-04 | `TestSealOpen`, `TestKeyIsStableAndProtected`, `TestConnectionSecretIsEncryptedAndKept`, `TestChannelSecretEncrypted` |
| RNF-05 | `TestReopenKeepsData`, `TestMigratesExistingDatabase`; el binario se compila con `CGO_ENABLED=0` |
| RNF-06 | Medición manual en reposo, ver [Estado y cambios](06-estado-y-cambios.md#consumo-medido) |
| RNF-07 | `TestRemoveAndUnsupported`; el recolector solo depende de `integrations.Registry` |
| RNF-08 | E2E-24 para los controles sin arrastrar; revisión de código y de capturas; sin auditoría formal |
| RNF-09 | Versiones `v0.1.0-rc.1`, `v0.1.0` y `v0.2.0` publicadas por `release.yml`, con ambos binarios, sumas e instalador |
| RNF-10 | Revisión del repositorio |
| RNF-11 | `make docs-check` |
