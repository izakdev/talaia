// AdGuard Home simulado, con la forma de respuesta de /control/status y /control/stats.
import { createServer } from "node:http";

export const USER = "admin-adguard";
export const PASSWORD = "contrasena-de-prueba";

export function fakeAdGuard(port) {
  const state = { protection: true };
  const authorized = (req) =>
    req.headers.authorization === `Basic ${Buffer.from(`${USER}:${PASSWORD}`).toString("base64")}`;
  const handler = (req, res) => {
    if (!authorized(req)) {
      res.writeHead(401).end();
      return;
    }
    res.setHeader("Content-Type", "application/json");
    if (req.url === "/control/status") {
      res.end(JSON.stringify({ version: "v0.107.0-simulado", running: true, protection_enabled: state.protection }));
    } else if (req.url === "/control/stats") {
      res.end(
        JSON.stringify({
          time_units: "hours",
          num_dns_queries: 128450,
          num_blocked_filtering: 21360,
          num_replaced_safebrowsing: 12,
          num_replaced_parental: 0,
          avg_processing_time: 0.0124,
          top_queried_domains: [{ "actualizaciones.example": 8120 }, { "correo.example": 5233 }, { "noticias.example": 2940 }],
          top_blocked_domains: [{ "anuncios.example": 9110 }, { "rastreador.example": 4021 }],
          dns_queries: new Array(24).fill(0),
        }),
      );
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
