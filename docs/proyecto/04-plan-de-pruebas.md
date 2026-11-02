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
| `TestDashboardIsPublicAndHidesConfiguration` | El dashboard es público y sin URL, usuarios ni credenciales | RF-01, RNF-03 |
| `TestMultipleConnectionsOfSameService` | Admite varias conexiones del mismo servicio, ordenadas por nombre | RF-11 |
| `TestDashboardFollowsConnectionChanges` | Editar y eliminar se refleja; un fallo conserva el último dato | RF-20, RF-21 |
| `TestDashboardEvents` | El flujo SSE envía el estado inicial y los cambios | RF-20 |
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
| `TestMigratesExistingDatabase` | Una base de datos anterior migra y conserva sus conexiones | RNF-05 |

### `internal/collector`

| Prueba | Verifica | Requisitos |
|---|---|---|
| `TestCollectsAndPassesDecryptedSecret` | La integración recibe la credencial y el estado no filtra datos de configuración | RNF-03, RNF-04 |
| `TestErrorKeepsLastDataAndRecovers` | Un fallo conserva el último dato y su fecha; luego se recupera | RF-21 |
| `TestChangingTargetDropsOldData` | Cambiar de destino descarta los datos anteriores | RF-21 |
| `TestChangingOptionsDropsOldData` | Cambiar opciones descarta los datos anteriores | RF-32 |
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
| E2E-10 | Servicio sin integración disponible | RF-11, RF-12 |
| E2E-11 | Editar y eliminar conexiones | RF-10, RF-14 |
| E2E-12 | Cambio de idioma recordado | RF-05 |
| E2E-13 | Móvil y modo oscuro sin desbordamiento | RF-06 |
| E2E-14 | La API pública no expone configuración ni credenciales | RNF-03 |
| E2E-15 | Sin errores de JavaScript durante el recorrido | RF-01 |
| E2E-16 | Alta de Proxmox con validación del token | RF-10, RF-13 |
| E2E-17 | Clúster en el panel: agregados, nodos y detalle | RF-40, RF-41 |
| E2E-18 | Nodo caído en el clúster | RF-21, RF-41 |
| E2E-19 | Detalle de VM y LXC opcional y excluido de la API pública | RF-42 |

Los servicios simulados reproducen las rutas y la forma de respuesta usadas. Las discrepancias con el servicio real las cubren las verificaciones manuales.

## Verificaciones manuales

| Fecha | Versión de Talaia | Servicio | Resultado |
|---|---|---|---|
| 2026-11-02 | Hito 2 | AdGuard Home v0.107.79 real en local, con DNS y reglas de bloqueo reales | Correcto: credenciales erróneas, datos, protección desactivada, caída y recuperación, y actualización en vivo durante más de 35 s |
| 2026-11-02 | Hito 2 | Validación del cliente | Correcto: catálogo de servicios y opción de ocultar el top de dominios |

**Contraste con fuentes oficiales:**
- AdGuard Home: especificación OpenAPI.
- Proxmox VE: fuente de la API en `PVE/API2/Cluster.pm`, de `pve-manager`, y documentación de API tokens.

**Aún sin verificar:**
- **Un Proxmox VE real.** Es necesario para dar por cerrado el hito 3. En especial: formato real de las respuestas, permisos de `PVEAuditor`, clúster y nodo independiente, y certificado autofirmado.
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
| RF-11 | `TestMultipleConnectionsOfSameService`, E2E-10 |
| RF-12 | E2E-04, E2E-10 |
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
| RNF-01 | Por diseño: el modelo de datos no tiene tablas de métricas |
| RNF-02 | `TestCollect` y `TestClusterAggregates` usan solo lecturas; por diseño, el cliente solo ofrece `GetJSON` |
| RNF-03 | `TestDashboardIsPublicAndHidesConfiguration`, `TestCollectsAndPassesDecryptedSecret`, `TestRedirectIsNotFollowed`, E2E-14 |
| RNF-04 | `TestSealOpen`, `TestKeyIsStableAndProtected`, `TestConnectionSecretIsEncryptedAndKept` |
| RNF-05 | `TestReopenKeepsData`, `TestMigratesExistingDatabase`; el binario se compila con `CGO_ENABLED=0` |
| RNF-06 | Medición manual en reposo, ver [Estado y cambios](06-estado-y-cambios.md#consumo-medido) |
| RNF-07 | `TestRemoveAndUnsupported`; el recolector solo depende de `integrations.Registry` |
| RNF-08 | Revisión de código y de capturas; sin auditoría formal |
| RNF-09 | Compilación para arm64 en local; publicación definida en `release.yml`, aún sin ejecutar |
| RNF-10 | Revisión del repositorio |
| RNF-11 | `make docs-check` |
