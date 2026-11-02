# Estado y cambios

## Hitos

| Hito | Contenido | Estado |
|---|---|---|
| 1 | Base: repositorio, CI, autenticación, SQLite y conexiones | Terminado, en `main` |
| 2 | AdGuard y flujo completo, catálogo de servicios y documentación | Terminado y validado por el cliente, en `main` |
| 3 | Proxmox: visión agregada y detalle por nodo | Terminado y verificado con un clúster real, en `main` |
| 4 | Home Assistant: estado, entidades, consumos, recursos y resumen; enlace a cada servicio | Terminado y verificado con la instalación del cliente, en `main` |
| 5 | Editor: rejilla, varias páginas y temas; login opcional; cambio y recuperación de contraseña | Implementado, pendiente de validar por el cliente |
| 6 | Distribución: instalador LXC, actualización y documentación final | Pendiente |

## Historial

### Hito 5 (2026-11-02)

**Añadido:**
- Editor del panel: elegir servicios, colocarlos y redimensionarlos, con controles de teclado.
- Varias páginas, con pestañas en el panel.
- Tema claro, oscuro o del sistema, y color de acento.
- Inicio de sesión opcional para ver el panel.
- Recuadro en cada bloque del panel, a petición del cliente.
- Cambio de contraseña desde la web y recuperación con `talaia reset-password`.

**Cambiado:**
- El panel muestra lo colocado en el editor, y lo que no está en él deja de salir por la API pública.
- Las conexiones existentes pasan a la página «Inicio», y las nuevas se añaden a la primera página.
- La administración se organiza en Conexiones, Panel y Ajustes.

### Hito 4 (2026-11-02)

**Añadido:**
- Integración de Home Assistant con estado, consumos y entidades elegidas.
- Selector de entidades con búsqueda y filtros.
- Detección de arranque, modo seguro y modo de recuperación.
- CPU, memoria, disco y arranque desde Supervisor, a petición del cliente.
- Resumen de luces, interruptores y otros elementos activos, a petición del cliente.
- Enlace al servicio en todos los bloques, con dirección propia opcional, a petición del cliente.

**Cambiado:**
- Desaparece el aviso «Sin datos todavía» del catálogo, porque todos los servicios ya leen datos.
- El límite de respuesta de los servicios sube de 8 a 32 MiB.
- El cambio y la recuperación de contraseña se planifican en el hito 5, a petición del cliente.

### Hito 3 (2026-11-02)

**Añadido:**
- Integración de Proxmox VE, con clúster o nodo independiente.
- Bloque con quórum, nodos, VM, LXC, CPU y RAM.
- Consumos por nodo y detalle desplegable de VM y LXC.
- Opción por conexión para ocultar el detalle.

**Cambiado:**
- Cada integración tiene su propio widget en `web/src/widgets`.
- El token de Proxmox se valida con el formato completo.
- Una URL `https` contra un servidor HTTP se clasifica como error de TLS, con un mensaje que lo indica.

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
| Sin instalador | Hay que compilar o copiar el binario y crear el servicio a mano | Hito 6 |
| CPU, memoria y disco de Home Assistant requieren Supervisor y token de administrador | Otras instalaciones no los muestran; pueden elegir sensores de System Monitor | Por diseño de Home Assistant |
| El disco de Home Assistant se muestra en GiB | Las cifras son algo menores que los GB que muestra Home Assistant, aunque el porcentaje coincide | Bajo demanda |
| Home Assistant sin probar con miles de entidades | Cada ciclo descarga todas las entidades, y podría tardar | Si se da el caso, subir el refresco |
| Estados de Home Assistant sin traducir | Salvo encendido, apagado y sin datos, se muestran como llegan, por ejemplo `heat` | Bajo demanda |
| Los consumos no se suman | No hay un total automático | Por diseño: se puede crear un sensor agregado en Home Assistant |
| Nodo caído, pérdida de quórum y nodo independiente solo probados con simulador | Puede haber diferencias en esos casos | Al darse la ocasión |
| Sin almacenamiento en la vista de Proxmox | Solo se ve el disco de cada nodo | Bajo demanda |
| Bloqueo de login por IP detrás de un proxy inverso | Los fallos cuentan para todos los clientes | Sin hito asignado |
| Una conexión tiene un solo contenido | No se pueden mostrar dos vistas distintas de un servicio | Bajo demanda, ver DEC-21 |
| Arrastrar sin probar en pantallas táctiles | Puede no ir fino en un móvil; hay controles alternativos | Al darse la ocasión |
| Primer arranque abierto hasta crear el administrador | Alguien en la red podría crearlo primero | Revisar en el hito 6 |

## Consumo medido

Medición en reposo con una conexión de AdGuard y un intervalo de 5 segundos, sin navegador conectado. Es orientativa y no se ha hecho en un LXC real.

| Fecha | Binario | Memoria (RSS) |
|---|---|---|
| 2026-11-02 | 15 MB | 141 MB, unos 30 segundos después de crear el administrador |

Cada cálculo de contraseña reserva 64 MiB, con un máximo de 2 simultáneos, y parte de esa memoria puede seguir contando en la medición. El objetivo de 512 MB de RNF-06 queda por confirmar en un LXC real y con varias integraciones.
