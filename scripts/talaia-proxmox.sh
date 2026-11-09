#!/usr/bin/env bash
# Instala y actualiza Talaia en un contenedor LXC de Proxmox VE.
# Se ejecuta como root en el host de Proxmox. Uso: talaia-proxmox.sh --help
set -euo pipefail

BASE_URL="${TALAIA_DOWNLOAD_BASE:-https://github.com/izakdev/talaia/releases}"
DATA_DIR="/var/lib/talaia"
HEALTH_ATTEMPTS="${TALAIA_HEALTH_ATTEMPTS:-30}"
BACKUPS_KEPT=3

CTID=""
HOSTNAME_CT="talaia"
CORES=1
MEMORY=512
SWAP=256
DISK=4
STORAGE=""
TEMPLATE_STORAGE=""
BRIDGE="vmbr0"
IP="dhcp"
GATEWAY=""
VLAN=""
PORT=8080
VERSION="latest"
BINARY=""
ASSUME_YES=0
FORCE=0

WORK_DIR=""
CREATED_CT=""
CT_COMMAND=""
# Copia de la actualización en curso, para poder deshacerla si falla un paso.
UPDATE_BACKUP=""
UPDATE_BACKUP_DONE=0

log() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mAviso:\033[0m %s\n' "$*" >&2; }
die() {
  printf '\033[1;31mError:\033[0m %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Instala y actualiza Talaia en un contenedor LXC de Proxmox VE.

Uso:
  talaia-proxmox.sh install [opciones]
  talaia-proxmox.sh update --ctid ID [opciones]

Opciones de instalación:
  --ctid ID              ID del contenedor (por defecto, el siguiente libre)
  --hostname NOMBRE      Nombre del contenedor (talaia)
  --cores N              Número de vCPU (1)
  --memory MB            Memoria en MB (512)
  --swap MB              Intercambio en MB (256)
  --disk GB              Disco en GB (4)
  --storage NOMBRE       Almacenamiento del disco (el primero disponible)
  --template-storage N   Almacenamiento de la plantilla (el primero disponible)
  --bridge NOMBRE        Puente de red (vmbr0)
  --ip CIDR|dhcp         Dirección, por ejemplo 192.0.2.50/24 (dhcp)
  --gateway IP           Puerta de enlace, si la dirección es fija
  --vlan ID              Etiqueta VLAN
  --port N               Puerto de Talaia (8080)

Opciones comunes:
  --version vX.Y.Z       Versión que instalar (la última)
  --binary RUTA          Usa un binario local en vez de descargarlo
  --yes                  No pide confirmación
  --force                Al actualizar, reinstala aunque sea la misma versión
  -h, --help             Muestra esta ayuda
EOF
}

parse_args() {
  while (($# > 0)); do
    case "$1" in
      --ctid) CTID="${2:?Falta el valor de --ctid}"; shift 2 ;;
      --hostname) HOSTNAME_CT="${2:?Falta el valor de --hostname}"; shift 2 ;;
      --cores) CORES="${2:?Falta el valor de --cores}"; shift 2 ;;
      --memory) MEMORY="${2:?Falta el valor de --memory}"; shift 2 ;;
      --swap) SWAP="${2:?Falta el valor de --swap}"; shift 2 ;;
      --disk) DISK="${2:?Falta el valor de --disk}"; shift 2 ;;
      --storage) STORAGE="${2:?Falta el valor de --storage}"; shift 2 ;;
      --template-storage) TEMPLATE_STORAGE="${2:?Falta el valor de --template-storage}"; shift 2 ;;
      --bridge) BRIDGE="${2:?Falta el valor de --bridge}"; shift 2 ;;
      --ip) IP="${2:?Falta el valor de --ip}"; shift 2 ;;
      --gateway) GATEWAY="${2:?Falta el valor de --gateway}"; shift 2 ;;
      --vlan) VLAN="${2:?Falta el valor de --vlan}"; shift 2 ;;
      --port) PORT="${2:?Falta el valor de --port}"; shift 2 ;;
      --version) VERSION="${2:?Falta el valor de --version}"; shift 2 ;;
      --binary) BINARY="${2:?Falta el valor de --binary}"; shift 2 ;;
      --yes | -y) ASSUME_YES=1; shift ;;
      --force) FORCE=1; shift ;;
      -h | --help) usage; exit 0 ;;
      *) die "Opción desconocida: $1. Consulta --help." ;;
    esac
  done
  validate_args
}

