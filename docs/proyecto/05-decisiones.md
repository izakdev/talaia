# Decisiones

Cada decisión recoge su motivo y sus consecuencias. Si una decisión cambia, se añade una nueva que la sustituye.

## DEC-01: Go y React en un único binario
- **Contexto:** debe instalarse en un LXC pequeño y ser fácil de distribuir.
- **Decisión:** backend en Go con la web React incluida mediante `go:embed`.
- **Alternativas descartadas:** Next.js, que requiere Node en ejecución y consume más; FastAPI, con más dependencias en ejecución.
- **Consecuencias:** despliegue de un solo archivo, aunque compilar requiere Go y Node.

## DEC-02: SQLite sin CGO
- **Contexto:** el LXC debe ser autónomo y hay que compilar para amd64 y arm64.
- **Decisión:** SQLite con `modernc.org/sqlite`.
- **Consecuencias:** compilación cruzada sencilla, a cambio de un binario mayor (unos 15 MB).

## DEC-03: el backend hace de intermediario
- **Contexto:** los tokens no deben llegar al navegador.
- **Decisión:** el navegador solo habla con Talaia, y los recolectores consultan en segundo plano y guardan el resultado en caché.
- **Consecuencias:** más navegadores no suponen más carga sobre los servicios, y Talaia debe tener acceso de red a todos ellos.

## DEC-04: SSE para actualizar en vivo
- **Contexto:** el dashboard debe actualizarse solo, y la comunicación solo va del servidor al navegador.
- **Decisión:** Server-Sent Events con el estado completo en cada cambio.
- **Alternativas descartadas:** WebSocket, innecesario sin mensajes del navegador; sondeo periódico, con más latencia y peticiones.
- **Consecuencias:** implementación simple y reconexión automática. Enviar el estado completo es asumible con los volúmenes previstos, y se podría revisar si crece mucho.

## DEC-05: sin histórico propio
- **Contexto:** el principio del proyecto es no replicar datos que ya existen.
- **Decisión:** solo el último estado, en memoria.
- **Consecuencias:** sin gráficas propias: si hacen falta, se consultarán a la API de origen. Tras reiniciar, el estado tarda una consulta en volver.

## DEC-06: credenciales cifradas con clave en un archivo aparte
- **Contexto:** una copia de la base de datos no debería revelar las credenciales.
- **Decisión:** AES-256-GCM con una clave generada al arrancar por primera vez y guardada en `secret.key`.
- **Consecuencias:** las copias de seguridad deben incluir la clave. Si la clave y la base de datos están en la misma copia, el cifrado no protege esa copia.

## DEC-07: catálogo visual de servicios
- **Contexto:** el cliente descartó el selector de opciones porque no sería usable con muchas integraciones.
- **Decisión:** una página con tarjetas, iconos y búsqueda.
- **Consecuencias:** escala con el número de integraciones, y cada una necesita su entrada en el catálogo.

## DEC-08: opciones por conexión hasta tener widgets
- **Contexto:** el cliente quiere decidir si muestra el top de dominios, pero el editor de widgets llega en el hito 5.
- **Decisión:** columna `options` en la conexión, validada por tipo y aplicada en el backend para que el dato no salga por la API pública.
- **Consecuencias:** en el hito 5 habrá que decidir si estas opciones pasan al widget, porque dos widgets de una misma conexión podrían querer valores distintos.

## DEC-09: documentación en el repositorio
- **Contexto:** debe mantenerse al día con cada paso.
- **Decisión:** Markdown versionado con el código, diagramas Mermaid y comprobaciones en CI.
- **Consecuencias:** cada cambio incluye su documentación y GitHub la muestra directamente. Las comprobaciones cubren referencias y coherencia, no la exactitud del texto.

## DEC-10: pruebas E2E con servicio simulado y verificación real manual
- **Contexto:** la CI no dispone de los servicios reales.
- **Decisión:** E2E en CI contra un simulador, y verificación manual contra el servicio real al cerrar cada integración.
- **Consecuencias:** CI rápida y reproducible. Un cambio en la API real solo se detecta en las verificaciones manuales.

## DEC-11: una rama y un pull request por hito
- **Contexto:** el cliente quiere revisar el resultado antes de darlo por bueno.
- **Decisión:** desde el hito 2, cada hito se entrega en una rama con su pull request.
- **Consecuencias:** `main` contiene solo lo aceptado por el cliente.

