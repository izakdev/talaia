// OPNsense simulado, con la forma de respuesta observada en un OPNsense 26.7 real.
import { createServer } from "node:http";

export const OPN_KEY = "clave-de-api-de-prueba";
export const OPN_SECRET = "secreto-de-api-de-prueba";

export function fakeOPNsense(port) {
  const state = { gateway: "none", monitored: true, limited: false, received: 88000000000, sent: 21000000000, wireguard: true, tunnelDown: false };
  const authorized = (req) =>
    req.headers.authorization === `Basic ${Buffer.from(`${OPN_KEY}:${OPN_SECRET}`).toString("base64")}`;
  const routes = {
    "/api/diagnostics/activity/get_activity": () => ({
      headers: [
        "last pid:  8875;  load averages:    0.07,    0.14,    0.11  up 45+13:29:07    15:43:52",
        "409 threads:   5 running, 362 sleeping, 42 waiting",
        "CPU:  7.0% user,  0.0% nice,  3.0% system,  0.0% interrupt, 90.0% idle",
        "Mem: 163M Active, 732M Inact, 2043M Wired, 56K Buf, 4993M Free",
      ],
      details: [{ PID: "11", USERNAME: "root", COMMAND: "proceso-simulado" }],
    }),
    "/api/routes/gateway/status": () => ({
      items: [
        { name: "WAN_V6", address: "~", status: "none", loss: "~", delay: "~", stddev: "~", monitor: "~", status_translated: "Online" },
        {
          name: "WAN_V4",
          address: "192.0.2.50",
          status: state.gateway,
          loss: state.monitored ? "0.0 %" : "~",
          delay: state.monitored ? "4.2 ms" : "~",
          stddev: "0.3 ms",
          monitor: "192.0.2.51",
          status_translated: "Online",
        },
      ],
      status: "ok",
    }),
    "/api/diagnostics/system/system_information": () => ({
      name: "cortafuegos.simulado",
      versions: ["OPNsense 26.7-simulado-amd64", "FreeBSD 15.1-RELEASE", "OpenSSL 3.5.7"],
      updates: "Click to view pending updates.",
    }),
    "/api/diagnostics/system/system_resources": () => ({
      memory: { total: "8589934592", total_frmt: "8192", used: 2147483648, used_frmt: "2048", arc: "1", arc_frmt: "1", arc_txt: "ARC" },
    }),
    "/api/diagnostics/system/system_disk": () => ({
      devices: [{ device: "zroot/ROOT/default", type: "zfs", blocks: "20G", used: "5.0G", available: "15G", used_pct: 25, mountpoint: "/" }],
    }),
    "/api/diagnostics/traffic/interface": () => {
      state.received += 12500000;
      state.sent += 2500000;
      return {
        interfaces: {
          lan: { device: "vtnet1", "bytes received": "1", "bytes transmitted": "2", name: "LAN" },
          wan: { device: "pppoe1", "bytes received": String(state.received), "bytes transmitted": String(state.sent), name: "WAN" },
        },
        time: 1,
      };
    },
  };
  // Forma de /api/wireguard/service/show de un OPNsense 26.7 real; los peers siguen
  // el código de wg_show.py y ServiceController.php.
  const peer = (name, status, age, rx, tx) => ({
    if: "wg0",
    type: "peer",
    "public-key": `clave-publica-${name}`,
    endpoint: "198.51.100.7:4500",
    "allowed-ips": "10.77.0.2/32",
    "latest-handshake": age === null ? 0 : Math.floor(Date.now() / 1000) - age,
    "transfer-rx": rx,
    "transfer-tx": tx,
    "persistent-keepalive": "off",
    name,
    "latest-handshake-age": age,
    "peer-status": status,
    ifname: "Acceso remoto",
  });
  routes["/api/wireguard/service/show"] = () => {
    const rows = [
      { if: "wg0", type: "interface", "public-key": "clave-publica-servidor", "listen-port": "51777", fwmark: "off", endpoint: "51777",
        status: state.tunnelDown ? "down" : "up", name: "Acceso remoto", "latest-handshake-age": null, "peer-status": "offline", ifname: "Acceso remoto" },
      peer("peer-simulado-movil", "online", 30, 536870912, 1610612736),
      peer("peer-simulado-portatil", "offline", null, 0, 0),
    ];
    return { total: rows.length, rowCount: rows.length, current: 1, rows };
  };
  const handler = (req, res) => {
    if (!authorized(req)) {
      res.writeHead(401).end();
      return;
    }
    if ((state.limited && req.url.startsWith("/api/diagnostics/system/")) || (!state.wireguard && req.url.startsWith("/api/wireguard/"))) {
      res.writeHead(403).end(JSON.stringify({ status: 403, message: "Forbidden" }));
      return;
    }
    const route = routes[req.url];
    if (req.method !== "GET" || !route) {
      res.writeHead(404).end();
      return;
    }
    res.setHeader("Content-Type", "application/json");
    res.end(JSON.stringify(route()));
  };
  let server;
  return {
    state,
    url: `http://localhost:${port}`,
    start: () =>
      new Promise((resolve, reject) => {
        server = createServer(handler);
        server.once("error", reject);
        server.listen(port, "127.0.0.1", resolve);
      }),
    stop: () =>
      new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      }),
  };
}