is_number() { [[ "$1" =~ ^[0-9]+$ ]]; }

validate_args() {
  local value
  for value in "$CORES" "$MEMORY" "$SWAP" "$DISK" "$PORT"; do
    is_number "$value" || die "Se esperaba un número en lugar de: $value"
  done
  [[ -z "$CTID" ]] || is_number "$CTID" || die "El ID del contenedor debe ser un número."
  [[ -z "$VLAN" ]] || is_number "$VLAN" || die "La VLAN debe ser un número."
  ((PORT >= 1 && PORT <= 65535)) || die "El puerto debe estar entre 1 y 65535."
  [[ "$HOSTNAME_CT" =~ ^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$ ]] || die "Nombre de contenedor no válido: $HOSTNAME_CT"
  [[ "$VERSION" == latest || "$VERSION" =~ ^v[0-9A-Za-z.+-]+$ ]] || die "Versión no válida: $VERSION"
  [[ "$IP" == dhcp || "$IP" =~ ^[0-9a-fA-F:.]+/[0-9]+$ ]] || die "Dirección no válida: $IP. Usa dhcp o un CIDR."
  if [[ -n "$GATEWAY" && "$IP" == dhcp ]]; then die "--gateway requiere una dirección fija en --ip."; fi
  if [[ -n "$BINARY" && ! -f "$BINARY" ]]; then die "No existe el binario: $BINARY"; fi
  return 0
}

require_host() {
  ((EUID == 0)) || die "Ejecútalo como root en el host de Proxmox."
  local command
  for command in pct pveam pvesh pvesm curl sha256sum; do
    command -v "$command" >/dev/null || die "Falta $command. Este script debe ejecutarse en un host de Proxmox VE."
  done
}

host_arch() {
  local arch
  arch="$(dpkg --print-architecture)"
  case "$arch" in
    amd64 | arm64) printf '%s' "$arch" ;;
    *) die "Arquitectura no admitida: $arch" ;;
  esac
}

cleanup() {
  local status=$?
  if ((status != 0)) && [[ -n "$CREATED_CT" ]]; then
    warn "La instalación ha fallado. Se elimina el contenedor $CREATED_CT."
    pct stop "$CREATED_CT" >/dev/null 2>&1 || true
    pct destroy "$CREATED_CT" >/dev/null 2>&1 || warn "No se pudo eliminar el contenedor $CREATED_CT."
  fi
  if [[ -n "$WORK_DIR" ]]; then rm -rf "$WORK_DIR"; fi
  exit "$status"
}

# Deja en $WORK_DIR/talaia el binario que se va a instalar, ya verificado.
fetch_binary() {
  local arch="$1" file url
  if [[ -n "$BINARY" ]]; then
    warn "Se usa un binario local, sin comprobar su suma."
    install -m 0755 "$BINARY" "$WORK_DIR/talaia"
    return
  fi
  file="talaia-linux-$arch"
  if [[ "$VERSION" == latest ]]; then url="$BASE_URL/latest/download"; else url="$BASE_URL/download/$VERSION"; fi
  log "Descargando Talaia ($VERSION, $arch)"
  curl -fsSL --retry 3 -o "$WORK_DIR/$file" "$url/$file" \
    || die "No se pudo descargar $url/$file. Si el repositorio es privado, usa --binary."
  curl -fsSL --retry 3 -o "$WORK_DIR/SHA256SUMS" "$url/SHA256SUMS" \
    || die "No se pudieron descargar las sumas de comprobación."
  grep -E "[[:space:]]\*?$file\$" "$WORK_DIR/SHA256SUMS" >"$WORK_DIR/expected" \
    || die "SHA256SUMS no incluye $file."
  (cd "$WORK_DIR" && sha256sum --check --status expected) \
    || die "La suma de comprobación no coincide. No se instala nada."
  chmod 0755 "$WORK_DIR/$file"
  mv "$WORK_DIR/$file" "$WORK_DIR/talaia"
}

first_storage() {
  pvesm status --content "$1" | awk 'NR > 1 && $3 == "active" { print $1; exit }'
}

# La lista incluye plantillas de varias arquitecturas: hay que elegir la del nodo.
latest_template() {
  pveam update >/dev/null
  pveam available --section system | awk '{ print $2 }' \
    | grep -E "^debian-[0-9]+-standard_.*_$1\.tar\." | sort -V | tail -n 1 || true
}

