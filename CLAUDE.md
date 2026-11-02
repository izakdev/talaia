# Talaia

Dashboard autoalojado en Go y React. Empieza por [docs/README.md](docs/README.md).

## Documentación obligatoria

Un cambio no está terminado hasta que su documentación está actualizada en el mismo cambio. Idioma: español.

| Si cambia… | Actualiza |
|---|---|
| Un requisito o su estado | `docs/proyecto/01-requisitos.md` |
| Componentes, API, datos, seguridad o despliegue | `docs/proyecto/02-arquitectura.md` |
| Pantallas, reglas o mensajes | `docs/proyecto/03-diseno-funcional.md` y el manual de usuario |
| Pruebas | `docs/proyecto/04-plan-de-pruebas.md`, incluida la trazabilidad |
| Una decisión técnica | `docs/proyecto/05-decisiones.md`; no se reescriben las anteriores |
| Cualquier entrega o limitación | `docs/proyecto/06-estado-y-cambios.md` |
| Algo visible para el usuario | `docs/usuario/manual-de-usuario.md` y, con `make screenshots`, sus capturas |

Al cerrar un hito o una integración, registra también la verificación contra el servicio real en el plan de pruebas y los datos sin verificar.

No incluyas IP, dominios ni otros datos del laboratorio del autor en el repositorio.

## Comprobaciones antes de entregar

```sh
make test
make e2e
make docs-check
```

## Convenciones

- Una rama y un pull request por hito; si el anterior no está fusionado, la rama parte de él.
- Mensajes de commit en español.
- Integraciones de solo lectura; los datos que devuelven son públicos, sin URL ni credenciales.
