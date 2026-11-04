# Talaia

Dashboard autoalojado para ver el estado de tus servicios de un vistazo.

> **Estado:** en desarrollo. La instalación, la actualización y el arranque automático en Proxmox VE están verificados en un servidor real; falta publicar la primera versión.

*Talaia* significa «atalaya» en catalán: una torre desde la que se vigila el entorno.

## Fase 1

- Dashboard de cliente, pensado como página inicial del navegador.
- Administración protegida para configurar conexiones, páginas y widgets.
- Integraciones de solo lectura: Proxmox VE, Home Assistant y AdGuard Home.
- Instalación y actualización en un LXC autónomo de Proxmox.

## Instalación en Proxmox VE

Como root, en un nodo de Proxmox:

```sh
curl -fsSL https://github.com/izakdev/talaia/releases/latest/download/talaia-proxmox.sh -o talaia-proxmox.sh
bash talaia-proxmox.sh install
```

Crea un contenedor LXC no privilegiado y muestra el enlace para crear el administrador. Más opciones en el [manual de usuario](docs/usuario/manual-de-usuario.md#instalación).

## Documentación

- [Manual de usuario](docs/usuario/manual-de-usuario.md)
- [Documentación del proyecto](docs/README.md)

## Desarrollo

Requiere Go y Node.js 22.

```sh
make build   # compila la web y el binario en bin/talaia
make test    # ejecuta go vet y los tests
make e2e     # ejecuta las pruebas en el navegador
./bin/talaia -listen :8080 -data ./data
```

## Licencia

[MIT](LICENSE)
