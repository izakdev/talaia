# Estado y cambios

## Hitos

| Hito | Contenido | Estado |
|---|---|---|
| 1 | Base: repositorio, CI, autenticación, SQLite y conexiones | Terminado, en `main` |
| 2 | AdGuard y flujo completo, catálogo de servicios y documentación | Terminado y validado por el cliente, en `main` |
| 3 | Proxmox: visión agregada y detalle por nodo | Terminado y verificado con un clúster real, en `main` |
| 4 | Home Assistant: estado, entidades, consumos, recursos y resumen; enlace a cada servicio | Terminado y verificado con la instalación del cliente, en `main` |
| 5 | Editor: rejilla, varias páginas y temas; login opcional; cambio y recuperación de contraseña | Terminado y validado por el cliente, en `main` |
| 6 | Distribución: instalador LXC, actualización y documentación final | Terminado y verificado en un Proxmox real, publicado como `v0.1.0` |
| 7 | OPNsense: gateways, tráfico WAN y recursos | Terminado y verificado con el OPNsense del cliente, publicado en `v0.2.0` |
| 8 | Portainer: entornos y contenedores | Terminado y verificado con el Portainer del cliente, publicado en `v0.2.0` |
| 9 | WGDashboard: túneles y peers | Terminado y verificado con el WGDashboard del cliente, publicado en `v0.2.0` |
| 10 | OPNsense: estado de WireGuard | Terminado y verificado con el OPNsense del cliente y un peer conectado, publicado en `v0.2.1` |
| 11 | Panel de un vistazo: estado global en la atalaya y bloques con aviso visibles | Terminado y verificado con el panel real del cliente, publicado en `v0.2.2` |
| 12 | Pulido del panel: antigüedad en la cabecera, secciones coherentes y bloques sin huecos | Terminado y verificado con el panel real del cliente, publicado en `v0.2.3` |
| 13 | Bloques que se apilan solos y panel a todo el ancho | Terminado y verificado con el panel real del cliente, publicado en `v0.2.5` |
| 14 | QNAP por SNMPv3: discos, volúmenes y recursos | Terminado y verificado con el QNAP del cliente, publicado en `v0.3.0` |
| 15 | SAI a través de un servidor NUT: estado, batería y autonomía | Terminado y verificado con el SAI del cliente, publicado en `v0.3.1` |
| 16 | Notificaciones por Telegram, ntfy o webhook cuando un servicio cambia de color | Terminado y verificado con el Telegram del cliente, publicado en `v0.4.0` |

## Historial

### Iconos en los bloques (2026-11-20)

**Añadido:**
- Cada bloque del panel lleva junto a su nombre el icono de su servicio, el mismo del catálogo.
- Campo «Icono» en cada conexión, con un buscador de los 3.467 logotipos de `simple-icons` (por ejemplo, Pi-hole para un AdGuard o el de tu router), «Sin icono» y «Restablecer». Los iconos van dentro del binario: el navegador no pide nada a terceros; ver DEC-56.
- La API pública da el icono de cada bloque (`slug`, título y color), un dato visual que no expone nada del servicio.

**Cambiado:**
- En modo oscuro, un logotipo negro (o en modo claro, uno casi blanco) se pinta con el color del texto para que se vea.
- El binario crece unos 5 MB por los iconos.