## DEC-12: detalle de VM y LXC desplegable y activado por defecto
- **Contexto:** el requisito pide ver cada nodo con sus VM y LXC, pero el cliente prefiere una vista rápida y poder ocultar lo adicional.
- **Decisión:** los agregados y los consumos por nodo son fijos. El detalle se despliega por nodo y se puede desactivar por conexión, en cuyo caso no sale por la API pública.
- **Consecuencias:** a diferencia del top de dominios de AdGuard, está activado por defecto porque forma parte del requisito. Cualquiera que abra el panel ve los nombres de las VM y LXC, y las conexiones de Proxmox guardadas antes de este cambio lo tienen desactivado hasta editarlas.

## DEC-13: sin almacenamiento en la vista agregada de Proxmox
- **Contexto:** en un clúster, un almacenamiento compartido aparece una vez por nodo, y sumarlo daría un total falso.
- **Decisión:** se muestra solo el disco de cada nodo.
- **Consecuencias:** no se ve el estado de los almacenamientos. Si hiciera falta, se añadiría como opción deduplicando los compartidos.

## DEC-14: ramas de hito encadenadas si el anterior no está fusionado
- **Contexto:** el cliente puede aprobar la continuación sin fusionar todavía el pull request anterior.
- **Decisión:** la rama del hito parte de la anterior, y su pull request apunta a ella para que muestre solo los cambios nuevos.
- **Consecuencias:** hay que fusionar los pull requests por orden.

## DEC-15: entidades de Home Assistant elegidas una a una
- **Contexto:** una instalación puede tener miles de entidades, y algunas revelan datos personales, como la ubicación de una persona.
- **Decisión:** por defecto no se muestra ninguna. Se eligen en un selector con búsqueda y filtros, y solo las elegidas salen por la API pública.
- **Consecuencias:** configurarlo lleva algo más de trabajo, a cambio de una vista rápida y sin exponer datos por sorpresa. En el hito 5 habrá que decidir si la selección pasa al widget, como en DEC-08.

## DEC-16: consumos sin sumar
- **Contexto:** sumar todos los sensores de potencia contaría dos veces un medidor general y los enchufes que cuelgan de él. Además, un sensor de energía puede ser acumulado o diario.
- **Decisión:** cada sensor elegido se muestra con su valor y su unidad, sin totales.
- **Consecuencias:** las cifras coinciden siempre con Home Assistant. Quien quiera un total puede crear un sensor agregado allí y elegirlo aquí.

## DEC-17: consultar todos los estados de una vez
- **Contexto:** se podría pedir cada entidad con `/api/states/<entity_id>`, pero con 200 entidades serían 200 peticiones por ciclo.
- **Decisión:** una sola consulta a `/api/states`, solo si hay selección, y límite de respuesta ampliado a 32 MiB.
- **Consecuencias:** en instalaciones muy grandes, cada ciclo descarga todas las entidades. Se puede aliviar con un intervalo de refresco mayor.

## DEC-18: recursos de Home Assistant por WebSocket
- **Contexto:** el cliente quiere ver CPU y memoria. La API REST no las ofrece, y su proxy hacia Supervisor no permite esas rutas.
- **Decisión:** usar el comando `supervisor/api` del WebSocket de Home Assistant, con `/core/stats` y `/host/info`, sin que su fallo afecte al resto del bloque.
- **Alternativa:** los sensores de System Monitor se pueden elegir como entidades, y sirven en instalaciones sin Supervisor.
- **Consecuencias:** nueva dependencia, `coder/websocket`, y conviene dar a Talaia un token de administrador si se quieren los recursos. Sustituye la recomendación de usar un usuario sin permisos cuando se necesiten estos datos.

## DEC-19: enlace al servicio en la API pública
- **Contexto:** el cliente quiere abrir cada servicio desde su bloque. Hasta ahora la API pública no exponía ninguna URL.
- **Decisión:** cada estado incluye un enlace: una dirección propia si se configura, o la URL de conexión. Se admiten solo `http` y `https`, sin credenciales.
- **Consecuencias:** cualquiera que abra el panel ve esas direcciones, y una dirección propia evita mostrar la URL interna. Sigue sin publicarse ningún usuario ni credencial.

## DEC-20: resumen de Home Assistant sin datos de seguridad
- **Contexto:** el panel no exige inicio de sesión, y un contador puede revelar si hay alguien en casa o si una puerta está abierta.
- **Decisión:** el resumen excluye personas, cerraduras, alarmas, y puertas, garajes y verjas. Los grupos tampoco cuentan, para no contar dos veces a sus miembros.
- **Consecuencias:** quien quiera ver esos datos puede elegir las entidades concretas, bajo su responsabilidad.

