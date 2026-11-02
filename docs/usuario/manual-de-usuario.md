# Manual de usuario de Talaia

Talaia es un panel para ver de un vistazo el estado de los servicios de tu laboratorio doméstico. Está pensado para usarlo como página inicial del navegador.

> **Versión de este manual:** hito 3, en desarrollo. Hoy muestran datos AdGuard Home y Proxmox VE. Home Assistant se puede configurar, pero todavía no aparece en el panel. Consulta las [limitaciones actuales](#limitaciones-actuales).

## Índice

1. [Instalación](#instalación)
2. [Primer arranque](#primer-arranque)
3. [El panel](#el-panel)
4. [La administración](#la-administración)
5. [Añadir una conexión](#añadir-una-conexión)
6. [Editar y eliminar conexiones](#editar-y-eliminar-conexiones)
7. [Idioma y apariencia](#idioma-y-apariencia)
8. [Seguridad y acceso remoto](#seguridad-y-acceso-remoto)
9. [Copias de seguridad](#copias-de-seguridad)
10. [Solución de problemas](#solución-de-problemas)
11. [Limitaciones actuales](#limitaciones-actuales)

## Instalación

Todavía no hay un instalador para Proxmox. Mientras tanto, hay que compilar Talaia a partir del código.

**Necesitas:** `git`, `make`, Go y Node.js 22 o posterior.

```sh
git clone https://github.com/izakdev/talaia.git
cd talaia
make build
./bin/talaia -listen :8080 -data ./data
```

Abre `http://IP_DEL_EQUIPO:8080` en el navegador.

**Opciones de arranque:**

| Parámetro | Variable de entorno | Por defecto | Función |
|---|---|---|---|
| `-listen` | `TALAIA_LISTEN` | `:8080` | Dirección y puerto |
| `-data` | `TALAIA_DATA_DIR` | `data` | Carpeta de datos |
| `-key-file` | `TALAIA_KEY_FILE` | `secret.key` en la carpeta de datos | Clave de cifrado |

## Primer arranque

La primera vez, Talaia te pide crear el administrador. Hazlo nada más arrancar, porque hasta entonces cualquiera en tu red podría crearlo.

![Pantalla para crear el administrador](img/01-primer-arranque.png)

1. Escribe un usuario.
2. Escribe una contraseña de al menos 12 caracteres.
3. Pulsa **Crear administrador**.

Guarda bien la contraseña: todavía no se puede cambiar ni recuperar.

## El panel

El panel es la página principal y no necesita iniciar sesión. Se actualiza solo, sin recargar la página.

Si aún no hay servicios, te lleva a la administración para añadirlos:

![Panel sin servicios](img/02-panel-vacio.png)

Cuando hay servicios, cada uno tiene su propio bloque:

![Panel con un servicio de AdGuard](img/07-panel.png)

**Estado de AdGuard:**

| Indicador | Significado |
|---|---|
| Círculo verde: **Protección activa** | Todo funciona y se están bloqueando dominios |
| Triángulo ámbar: **Protección desactivada** | AdGuard responde, pero no está filtrando |
| Cuadrado rojo: **Servidor DNS parado** | AdGuard responde, pero su DNS no está en marcha |
| Cuadrado rojo: **Con fallos** | Talaia no puede consultar el servicio |
| Círculo vacío: **Consultando…** | Aún no hay resultado |

**Datos de AdGuard:**

| Dato | Significado |
|---|---|
| Consultas | Consultas DNS en el periodo indicado. Pasa el cursor para ver el número exacto |
| Bloqueadas | Porcentaje bloqueado por filtros, navegación segura y control parental |
| Tiempo medio | Tiempo medio de respuesta |

El porcentaje puede ser algo mayor que el dato «bloqueadas por filtros» de AdGuard, porque incluye también los otros dos motivos.

**Estado de Proxmox:**

| Indicador | Significado |
|---|---|
| Círculo verde: **Clúster correcto** o **Nodo en línea** | Todos los nodos responden |
| Triángulo ámbar: **N de M nodos en línea** | Algún nodo está sin conexión |
| Cuadrado rojo: **Clúster sin quórum** | No hay mayoría de nodos para tomar decisiones |
| Cuadrado rojo: **Con fallos** | Talaia no puede consultar el servicio |

**Datos de Proxmox:**

![Panel con un clúster de Proxmox](img/11-panel-proxmox.png)

| Dato | Significado |
|---|---|
| Nodos | Nodos en línea sobre el total |
| VM y LXC en marcha | En marcha sobre el total, sin contar plantillas |
| CPU y RAM | Uso conjunto de los nodos en línea |
| Por nodo | Uso de CPU, RAM y disco. Con un 90 % o más, el valor se resalta |

Si tienes activo el detalle, pulsa un nodo para ver sus VM y LXC:

![Detalle de las VM y LXC de un nodo](img/12-proxmox-detalle.png)

**Si un servicio falla,** su bloque indica el motivo y mantiene atenuado el último dato conocido, junto con su antigüedad. Se recupera solo cuando el servicio vuelve.

![Bloque de un servicio con fallos](img/09-panel-con-fallos.png)

Si aparece **Sin actualización en vivo**, se ha perdido la comunicación con Talaia. El panel reintenta solo, y los datos pueden estar desactualizados hasta entonces.

## La administración

Entra con el enlace **Administración** del panel o en `/admin`. Si no tienes sesión, te pedirá tu usuario y contraseña. La sesión dura 30 días o hasta que pulses **Cerrar sesión**.

![Pantalla de inicio de sesión](img/03-inicio-de-sesion.png)

Tras 10 intentos fallidos desde una misma IP, tendrás que esperar 15 minutos o reiniciar Talaia.

En **Conexiones** ves tus servicios, su configuración y si funcionan:

![Lista de conexiones](img/06-conexiones.png)

## Añadir una conexión

1. En **Conexiones**, pulsa **Añadir conexión**.
2. Busca o elige el servicio. Los marcados con **Sin datos todavía** se pueden guardar, pero aún no aparecen en el panel.

   ![Catálogo de servicios](img/04-catalogo.png)

3. Rellena el formulario.
4. Pulsa **Probar conexión**, si está disponible. No guarda nada.
5. Pulsa **Guardar**.

![Formulario de conexión de AdGuard](img/05-formulario-adguard.png)

**Campos comunes:**

| Campo | Qué poner |
|---|---|
| Nombre | Cómo quieres verlo en el panel |
| URL | Dirección del servicio con `http://` o `https://` y su puerto |
| Refresco | Cada cuántos segundos se consulta, de 5 a 3600 |
| No verificar el certificado TLS | Solo para certificados autofirmados en una red de confianza |

Talaia, no tu navegador, es quien se conecta al servicio. La URL debe ser accesible desde el equipo donde se ejecuta Talaia.

### AdGuard Home

| Campo | Qué poner |
|---|---|
| URL | La de la interfaz web de AdGuard, por ejemplo `http://adguard.local:3000` |
| Usuario de AdGuard | Tu usuario de la interfaz web |
| Contraseña de AdGuard | Su contraseña |
| Mostrar los dominios más consultados y bloqueados | Opcional. Añade dos listas al panel |

Las listas de dominios están desactivadas por defecto porque añaden detalle a una vista rápida y pueden revelar hábitos de navegación: **cualquiera que pueda abrir el panel las verá**.

![Panel con el top de dominios](img/08-panel-con-dominios.png)

### Proxmox VE

Talaia solo necesita permisos de lectura. Crea un usuario y un token específicos ejecutando esto en un nodo:

```sh
pveum user add talaia@pve
pveum acl modify / --users talaia@pve --roles PVEAuditor
pveum user token add talaia@pve dashboard --privsep 1
pveum acl modify / --tokens 'talaia@pve!dashboard' --roles PVEAuditor
```

El tercer comando muestra el valor del token una sola vez. Cópialo.

| Campo | Qué poner |
|---|---|
| URL | La de cualquier nodo, por ejemplo `https://proxmox.local:8006` |
| API token de Proxmox | `talaia@pve!dashboard=VALOR`, con el valor que has copiado |
| Mostrar el detalle de cada VM y LXC | Activado por defecto. Desactívalo para ver solo los consumos por nodo |
| No verificar el certificado TLS | Márcalo si usas el certificado autofirmado de Proxmox |

**Clúster:** basta con una conexión a cualquiera de sus nodos. Si ese nodo se cae, Talaia no podrá consultar el clúster.

**Nodos independientes:** añade una conexión por nodo.

**Datos visibles:** cualquiera que pueda abrir el panel verá los nombres de los nodos y, con el detalle activo, los de las VM y LXC.

**Si actualizas desde una versión anterior:** las conexiones de Proxmox ya guardadas tienen el detalle desactivado hasta que lo actives al editarlas.

### Home Assistant

Se puede guardar, pero todavía no se muestran datos. Usa un token de larga duración, que se crea en tu perfil de Home Assistant, en **Seguridad**.

## Editar y eliminar conexiones

**Editar:** pulsa **Editar** en la conexión. Deja la credencial vacía para conservar la actual. Si cambias la URL, tendrás que volver a escribirla: así evitas enviarla por error a otro servidor.

**Eliminar:** pulsa **Eliminar** y confirma en la misma fila. No se puede deshacer, y el servicio desaparece del panel.

## Idioma y apariencia

- **Idioma:** Talaia usa el de tu navegador si es español y, si no, inglés. El botón **English** o **Español** lo cambia y se recuerda en ese navegador.
- **Modo claro u oscuro:** sigue la configuración de tu sistema.
- **Móvil:** la interfaz se adapta a pantallas pequeñas.

![Panel en móvil y modo oscuro](img/10-movil-oscuro.png)

## Seguridad y acceso remoto

- **El panel es público:** cualquiera que llegue a su puerto puede ver el estado de tus servicios. **No lo publiques en Internet** sin protegerlo con otra capa, como una VPN o la autenticación de un proxy inverso.
- **Credenciales:** se guardan cifradas y nunca se muestran.
- **HTTPS:** Talaia sirve HTTP. Para usar HTTPS, ponlo detrás de un proxy inverso, como Nginx Proxy Manager, que envíe la cabecera `X-Forwarded-Proto`.
- **Detrás de un proxy:** Talaia ve la IP del proxy, de modo que los inicios de sesión fallidos cuentan para todos los clientes.

## Copias de seguridad

Toda la configuración está en la carpeta de datos:

| Archivo | Contenido |
|---|---|
| `talaia.db` y archivos `talaia.db-*` | Administrador y conexiones |
| `secret.key` | Clave para descifrar las credenciales |

1. Para Talaia.
2. Copia la carpeta de datos completa a un lugar seguro.
3. Vuelve a arrancarlo.

**Protege la copia:** contiene la clave junto a las credenciales cifradas, así que quien la tenga puede leerlas. Sin `secret.key`, las credenciales no se pueden recuperar y tendrás que volver a escribirlas en cada conexión.

## Solución de problemas

**Mensajes de un servicio:**

| Mensaje | Qué hacer |
|---|---|
| Credenciales rechazadas por el servicio | Revisa el usuario y la credencial. En Proxmox, comprueba también que el token tenga el rol `PVEAuditor` en `/`: con la separación de privilegios, tanto el usuario como el token lo necesitan |
| El servicio no ha respondido a tiempo | Comprueba que el servicio funciona y no está saturado |
| No se puede conectar con el servicio | Revisa la URL y el puerto, y que el equipo de Talaia tenga acceso, incluidas las reglas del firewall entre redes |
| Error de TLS | Revisa si la URL usa `http` o `https`. Si el certificado es autofirmado, marca **No verificar el certificado TLS** |
| La respuesta no es la esperada | Revisa que la URL sea la del servicio correcto. Algunos proxies redirigen: usa la URL final |
| No se puede descifrar la credencial | `secret.key` ha cambiado o se ha perdido. Restáurala o vuelve a escribir la credencial |
| Error inesperado al consultar el servicio | Revisa los registros de Talaia, en su salida estándar de error |

**Otros problemas:**

| Problema | Causa y solución |
|---|---|
| Talaia no arranca y menciona permisos de la clave | `secret.key` es accesible por otros usuarios. Ejecuta `chmod 600` sobre ese archivo |
| «Demasiados intentos fallidos» | Espera 15 minutos o reinicia Talaia |
| He olvidado la contraseña | Aún no se puede recuperar. Para Talaia, borra la carpeta de datos y empieza de cero: perderás las conexiones |
| El panel no se actualiza detrás de un proxy | Comprueba que el proxy no retenga las respuestas de `/api/dashboard/events` |
| Un servicio no aparece en el panel | Si en **Conexiones** indica «Lectura de datos aún no disponible», su integración todavía no existe |

## Limitaciones actuales

- Home Assistant todavía no muestra datos.
- La integración de Proxmox aún no se ha verificado con un servidor real.
- La vista de Proxmox no incluye el almacenamiento.
- No se puede elegir qué aparece en el panel ni colocarlo: se muestran todas las conexiones con integración, por orden alfabético.
- El panel no tiene inicio de sesión.
- No se puede cambiar ni recuperar la contraseña.
- No hay instalador ni actualización automática.

El estado de cada punto está en [Estado y cambios](../proyecto/06-estado-y-cambios.md).
