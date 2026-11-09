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

## DEC-36: `trap ERR` para deshacer una actualización fallida
- **Contexto:** con `set -e`, cualquier paso fallido entre parar el servicio y comprobar la versión nueva cortaba el script, y Talaia quedaba parado, a veces sin ningún mensaje. Envolver cada paso en una función desactivaría `set -e` dentro de las funciones que agrupan varios pasos, y sus fallos intermedios pasarían inadvertidos.
- **Decisión:** armar un `trap ERR` con `set -E`, y otro para `INT`, `TERM` y `HUP`, solo en ese tramo. Ambos llevan a una única marcha atrás, compartida con el caso de que la versión nueva no arranque. La copia se restaura solo si está completa: hasta entonces no se ha cambiado nada, y borrar la base de datos para restaurar una copia parcial la perdería.
- **Consecuencias:** un paso nuevo queda cubierto sin hacer nada más. Durante la marcha atrás se ignoran las señales para no dejarla a medias, de modo que puede durar hasta agotar los intentos de `/healthz`.
