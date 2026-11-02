# Requisitos

## Objetivo

Talaia es un dashboard autoalojado para ver de un vistazo el estado de los servicios de un laboratorio doméstico. Debe servir como página inicial del navegador y poder instalarlo cualquiera en Proxmox.

**Principio rector:** hacer solo lo necesario para visualizar los servicios. Si un dato ya existe en otra aplicación, se consulta allí.

## Alcance de la fase 1

- **Incluido:** dashboard, administración e integraciones de solo lectura con Proxmox VE, Home Assistant y AdGuard Home, más un LXC instalable.
- **Excluido:** histórico propio, alertas y acciones sobre los servicios, como arrancar máquinas o activar entidades.

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
| RF-07 | El dashboard puede exigir inicio de sesión si se configura así | 5 | Pendiente |
| RF-08 | El administrador puede cambiar y recuperar su contraseña | Sin asignar | Propuesto |

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
| RF-22 | El administrador elige qué widgets se muestran, y los coloca y redimensiona arrastrando | 5 | Pendiente |
| RF-23 | El dashboard admite varias páginas | 5 | Pendiente |
| RF-24 | El dashboard admite temas configurables | 5 | Pendiente |

### AdGuard Home

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-30 | Muestra el estado de la protección y del servidor DNS | 2 | Implementado |
| RF-31 | Muestra consultas, porcentaje de bloqueo y tiempo medio de respuesta | 2 | Implementado |
| RF-32 | El top de dominios consultados y bloqueados es opcional por conexión y está desactivado por defecto; desactivado, no sale por la API pública | 2 | Implementado |

### Proxmox VE

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-40 | Muestra una visión agregada del clúster | 3 | Pendiente |
| RF-41 | Muestra cada nodo con sus VM y LXC y sus consumos | 3 | Pendiente |

Una conexión de Proxmox ya se puede guardar, pero todavía no se consulta ni aparece en el dashboard.

### Home Assistant

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-50 | Muestra el estado del servicio | 4 | Pendiente |
| RF-51 | Muestra las entidades elegidas en la administración, no todas | 4 | Pendiente |
| RF-52 | Muestra consumos a partir de los sensores de potencia y energía | 4 | Pendiente |

Una conexión de Home Assistant ya se puede guardar, pero todavía no se consulta ni aparece en el dashboard.

### Instalación

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RF-60 | Un comando en el host Proxmox crea y configura un LXC autónomo | 6 | Pendiente |
| RF-61 | Hay un procedimiento de actualización | 6 | Pendiente |

## Requisitos no funcionales

| ID | Requisito | Hito | Estado |
|---|---|---|---|
| RNF-01 | Solo datos en vivo: Talaia no guarda histórico | 2 | Implementado |
| RNF-02 | Las integraciones son de solo lectura | 2 | Implementado |
| RNF-03 | El navegador nunca contacta con los servicios ni recibe sus tokens: el backend actúa de intermediario | 2 | Implementado |
| RNF-04 | Las credenciales se guardan cifradas, con la clave fuera de la base de datos | 1 | Implementado |
| RNF-05 | Autónomo: un binario con SQLite embebida y sin dependencias externas ni de Internet en ejecución | 1 | Implementado |
| RNF-06 | Funciona en un LXC de 1 vCPU, 512 MB de RAM y 4 GB de disco | 6 | Parcial: el consumo se ha medido en reposo, sin probarlo aún en un LXC real |
| RNF-07 | Añadir una integración no exige cambiar el núcleo de recolección | 2 | Implementado |
| RNF-08 | Accesibilidad básica: teclado, foco visible, estados no solo por color y respeto a la reducción de movimiento | 1 | Parcial: aplicado en el diseño, sin auditoría formal |
| RNF-09 | Binarios para Linux amd64 y arm64 | 1 | Parcial: ambos compilan; la publicación automática aún no se ha ejecutado |
| RNF-10 | Código abierto con licencia MIT y sin datos del laboratorio del autor en el repositorio | 1 | Implementado |
| RNF-11 | La documentación se mantiene al día con cada cambio | 2 | Implementado |

## Trazabilidad

La relación entre requisitos y pruebas está en el [plan de pruebas](04-plan-de-pruebas.md#trazabilidad).
