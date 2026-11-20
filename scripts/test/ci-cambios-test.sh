#!/usr/bin/env bash
# Pruebas de scripts/ci-cambios.sh (CI-01 a CI-08).
set -euo pipefail
cd "$(dirname "$0")/../.."

fallos=0
comprobar() {
  local id=$1 esperado=$2 lista=$3 obtenido
  obtenido=$(printf '%b' "$lista" | scripts/ci-cambios.sh)
  if [[ $obtenido == "codigo=$esperado" ]]; then
    echo "ok   $id"
  else
    echo "FALLO $id: se esperaba codigo=$esperado y salió $obtenido"
    fallos=$((fallos + 1))
  fi
}

comprobar CI-01 false 'README.md\nREADME.es.md\n'
comprobar CI-02 false 'docs/proyecto/04-plan-de-pruebas.md\ndocs/usuario/img/panel.png\ndocs/img/social-preview.html\n'
comprobar CI-03 false '.claude/skills/publicar/SKILL.md\n.claude/hooks/formatear.sh\nCLAUDE.md\n'
comprobar CI-04 true 'docs/proyecto/06-estado-y-cambios.md\ninternal/api/server.go\n'
comprobar CI-05 true '.github/workflows/ci.yml\n'
comprobar CI-06 true 'scripts/check_docs.py\n'
comprobar CI-07 true ''
comprobar CI-08 false 'README.md'

if ((fallos > 0)); then
  echo "$fallos pruebas fallidas"
  exit 1
fi
echo "Clasificación de cambios correcta"
