---
name: publicar
description: Receta para publicar una versión de Talaia (vX.Y.Z) en GitHub y dejarla descargable con el instalador. Úsala cuando el usuario pida publicar, sacar o lanzar una versión, sin repasar cómo se hicieron las anteriores.
---

# Publicar una versión de Talaia

Sigue estos pasos en orden. No hace falta revisar cómo se publicaron las versiones anteriores: esta receta ya lo recoge. Pide permiso al usuario antes de empezar y no fusiones nada hasta que pase la CI.

`gh` está en `/opt/homebrew/bin`: antepón `export PATH="/opt/homebrew/bin:$PATH"` en cada comando que lo use. El repositorio no permite auto-merge.

## 0. Elegir el número

- Mantenimiento o un hito pequeño sobre la última versión: sube el parche (`v0.2.0` → `v0.2.1`).
- Una fase nueva de integraciones: sube la menor (`v0.2.x` → `v0.3.0`).
- Con guion (`v1.0.0-rc.1`), `release.yml` la publica como versión preliminar.

Comprueba que `main` tiene fusionados todos los hitos que van en la versión y que su verificación real ya está en `docs/proyecto/04-plan-de-pruebas.md`.

## 1. PR de preparación (rama `publica-vX.Y.Z` desde `main` actualizado)

No hay ningún archivo de versión en el código: `release.yml` la toma de la etiqueta con `-X main.version`. Solo cambia la documentación:

- `README.md` (en inglés): la línea `> **Status:** \`vX.Y.Z\` released. …`.
- `README.es.md`: la línea `> **Estado:** \`vX.Y.Z\` publicada. …`.
- `docs/usuario/manual-de-usuario.md`: la línea `> **Versión de este manual:** \`vX.Y.Z\`. …` y, si hace falta, «Limitaciones actuales».
- `docs/proyecto/06-estado-y-cambios.md`:
  - En la tabla de hitos, cada hito incluido pasa a «…, publicado en \`vX.Y.Z\`».
  - En «Historial», justo bajo `## Historial`, una entrada `### \`vX.Y.Z\` (AAAA-MM-DD)` con un párrafo: qué hitos o integraciones trae, que incluye los binarios para amd64 y arm64, las sumas y el instalador, y que se actualiza desde la versión anterior con `talaia-proxmox.sh update`.

Después:

```sh
make docs-check
git commit -am "Prepara la publicación de vX.Y.Z"   # con la línea Co-Authored-By
git push -u origin publica-vX.Y.Z
gh pr create --base main --title "Prepara la publicación de vX.Y.Z" --body-file -
```

El cuerpo del PR lleva un resumen, las pruebas (`make docs-check`; sin cambios de código si es el caso) y la línea de atribución.

## 2. CI y fusión

```sh
gh pr checks N --watch --fail-fast    # en segundo plano
gh pr merge N --merge                 # solo si pasa y el usuario lo aprobó
```

## 3. Etiqueta y publicación

La etiqueta es ligera y va sobre el commit de fusión en `main`:

```sh
git checkout main && git pull
git tag vX.Y.Z && git push origin vX.Y.Z
gh run watch $(gh run list --workflow release.yml --limit 1 --json databaseId -q '.[0].databaseId') --exit-status
```

## 4. Comprobar lo publicado

En un directorio nuevo del scratchpad:

```sh
gh release view vX.Y.Z --json isPrerelease,isDraft,assets
curl -fsSLO https://github.com/izakdev/talaia/releases/latest/download/{SHA256SUMS,talaia-linux-amd64,talaia-linux-arm64,talaia-proxmox.sh}
shasum -a 256 -c SHA256SUMS
file talaia-linux-*
cmp talaia-proxmox.sh <repo>/scripts/talaia-proxmox.sh
```

En macOS el binario de Linux no se ejecuta: para comprobar la versión y las integraciones incluidas, busca las cadenas (`strings talaia-linux-amd64 | grep -m1 vX.Y.Z`, y una ruta de cada integración nueva).

Debe ser no preliminar (salvo con guion), marcada como la última y con los cuatro archivos.

## 5. Actualizar el Proxmox real

Esto lo hace el usuario en su terminal; dale el comando sin `--binary`:

```sh
ssh -t root@$PVE_HOST 'cd /root && curl -fsSLO https://github.com/izakdev/talaia/releases/latest/download/talaia-proxmox.sh && bash talaia-proxmox.sh update --ctid ID'
```

Recuérdale que `PVE_HOST` e `ID` son suyos. No escribas su IP ni su dominio en el repositorio.

## 6. PR de registro (rama `registra-vX.Y.Z`)

En `docs/proyecto/04-plan-de-pruebas.md`, en la tabla de verificaciones (más reciente arriba), añade dos filas:

1. `| fecha | \`vX.Y.Z\` | Versión publicada por \`release.yml\` | Correcto: no preliminar y marcada como la última, con ambos binarios, sumas e instalador. … |`, con lo comprobado en el paso 4.
2. `| fecha | \`vX.Y.Z\` | Proxmox VE real del cliente, actualización con el instalador descargado de GitHub y sin \`--binary\` | Correcto según la salida del instalador … |`, cuando el usuario haya actualizado. Indica la versión anterior, la copia de seguridad y lo que se ve en su captura.

Después: `make docs-check`, commit «Registra la publicación de vX.Y.Z», PR y fusión tras la CI.