**Sin verificar:** aún no se ha probado en una instalación real ni en Safari o Firefox; ver el [plan de pruebas](04-plan-de-pruebas.md#verificaciones-manuales).

### `v0.4.1` (2026-11-20)

Corrige los avisos: un canal que tarda en responder ya no recibe el mismo aviso varias veces, porque un envío que salió sin respuesta queda «Sin confirmar» y no se repite (DEC-53), y en ntfy el título y el texto ya no se repiten. Incluye los binarios para amd64 y arm64, las sumas SHA-256 y el instalador; se actualiza desde `v0.4.0` con `talaia-proxmox.sh update`.

### Formato de los avisos en ntfy (2026-11-20)

**Corregido:**
- En ntfy, el emoji salía dos veces y el texto repetía el título. Ahora el título va sin emoji, porque ntfy ya pone el de la etiqueta, y el texto empieza en la línea siguiente: la lista de servicios, o «Si lo ves, este canal funciona.» en la prueba. Telegram y el webhook no cambian. Se vio al configurar ntfy en la verificación real.

### Avisos sin duplicados (2026-11-20)

**Corregido:**
- Un canal que tarda en responder ya no recibe el mismo aviso dos o tres veces. Si el mensaje salió entero y no llega la respuesta, queda «Sin confirmar» en «Últimos envíos» y no se reintenta; solo se reintenta si el mensaje no llegó a salir o si el canal contesta con un error. Ver DEC-53.
- Los canales esperan 30 s en lugar de 10 s, y «Enviar prueba» avisa en ámbar de que el mensaje puede haber llegado, en lugar de dar un error que invitaba a repetirla.

### CI más ligera para la documentación (2026-11-20)

**Cambiado:**
- Si un cambio solo toca documentación (Markdown, `docs/` o `.claude/`), la CI se salta las pruebas de Go, las E2E y las del instalador, y ejecuta solo `docs-check` y las pruebas de la clasificación, en un minuto. Con cualquier otro archivo, o si no puede saber qué ha cambiado, lo prueba todo como antes; ver DEC-52.
- Nuevo `make test-ci` para las pruebas de esa clasificación, y las recetas `publicar` y `nueva-integracion` explican qué jobs esperar en cada caso.

### `v0.4.0` (2026-11-20)

Trae el hito 16: las notificaciones por Telegram, ntfy o un webhook cuando un servicio del panel cambia de color, con su sección en los ajustes y la casilla para silenciar cada conexión. Incluye los binarios para amd64 y arm64, las sumas SHA-256 y el instalador; se actualiza desde `v0.3.1` con `talaia-proxmox.sh update`.

### Hito 16 (2026-11-19)

**Añadido:**
- Notificaciones: cuando un servicio del panel pasa a ámbar o a rojo, y cuando vuelve a estar en orden, Talaia envía un aviso por Telegram, ntfy o un webhook JSON, sin ningún navegador abierto. El aviso lleva el nombre del servicio y su estado tal como sale en el bloque, en español o en inglés.
- Sección «Notificaciones» en los ajustes: idioma de los avisos, tiempo que debe durar un cambio antes de avisar, canales con «Enviar prueba» y los 50 últimos envíos con su resultado.
- Casilla «Avisar de sus cambios» en cada conexión, para silenciar una sin quitarla del panel.

**Cambiado:**
- El color y la etiqueta de cada bloque los calcula ahora el servidor, con la misma regla para el panel, la cabecera y los avisos; ver DEC-48. El panel se ve igual que antes.


### `v0.3.1` (2026-11-19)

Trae el hito 15: el SAI a través de un servidor NUT, con su estado, la carga de la batería y la autonomía estimada. Incluye los binarios para amd64 y arm64, las sumas SHA-256 y el instalador; se actualiza desde `v0.3.0` con `talaia-proxmox.sh update`.

### Hito 15 (2026-11-19)

**Añadido:**
- Integración con un SAI a través de su servidor NUT (`upsd`): si tiene red o tira de batería, carga de la batería, autonomía estimada, modelo y versión de NUT, con los avisos de batería baja, cambiar batería y sobrecarga.
- Direcciones `nut://host[:puerto][/sai]` en el formulario de conexión, sin usuario ni contraseña, porque NUT no los pide para leer.

### `v0.3.0` (2026-11-18)

Trae el hito 14: la integración con QNAP (QTS y QuTS hero) por SNMPv3, con discos y su salud SMART, volúmenes, recursos, temperaturas y firmware. Incluye también la corrección de los nombres largos en las secciones Gateways y WireGuard de OPNsense. Incluye los binarios para amd64 y arm64, las sumas de comprobación y el instalador, y se actualiza desde `v0.2.5` con `talaia-proxmox.sh update`.

### Hito 14 (2026-11-17)

**Añadido:**
- Integración con QNAP (QTS y QuTS hero) por SNMPv3: CPU, memoria, almacenamiento, discos con su salud SMART y su temperatura, volúmenes con su estado y su uso, temperaturas y firmware.
- Direcciones `snmp://host[:puerto]` y clave de cifrado en el formulario de conexión, solo para QNAP.

### Nombres largos en OPNsense (2026-11-16)

**Corregido:**
- En las secciones Gateways y WireGuard del bloque de OPNsense, un nombre largo sin espacios ya no se parte a mitad de palabra: se reparte en dos líneas el estado.

### `v0.2.5` (2026-11-16)

Trae el hito 13: los bloques del panel se apilan solos según la altura real de su contenido, de modo que dos bloques cortos caben al lado de uno largo, y el panel usa todo el ancho de la ventana. En el editor solo se elige el ancho, de un cuarto a la fila entera. Incluye los binarios para amd64 y arm64, las sumas de comprobación y el instalador, y se actualiza desde `v0.2.4` con `talaia-proxmox.sh update`.

### Hito 13 (2026-11-13)

**Cambiado:**
- Cada bloque del panel queda justo debajo del anterior de sus columnas, según la altura real de su contenido, así que dos bloques cortos caben al lado de uno largo sin huecos.
- El panel y su editor usan todo el ancho de la ventana.
- En el editor, la altura se ajusta sola y solo se elige el ancho: los botones van de un cuarto a la fila entera, y un bloque nuevo ocupa un tercio.

### `v0.2.4` (2026-11-12)

Versión de mantenimiento: el pie del panel muestra la versión de Talaia, y el idioma elegido se aplica a las demás pestañas abiertas del navegador. Coincide con la nueva portada del repositorio y el manual de usuario en inglés. Incluye los binarios para amd64 y arm64, las sumas de comprobación y el instalador, y se actualiza desde `v0.2.3` con `talaia-proxmox.sh update`.

### Manual en inglés (2026-11-12)

**Añadido:**
- Manual de usuario en inglés, `docs/user/user-manual.md`, con capturas de la interfaz en inglés. El README en inglés enlaza con él.
- Al cambiar el idioma en una pestaña, las demás pestañas abiertas de Talaia en ese navegador también cambian.

### Portada del repositorio (2026-11-12)

**Cambiado:**
- README en inglés, con `README.es.md` en español: qué resuelve Talaia, una captura del panel con un aviso, los servicios compatibles, la instalación rápida y más capturas.
- Imagen de vista previa para GitHub y redes sociales (`docs/img/social-preview.png`, 1280 × 640): la atalaya de noche con su haz. Se genera desde `docs/img/social-preview.html` con `make social-preview`, y se sube a mano en los ajustes del repositorio.

### Versión en el panel (2026-11-11)

**Añadido:**
- El pie del panel muestra la versión de Talaia, como el de la administración. Hasta ahora solo se veía al iniciar sesión.

### `v0.2.3` (2026-11-11)

Versión de mantenimiento con el hito 12, que pule el panel (antigüedad en la cabecera, secciones coherentes y bloques sin huecos), y la corrección del espaciado de la puntuación en direcciones y valores. Incluye los binarios para amd64 y arm64, las sumas de comprobación y el instalador, y se actualiza desde `v0.2.2` con `talaia-proxmox.sh update`.

### Hito 12 (2026-11-09)

**Cambiado:**
- La antigüedad de los datos aparece una sola vez, en la cabecera del panel. Un bloque solo indica la suya si falla, y su pie da el servicio y su versión.
- Cada sección de un bloque lleva una línea encima y ninguna debajo.
- Un bloque mide lo que su contenido, sin estirarse hasta la altura de su fila.

### Espaciado de cifras y direcciones (2026-11-09)

**Corregido:**
- Las direcciones de Conexiones y algunos valores de los bloques (tops de AdGuard, tabla de VM y LXC, entidades de Home Assistant) se veían con espacios alrededor de los puntos y las comas, como `https : //…`. Usaban dígitos de ancho fijo, que en Schibsted Grotesk también ensanchan la puntuación (ver DEC-36). Ahora solo los usan los porcentajes de las barras de los nodos, que no llevan puntuación.

### `v0.2.2` (2026-11-09)

Versión de mantenimiento con el hito 11, que resume el estado de todos los servicios en la cabecera del panel, y un instalador que ya no deja Talaia parado sin avisar si falla una actualización. Incluye los binarios para amd64 y arm64, las sumas de comprobación y el instalador, y se actualiza desde `v0.2.1` con `talaia-proxmox.sh update`.

### Actualización más robusta (2026-11-09)

**Corregido:**
- Si fallaba un paso de `talaia-proxmox.sh update` con el servicio parado, el script podía terminar sin ningún mensaje y dejar Talaia parado. Ahora indica el comando y la línea, restaura la copia si está completa, vuelve a arrancar el servicio y termina con error. También lo hace si se interrumpe o se corta la sesión.
- Un fallo al copiar la base de datos se ignoraba, y la marcha atrás podía borrar la base de datos sin tener copia. Ahora ese fallo detiene la actualización y solo se restaura una copia completa.
- La pregunta «¿Continuar?» se escribe en la terminal: con `2>archivo` no se veía y parecía que el script se colgaba.
- `systemctl enable` ya no oculta sus errores.
- Varios intentos seguidos podían agotar el límite de arranques de systemd e impedir que Talaia volviera a arrancar. El instalador limpia ese límite antes de arrancar el servicio al actualizar.

**Sin verificar:** las pruebas INS-11 e INS-12 simulan los fallos en CI. No se han provocado en un Proxmox VE real, y la causa exacta del fallo observado allí no se pudo reproducir.

### Hito 11 (2026-11-09)

**Añadido:**
- Resumen del estado de todos los servicios en la cabecera del panel, con enlace a cada bloque que necesita atención, y la atalaya del color del peor estado.

**Cambiado:**
- Los bloques con aviso o fallo llevan un borde superior ámbar o rojo, y ningún bloque lleva sombra.
- Las cifras de los bloques son un escalón más pequeñas.

### Herramientas de desarrollo (2026-11-09)

**Añadido:**
- Skills de Claude Code para publicar una versión, añadir una integración y explorar la API real de un servicio, y un subagente que revisa cada hito antes del PR.
- Hooks que impiden escribir en el repositorio IP domésticas y otros datos del laboratorio, formatean el Go y comprueban la documentación al editarla.

### `v0.2.1` (2026-11-07)

Versión de mantenimiento con el hito 10: el bloque de OPNsense puede mostrar el estado de WireGuard. Incluye los binarios para amd64 y arm64, las sumas de comprobación y el instalador, y se actualiza desde `v0.2.0` con `talaia-proxmox.sh update`.

### Hito 10 (2026-11-06)

**Añadido:**
- Opción de OPNsense para mostrar sus instancias de WireGuard: estado, peers conectados y tráfico.

**Verificado:**
- Con el OPNsense 26.7.1_1 del cliente y un peer conectado, instalado en su Proxmox VE con `talaia-proxmox.sh update --binary`.

### `v0.2.0` (2026-11-06)

Segunda versión estable, con las integraciones de la fase 2: OPNsense, Portainer y WGDashboard. Incluye los binarios para amd64 y arm64, las sumas de comprobación y el instalador, y se actualiza desde `v0.1.0` con `talaia-proxmox.sh update`.

### Hito 9 (2026-11-06)

**Añadido:**
- Integración con WGDashboard: túneles, peers conectados y tráfico acumulado.

### Hito 8 (2026-11-05)

**Añadido:**
- Integración con Portainer: entornos, contenedores en marcha y con problemas, y stacks.

### Hito 7 (2026-11-04)

**Añadido:**
- Integración con OPNsense: CPU, RAM, disco, tiempo encendido, gateways y tráfico WAN.
- Aviso al probar la conexión si faltan datos por los privilegios del usuario.
- Fase 2 en los requisitos: OPNsense, Portainer y WGDashboard. Grafana queda aparcado.

### `v0.1.0` (2026-11-04)

Primera versión estable, con los binarios para amd64 y arm64, las sumas de comprobación y el instalador. El repositorio pasa a ser público.

### Hito 6 (2026-11-02)

**Añadido:**
- Instalador y actualizador para Proxmox VE en un solo script.
- Servicio con usuario propio y sistema de archivos protegido.
- Código de un solo uso para proteger el primer arranque.
- Actualización con copia de seguridad y marcha atrás automática.
- Pruebas del instalador en CI, con systemd real.

**Corregido:**
- El instalador elegía la plantilla de Debian con la versión más alta, aunque fuera de otra arquitectura.
- Un token pegado con un salto de línea daba un error inesperado. Ahora se limpia.

**Cambiado:**
- Las versiones con guion, como `v1.0.0-rc.1`, se publican como preliminares.
- Cada versión publicada incluye el instalador.

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
| Marcha atrás automática solo probada en CI | No se ha provocado un fallo de arranque en un Proxmox real | Al darse la ocasión |
| Actualizaciones de OPNsense sin mostrar | Requerirían un `POST`; ver DEC-29 | Sin hito asignado |
| Velocidad WAN media del intervalo | Se recalcula en cada refresco y se mide de nuevo al reiniciar Talaia | Por diseño: ver DEC-31 |
| Portainer sin detalle por contenedor | Los recuentos son de la última instantánea; ver DEC-32 | Por diseño |
| WGDashboard sin detalle por peer | Solo recuentos, para no publicar sus nombres; ver DEC-33 | Por diseño |
| Clave de WGDashboard con acceso completo | WGDashboard no tiene claves de solo lectura; ver DEC-34 | Depende de WGDashboard |
| QNAP sin los umbrales de temperatura de QTS | Se usan sus valores por defecto; ver DEC-44 | Por diseño de SNMP |
| QNAP sin distinguir volúmenes USB | Los volúmenes se numeran todos igual; ver DEC-44 | Depende del MIB de QNAP |
| Discos USB de QNAP sin SMART | Si la caja no lo pasa al NAS, no se ve su salud; ver DEC-45 | Depende de la caja USB |
| SNMPv3 de QNAP con cifrado DES | QTS no ofrece otro; ver DEC-43 | Depende de QNAP |
| Estados de RAID y volumen de QNAP no documentados | Se interpretan por palabras («Ready», «Degraded», «Rebuilding»…); un texto desconocido da ámbar | Al darse la ocasión |
| Un SAI por conexión | Si el servidor NUT tiene varios, cada uno necesita su conexión con `nut://host/sai`; ver DEC-46 | Bajo demanda |
| NUT sin STARTTLS | El estado del SAI viaja en claro por la red, sin credenciales; ver DEC-47 | Bajo demanda |
| SAI sin tensiones ni consumo | Solo estado, batería y autonomía, como pidió el cliente | Bajo demanda |
| Avisos sin recordatorios | Un servicio que sigue en rojo no vuelve a avisar hasta que cambia | Por diseño, a petición del cliente |
| Avisos solo por cambio de color | Un cambio de etiqueta dentro del mismo color (de un ámbar a otro) no avisa | Bajo demanda |
| Telegram y ntfy.sh necesitan Internet | Sin salida a Internet, solo funcionan un ntfy propio o un webhook local | Por diseño de esos servicios |
| Un envío sin confirmar no se repite | Si el canal recibe el mensaje y no responde en 30 s, queda «Sin confirmar» y no se reintenta; si el canal lo perdió, ese aviso no llega. Ver DEC-53 | Por diseño |
| Sin reglas por canal | Todos los canales activos reciben todos los avisos | Bajo demanda |
| Sistema del contenedor sin actualizaciones automáticas | Hay que actualizarlo con `apt` | Bajo demanda |

## Consumo medido

Medición en reposo. Es orientativa.

| Fecha | Binario | Memoria (RSS) |
|---|---|---|
| 2026-11-02 | 15 MB | 141 MB, unos 30 segundos después de crear el administrador, fuera de un LXC |
| 2026-11-04 | 14 MB, versión publicada | 18,3 MB en un LXC real de 512 MB, con tres conexiones; pico de 140,3 MB |

Cada cálculo de contraseña reserva 64 MiB, con un máximo de 2 simultáneos, y parte de esa memoria puede seguir contando en la medición. El objetivo de 512 MB de RNF-06 queda por confirmar en un LXC real y con varias integraciones.
