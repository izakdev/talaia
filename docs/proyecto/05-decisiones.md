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
