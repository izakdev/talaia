# Diseño de la fase 1

## Principios

- Solo datos en vivo: Talaia no guarda histórico. Si hay gráficas, se consultan a la API de origen.
- El navegador nunca ve tokens ni llama directamente a los servicios.
- Integraciones de solo lectura e implementadas como módulos con una interfaz común.
- Varias conexiones por integración.
- Interfaz en español e inglés.

## Arquitectura

```text
Navegador ──HTTP/SSE──> Binario Go (LXC) ──> Proxmox / Home Assistant / AdGuard
                         ├─ Web React embebida
                         ├─ API pública y API de administración
                         ├─ Recolectores con caché en memoria
                         └─ SQLite
```

- Los recolectores consultan cada conexión en segundo plano y guardan el resultado en caché.
- El navegador recibe los cambios por SSE.
- Si una integración cae, solo se degrada su widget, que muestra el último dato y su antigüedad.

## Stack

- Backend: Go, `chi` y SQLite con `modernc.org/sqlite`, sin CGO.
- Frontend: React, TypeScript, Vite, TanStack Query y `react-grid-layout`.
- Empaquetado: la web va dentro del binario con `go:embed`.
- Publicación: GitHub Actions genera versiones para amd64 y arm64.

## Integraciones

| Integración | Autenticación | Endpoints | Widgets |
|---|---|---|---|
| Proxmox | API token de solo lectura (`PVEAuditor`) | `/cluster/status`, `/cluster/resources` | Resumen agregado; nodo con sus VM y LXC, y CPU, RAM y disco |
| Home Assistant | Token de larga duración | `/api/`, `/api/config`, `/api/states` | Estado; entidades elegidas; consumos |
| AdGuard | Basic Auth | `/control/status`, `/control/stats` | Protección, consultas, % de bloqueo y top de dominios |

Hasta el editor del hito 5, el dashboard muestra todas las conexiones con una integración disponible. Las URL, los usuarios y las credenciales nunca se envían a la API pública.

## Modelo de datos

- `users`: administradores.
- `connections`: tipo, URL, secreto cifrado, intervalo de refresco y opciones TLS.
- `pages`: páginas del dashboard.
- `widgets`: tipo, conexión, configuración JSON y posición por tamaño de pantalla.
- `settings`: tema, idioma y exigencia de login en el dashboard.

## Seguridad

- Administración: contraseña con Argon2id y sesión en cookie `HttpOnly`.
- Secretos: cifrados con AES-256-GCM. La clave se genera al instalar y se guarda fuera de la base de datos.
- API pública: solo devuelve los datos de los widgets, nunca la configuración ni las credenciales.
- TLS: verificación activa por defecto, con excepción por conexión para certificados autofirmados.
- El dashboard es abierto por defecto. Si se publica en Internet, hay que activar el login.

## Instalación

- Un comando en el host Proxmox crea y configura el LXC.
- Contenedor no privilegiado con Debian estable, 1 vCPU, 512 MB de RAM y 4 GB de disco.
- Servicio `systemd`, con verificación del checksum del binario descargado.
- En el primer arranque, un asistente crea el usuario administrador.
- Script de actualización. Las copias de seguridad de Proxmox cubren los datos y la clave.

## Hitos

1. Base: repositorio, CI, autenticación, SQLite y gestión de conexiones.
2. AdGuard: la integración más simple, para validar el flujo completo.
3. Proxmox: agregados y detalle por nodo.
4. Home Assistant: estado, selector de entidades y consumos.
5. Editor: rejilla drag & drop, varias páginas y temas.
6. Distribución: instalador, actualización y documentación.
