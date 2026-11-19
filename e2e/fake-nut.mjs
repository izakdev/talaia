// Servidor NUT (upsd) simulado, con la forma de respuesta observada en un upsd 2.8.1 real (NETVER 1.3).
// Solo responde a las órdenes de consulta; cualquier otra recibe ERR, como haría upsd sin LOGIN.
import { createServer } from "node:net";

export const NUT_UPS = "sai-simulado";
export const NUT_SERIAL = "SIM-SERIE-0001";

export function fakeNUT(port) {
  // status es ups.status tal cual; several añade un segundo SAI; stale simula el driver desconectado.
  const state = { status: "OL CHRG", charge: 100, runtime: 2400, several: false, stale: false };
  const vars = () => [
    ["battery.charge", String(state.charge)],
    ["battery.charge.low", "10"],
    ["battery.runtime", String(state.runtime)],
    ["battery.type", "PbAcid"],
    ["device.mfr", "Simulado"],
    ["device.model", "Simulado 1500"],
    ["device.serial", NUT_SERIAL],
    ["driver.name", "usbhid-ups"],
    ["driver.parameter.port", "auto"],
    ["input.voltage", "231.0"],
    ["ups.load", "10"],
    ["ups.model", "Simulado 1500"],
    ["ups.serial", NUT_SERIAL],
    ["ups.status", state.status],
  ];
  const answer = (cmd) => {
    if (cmd === "VER") return ["Network UPS Tools upsd 2.8.1 - https://www.networkupstools.org/"];
    if (cmd === "LIST UPS") {
      const list = [`UPS ${NUT_UPS} "SAI del rack simulado"`];
      if (state.several) list.push('UPS otro-sai "Otro SAI"');
      return ["BEGIN LIST UPS", ...list, "END LIST UPS"];
    }
    if (cmd === `LIST VAR ${NUT_UPS}`) {
      if (state.stale) return ["ERR DATA-STALE"];
      return [`BEGIN LIST VAR ${NUT_UPS}`, ...vars().map(([k, v]) => `VAR ${NUT_UPS} ${k} "${v}"`), `END LIST VAR ${NUT_UPS}`];
    }
    if (cmd.startsWith("LIST VAR ")) return ["ERR UNKNOWN-UPS"];
    return ["ERR UNKNOWN-COMMAND"];
  };
  const handler = (socket) => {
    let buffer = "";
    socket.on("error", () => {});
    socket.on("data", (chunk) => {
      buffer += chunk;
      let end;
      while ((end = buffer.indexOf("\n")) >= 0) {
        const cmd = buffer.slice(0, end).replace(/\r$/, "");
        buffer = buffer.slice(end + 1);
        if (cmd === "LOGOUT") {
          socket.end("OK Goodbye\n");
          return;
        }
        socket.write(answer(cmd).join("\n") + "\n");
      }
    });
  };
  let server;
  const sockets = new Set();
  return {
    state,
    url: `nut://127.0.0.1:${port}`,
    start: () =>
      new Promise((resolve, reject) => {
        server = createServer(handler);
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
