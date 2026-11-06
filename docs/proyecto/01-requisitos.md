# Requisitos

## Objetivo

Talaia es un dashboard autoalojado para ver de un vistazo el estado de los servicios de un laboratorio doméstico. Debe servir como página inicial del navegador y poder instalarlo cualquiera en Proxmox.

**Principio rector:** hacer solo lo necesario para visualizar los servicios. Si un dato ya existe en otra aplicación, se consulta allí.

## Alcance de la fase 1

- **Incluido:** dashboard, administración e integraciones de solo lectura con Proxmox VE, Home Assistant y AdGuard Home, más un LXC instalable.
- **Excluido:** histórico propio, alertas y acciones sobre los servicios, como arrancar máquinas o activar entidades.

## Alcance de la fase 2

- **Incluido:** integraciones de solo lectura con OPNsense, Portainer y WGDashboard, en este orden.
- **Aparcado:** Grafana, a petición del cliente, porque no lo usa de momento.

## Estados

- **Implementado:** disponible en el código y cubierto por pruebas.
- **Parcial:** disponible con limitaciones, indicadas en el requisito.
- **Pendiente:** planificado para el hito indicado.
- **Propuesto:** detectado durante el proyecto, todavía sin hito asignado.

## Requisitos funcionales

### Acceso y administración

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-01 | El dashboard es accesible sin iniciar sesión, para usarlo como página inicial | 1 | Implementado |
| RF-02 | La administración está separada y exige iniciar sesión | 1 | Implementado |
| RF-03 | Un asistente crea el administrador en el primer arranque y no se puede repetir | 1 | Implementado |
| RF-04 | El administrador puede iniciar y cerrar sesión; los intentos fallidos repetidos se bloquean | 1 | Implementado |
| RF-05 | La interfaz está disponible en español e inglés | 1 | Implementado |
| RF-06 | La interfaz se adapta a móvil y sigue el modo claro u oscuro del sistema | 1 | Implementado |
| RF-07 | El dashboard puede exigir inicio de sesión si se configura así, también para quien ya lo tenga abierto | 5 | Implementado |
| RF-08 | El administrador puede cambiar su contraseña desde la web y recuperarla con acceso al equipo | 5 | Implementado |

### Conexiones

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-10 | El administrador puede crear, editar y eliminar conexiones | 1 | Implementado |
| RF-11 | Puede haber varias conexiones del mismo servicio | 1 | Implementado |
| RF-12 | Los servicios se eligen en un catálogo con iconos y búsqueda, usable con decenas de entradas | 2 | Implementado |
| RF-13 | Una conexión se puede probar antes de guardarla, sin almacenar nada | 2 | Implementado |
| RF-14 | Las credenciales nunca se devuelven y al cambiar la URL hay que volver a escribirlas | 1 | Implementado |

### Dashboard

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-20 | El dashboard se actualiza solo, sin recargar la página | 2 | Implementado |
| RF-21 | Si un servicio falla, solo se degrada su bloque, que indica el fallo y conserva el último dato con su antigüedad | 2 | Implementado |
| RF-25 | Cada bloque enlaza con su servicio. El enlace es la URL de conexión o una dirección propia, porque Talaia y el navegador pueden llegar al servicio por rutas distintas | 4 | Implementado |
| RF-22 | El administrador elige qué servicios se muestran, y los coloca y redimensiona arrastrando o con controles de teclado. Lo que no está en el panel no sale por la API pública | 5 | Implementado |
| RF-23 | El dashboard admite varias páginas | 5 | Implementado |
| RF-24 | El dashboard admite tema claro, oscuro o del sistema, y color de acento | 5 | Implementado |

### AdGuard Home

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-30 | Muestra el estado de la protección y del servidor DNS | 2 | Implementado |
| RF-31 | Muestra consultas, porcentaje de bloqueo y tiempo medio de respuesta | 2 | Implementado |
| RF-32 | El top de dominios consultados y bloqueados es opcional por conexión y está desactivado por defecto; desactivado, no sale por la API pública | 2 | Implementado |

### Proxmox VE

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-40 | Muestra una visión agregada: quórum, nodos, VM, LXC, CPU y RAM. Admite clúster y nodo independiente | 3 | Implementado |
| RF-41 | Muestra cada nodo con su CPU, RAM y disco, y sus VM y LXC con sus consumos | 3 | Implementado |
| RF-42 | El detalle de VM y LXC es opcional por conexión y está activado por defecto; desactivado, no sale por la API pública | 3 | Implementado |