ct_running() { pct status "$1" 2>/dev/null | grep -q 'running'; }

ct_exec() {
  local id="$1"
  shift
  CT_COMMAND="pct exec $id -- $*"
  pct exec "$id" -- "$@"
}

write_setup_script() {
  cat <<'INNER' >"$WORK_DIR/setup.sh"
#!/bin/bash
# Se ejecuta dentro del contenedor. $1: puerto, o vacío para conservar la configuración.
set -euo pipefail
port="${1:-}"
data=/var/lib/talaia

id -u talaia >/dev/null 2>&1 || useradd --system --home-dir "$data" --no-create-home --shell /usr/sbin/nologin talaia
install -d -m 0700 -o talaia -g talaia "$data"
install -d -m 0755 /etc/talaia

if [[ -n "$port" ]]; then
  cat <<ENV >/etc/talaia/talaia.env
TALAIA_LISTEN=:$port
TALAIA_DATA_DIR=$data
TALAIA_SETUP_TOKEN_FILE=$data/setup.token
GOMEMLIMIT=400MiB
ENV
  chmod 0644 /etc/talaia/talaia.env
fi

cat <<'UNIT' >/etc/systemd/system/talaia.service
[Unit]
Description=Talaia
Documentation=https://github.com/izakdev/talaia
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=talaia
Group=talaia
EnvironmentFile=/etc/talaia/talaia.env
ExecStart=/usr/local/bin/talaia
Restart=on-failure
RestartSec=5
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/talaia
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
RestrictNamespaces=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
SystemCallArchitectures=native
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
CapabilityBoundingSet=

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl --quiet enable talaia
INNER
}

push_setup_and_run() {
  local id="$1" port="$2"
  write_setup_script
  pct push "$id" "$WORK_DIR/setup.sh" /root/talaia-setup.sh --perms 0700
  ct_exec "$id" bash /root/talaia-setup.sh "$port"
  ct_exec "$id" rm -f /root/talaia-setup.sh
}

# Comprueba /healthz desde dentro del contenedor, sin depender de curl ni de la red.
wait_healthy() {
  local id="$1" attempt
  # shellcheck disable=SC2016 # Se expande dentro del contenedor.
  local check='. /etc/talaia/talaia.env; exec 3<>"/dev/tcp/127.0.0.1/${TALAIA_LISTEN##*:}"; printf "GET /healthz HTTP/1.0\r\n\r\n" >&3; grep -q "200 OK" <&3'
  for ((attempt = 0; attempt < HEALTH_ATTEMPTS; attempt++)); do
    if ct_exec "$id" bash -c "$check" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  return 1
}

show_failure_logs() {
  ct_exec "$1" journalctl -u talaia -n 20 --no-pager >&2 || true
}

container_ip() {
  local id="$1" attempt address
  for ((attempt = 0; attempt < 15; attempt++)); do
    address="$(ct_exec "$id" hostname -I 2>/dev/null | awk '{ print $1 }')" || address=""
    if [[ -n "$address" ]]; then
      printf '%s' "$address"
      return
    fi
    sleep 2
  done
}

confirm() {
  ((ASSUME_YES == 1)) && return 0
  local answer=""
  if ! { : >/dev/tty; } 2>/dev/null; then die "Sin terminal para confirmar. Usa --yes."; fi
  # A la terminal y no a stderr: con 2>archivo, la pregunta desaparecería y parecería colgado.
  printf '%s [s/N] ' "$1" >/dev/tty
  read -r answer </dev/tty || true
  [[ "$answer" =~ ^[sSyY] ]] || die "Cancelado."
}

