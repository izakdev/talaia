"""Explora la API de un servicio sin mostrar secretos ni datos del laboratorio.

Solo hace peticiones GET. Pide la URL y la credencial por teclado, sin eco, para
que nunca pasen por el chat. Oculta claves, direcciones, puertos, dominios y los
nombres de los campos que se indiquen, y resume las listas largas.

Uso (siempre con -I, desde fuera del repositorio):
    python3 -I explorar.py --auth header:X-Api-Key /api/version /api/estado
    python3 -I explorar.py --auth basic --nombres name,hostname /api/a
    python3 -I explorar.py --auth bearer --sin-clave /api/version

--auth: none | basic (usuario y secreto) | bearer | header:<Nombre-Cabecera>
--sin-clave: además, repite cada ruta sin credencial para ver el error.
--nombres: campos cuyo valor se oculta siempre (nombres de equipos, peers…).
"""
import argparse
import base64
import getpass
import json
import re
import ssl
import urllib.error
import urllib.request

SECRET_KEY = re.compile(
    r"key|secret|token|pass|auth|cookie|session|endpoint|allowed|address|addr|ip$|^ip|dns|psk|mac|serial|email|user|host|url|domain|port|^(pre|post)(up|down)$|script|command|cmd",
    re.I,
)
IPV4 = re.compile(r"\b\d{1,3}(?:\.\d{1,3}){3}(?:/\d+)?\b")
# Al menos tres grupos con ':' para no confundir horas como 0:01:02.
IPV6 = re.compile(r"\b(?:[0-9a-f]{0,4}:){3,7}[0-9a-f]{0,4}\b", re.I)
DOMAIN = re.compile(r"\b(?:[a-z0-9-]+\.)+(?:lan|local|home|internal|arpa|com|net|org|es|cat|dev|io)\b", re.I)
URL = re.compile(r"[a-z]+://\S+", re.I)
MAX_ITEMS = 3


def parse_args():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("rutas", nargs="+", help="rutas a consultar, por ejemplo /api/version")
    p.add_argument("--auth", default="none")
    p.add_argument("--sin-clave", action="store_true")
    p.add_argument("--nombres", default="", help="campos cuyo valor se oculta, separados por comas")
    p.add_argument("--profundidad", type=int, default=6)
    return p.parse_args()


def mask_text(value):
    value = URL.sub("<url>", value)
    value = IPV4.sub("<ip>", value)
    value = IPV6.sub("<ip6>", value)
    return DOMAIN.sub("<dominio>", value)


def clean(value, names, depth, key=""):
    if key and (SECRET_KEY.search(key) or key.lower() in names):
        if value in (None, "", [], {}):
            return value
        return f"<oculto: {type(value).__name__}>"
    if depth <= 0:
        return f"<{type(value).__name__}>"
    if isinstance(value, dict):
        return {k: clean(v, names, depth - 1, k) for k, v in value.items()}
    if isinstance(value, list):
        items = [clean(v, names, depth - 1) for v in value[:MAX_ITEMS]]
        if len(value) > MAX_ITEMS:
            items.append(f"<… {len(value) - MAX_ITEMS} más>")
        return items
    if isinstance(value, str):
        return mask_text(value)
    return value


def request(base, path, headers, ctx):
    req = urllib.request.Request(base + path, headers={"Accept": "application/json", **headers})
    try:
        with urllib.request.urlopen(req, context=ctx, timeout=20) as r:
            raw = r.read()
            status = r.status
    except urllib.error.HTTPError as e:
        raw, status = e.read(), e.code
    except Exception as e:  # noqa: BLE001
        return type(e).__name__, None, ""
    try:
        return status, json.loads(raw), ""
    except ValueError:
        return status, None, mask_text(raw[:200].decode("utf-8", "replace"))


def main():
    args = parse_args()
    names = {n.strip().lower() for n in args.nombres.split(",") if n.strip()}
    base = input("URL del servicio (por ejemplo https://servicio.local): ").strip().rstrip("/")
    headers = {}
    if args.auth == "basic":
        user = getpass.getpass("Usuario o clave (no se muestra): ").strip()
        secret = getpass.getpass("Secreto (no se muestra): ").strip()
        headers["Authorization"] = "Basic " + base64.b64encode(f"{user}:{secret}".encode()).decode()
    elif args.auth == "bearer":
        headers["Authorization"] = "Bearer " + getpass.getpass("Token (no se muestra): ").strip()
    elif args.auth.startswith("header:"):
        headers[args.auth.split(":", 1)[1]] = getpass.getpass("Clave (no se muestra): ").strip()
    elif args.auth != "none":
        raise SystemExit(f"--auth desconocido: {args.auth}")
    ctx = ssl._create_unverified_context()

    for path in args.rutas:
        tries = [("con credencial", headers)]
        if args.sin_clave and headers:
            tries.append(("sin credencial", {}))
        for label, hdrs in tries:
            status, body, text = request(base, path, hdrs, ctx)
            print(f"=== GET {path} ({label}): HTTP {status}")
            if body is not None:
                if isinstance(body, dict):
                    print("claves:", sorted(body))
                print(json.dumps(clean(body, names, args.profundidad), ensure_ascii=False, indent=1))
            elif text:
                print("respuesta no JSON:", text)
            print()


if __name__ == "__main__":
    main()