Verificado con un clúster real. Los casos aún sin probar en real están en el [plan de pruebas](04-plan-de-pruebas.md#verificaciones-manuales).

### Home Assistant

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-50 | Muestra el estado del servicio: en marcha, arrancando, modo seguro o de recuperación | 4 | Implementado |
| RF-51 | Muestra solo las entidades elegidas en la administración, hasta 200; las demás no salen por la API pública | 4 | Implementado |
| RF-52 | Muestra por separado los consumos, a partir de los sensores elegidos con clase de potencia o energía | 4 | Implementado |
| RF-53 | Muestra CPU y memoria de Home Assistant, y disco y arranque del equipo, si hay Supervisor y token de administrador | 4 | Implementado |
| RF-54 | Muestra un resumen opcional, activado por defecto: luces, interruptores y otros elementos activos, entidades, no disponibles, actualizaciones e integraciones. No incluye personas, cerraduras, alarmas ni puertas | 4 | Implementado |

Verificado con un Home Assistant real en local y con la instalación del cliente, con Supervisor. Los casos aún sin probar en real están en el [plan de pruebas](04-plan-de-pruebas.md#verificaciones-manuales).

### OPNsense

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-70 | Muestra CPU, tiempo encendido y, si los privilegios lo permiten, memoria, disco y versión | 7 | Implementado |
| RF-71 | Muestra el estado de cada gateway, con su latencia y pérdida si se monitoriza, y distingue los que no se monitorizan | 7 | Implementado |
| RF-72 | Muestra la velocidad y el total de tráfico de la WAN | 7 | Implementado |
| RF-73 | Solo hace lecturas, funciona con un usuario de privilegios limitados y avisa si faltan datos por ello | 7 | Implementado |

### Portainer

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-80 | Muestra cada entorno Docker y, a partir del recuento de Portainer, sus contenedores en marcha y con problemas | 8 | Implementado |
| RF-81 | Solo hace lecturas, funciona sin un usuario administrador y no publica URL ni datos de los entornos | 8 | Implementado |

### WGDashboard

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-90 | Muestra cada túnel con su estado y, según WGDashboard, sus peers conectados sobre el total y su tráfico acumulado | 9 | Implementado |
| RF-91 | Solo hace lecturas y no publica claves, direcciones, puertos ni nombres de peers | 9 | Implementado |

### Instalación

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-60 | Un comando en el host Proxmox crea y configura un LXC autónomo, no privilegiado y con arranque automático | 6 | Implementado |
| RF-61 | Un comando actualiza Talaia con copia de seguridad y vuelve a la versión anterior si la nueva no arranca | 6 | Implementado |
| RF-62 | El primer arranque instalado está protegido con un código de un solo uso, para que nadie en la red cree antes el administrador | 6 | Implementado |

## Requisitos no funcionales

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RNF-01 | Solo datos en vivo: Talaia no guarda histórico | 2 | Implementado |
| RNF-02 | Las integraciones son de solo lectura | 2 | Implementado |
| RNF-03 | El navegador nunca contacta con los servicios ni recibe sus tokens: el backend actúa de intermediario | 2 | Implementado |
| RNF-04 | Las credenciales se guardan cifradas, con la clave fuera de la base de datos | 1 | Implementado |
| RNF-05 | Autónomo: un binario con SQLite embebida y sin dependencias externas ni de Internet en ejecución | 1 | Implementado |
| RNF-06 | Funciona en un LXC de 1 vCPU, 512 MB de RAM y 4 GB de disco | 6 | Implementado |
| RNF-07 | Añadir una integración no exige cambiar el núcleo de recolección | 2 | Implementado |
| RNF-08 | Accesibilidad básica: teclado, foco visible, estados no solo por color y respeto a la reducción de movimiento | 1 | Parcial: aplicado en el diseño, sin auditoría formal |
| RNF-09 | Binarios para Linux amd64 y arm64, publicados con sus sumas de comprobación | 1 | Implementado |
| RNF-10 | Código abierto con licencia MIT y sin datos del laboratorio del autor en el repositorio | 1 | Implementado |
| RNF-11 | La documentación se mantiene al día con cada cambio | 2 | Implementado |

## Trazabilidad

La relación entre requisitos y pruebas está en el [plan de pruebas](04-plan-de-pruebas.md#trazabilidad).
