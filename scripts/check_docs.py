#!/usr/bin/env python3
"""Comprueba que la documentación es coherente con el código."""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DOCS = ROOT / "docs"
PLAN = DOCS / "proyecto/04-plan-de-pruebas.md"
REQUIREMENTS = DOCS / "proyecto/01-requisitos.md"
MANUAL = DOCS / "usuario/manual-de-usuario.md"
MANUAL_EN = DOCS / "user/user-manual.md"

errors = []
plan = PLAN.read_text(encoding="utf-8")
requirements = REQUIREMENTS.read_text(encoding="utf-8")

# Cada prueba de Go está en el plan, y el plan no cita pruebas inexistentes.
go_tests = set()
for path in (ROOT / "internal").rglob("*_test.go"):
    go_tests.update(re.findall(r"^func (Test\w+)\(", path.read_text(encoding="utf-8"), re.M))
planned = set(re.findall(r"`(Test\w+)`", plan))
errors += [f"Prueba de Go sin documentar: {name}" for name in sorted(go_tests - planned)]
errors += [f"El plan cita una prueba inexistente: {name}" for name in sorted(planned - go_tests)]

# Lo mismo con los escenarios E2E.
e2e = set(re.findall(r'\["(E2E-\d+)"', (ROOT / "e2e/run.mjs").read_text(encoding="utf-8")))
planned_e2e = set(re.findall(r"E2E-\d+", plan))
errors += [f"Escenario E2E sin documentar: {name}" for name in sorted(e2e - planned_e2e)]
errors += [f"El plan cita un escenario inexistente: {name}" for name in sorted(planned_e2e - e2e)]

# Lo mismo con las pruebas del instalador.
installer = set(re.findall(r"INS-\d+", (ROOT / "scripts/test/proxmox-integration.sh").read_text(encoding="utf-8")))
planned_installer = set(re.findall(r"INS-\d+", plan))
errors += [f"Prueba del instalador sin documentar: {name}" for name in sorted(installer - planned_installer)]
errors += [f"El plan cita una prueba del instalador inexistente: {name}" for name in sorted(planned_installer - installer)]

# Los requisitos citados existen, y los implementados tienen cobertura.
rows = re.findall(r"^\| (RN?F-\d+) \|.*\| ([^|]+?) \|$", requirements, re.M)
known = {req for req, _ in rows}
cited = set(re.findall(r"RN?F-\d+", plan))
errors += [f"El plan cita un requisito inexistente: {req}" for req in sorted(cited - known)]
trace = plan.split("## Trazabilidad", 1)[-1]
covered = set(re.findall(r"^\| (RN?F-\d+) \|", trace, re.M))
for req, state in rows:
    if state.startswith(("Implementado", "Parcial")) and req not in covered:
        errors.append(f"Requisito {state.split(':')[0].lower()} sin trazabilidad: {req}")

# Los enlaces e imágenes relativos existen.
for path in [*DOCS.rglob("*.md"), ROOT / "README.md", ROOT / "README.es.md", ROOT / "CLAUDE.md"]:
    for target in re.findall(r"\]\(([^)\s]+)\)", path.read_text(encoding="utf-8")):
        if re.match(r"[a-z]+://", target) or target.startswith("#"):
            continue
        if not (path.parent / target.split("#")[0]).exists():
            errors.append(f"{path.relative_to(ROOT)}: enlace roto a {target}")

# Las capturas generadas se usan en el manual.
manual = MANUAL.read_text(encoding="utf-8")
for image in sorted((DOCS / "usuario/img").glob("*.png")):
    if f"img/{image.name}" not in manual:
        errors.append(f"Captura sin usar en el manual: {image.name}")

# El manual en inglés es la traducción del español: mismas capturas y mismas secciones.
manual_en = MANUAL_EN.read_text(encoding="utf-8")
images_es = set(re.findall(r"\(img/([\w-]+\.png)\)", manual))
images_en = set(re.findall(r"\(img/([\w-]+\.png)\)", manual_en))
errors += [f"El manual en inglés no usa la captura {name}" for name in sorted(images_es - images_en)]
errors += [f"El manual en inglés usa una captura que el español no tiene: {name}" for name in sorted(images_en - images_es)]
errors += [f"Falta la captura en inglés: docs/user/img/{name}" for name in sorted(images_en) if not (DOCS / "user/img" / name).exists()]
sections_es = len(re.findall(r"^#{2,3} ", manual, re.M))
sections_en = len(re.findall(r"^#{2,3} ", manual_en, re.M))
if sections_es != sections_en:
    errors.append(f"Los manuales no tienen las mismas secciones: {sections_es} en español y {sections_en} en inglés")

if errors:
    print("\n".join(errors))
    sys.exit(1)
print(f"Documentación coherente: {len(go_tests)} pruebas de Go, {len(e2e)} E2E, {len(installer)} del instalador, {len(rows)} requisitos.")