## DEC-21: el bloque sigue siendo la conexión
- **Contexto:** DEC-08 y DEC-15 dejaron pendiente si las opciones y la selección de entidades debían pasar al widget.
- **Decisión:** se mantienen en la conexión. Un widget solo guarda página, posición y tamaño, y una conexión puede estar en varias páginas, con el mismo contenido.
- **Consecuencias:** configuración simple y sin migrar datos. Si se quisieran dos vistas distintas de un mismo servicio, habría que crear otra conexión o revisar esta decisión.

## DEC-22: rejilla CSS en el panel y arrastrable solo en el editor
- **Contexto:** con `react-grid-layout`, la altura es fija, y el detalle desplegable de Proxmox o una lista larga quedarían recortados.
- **Decisión:** el panel usa CSS Grid con la misma rejilla y filas que crecen con el contenido. La librería solo se carga en el editor.
- **Consecuencias:** nada se recorta ni se solapa y el panel carga menos código. La altura elegida es un mínimo, por lo que la vista previa del editor puede recortar lo que en el panel se ve entero.

## DEC-23: recuperación de contraseña por línea de comandos
- **Contexto:** Talaia no tiene correo ni servicios externos, y una pregunta de seguridad sería más débil.
- **Decisión:** `talaia reset-password` genera una contraseña aleatoria y revoca las sesiones. Requiere acceso al equipo.
- **Consecuencias:** sirve en un LXC sin dependencias. Quien no tenga acceso al equipo no puede recuperar la cuenta, y eso es lo que se busca.

## DEC-24: tema y acento globales, con opciones cerradas
- **Contexto:** los colores libres permitirían combinaciones sin contraste suficiente y valores no controlados en el CSS.
- **Decisión:** tres temas y cinco acentos validados en el servidor, todos con texto oscuro legible encima.
- **Consecuencias:** menos libertad, sin riesgo de un panel ilegible. El idioma sigue siendo de cada navegador.

## DEC-25: el host descarga y el contenedor no necesita Internet
- **Contexto:** muchos laboratorios limitan la salida de los contenedores, y verificar antes de crear nada evita dejar restos.
- **Decisión:** el host descarga y comprueba el binario, y lo empuja con `pct push`. La salud se comprueba desde dentro, sin `curl`.
- **Alternativa descartada:** instalar paquetes dentro del contenedor, como otros scripts, que añade dependencias y puntos de fallo.
- **Consecuencias:** el contenedor es mínimo, pero sus actualizaciones de sistema quedan a cargo del usuario.

## DEC-26: código de un solo uso en el primer arranque
- **Contexto:** con una instalación pensada para cualquiera, el servicio puede quedar accesible sin administrador durante mucho tiempo.
- **Decisión:** el instalador genera un código, lo guarda con permisos `600` y muestra un enlace que lo incluye. Ejecutar el binario a mano no lo exige, salvo que se indique `-setup-token-file`.
- **Consecuencias:** nadie más en la red puede tomar la instalación. El código viaja en la URL por HTTP una vez, y deja de valer al usarlo.

## DEC-27: pruebas del instalador con Docker
- **Contexto:** la CI no dispone de Proxmox, y el script no se puede dar por bueno solo con análisis estático.
- **Decisión:** ejecutar el script real con `pct` y similares simulados sobre un contenedor Docker con systemd.
- **Consecuencias:** cubre el flujo, el servicio, la actualización y la marcha atrás. No cubre el comportamiento propio de Proxmox ni de un LXC no privilegiado, que requiere verificación manual.

## DEC-28: marcha atrás automática al actualizar
- **Contexto:** las migraciones de la base de datos no se pueden deshacer, y una versión nueva podría no arrancar.
- **Decisión:** copiar base de datos y binario antes de actualizar, y restaurar ambos si `/healthz` no responde.
- **Consecuencias:** no se queda sin servicio. No detecta fallos que no impidan arrancar, por lo que se conservan las 3 últimas copias.

## DEC-29: en OPNsense, solo consultas GET
- **Contexto:** las actualizaciones pendientes parecen requerir `POST /api/core/firmware/status`, y las integraciones son de solo lectura.
- **Decisión:** usar solo `GET`. No se muestran las actualizaciones pendientes de OPNsense.
- **Consecuencias:** un `POST` en ese módulo podría lanzar una comprobación en el cortafuegos, y se evita. El dato se consulta en OPNsense.

