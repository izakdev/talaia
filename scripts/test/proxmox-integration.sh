#!/usr/bin/env bash
# Prueba el instalador con systemd real en Docker, simulando los comandos de Proxmox.
# Los identificadores INS-xx están descritos en docs/proyecto/04-plan-de-pruebas.md.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_DIR="$ROOT/scripts/test"
INSTALLER="$ROOT/scripts/talaia-proxmox.sh"
WORK="$(mktemp -d)"
SITE="$WORK/site"
SERVER_PID=""
export STUB_LOG="$WORK/stub.log"
export STUB_IMAGE="talaia-test-ct"
export PATH="$TEST_DIR/stubs:$PATH"
export TALAIA_DOWNLOAD_BASE="http://127.0.0.1:18999"
export TALAIA_HEALTH_ATTEMPTS=15
CT=900
PORT=8081
ARCH="$(dpkg --print-architecture)"
PASSWORD=""
: >"$STUB_LOG"

cleanup() {
  [[ -z "$SERVER_PID" ]] || kill "$SERVER_PID" 2>/dev/null || true
  for id in 900 901 902 903; do docker rm -f "talaia-test-ct-$id" >/dev/null 2>&1 || true; done
  rm -rf "$WORK"
}
trap cleanup EXIT

fail() {
  echo "FALLO: $*" >&2
  echo "--- registro del servicio ---" >&2
  docker exec "talaia-test-ct-$CT" journalctl -u talaia -n 30 --no-pager >&2 || true
  exit 1
}
pass() { echo "ok    $1"; }
inside() { docker exec "talaia-test-ct-$CT" "$@"; }
api() {
  local method="$1" path="$2" body="${3:-}"
  inside curl -sS -o /dev/null -w '%{http_code}' -X "$method" -H 'Content-Type: application/json' \
    -c /tmp/cookies -b /tmp/cookies ${body:+-d "$body"} "http://127.0.0.1:$PORT$path"
}
expect() { [[ "$2" == "$3" ]] || fail "$1: se esperaba '$3' y se obtuvo '$2'"; }

publish() {
  local version="$1"
  local dir="$SITE/download/$version"
  mkdir -p "$dir"
  (cd "$ROOT" && CGO_ENABLED=0 go build -ldflags "-X main.version=$version" -o "$dir/talaia-linux-$ARCH" ./cmd/talaia)
  (cd "$dir" && sha256sum "talaia-linux-$ARCH" >SHA256SUMS)
}

publish v0.0.1
publish v0.0.2
mkdir -p "$SITE/latest"
ln -s ../download/v0.0.2 "$SITE/latest/download"
publish v0.0.9
echo "0000000000000000000000000000000000000000000000000000000000000000  talaia-linux-$ARCH" >"$SITE/download/v0.0.9/SHA256SUMS"
cat >"$WORK/bad-binary" <<'BAD'
#!/bin/sh
if [ "$1" = "-version" ]; then echo v0.0.3; exit 0; fi
exit 1
BAD
chmod +x "$WORK/bad-binary"

docker build -q -t "$STUB_IMAGE" -f "$TEST_DIR/Dockerfile.ct" "$TEST_DIR" >/dev/null
python3 -m http.server 18999 --bind 127.0.0.1 --directory "$SITE" >/dev/null 2>&1 &
SERVER_PID=$!
sleep 1

# INS-10: opciones no válidas, sin tocar nada.
for args in "--port abc" "--ip nada" "--ctid x" "--version 1.0" "--hostname a_b" "--gateway 192.0.2.1" "--binary /no/existe" "--desconocida"; do
  # shellcheck disable=SC2086 # Se separan las opciones a propósito.
  if "$INSTALLER" install --yes $args >/dev/null 2>&1; then fail "aceptó opciones no válidas: $args"; fi
done
if grep -q 'pct create' "$STUB_LOG"; then fail "las opciones no válidas crearon un contenedor"; fi
pass "INS-10 Opciones no válidas"

