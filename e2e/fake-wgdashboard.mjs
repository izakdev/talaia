// WGDashboard simulado, con la forma de respuesta observada en un WGDashboard 4.3.3 real.
import { createServer } from "node:http";

export const WGDASHBOARD_KEY = "clave-de-wgdashboard-de-prueba";

const GiB = 1024 ** 3;

export function fakeWGDashboard(port) {
  const state = { stopped: false, empty: false };
  const tunnel = (name, status, connected, total, sent, receive) => ({
    Status: status,
    Name: name,
    PrivateKey: "clave-privada-simulada",
    PublicKey: "clave-publica-simulada",
    Address: "10.66.0.1/24",
    ListenPort: "51666",
    PreUp: "",
    PreDown: "",
    PostUp: "iptables -A FORWARD -i wg-simulado -j ACCEPT",
    PostDown: "",
    SaveConfig: true,
    DataUsage: { Total: sent + receive, Sent: sent, Receive: receive },
    ConnectedPeers: connected,
    TotalPeers: total,
    Protocol: "wg",
    Table: "",
    Info: { Description: "", OverridePeerSettings: { DNS: "192.0.2.53", PeerRemoteEndpoint: "vpn.simulado" }, PeerGroups: {} },
  });
  const configurations = () =>
    state.empty
      ? []
      : [tunnel("wg0", true, 2, 3, 1.25, 0.25), tunnel("wg1", !state.stopped, state.stopped ? 0 : 1, 1, 0.5, 0.5)];
  const reply = (res, status, data, message = null) =>
    res.writeHead(status).end(JSON.stringify({ status: status === 200, message, data }));
  const handler = (req, res) => {
    res.setHeader("Content-Type", "application/json");
    if (req.method !== "GET") {
      res.writeHead(405).end();
    } else if (req.url === "/api/getDashboardVersion") {
      reply(res, 200, "v4.3-simulado");
    } else if (req.url === "/api/getWireguardConfigurations") {
      const key = req.headers["wg-dashboard-apikey"];
      if (!key) return reply(res, 401, null, "Unauthorized access.");
      if (key !== WGDASHBOARD_KEY) return reply(res, 401, null, "API Key does not exist");
      reply(res, 200, configurations());
    } else {
      res.writeHead(404).end();
    }
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