## DEC-30: gateways sin monitorizar, en neutro
- **Contexto:** OPNsense devuelve `none` y «Online» también en gateways sin medidas, con `~` en la latencia y la pérdida. Verde daría a entender que hay Internet sin haberlo comprobado.
- **Decisión:** tratarlos como un estado propio, «Sin monitorizar», que no pone el bloque en verde por sí solo.
- **Consecuencias:** un panel sin monitorización se ve en neutro hasta activarla en OPNsense. Comprobado en un OPNsense real: con la monitorización desactivada devuelve `~`, y al activarla, medidas reales.

## DEC-31: velocidad WAN calculada en memoria
- **Contexto:** la API da contadores acumulados, y Talaia no guarda histórico.
- **Decisión:** calcular la velocidad frente a la lectura anterior de la misma conexión, sin persistirla.
- **Consecuencias:** tras reiniciar Talaia o editar la conexión, la primera lectura indica «Midiendo…». La velocidad es la media del intervalo de refresco, no un valor instantáneo.

## DEC-32: en Portainer, recuentos de la instantánea
- **Contexto:** con un usuario no administrador, la lista de contenedores llega vacía, mientras que `/api/endpoints` da los recuentos de cada entorno. En Community Edition no parece haber un rol de solo lectura, y un administrador daría más permisos de los necesarios.
- **Decisión:** usar solo los recuentos de la última instantánea, sin consultar Docker a través de Portainer.
- **Consecuencias:** no hay detalle por contenedor, y los recuentos pueden tener varios minutos, según el intervalo de instantáneas de Portainer. El bloque indica su antigüedad y el detalle se consulta en Portainer.

## DEC-33: en WGDashboard, solo recuentos de peers
- **Contexto:** la lista de peers incluye sus nombres, que suelen identificar a personas o dispositivos, y el panel puede ser público.
- **Decisión:** publicar por túnel solo su estado, los peers conectados sobre el total y el tráfico. Un túnel parado da un aviso ámbar y no un fallo, porque puede estar parado a propósito.
- **Consecuencias:** para saber qué peer está conectado hay que abrir WGDashboard. Basta una consulta por ciclo, sin pedir los peers de cada túnel.

## DEC-34: clave de API de WGDashboard con acceso completo
- **Contexto:** WGDashboard no tiene claves ni usuarios de solo lectura, y anota en su registro cada petición con clave, incluida la propia clave.
- **Decisión:** aceptar su clave de API, usar solo `GET`, hacer una única petición con clave por ciclo y pedir la versión sin ella.
- **Consecuencias:** la clave podría modificar WGDashboard. Talaia la guarda cifrada como las demás, y el manual y el formulario avisan de que da acceso completo.

## DEC-35: WireGuard de OPNsense como opción de la conexión
- **Contexto:** OPNsense incluye WireGuard y da su estado en `/api/wireguard/service/show`, pero necesita el privilegio «VPN: WireGuard: Status», que además permite arrancar, parar y reconfigurar el servicio. Muchas instalaciones no usan WireGuard.
- **Decisión:** mostrarlo solo si se activa la opción «Mostrar el estado de WireGuard», con los mismos criterios que WGDashboard (DEC-33): por instancia, estado, peers conectados sobre el total y tráfico, sin nombres de peers. Una instancia caída da un aviso ámbar, por debajo de un gateway con fallos.
- **Consecuencias:** las conexiones existentes no cambian ni reciben avisos de privilegios nuevos. Quien active la opción debe añadir el privilegio, y el manual avisa de su alcance. Talaia solo usa `GET`.

## DEC-36: el estado global, en la atalaya
- **Contexto:** el panel debe decir de un vistazo si algo va mal, pero cada bloque daba su estado solo en su esquina, y un fallo en otra página no se veía. Las cifras grandes, todas del mismo peso, competían con el estado.
- **Decisión:** la cabecera resume el peor estado de todos los servicios del panel y nombra los que necesitan atención; la luz y el haz de la atalaya toman su color. Los bloques con aviso o fallo llevan un borde superior grueso de ese color y los demás pierden la sombra. Las cifras bajan un escalón. El cálculo del estado es el mismo para el bloque y para la cabecera.
- **Consecuencias:** con el acento ámbar por defecto, un aviso apenas cambia el color de la luz, así que el aviso se lee en el texto, en el triángulo, en la intensidad del haz y en la línea inferior. Las cifras no usan dígitos de ancho fijo, porque en Schibsted Grotesk separan la coma decimal.