# INS-01: instalación.
output="$("$INSTALLER" install --yes --ctid "$CT" --version v0.0.1 --port "$PORT" --ip 192.0.2.50/24 --gateway 192.0.2.1 --vlan 20 2>&1)" \
  || { echo "$output" >&2; fail "la instalación ha fallado"; }
create="$(grep "pct create $CT" "$STUB_LOG")"
for option in "--unprivileged 1" "--features nesting=1" "--onboot 1" "--memory 512" "--cores 1" "--rootfs local:4" \
  "--arch $ARCH" "debian-13-standard_13.1-2_${ARCH}.tar" "ip=192.0.2.50/24,gw=192.0.2.1,tag=20"; do
  [[ "$create" == *"$option"* ]] || fail "falta $option en: $create"
done
expect "servicio" "$(inside systemctl is-active talaia)" active
expect "arranque automático" "$(inside systemctl is-enabled talaia)" enabled
# shellcheck disable=SC2016 # Se expande dentro del contenedor.
expect "usuario" "$(inside sh -c 'stat -c %U "/proc/$(systemctl show -p MainPID --value talaia)"')" talaia
expect "versión" "$(inside /usr/local/bin/talaia -version)" v0.0.1
expect "permisos de datos" "$(inside stat -c '%U %a' /var/lib/talaia)" "talaia 700"
expect "permisos de la clave" "$(inside stat -c '%U %a' /var/lib/talaia/secret.key)" "talaia 600"
expect "permisos del código" "$(inside stat -c '%U %a' /var/lib/talaia/setup.token)" "talaia 600"
expect "sin privilegios nuevos" "$(inside systemctl show -p NoNewPrivileges --value talaia)" yes
expect "sistema protegido" "$(inside systemctl show -p ProtectSystem --value talaia)" strict
pass "INS-01 Instalación con servicio endurecido"

# INS-02: el código de instalación protege el primer arranque.
token="$(inside cat /var/lib/talaia/setup.token)"
[[ "$output" == *"?setup=$token"* ]] || [[ "$output" == *"$token"* ]] || fail "el instalador no muestra el código"
expect "sin código" "$(api POST /api/setup '{"username":"admin","password":"una-clave-bien-larga"}')" 403
expect "con código" "$(api POST /api/setup "{\"username\":\"admin\",\"password\":\"una-clave-bien-larga\",\"token\":\"$token\"}")" 200
if inside test -e /var/lib/talaia/setup.token; then fail "el código sigue existiendo tras usarlo"; fi
pass "INS-02 Código de instalación"

# INS-03: la recuperación documentada no deja archivos de root.
recovered="$(inside runuser -u talaia -- /usr/local/bin/talaia reset-password -data /var/lib/talaia)"
PASSWORD="$(sed -n 's/^Contraseña nueva: //p' <<<"$recovered")"
[[ -n "$PASSWORD" ]] || fail "la recuperación no muestra la contraseña"
expect "archivos ajenos" "$(inside find /var/lib/talaia ! -user talaia | wc -l)" 0
expect "entrar tras recuperar" "$(api POST /api/auth/login "{\"username\":\"admin\",\"password\":\"$PASSWORD\"}")" 200
expect "alta de conexión" "$(api POST /api/admin/connections '{"name":"Prueba","type":"adguard","base_url":"http://127.0.0.1:9","username":"u","secret":"s","interval_seconds":30}')" 201
pass "INS-03 Recuperación de contraseña dentro del contenedor"

