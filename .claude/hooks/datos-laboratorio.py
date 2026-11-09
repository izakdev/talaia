#!/usr/bin/env python3
"""Hook PreToolUse: impide escribir en el repositorio datos del laboratorio del autor.

Bloquea IP privadas de las redes domésticas habituales (192.168.x.x y
172.16-31.x.x) y las expresiones de `.claude/datos-laboratorio.local`, una lista
privada y fuera de Git (una expresión regular por línea; `#` para comentarios).
Las pruebas usan rangos de documentación (192.0.2.x, 198.51.100.x) y 10.x, que
se permiten. No repite el valor encontrado, para no llevarlo al chat.
"""
import json
import os
import re
import sys
from pathlib import Path

PRIVATE_IP = re.compile(r"\b(?:192\.168|172\.(?:1[6-9]|2\d|3[01]))\.\d{1,3}\.\d{1,3}\b")


def texts(tool_input):
    for key in ("content", "new_string"):
        if isinstance(tool_input.get(key), str):
            yield tool_input[key]
    for edit in tool_input.get("edits") or []:
        if isinstance(edit, dict) and isinstance(edit.get("new_string"), str):
            yield edit["new_string"]


def main():
    event = json.load(sys.stdin)
    tool_input = event.get("tool_input") or {}
    path = tool_input.get("file_path") or tool_input.get("notebook_path") or ""
    root = Path(os.environ.get("CLAUDE_PROJECT_DIR") or event.get("cwd") or ".").resolve()
    try:
        target = Path(path).resolve()
        rel = target.relative_to(root)
    except ValueError:
        return 0  # Fuera del repositorio: no se versiona.
    private_list = root / ".claude/datos-laboratorio.local"
    if target == private_list:
        return 0

    patterns = [("IP privada de red doméstica", PRIVATE_IP)]
    if private_list.exists():
        for n, line in enumerate(private_list.read_text(encoding="utf-8").splitlines(), 1):
            line = line.strip()
            if line and not line.startswith("#"):
                try:
                    patterns.append((f"expresión {n} de datos-laboratorio.local", re.compile(line, re.I)))
                except re.error:
                    patterns.append((f"expresión {n} de datos-laboratorio.local", re.compile(re.escape(line), re.I)))

    found = []
    for text in texts(tool_input):
        for n, line in enumerate(text.splitlines(), 1):
            for label, pattern in patterns:
                if pattern.search(line):
                    found.append(f"línea {n} del texto nuevo: {label}")
    if not found:
        return 0
    print(
        f"Bloqueado: {rel} recibiría datos del laboratorio del autor ({'; '.join(sorted(set(found))[:5])}). "
        "CLAUDE.md prohíbe incluirlos en el repositorio. Usa valores inventados: 192.0.2.x, 198.51.100.x o *.simulado.",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
