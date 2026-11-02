# Talaia

Dashboard autoalojado para ver el estado de tus servicios de un vistazo.

> **Estado:** en desarrollo inicial. Todavía no hay una versión instalable.

*Talaia* significa «atalaya» en catalán: una torre desde la que se vigila el entorno.

## Fase 1

- Dashboard de cliente, pensado como página inicial del navegador.
- Administración protegida para configurar conexiones, páginas y widgets.
- Integraciones de solo lectura: Proxmox VE, Home Assistant y AdGuard Home.
- Instalación en un LXC autónomo de Proxmox.

El diseño está en [docs/design.md](docs/design.md).

## Desarrollo

Requiere Go y Node.js 22.

```sh
make build   # compila la web y el binario en bin/talaia
make test    # ejecuta go vet y los tests
./bin/talaia -listen :8080 -data ./data
```

## Licencia

[MIT](LICENSE)
