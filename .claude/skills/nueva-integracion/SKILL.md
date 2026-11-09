---
name: nueva-integracion
description: Receta para añadir a Talaia una integración nueva (un servicio con su bloque en el panel) o una opción nueva en una existente, con código, pruebas, simulador, E2E, documentación y verificación real. Úsala al empezar un hito de integración, sin repasar cómo se hicieron las anteriores.
---

# Añadir una integración a Talaia

Sigue los pasos en orden. El modelo de referencia más limpio es WGDashboard (hito 9, commit `3611893`): `git show 3611893 --stat` da la lista completa de archivos. Para una opción dentro de una integración existente, el modelo es el WireGuard de OPNsense (hito 10, commit `016a514`).

Reglas que no se negocian (CLAUDE.md):
- Solo lectura: únicamente peticiones `GET` (o las de consulta que el servicio exija).
- Los datos públicos no llevan URL, credenciales, direcciones IP, puertos, claves, nombres de equipos ni de personas o peers. Ante la duda, solo recuentos.
- Ningún dato del laboratorio del cliente en el repositorio. En pruebas y simuladores, usa valores inventados y rangos de documentación (`192.0.2.0/24`, `198.51.100.0/24`, dominios `*.simulado`).
- Rama `hito-N-<nombre>` desde `main` (o desde el hito anterior si no está fusionado). Commits en español.

## 0. Explorar el servicio real antes de programar

Usa la skill `explorar-api` para ver la forma real de las respuestas y los errores de autenticación (sin clave, con clave mala). Contrasta con la documentación oficial o el código fuente del servicio y anota la fuente. Registra la exploración en `04-plan-de-pruebas.md` como fila «Hito N, antes de programar».

Pregunta al usuario las decisiones de producto que no se deducen: qué mostrar, qué estado da un fallo parcial (ámbar o rojo), qué privilegio mínimo pedir.

## 1. Backend (Go)

- `internal/integrations/<x>/<x>.go`:
  - `New(cfg integrations.Config) integrations.Integration`, con `integrations.NewHTTPClient(cfg)`.
  - `Collect(ctx) (any, error)` que devuelve un `Data` con etiquetas `json` en `snake_case`.
  - Peticiones con `integrations.GetJSON`. Los `401`/`403` ya salen como `integrations.CodeUnauthorized`. Una respuesta con forma ajena es `integrations.CodeBadResponse`.
  - Decodifica solo los campos públicos: los secretos de la respuesta nunca se leen en un struct.
  - Comentarios en español.
- `internal/integrations/<x>/<x>_test.go`, con `httptest`. Como mínimo:
  - `TestCollect…`: lee una respuesta con la forma real.
  - `Test<X>DataHides…`: serializa `Data` y comprueba que no aparece ninguna clave, IP, puerto ni nombre de la respuesta.
  - `Test<X>Errors`: clave rechazada, respuesta ajena y fallo del servicio.
- Registro en `cmd/talaia/main.go` (`integrations.Registry{…}`) y en `internal/api/api.go` (`connectionTypes`: `needsUsername`, `options`, `hasEntities`).
- Opción nueva en una integración existente: constante `Option…` en el paquete, añadida a `options` en `api.go`. Si el dato opcional da `403`, se omite y se marca `limited` (nota «Faltan algunos datos»), no es un fallo.

## 2. Frontend (React + TypeScript)

- `web/src/api.ts`: el tipo en `ConnectionType` y la interfaz `<X>Data`.
- `web/src/catalog.ts`: la entrada con `icon` de `simple-icons`, `urlPlaceholder` (`http://<x>.local:puerto`), `hasEntities`, `options` y `defaultOptions`.
- `web/src/widgets/<x>.tsx`: un `Widget<XData>` con `state` (tono `good`, `warn`, `bad` o `muted` y su etiqueta), `Body` y `footer` (nombre y versión). Reutiliza `formatBytes` y las clases existentes (`figures compact`, `ha-section`, `ha-entities gateways`, `service-foot`, `no-data`).
- `web/src/Dashboard.tsx`: importa y registra el widget.
- `web/src/i18n.tsx`: todas las claves en español y en inglés (`catalog.desc.<x>`, `form.secret.<x>`, `form.secretHint.<x>`, `type.<x>` y las del widget; para opciones, `form.option.<opción>` y `.hint`).

## 3. Simulador y E2E

- `e2e/fake-<x>.mjs`: servidor con `state` modificable (caído, vacío, sin privilegio…), datos inventados y la credencial exportada como constante.
- `e2e/run.mjs`:
  - Importa el simulador, créalo con un puerto libre (`1300N`), y añade `start()` y `stop()` donde están los demás.
  - Escenarios nuevos `["E2E-NN", "…", async () => {…}]`: alta desde el catálogo con «Probar conexión» y bloque en el panel con cada estado. Toma la captura con `shot(block, "NN-panel-<x>")` cuando los datos estén estables (espera a que desaparezca «Midiendo…» si hay velocidades).
  - Sube el recuento del catálogo en E2E-04 si añades un servicio.
  - Añade al escenario de fugas de la API pública las credenciales y valores sensibles del simulador.

## 4. Documentación (español, en el mismo cambio)

| Archivo | Qué añadir |
|---|---|
| `docs/proyecto/01-requisitos.md` | Sección del servicio con sus `RF-NN` (estado «Implementado», hito N) |
| `docs/proyecto/02-arquitectura.md` | Paquete en la tabla de componentes, nodo en el diagrama y sección del servicio: autenticación, consultas, cálculo de cada dato y qué no se publica |
| `docs/proyecto/03-diseno-funcional.md` | Credencial en la tabla del formulario, tabla de estados del bloque y datos que muestra |
| `docs/proyecto/04-plan-de-pruebas.md` | Cada `Test…` y `E2E-NN` con sus requisitos, fuente oficial contrastada, «Aún sin verificar» y trazabilidad de cada RF |
| `docs/proyecto/05-decisiones.md` | Una `DEC-NN` nueva por decisión (qué se publica, privilegios…); no se reescriben las anteriores |
| `docs/proyecto/06-estado-y-cambios.md` | Fila del hito («Programado; pendiente de verificar contra el servicio del cliente»), entrada en «Historial» y limitaciones nuevas |
| `docs/usuario/manual-de-usuario.md` | Sección del servicio en «Añadir una conexión»: cómo crear la credencial con el mínimo privilegio, qué muestra el bloque y sus estados; la captura nueva; limitaciones |
| `README.md` | La lista de integraciones, si cambia |

## 5. Comprobaciones

```sh
make test
make e2e
make docs-check
make screenshots   # luego revisa git status: restaura las capturas que solo cambian por el renderizado
```

Usa el subagente `revisor-de-hito` antes de pedir el commit.

## 6. Verificación real y PR

1. Commit y push cuando el usuario lo pida.
2. El usuario lo instala en su Proxmox con `make dist` y `talaia-proxmox.sh update --ctid ID --binary ./talaia-linux-amd64` (dale los comandos con `$PVE_HOST` e `ID` como variables suyas).
3. Con su captura, registra la verificación en `04-plan-de-pruebas.md` (qué se vio y qué no) y cambia la fila del hito en `06-estado-y-cambios.md` a «Terminado y verificado con el … del cliente».
4. Abre el PR (resumen, pruebas, verificación real y lo que queda sin verificar) y fusiona solo cuando pase la CI y el usuario lo apruebe.
