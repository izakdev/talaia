#!/usr/bin/env bash
# Hook PostToolUse: formatea el Go editado y comprueba la documentación al tocarla.
# Si algo falla, sale con 2 para que Claude vea el aviso; el cambio ya está hecho.
set -u
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
file=$(jq -r '.tool_input.file_path // empty')
[ -n "$file" ] || exit 0
rel=${file#"$PWD"/}

case "$rel" in
  *.go)
    if ! out=$(gofmt -l -w "$file" 2>&1); then
      echo "gofmt no ha podido formatear $rel: $out" >&2
      exit 2
    fi
    ;;
  docs/*.md | README.md | CLAUDE.md)
    if ! out=$(python3 scripts/check_docs.py 2>&1); then
      echo "make docs-check falla tras editar $rel (puede ser normal a mitad de un cambio):" >&2
      echo "$out" >&2
      exit 2
    fi
    ;;
esac
exit 0