## DEC-37: `trap ERR` para deshacer una actualización fallida
- **Contexto:** con `set -e`, cualquier paso fallido entre parar el servicio y comprobar la versión nueva cortaba el script, y Talaia quedaba parado, a veces sin ningún mensaje. Envolver cada paso en una función desactivaría `set -e` dentro de las funciones que agrupan varios pasos, y sus fallos intermedios pasarían inadvertidos.
- **Decisión:** armar un `trap ERR` con `set -E`, y otro para `INT`, `TERM` y `HUP`, solo en ese tramo. Ambos llevan a una única marcha atrás, compartida con el caso de que la versión nueva no arranque. La copia se restaura solo si está completa: hasta entonces no se ha cambiado nada, y borrar la base de datos para restaurar una copia parcial la perdería.
- **Consecuencias:** un paso nuevo queda cubierto sin hacer nada más. Durante la marcha atrás se ignoran las señales para no dejarla a medias, de modo que puede durar hasta agotar los intentos de `/healthz`.

## DEC-38: bloques de la altura de su contenido y antigüedad en la cabecera
- **Contexto:** cada bloque repetía «Actualizado hace…» aunque todo estuviera al día, las secciones internas llevaban líneas de forma desigual, y un bloque se estiraba hasta la altura de su fila y dejaba huecos en blanco dentro del recuadro.
- **Decisión:** la cabecera da una sola antigüedad, la del dato más reciente; un bloque solo da la suya si falla. Cada sección lleva una línea encima y ninguna debajo. Un bloque mide lo que su contenido, sin estirarse.
- **Consecuencias:** los bloques de una fila pueden acabar a distinta altura, y el espacio sobrante queda fuera del recuadro. Para saber cuándo respondió un servicio concreto que funciona, hay que fiarse de la cabecera, porque todos se consultan en el mismo ciclo.

## DEC-39: README en inglés, con versión en español
- **Contexto:** la comunidad que puede descubrir Talaia (foros de homelab y autoalojamiento, listas como awesome-selfhosted) busca y lee en inglés, y la portada del repositorio estaba solo en español y sin capturas.
- **Decisión:** `README.md` en inglés y `README.es.md` en español, con el mismo contenido y una captura de portada en cada idioma, generada por las pruebas E2E en `docs/img/`. El resto de la documentación sigue en español.
- **Consecuencias:** hay que mantener los dos README a la vez, también al publicar una versión. Los enlaces de ambos los comprueba `make docs-check`.

## DEC-40: manual de usuario en inglés, como traducción
- **Contexto:** con el README en inglés (DEC-39), quien llega al repositorio desde fuera encontraba el manual solo en español. La documentación del proyecto la leen quienes desarrollan Talaia y no necesita traducción.
- **Decisión:** `docs/user/user-manual.md` es la traducción al inglés del manual, que sigue siendo la referencia, con capturas propias en `docs/user/img/`. `make screenshots` toma cada captura en los dos idiomas: tras la española, cambia el idioma desde otra pestaña del mismo navegador, y la página lo aplica sin recargar gracias a que el idioma se sincroniza entre pestañas.
- **Consecuencias:** cualquier cambio visible para el usuario se escribe en los dos manuales. `make docs-check` comprueba que ambos usan las mismas capturas y tienen las mismas secciones. Los nombres de los servicios simulados siguen en español en las capturas en inglés, porque son datos del usuario.

## DEC-41: bloques que se apilan solos según su altura real
- **Contexto:** la altura de cada bloque se elegía en el editor en filas de la rejilla, sin relación con su contenido, y el dashboard reservaba esas filas. Un bloque largo fijaba la altura de su fila y los cortos de al lado dejaban un hueco (DEC-38 solo evitó que se estiraran). Además, el dashboard no pasaba de 92rem de ancho.
- **Decisión:** el dashboard coloca cada bloque justo debajo del último de sus columnas, con su altura real medida en el navegador, y usa todo el ancho de la ventana. Las columnas, el ancho y el orden siguen siendo del editor, que mide las alturas, solo deja cambiar el ancho y no cuenta las alturas medidas como cambios. Se descartaron las alturas fijas (recortes o scroll al crecer el contenido) y las columnas fijas (sin anchos libres).
- **Consecuencias:** sin cambios en la base de datos ni en la API, que mantiene el ancho mínimo de 2 para los paneles guardados; el editor usa 3 como mínimo y 4 por defecto. Si un bloque crece, los de debajo bajan. En pantallas de 900 a 1400 px los bloques estrechos pasan a media fila. Sustituye a DEC-38 en lo que se refiere a la altura de los bloques.