cmd_install() {
  local arch template net token address
  require_host
  arch="$(host_arch)"

  if [[ -z "$CTID" ]]; then CTID="$(pvesh get /cluster/nextid)"; fi
  if pct status "$CTID" >/dev/null 2>&1; then die "Ya existe el contenedor $CTID."; fi
  if [[ -z "$STORAGE" ]]; then STORAGE="$(first_storage rootdir)"; fi
  [[ -n "$STORAGE" ]] || die "No hay almacenamiento para contenedores. Indícalo con --storage."
  if [[ -z "$TEMPLATE_STORAGE" ]]; then TEMPLATE_STORAGE="$(first_storage vztmpl)"; fi
  [[ -n "$TEMPLATE_STORAGE" ]] || die "No hay almacenamiento para plantillas. Indícalo con --template-storage."

  net="name=eth0,bridge=$BRIDGE,ip=$IP"
  if [[ -n "$GATEWAY" ]]; then net+=",gw=$GATEWAY"; fi
  if [[ -n "$VLAN" ]]; then net+=",tag=$VLAN"; fi

  cat <<EOF
Se va a crear este contenedor:
  ID:             $CTID
  Nombre:         $HOSTNAME_CT
  Recursos:       $CORES vCPU, $MEMORY MB de RAM, $DISK GB de disco ($STORAGE)
  Red:            $net
  Talaia:         ${BINARY:-$VERSION}, puerto $PORT
EOF
  confirm "¿Continuar?"

  fetch_binary "$arch"

  template="$(latest_template "$arch")"
  [[ -n "$template" ]] || die "No se encuentra una plantilla de Debian para $arch."
  if ! pveam list "$TEMPLATE_STORAGE" | grep -q "$template"; then
    log "Descargando la plantilla $template"
    pveam download "$TEMPLATE_STORAGE" "$template" >/dev/null
  fi

  log "Creando el contenedor $CTID"
  pct create "$CTID" "$TEMPLATE_STORAGE:vztmpl/$template" \
    --arch "$arch" --hostname "$HOSTNAME_CT" --cores "$CORES" --memory "$MEMORY" --swap "$SWAP" \
    --rootfs "$STORAGE:$DISK" --net0 "$net" --ostype debian \
    --unprivileged 1 --features nesting=1 --onboot 1 >/dev/null
  CREATED_CT="$CTID"
  pct start "$CTID"

  log "Instalando Talaia"
  # Hay que esperar a que el sistema del contenedor esté listo.
  local attempt
  for ((attempt = 0; attempt < 60; attempt++)); do
    if ct_exec "$CTID" systemctl is-system-running >/dev/null 2>&1; then break; fi
    if ct_exec "$CTID" test -d /run/systemd/system >/dev/null 2>&1 && ((attempt > 5)); then break; fi
    sleep 1
  done
  pct push "$CTID" "$WORK_DIR/talaia" /usr/local/bin/talaia --perms 0755
  push_setup_and_run "$CTID" "$PORT"

  token="$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  printf '%s\n' "$token" >"$WORK_DIR/setup.token"
  pct push "$CTID" "$WORK_DIR/setup.token" "$DATA_DIR/setup.token" --perms 0600
  ct_exec "$CTID" chown talaia:talaia "$DATA_DIR/setup.token"

  ct_exec "$CTID" systemctl start talaia
  if ! wait_healthy "$CTID"; then
    show_failure_logs "$CTID"
    die "Talaia no ha arrancado."
  fi
  CREATED_CT=""

  address="$(container_ip "$CTID" || true)"
  log "Talaia está en marcha en el contenedor $CTID"
  if [[ -n "$address" ]]; then
    printf '\nAbre este enlace para crear el administrador:\n  http://%s:%s/?setup=%s\n\n' "$address" "$PORT" "$token"
  else
    printf '\nNo se ha podido obtener la dirección del contenedor.\nCódigo de instalación: %s\n\n' "$token"
  fi
  echo "El código solo sirve una vez. Hasta usarlo, nadie más puede crear el administrador."
}

# Lo llama el trap ERR, que se ejecuta en la función donde falló el comando.
update_step_failed() {
  local status="$1" command="$2" line="$3"
  # $BASH_COMMAND llega sin expandir: de ct_exec interesa qué le pidieron y desde dónde.
  if [[ "${FUNCNAME[1]:-}" == ct_exec ]]; then
    command="$CT_COMMAND"
    line="${BASH_LINENO[1]}"
  fi
  warn "Ha fallado un paso de la actualización (línea $line, código $status): $command"
  rollback_update
}

disarm_update_traps() {
  trap - ERR INT TERM HUP
  set +E
}

