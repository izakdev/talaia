#!/usr/bin/env bash
# Decide si un cambio necesita las pruebas completas de la CI.
# Lee en la entrada los archivos cambiados, uno por línea, y escribe
# «codigo=false» solo si todos son documentación o herramientas de Claude Code:
# archivos Markdown, cualquier cosa bajo docs/ y cualquier cosa bajo .claude/.
# Cualquier otro archivo, o una lista vacía, da «codigo=true».
set -euo pipefail

hay_cambios=false
while IFS= read -r archivo || [[ -n $archivo ]]; do
  [[ -z $archivo ]] && continue
  hay_cambios=true
  case $archivo in
    *.md | docs/* | .claude/*) ;;
    *)
      echo "codigo=true"
      exit 0
      ;;
  esac
done

if $hay_cambios; then
  echo "codigo=false"
else
  echo "codigo=true"
fi