## DEC-42: QNAP por SNMPv3 y no por su API web
- **Contexto:** la API web de QTS (`authLogin.cgi` y los CGI de gestión) no está documentada, exige una sesión, y con una cuenta del grupo «everyone» no da ningún dato: hace falta una cuenta del grupo «administrators». Se comprobó con el NAS del cliente antes de programar.
- **Decisión:** consultar el NAS por SNMPv3 con autenticación y cifrado (authPriv), con la librería `gosnmp` (Go puro, sin cambiar el binario único), solo con GET y walk. No se ofrecen SNMP v1 ni v2c, que envían la comunidad en claro.
- **Consecuencias:** lectura real sin cuenta de administración ni sesión, con el MIB que publica el propio NAS. A cambio, el LXC necesita llegar al puerto UDP 161 del NAS, SNMP no da los umbrales de temperatura de QTS (DEC-44) ni qué volumen es USB, y la E2E necesita un agente SNMP simulado.

## DEC-43: cifrado DES en SNMPv3 si el NAS no ofrece otro
- **Contexto:** QTS solo ofrece CBC-DES como protocolo de privacidad de SNMPv3. DES es débil, pero sin cifrado las respuestas, que incluyen el modelo y el nombre del equipo, viajan legibles por la red.
- **Decisión:** autenticación HMAC-SHA fija y cifrado DES por defecto, con un selector para AES por si el NAS lo ofrece. Las dos claves se guardan cifradas juntas en el secreto de la conexión, separadas por un salto de línea, que ninguna clave puede contener; se cambian siempre a la vez.
- **Consecuencias:** el manual pide claves largas, aleatorias y distintas, para que el tráfico capturado no permita adivinarlas. El simulador de la E2E usa AES, porque el OpenSSL de Node no trae DES; DES se verifica con el NAS real.

## DEC-44: estado del QNAP calculado por Talaia, con umbrales de QTS por defecto
- **Contexto:** SNMP da la salud SMART de cada disco, el estado de los grupos RAID, de los pools y de los volúmenes, y las temperaturas, pero no los umbrales que el usuario tenga en QTS ni si un disco es SSD.
- **Decisión:** rojo para un fallo SMART, un RAID degradado, un pool o volumen con error y una temperatura crítica; ámbar para un aviso SMART, una sincronización, un pool en aviso, un volumen por encima del 90 %, una temperatura alta y un ventilador averiado, como pidió el cliente. Umbrales fijos, los de QTS por defecto: CPU 75/80 °C, sistema 60/70 °C y discos 60/70 °C (el error de un HDD como aviso, para no marcar en rojo un SSD normal). Un disco que falta se ve por el estado del RAID y del pool, no por las bahías vacías.
- **Consecuencias:** si el usuario cambia los umbrales en QTS, Talaia no lo sabe. Los volúmenes se numeran sin distinguir los USB, porque el MIB no lo dice; los discos de cajas USB sí se distinguen por su caja.

## DEC-45: discos USB sin SMART, sin datos de salud
- **Contexto:** en el NAS del cliente, los dos discos de una caja USB salen por SNMP como «Abnormal», uno con `diskSmartInfo` `-1` y otro con `2`, y con temperatura `-1`. QTS los muestra como dispositivos externos «Ready», sin ningún dato de salud: la caja no le pasa el SMART.
- **Decisión:** un disco de una caja distinta del NAS y sin temperatura se publica como «sin datos SMART» y no cambia el estado del bloque. Si una caja USB da temperatura, se considera que pasa el SMART y se respeta lo que diga. Los discos del NAS no cambian: un «Abnormal» en una bahía sigue en rojo.
- **Consecuencias:** un fallo real de un disco USB sin SMART no se ve en Talaia, igual que tampoco lo ve QTS.

## DEC-46: SAI leído de NUT y no de la API de PeaNUT
- **Contexto:** el cliente quiere ver su SAI en el panel. El SAI va por USB a un nodo de Proxmox, que corre el servidor NUT (`upsd`); otros equipos lo vigilan con `upsmon`, y PeaNUT, en un LXC, lo muestra en una web leyendo de ese mismo `upsd`.
- **Decisión:** Talaia habla directamente el protocolo de red de NUT (texto por TCP, puerto 3493) con un cliente propio, sin dependencias: solo `VER`, `LIST UPS`, `LIST VAR` y `LOGOUT`. Se descartaron la API de PeaNUT, que solo es una interfaz sobre NUT y obligaría a tenerlo instalado, y la biblioteca `go.nut`, sin mantener, para un protocolo tan sencillo.
- **Consecuencias:** sirve a cualquiera que tenga NUT, con o sin PeaNUT, y con cualquier SAI que NUT soporte. El LXC necesita llegar al puerto TCP 3493 del equipo que tiene el SAI, y `upsd` debe escuchar en la red (`LISTEN` en `upsd.conf`). Una conexión lee un SAI: si el servidor tiene varios, se indica cuál en la dirección (`nut://host/sai`). La E2E necesita un `upsd` simulado.

