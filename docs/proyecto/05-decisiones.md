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