# Deshace la actualización y termina con un error. Si la copia no se completó, aún no se
# había cambiado nada: se descarta la copia a medias y basta con volver a arrancar.
rollback_update() {
  local hint="" running
  disarm_update_traps
  set +e
  # Una interrupción o una sesión cortada no deben dejar la marcha atrás a medias.
  trap '' INT TERM HUP
  if ! ct_running "$CTID"; then pct start "$CTID"; sleep 5; fi
  ct_exec "$CTID" systemctl stop talaia
  if ((UPDATE_BACKUP_DONE == 1)); then
    hint=" Copia de seguridad en $UPDATE_BACKUP, dentro del contenedor."
    ct_exec "$CTID" cp -a "$UPDATE_BACKUP/talaia.previous" /usr/local/bin/talaia \
      || warn "No se pudo restaurar el binario anterior."
    ct_exec "$CTID" bash -c "[[ ! -e '$UPDATE_BACKUP/talaia.db' ]] || { rm -f '$DATA_DIR'/talaia.db* && cp -a '$UPDATE_BACKUP'/talaia.db* '$DATA_DIR'/; }" \
      || warn "No se pudo restaurar la base de datos."
  elif [[ -n "$UPDATE_BACKUP" ]]; then
    ct_exec "$CTID" rm -rf "$UPDATE_BACKUP"
  fi
  ct_exec "$CTID" systemctl start talaia
  if wait_healthy "$CTID"; then
    running="$(ct_exec "$CTID" /usr/local/bin/talaia -version 2>/dev/null || echo "desconocida")"
    die "Actualización cancelada. Sigue en marcha la versión $running.$hint"
  fi
  show_failure_logs "$CTID"
  die "Tampoco arranca la versión anterior.$hint"
}

cmd_update() {
  local arch current new stamp backup
  [[ -n "$CTID" ]] || die "Indica el contenedor con --ctid."
  require_host
  arch="$(host_arch)"
  pct status "$CTID" >/dev/null 2>&1 || die "No existe el contenedor $CTID."
  if ! ct_running "$CTID"; then pct start "$CTID"; sleep 5; fi
  ct_exec "$CTID" test -x /usr/local/bin/talaia || die "El contenedor $CTID no tiene Talaia instalado."

  fetch_binary "$arch"
  current="$(ct_exec "$CTID" /usr/local/bin/talaia -version)"
  chmod 0755 "$WORK_DIR/talaia"
  new="$("$WORK_DIR/talaia" -version 2>/dev/null || echo "desconocida")"
  if [[ "$current" == "$new" ]] && ((FORCE == 0)); then
    log "Talaia ya está en la versión $current. Usa --force para reinstalarla."
    return
  fi
  log "Actualizando de $current a $new"
  confirm "¿Continuar?"

  stamp="$(date +%Y%m%d-%H%M%S)"
  backup="$DATA_DIR/backups/$stamp"

  # Desde aquí, Talaia está parado: ningún fallo puede terminar sin volver a arrancarlo.
  set -E
  trap 'update_step_failed "$?" "$BASH_COMMAND" "$LINENO"' ERR
  trap 'warn "Actualización interrumpida."; rollback_update' INT TERM HUP

  log "Parando Talaia y haciendo la copia de seguridad"
  ct_exec "$CTID" systemctl stop talaia
  UPDATE_BACKUP="$backup"
  ct_exec "$CTID" install -d -m 0700 -o talaia -g talaia "$DATA_DIR/backups" "$backup"
  ct_exec "$CTID" bash -c "[[ ! -e '$DATA_DIR/talaia.db' ]] || cp -a '$DATA_DIR'/talaia.db* '$backup'/"
  ct_exec "$CTID" cp -a /usr/local/bin/talaia "$backup/talaia.previous"
  UPDATE_BACKUP_DONE=1

  log "Instalando la versión nueva"
  pct push "$CTID" "$WORK_DIR/talaia" /usr/local/bin/talaia --perms 0755
  push_setup_and_run "$CTID" ""
  ct_exec "$CTID" systemctl start talaia

  if ! wait_healthy "$CTID"; then
    warn "La versión nueva no arranca. Se restaura la anterior."
    show_failure_logs "$CTID"
    rollback_update
  fi
  disarm_update_traps

  # Conserva solo las copias más recientes. Talaia ya funciona: si falla, no es grave.
  ct_exec "$CTID" bash -c "cd '$DATA_DIR/backups' && ls -1 | sort -r | tail -n +$((BACKUPS_KEPT + 1)) | xargs -r rm -rf" \
    || warn "No se pudieron borrar las copias antiguas de $DATA_DIR/backups."
  log "Talaia actualizado a $new. Copia de la versión anterior en $backup, dentro del contenedor."
}

main() {
  local command="${1:-}"
  case "$command" in
    install | update) shift ;;
    -h | --help | "") usage; exit 0 ;;
    *) die "Comando desconocido: $command. Usa install o update." ;;
  esac
  parse_args "$@"
  WORK_DIR="$(mktemp -d)"
  trap cleanup EXIT
  "cmd_$command"
}

main "$@"
