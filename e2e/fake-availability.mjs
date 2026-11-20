// Equipo simulado para la integración de disponibilidad: un servidor HTTP que
// también sirve de destino TCP en el mismo puerto. No tiene credenciales.
import { createServer } from "node:http";

// Ruta que no debe aparecer en el panel público.
export const AVAILABILITY_PATH = "/ruta-privada";

export function fakeAvailability(port) {
  // status es el código HTTP que devuelve el equipo.
  const state = { status: 200 };
  let server;
  const sockets = new Set();
  return {
    state,
    host: `127.0.0.1:${port}`,
    start: () =>
      new Promise((resolve, reject) => {
        server = createServer((req, res) => {
          res.writeHead(state.status, { "Content-Type": "text/html" });
          res.end("<html><body>Switch simulado</body></html>");
        });
        server.on("connection", (s) => {
          sockets.add(s);
          s.on("close", () => sockets.delete(s));
        });
        server.once("error", reject);
        server.listen(port, "127.0.0.1", resolve);
      }),
    stop: () =>
      new Promise((resolve) => {
        server.close(resolve);
        for (const s of sockets) s.destroy();
      }),
  };
}