# INS-04: la actualización conserva datos y configuración.
output="$("$INSTALLER" update --yes --ctid "$CT" 2>&1)" || { echo "$output" >&2; fail "la actualización ha fallado"; }
expect "versión nueva" "$(inside /usr/local/bin/talaia -version)" v0.0.2
expect "servicio" "$(inside systemctl is-active talaia)" active
expect "arranque automático" "$(inside systemctl is-enabled talaia)" enabled
[[ "$(inside grep TALAIA_LISTEN /etc/talaia/talaia.env)" == "TALAIA_LISTEN=:$PORT" ]] || fail "se ha perdido el puerto"
expect "sesión y datos" "$(api GET /api/admin/connections)" 200
inside curl -sS -b /tmp/cookies "http://127.0.0.1:$PORT/api/admin/connections" | grep -q Prueba || fail "se ha perdido la conexión"
expect "copias" "$(inside sh -c 'ls /var/lib/talaia/backups | wc -l')" 1
inside sh -c 'ls /var/lib/talaia/backups/*/talaia.previous /var/lib/talaia/backups/*/talaia.db' >/dev/null || fail "copia incompleta"
pass "INS-04 Actualización con copia de seguridad"

# INS-05: no hace nada si ya está en esa versión.
before="$(inside sh -c 'ls /var/lib/talaia/backups | wc -l')"
output="$("$INSTALLER" update --yes --ctid "$CT" 2>&1)"
[[ "$output" == *"ya está en la versión"* ]] || fail "debería indicar que ya está actualizado: $output"
expect "sin copia nueva" "$(inside sh -c 'ls /var/lib/talaia/backups | wc -l')" "$before"
pass "INS-05 Misma versión"

# INS-06: si la versión nueva no arranca, restaura la anterior.
if "$INSTALLER" update --yes --ctid "$CT" --binary "$WORK/bad-binary" >"$WORK/out" 2>&1; then fail "una versión rota no debe darse por buena"; fi
grep -q "Sigue en marcha la versión v0.0.2" "$WORK/out" || { cat "$WORK/out" >&2; fail "falta el aviso de restauración"; }
expect "versión restaurada" "$(inside /usr/local/bin/talaia -version)" v0.0.2
expect "servicio" "$(inside systemctl is-active talaia)" active
expect "entrar tras restaurar" "$(api POST /api/auth/login "{\"username\":\"admin\",\"password\":\"$PASSWORD\"}")" 200
pass "INS-06 Marcha atrás automática"

# INS-07: una suma incorrecta no crea nada.
if "$INSTALLER" install --yes --ctid 901 --version v0.0.9 >"$WORK/out" 2>&1; then fail "aceptó una suma incorrecta"; fi
grep -q "suma de comprobación no coincide" "$WORK/out" || { cat "$WORK/out" >&2; fail "falta el motivo"; }
if grep -q 'pct create 901' "$STUB_LOG"; then fail "creó el contenedor con una suma incorrecta"; fi
pass "INS-07 Suma de comprobación incorrecta"

# INS-08: una instalación fallida no deja el contenedor.
if "$INSTALLER" install --yes --ctid 902 --binary "$WORK/bad-binary" >"$WORK/out" 2>&1; then fail "dio por buena una instalación rota"; fi
if docker inspect talaia-test-ct-902 >/dev/null 2>&1; then fail "quedó el contenedor de una instalación fallida"; fi
pass "INS-08 Limpieza tras un fallo"

# INS-09: no reutiliza un contenedor existente, y la opción por defecto usa la última versión.
if "$INSTALLER" install --yes --ctid "$CT" >"$WORK/out" 2>&1; then fail "reutilizó un contenedor existente"; fi
grep -q "Ya existe el contenedor" "$WORK/out" || fail "falta el motivo"
docker inspect "talaia-test-ct-$CT" >/dev/null || fail "un fallo no debe borrar un contenedor que ya existía"
CT=903
"$INSTALLER" install --yes --ctid "$CT" --port "$PORT" >"$WORK/out" 2>&1 || { cat "$WORK/out" >&2; fail "instalación por defecto"; }
expect "última versión" "$(inside /usr/local/bin/talaia -version)" v0.0.2
pass "INS-09 Contenedor existente y última versión"

echo "10 pruebas del instalador correctas"
