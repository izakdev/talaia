# Estado y cambios

## Hitos

| Hito | Contenido | Estado |
|---|---|---|
| 1 | Base: repositorio, CI, autenticación, SQLite y conexiones | Terminado, en `main` |
| 2 | AdGuard y flujo completo, catálogo de servicios y documentación | Terminado, pendiente de aceptar en el [PR #1](https://github.com/izakdev/talaia/pull/1) |
| 3 | Proxmox: visión agregada y detalle por nodo | Pendiente |
| 4 | Home Assistant: estado, entidades y consumos | Pendiente |
| 5 | Editor: rejilla, varias páginas, temas y login opcional | Pendiente |
| 6 | Distribución: instalador LXC, actualización y documentación final | Pendiente |

## Historial

### Hito 2 (2026-11-02)

**Añadido:**
- Integración de AdGuard Home con recolección en segundo plano.
- Dashboard en vivo mediante SSE.
- Conservación del último dato si un servicio falla.
- Botón «Probar conexión» y estado de cada conexión.
- Catálogo de servicios con iconos y búsqueda, a petición del cliente.
- Top de dominios opcional por conexión, a petición del cliente.
- Pruebas E2E reproducibles, también en CI.
- Documentación del proyecto y manual de usuario, a petición del cliente.

**Cambiado:**
- El selector de tipo de conexión desaparece del formulario.
- `docs/design.md` se sustituye por los documentos de `docs/proyecto`.

### Hito 1 (2026-11-02)

**Añadido:**
- Estructura del proyecto, licencia MIT, CI y publicación.
- Primer arranque, inicio y cierre de sesión.
- Gestión de conexiones con credenciales cifradas.
- Web en español e inglés, con modo oscuro y adaptada a móvil.

## Limitaciones conocidas

| Limitación | Impacto | Previsión |
|---|---|---|
| No se puede cambiar ni recuperar la contraseña | Olvidarla obliga a empezar de cero | RF-08, sin hito asignado |
| Sin instalador | Hay que compilar o copiar el binario y crear el servicio a mano | Hito 6 |
| Proxmox y Home Assistant no se consultan | Se pueden guardar, pero no aparecen en el dashboard | Hitos 3 y 4 |
| El dashboard muestra todas las conexiones con integración, en orden alfabético | No se puede elegir ni colocar | Hito 5 |
| Dashboard sin login | Cualquiera con acceso al puerto ve el estado | Hito 5 |
| Bloqueo de login por IP detrás de un proxy inverso | Los fallos cuentan para todos los clientes | Sin hito asignado |
| Primer arranque abierto hasta crear el administrador | Alguien en la red podría crearlo primero | Revisar en el hito 6 |

## Consumo medido

Medición en reposo con una conexión de AdGuard y un intervalo de 5 segundos, sin navegador conectado. Es orientativa y no se ha hecho en un LXC real.

| Fecha | Binario | Memoria (RSS) |
|---|---|---|
| 2026-11-02 | 15 MB | 141 MB, unos 30 segundos después de crear el administrador |

Cada cálculo de contraseña reserva 64 MiB, con un máximo de 2 simultáneos, y parte de esa memoria puede seguir contando en la medición. El objetivo de 512 MB de RNF-06 queda por confirmar en un LXC real y con varias integraciones.
