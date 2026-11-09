---
name: explorar-api
description: Explora la API real de un servicio del usuario (antes de programar una integración o para diagnosticar una) sin ver ni guardar secretos ni datos de su laboratorio. Úsala cuando haga falta conocer la forma real de las respuestas o los errores de autenticación de un servicio.
---

# Explorar la API real de un servicio

El usuario ejecuta la exploración en su terminal: la URL y la credencial se escriben allí, nunca en el chat. El script solo hace `GET` y oculta lo sensible antes de imprimir.

## Pasos

1. Busca primero la documentación oficial o el código fuente del servicio para saber qué rutas consultar y cómo se autentica.
2. Copia el script a un directorio nuevo del scratchpad (nunca lo ejecutes dentro del repositorio):

   ```sh
   mkdir -p <scratchpad>/explorar && cp .claude/skills/explorar-api/explorar.py <scratchpad>/explorar/
   ```

3. Dale al usuario el comando, siempre con `python3 -I`. Por ejemplo:

   ```sh
   python3 -I <scratchpad>/explorar/explorar.py --auth header:X-API-Key --sin-clave --nombres name,hostname /api/version /api/estado
   ```

   - `--auth`: `none`, `basic` (clave y secreto, como OPNsense), `bearer` (como Home Assistant) o `header:<Cabecera>` (como Portainer o WGDashboard).
   - `--sin-clave`: repite cada ruta sin credencial, para ver el código y el mensaje de rechazo.
   - `--nombres`: campos cuyo valor se oculta siempre, como los nombres de equipos, contenedores o peers.
   - Las listas se recortan a 3 elementos.

4. Lee la salida con `read_terminal` en la pestaña donde lo ejecutó.
5. Si un caso no está cubierto (un campo nuevo, otra forma de autenticarse), adapta la copia del scratchpad, no la del repositorio, y vuelve a comprobar que no se imprime nada sensible.

## Qué ocultar y qué no

- El script oculta por nombre de campo (`key`, `secret`, `token`, `address`, `endpoint`, `host`, `port`, `url`, `user`, `dns`, `mac`, scripts…) y por valor (IP, IPv6, URL y dominios).
- Revisa la salida antes de citarla. Si algo del laboratorio se ha colado, no lo repitas en el chat ni lo copies a ningún archivo, y amplía el filtro.
- En la documentación, describe la forma (`{rows, rowCount, total}`, «tráfico en GiB») y nunca los valores reales.

## Después

- Registra la exploración en `docs/proyecto/04-plan-de-pruebas.md`, en la tabla de verificaciones, como «Hito N, antes de programar», con la versión del servicio y lo que se ha confirmado.
- Las respuestas de los simuladores (`e2e/fake-*.mjs`) y de las pruebas de Go copian la forma real con valores inventados: `192.0.2.x`, `198.51.100.x`, `*.simulado`.
