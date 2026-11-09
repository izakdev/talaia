# Plan de pruebas

## Niveles de prueba

| Nivel | Herramienta | Comando | Se ejecuta |
|---|---|---|---|
| Unitarias y de integración | `go test` con detector de carreras | `make test` | CI, en cada cambio |
| Extremo a extremo (E2E) | Playwright y Chromium contra el binario real y un AdGuard simulado | `make e2e` | CI, en cada cambio |
| Compilación | `tsc`, Vite y `go build` | `make build` | CI, en cada cambio |
| Instalador | `shellcheck` y Bash con systemd real en Docker | `make test-installer`, como root | CI, en cada cambio |
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

### `internal/integrations/portainer`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollectEnvironments` | Lee versión, entornos y recuentos con la forma de un Portainer real, y omite los que no son Docker; solo usa GET | RF-80, RF-81 |
| `TestEdgeAgentUsesHeartbeat` | Deduce el estado de un agente Edge de su latido | RF-80 |
| `TestPortainerDataHidesInfrastructure` | No publica URL, claves ni identificador de la instancia | RF-81, RNF-02 |
| `TestPortainerErrors` | Clasifica el token rechazado y las respuestas ajenas | RF-21, RF-81 |
| `TestNoEnvironments` | Sin entornos devuelve una lista vacía | RF-80 |

### `internal/integrations/wgdashboard`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollectTunnels` | Lee versión, túneles, peers y tráfico con la forma de un WGDashboard real; pide la versión sin clave y solo usa GET | RF-90, RF-91 |
| `TestWGDashboardDataHidesInfrastructure` | No publica claves, direcciones, puerto, reglas ni ajustes de los peers | RF-91, RNF-02 |
| `TestWGDashboardErrors` | Clasifica la clave rechazada, las respuestas ajenas y las que WGDashboard marca sin éxito | RF-21, RF-91 |
| `TestNoTunnels` | Sin túneles devuelve una lista vacía | RF-90 |

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
| E2E-36 | WireGuard en el bloque de OPNsense, opcional y sin nombres de peers | RF-20, RF-73, RF-74 |
| E2E-37 | La atalaya resume el peor estado y lleva al bloque | RF-26 |