## DEC-47: NUT sin credenciales ni STARTTLS
- **Contexto:** NUT no tiene control de acceso para leer desde la versión 2.4: `upsd.users` solo hace falta para enviar órdenes al SAI (apagarlo, probar la batería…) o para que `upsmon` coordine el apagado. Se comprobó con el `upsd` 2.8.1 del cliente: desde el LXC de Talaia, todas las consultas responden sin `LOGIN`. `upsd` admite STARTTLS, pero pocas instalaciones lo configuran.
- **Decisión:** la conexión no pide usuario ni contraseña, tampoco opcionales, y guarda un secreto vacío; la API admite crearla y cambiarle la dirección sin secreto. Talaia nunca envía `USERNAME`, `PASSWORD`, `LOGIN`, `SET` ni `INSTCMD`. No se usa STARTTLS.
- **Consecuencias:** no hay credencial que filtrar ni que guardar. El estado del SAI viaja en claro por la red local, sin datos secretos. Quien restrinja `upsd` con un firewall debe abrir el puerto al LXC. Si en el futuro hiciera falta TLS o una cuenta, sería una opción nueva de la conexión.

## DEC-48: el color de cada servicio se calcula en el servidor
- **Contexto:** el hito 16 avisa de los cambios de color sin ningún navegador abierto. Hasta entonces, el color y la etiqueta de cada bloque los calculaba el navegador, con una función por integración, y el servidor solo sabía si el servicio respondía.
- **Decisión:** cada integración de Go implementa `Stater`, una traducción literal de su regla de TypeScript, y el recolector publica el estado (color, clave del texto y parámetros) en cada conexión. El panel solo traduce la clave. Los textos de estado salen de `i18n.tsx` a `web/src/i18n/states.json`, que importa el panel e incrusta el binario. Se descartaron mantener una copia de cada regla en Go, porque acabarían divergiendo y un aviso diría algo distinto que el bloque, y ejecutar el código del panel en el servidor, que exigiría un navegador o un intérprete de JavaScript en el LXC.
- **Consecuencias:** el panel, la cabecera y los avisos no pueden discrepar. Una integración nueva escribe su regla en Go y sus textos en `states.json`. Los colores de las filas internas de un bloque (discos de QNAP, por ejemplo) siguen en el navegador, porque solo decoran. En `homeassistant.Data`, el campo `State` pasa a llamarse `RunState`, sin cambiar su JSON.

## DEC-49: retardo de los avisos medido en tiempo, y el gris no cuenta
- **Contexto:** un servicio puede parpadear (un corte de unos segundos, un reinicio) y no conviene avisar de cada uno. Cada conexión tiene su propio intervalo, de 5 a 3600 s, y algunos estados son neutros: «Consultando…» tras arrancar, «Sin túneles» o «Sin gateways monitorizados».
- **Decisión:** un color nuevo avisa si dura el retardo elegido, medido en tiempo (2 minutos por defecto) y no en lecturas, porque «3 lecturas» serían 15 s en una conexión y 3 horas en otra. Un estado neutro se salta: no avisa, no cambia el color avisado y no reinicia el plazo. Cuando un cambio madura, se adelantan los que lleven al menos el retardo menos la mitad de este, con un máximo de 30 s, para que dos problemas simultáneos que Talaia ve con segundos de diferencia vayan en un solo mensaje.
- **Consecuencias:** con el valor por defecto, un problema avisa entre 2 y 2,5 minutos después de verse, más el intervalo de la conexión. Un servicio que alterna entre rojo y «Consultando…» sigue contando. Un servicio se puede avisar hasta 30 s antes de su plazo si coincide con otro. No hay recordatorios: el cliente prefiere un aviso al cambiar y otro al volver a verde.

## DEC-50: el primer estado es verde y el color avisado se guarda al decidir
- **Contexto:** hay que decidir qué pasa con un servicio sin historia (recién añadido al panel), cuándo se da un aviso por enviado y qué hacer con las conexiones silenciadas o sin canales, sin repetir avisos tras un reinicio ni guardar histórico (DEC-05).
- **Decisión:** solo se guarda en SQLite el último color avisado de cada conexión del panel; sin él, se parte de verde. Se guarda en cuanto se decide avisar, no cuando el canal confirma. Con la conexión silenciada o sin canales activos, se actualiza igual pero no se envía nada. Al quitar una conexión del panel, se olvida.
- **Consecuencias:** un servicio que entra ya en rojo avisa una vez, y uno que entra en verde no. Un canal caído no provoca repeticiones, aunque ese aviso se pierda para ese canal (queda en el registro con su error). Al activar el primer canal o reactivar una conexión no llega una ráfaga de problemas antiguos. Los candidatos pendientes no se guardan: tras reiniciar, el plazo empieza de nuevo.