Los servicios simulados reproducen las rutas y la forma de respuesta usadas. Las discrepancias con el servicio real las cubren las verificaciones manuales.

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
| 2026-11-02 | Hito 5 | Validación del cliente en su instalación | Correcto: editor y recuadro de los bloques |
| 2026-11-04 | Hito 6 | Proxmox VE real del cliente, primer intento | Fallo: el instalador eligió la plantilla arm64 en un nodo amd64 y el contenedor no arrancó. Corregido, con su prueba en INS-01. El contenedor fallido se eliminó solo, como estaba previsto |
| 2026-11-04 | Hito 6 | Proxmox VE real del cliente, segundo intento, LXC no privilegiado | Correcto según el cliente y el registro del servicio: plantilla amd64, servicio endurecido activo, enlace con código, administrador creado y las tres conexiones correctas. 18,3 MB de memoria y pico de 140,3 MB |
| 2026-11-04 | Hito 6 | Proxmox VE real del cliente, actualización y reinicio | Correcto: reinstalación con `--force` y copia de seguridad, servicio activo al reiniciar el contenedor, y conexiones y panel conservados según el cliente, con captura del panel con datos reales |
| 2026-11-04 | `v0.1.0` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Sumas correctas tras la descarga y el binario amd64 indica `v0.1.0`. No se ha instalado esta versión en un Proxmox real |
| 2026-11-04 | `v0.1.0` | Repositorio público, sin credenciales | Correcto: se descargan el instalador, los binarios y las sumas desde `releases/latest/download`, y las sumas coinciden. Sin IP ni dominios del laboratorio en el historial de Git |
| 2026-11-04 | `v0.1.0` | Proxmox VE real del cliente, actualización de `v0.1.0-rc.1` a `v0.1.0` con `--binary` | Correcto según la salida del instalador: cambio de versión con copia de seguridad; repetir el comando no hace nada porque ya está en esa versión. No se han vuelto a comprobar los datos tras este cambio |
| 2026-11-04 | `v0.1.0` | Proxmox VE real del cliente, instalador sin `--binary` | Correcto según la salida del instalador: descarga desde GitHub, suma comprobada (si no coincide, no instala) y reinstalación con `--force` y copia de seguridad, dos veces. Después, captura del cliente con su panel y datos actuales de Home Assistant y Proxmox; el contenido del bloque de AdGuard no se ve en ella |
| 2026-11-06 | Hito 8 | Portainer 2.39.1 real del cliente, con Talaia en Proxmox | Correcto según la captura y la confirmación del cliente: dos entornos en línea, 11 de 12 contenedores en marcha, 7 stacks, filas por entorno, antigüedad del recuento y versión |
| 2026-11-09 | `v0.2.2` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.2.2` e incluye el resumen del panel, y el instalador es igual al del repositorio. Aún no se ha instalado esta versión en un Proxmox real |
| 2026-11-09 | Hito 11 (sin confirmar, `v0.2.1-6-g933d621-dirty`) | Panel real del cliente con seis servicios, instalado con `talaia-proxmox.sh update --binary` | Correcto según tres capturas del cliente: «Los 6 servicios están en orden» con la atalaya del acento elegido; con la protección de AdGuard desactivada, «Necesita atención: Adguard» con el triángulo, haz y línea inferior en ámbar, y el bloque de AdGuard con su borde superior ámbar; cifras más pequeñas en todos los bloques. No se ha visto un fallo real (rojo) ni el salto a un bloque de otra página |
| 2026-11-09 | `v0.2.1` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary` | Correcto según la salida del instalador y una captura del cliente: descarga de la última versión para amd64, paso de `v0.2.0-4-g016a514` a `v0.2.1` y copia de seguridad de la anterior. En la captura, el pie indica `v0.2.1` y los bloques de Portainer, Proxmox y AdGuard muestran datos actuales en verde; el bloque de OPNsense no se ve en ella |
| 2026-11-07 | `v0.2.1` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.2.1` e incluye el WireGuard de OPNsense, y el instalador es igual al del repositorio. Aún no se ha instalado esta versión en un Proxmox real |
| 2026-11-07 | Hito 10 (`016a514`) | OPNsense 26.7.1_1 real del cliente, con una instancia y un peer conectado, y Talaia actualizado en su Proxmox VE con el instalador y `--binary` | Correcto según la salida del instalador y una captura del cliente: paso de `v0.2.0` a `v0.2.0-4-g016a514` con copia de seguridad, sección WireGuard con la instancia «Activa, 1/1 peers conectados», su tráfico (75,8 KiB recibidos y 85,3 KiB enviados) y estado general «Gateways en línea». No se ha visto una instancia caída ni un peer sin contacto reciente, ni se ha revisado la API pública con este servicio |
| 2026-11-06 | Hito 10, antes de programar | `/api/wireguard/service/show` en el OPNsense real del cliente, con el privilegio «VPN: WireGuard: Status», claves y direcciones ocultas | Responde a `GET` con `{rows, rowCount, total, current}`: una instancia `wg0` activa con su nombre en `name` e `ifname`, sin peers en `wg show`. `/api/wireguard/general/get` da `403` con ese privilegio |
| 2026-11-06 | `v0.2.0` | Versión publicada por `release.yml` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. Descargados desde `releases/latest/download`: sumas correctas, binarios ELF amd64 y arm64, el amd64 indica `v0.2.0` e incluye OPNsense, Portainer y WGDashboard, y el instalador es igual al del repositorio. No se ha instalado esta versión en un Proxmox real |
| 2026-11-06 | `v0.2.0` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin `--binary` | Correcto según la salida del instalador y una captura del cliente: descarga de la última versión para amd64, paso de `v0.1.0-16-g3611893` a `v0.2.0` con copia de seguridad, pie con `v0.2.0` y las seis conexiones (AdGuard, Home Assistant, Portainer, Proxmox, OPNsense y WGDashboard) en «Funciona» |
| 2026-11-06 | Hito 9 (`3611893`) | WGDashboard 4.3.3 real del cliente, con un túnel y un peer, y Talaia en su Proxmox VE | Correcto según dos capturas del cliente. Con el túnel activo: «Túneles activos», 1 de 1 túneles, 0 de 1 peers conectados, 1,5 GiB de tráfico (215 MiB recibidos y 1,3 GiB enviados, igual que en la exploración) y versión. Con el túnel parado: «Túnel parado» en ámbar, 0 de 1 túneles y la fila «Parado». No se ha visto un peer conectado ni se ha revisado la API pública con este servicio |
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
- WGDashboard: código de la v4.3.4, en `src/dashboard.py` y `src/modules/WireguardConfiguration.py`, para rutas, autenticación, registro de accesos y cálculo de los peers conectados.

**Aún sin verificar:**
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
| RF-22 | `TestPanelSaveAndValidation`, `TestUnplacedConnectionIsHidden`, `TestNewConnectionsFillTwoColumns`, `TestPanelSaveAndCascade`, E2E-24, E2E-25 |
| RF-23 | `TestPanelSaveAndValidation`, `TestPanelSaveAndCascade`, E2E-25 |
| RF-26 | E2E-25, E2E-37 |
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
| RF-61 | INS-04, INS-05, INS-06, INS-11, INS-12 |
| RF-62 | `TestSetupRequiresTokenWhenConfigured`, `TestSetupTokenAttemptsAreLimited`, `TestEmptySetupTokenNeverMatches`, E2E-29, INS-02 |
| RNF-01 | Por diseño: el modelo de datos no tiene tablas de métricas |
| RNF-02 | `TestCollect` y `TestClusterAggregates` usan solo lecturas; por diseño, el cliente solo ofrece `GetJSON` |
| RNF-03 | `TestDashboardIsPublicAndHidesConfiguration`, `TestCollectsAndPassesDecryptedSecret`, `TestRedirectIsNotFollowed`, E2E-14 |
| RNF-04 | `TestSealOpen`, `TestKeyIsStableAndProtected`, `TestConnectionSecretIsEncryptedAndKept` |
| RNF-05 | `TestReopenKeepsData`, `TestMigratesExistingDatabase`; el binario se compila con `CGO_ENABLED=0` |
| RNF-06 | Medición manual en reposo, ver [Estado y cambios](06-estado-y-cambios.md#consumo-medido) |
| RNF-07 | `TestRemoveAndUnsupported`; el recolector solo depende de `integrations.Registry` |
| RNF-08 | E2E-24 para los controles sin arrastrar; revisión de código y de capturas; sin auditoría formal |
| RNF-09 | Versiones `v0.1.0-rc.1`, `v0.1.0` y `v0.2.0` publicadas por `release.yml`, con ambos binarios, sumas e instalador |
| RNF-10 | Revisión del repositorio |
| RNF-11 | `make docs-check` |