## DEC-51: secretos de los canales cifrados, ntfy en JSON y API de Telegram configurable
- **Contexto:** cada canal necesita un dato que da acceso: el token del bot de Telegram, el tópico (y quizá un token) de ntfy, y la URL del webhook, que suele llevar un token. En ntfy.sh, quien conoce el tópico lee los avisos. Los títulos llevan emojis y tildes, y las pruebas necesitan un Telegram falso.
- **Decisión:** esos datos se guardan cifrados como las credenciales de las conexiones, vinculados al ID del canal, y la API nunca los devuelve; el chat ID y el servidor de ntfy no son secretos. ntfy recibe el aviso en JSON en la raíz del servidor, en lugar de cabeceras, porque admite UTF-8 en el título. La dirección de la API de Telegram se puede cambiar con `TALAIA_TELEGRAM_API`, que solo usan las pruebas.
- **Consecuencias:** quien lea la base de datos sin `secret.key` no puede leer los avisos. Al editar un canal, sus secretos se dejan en blanco para conservarlos. El mensaje de Telegram va en texto plano, sin `parse_mode`, para que un nombre con `*` o `_` no lo rompa.

## DEC-52: la CI se salta las pruebas completas si solo cambia documentación
- **Contexto:** cada publicación abre dos PR que solo cambian Markdown, el de preparación y el de registro, y cada PR y cada fusión en `main` repetían la CI completa. Las E2E tardan de 8 a 12 minutos, así que la v0.4.0 sumó unos 35 minutos de CI con el código ya probado.
- **Decisión:** un job `cambios` clasifica los archivos con `scripts/ci-cambios.sh`. Si todos son Markdown, están en `docs/` o en `.claude/`, se saltan `test` e `installer`. El job `docs` (`check_docs.py` y las pruebas de la clasificación) se ejecuta siempre. Ante la duda se prueba todo: lista vacía, un push que crea la rama o reescribe su historia, o un fallo de `git diff`. En un PR se compara con la base mediante `base...cabeza`, y en un push con `antes..después`. `release.yml` no cambia.
- **Consecuencias:** un PR de documentación pasa la CI en un minuto. Un cambio en `scripts/check_docs.py` o en un workflow cuenta como código. Las herramientas de `.claude/` (hooks incluidos) no las prueba la CI. El repositorio no tiene comprobaciones obligatorias, así que los jobs saltados no bloquean la fusión; si se añaden, deben ser `docs` y los jobs saltados, que GitHub da por superados.

## DEC-53: un aviso enviado sin respuesta no se reintenta
- **Contexto:** en la verificación real del hito 16, Telegram tardó más de 10 s en responder a una prueba que sí entregó. Con el plazo de 10 s de las integraciones, un aviso real en ese caso se daba por fallido y se reintentaba a los 30 s y a los 2 min, y llegaba dos o tres veces. Ni Telegram ni ntfy permiten descartar duplicados en su lado.
- **Decisión:** los canales esperan 30 s, porque los avisos van en segundo plano. Con `httptrace` se sabe si la petición salió entera. Si falla antes (DNS, conexión, TLS o tiempo agotado antes de enviarla), se reintenta como hasta ahora. Si falla después, el resultado es `unconfirmed`, no se reintenta y se muestra en ámbar como «Sin confirmar: puede haber llegado». Una respuesta `5xx` o `429` sigue reintentándose, porque el canal ha contestado que no lo ha procesado.
- **Consecuencias:** la marca de «enviado» se reinicia en cada conexión que pide el transporte, porque Go puede repetir la petición por una conexión nueva si la reutilizada estaba cerrada. Un caso extremo queda a favor de reintentar: si el plazo vence mientras se escribe el cuerpo, de pocos cientos de bytes, se trata como no enviado. Un canal lento ya no duplica avisos. A cambio, si el mensaje salió pero el canal lo perdió, ese aviso no llega y queda «Sin confirmar» en «Últimos envíos». Se prefiere perder un aviso dudoso a repetirlo, igual que en DEC-50. Un webhook podría recibir un identificador para descartar repeticiones, pero no hace falta mientras no se reintente.
