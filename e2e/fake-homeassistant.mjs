// Home Assistant simulado, con la forma de respuesta de la API REST real.
import { createServer } from "node:http";

export const HA_TOKEN = "token-de-prueba-larga-duracion";

export function fakeHomeAssistant(port) {
  const state = { light: "on", door: "off", starting: false };
  const entity = (entity_id, value, attributes) => ({
    entity_id, state: value, attributes,
    last_changed: "2026-11-02T10:00:00+00:00", last_updated: "2026-11-02T10:00:00+00:00",
  });
  const states = () => [
    entity("sensor.casa_potencia", "412.5", { friendly_name: "Potencia de casa", unit_of_measurement: "W", device_class: "power", state_class: "measurement" }),
    entity("sensor.casa_energia", "1204.3", { friendly_name: "Energía de casa", unit_of_measurement: "kWh", device_class: "energy", state_class: "total_increasing" }),
    entity("light.salon", state.light, { friendly_name: "Luz del salón", supported_features: 44 }),
    entity("binary_sensor.puerta", state.door, { friendly_name: "Sensor de puerta", device_class: "door" }),
    entity("sensor.temperatura", "21.4", { friendly_name: "Temperatura", unit_of_measurement: "°C", device_class: "temperature" }),
    entity("person.privada", "home", { friendly_name: "Persona privada", latitude: 40.1, longitude: -3.2 }),
    entity("sensor.sin_nombre", "unknown", {}),
  ];
  const config = () => ({
    version: "2026.2.0-simulado", state: state.starting ? "NOT_RUNNING" : "RUNNING",
    safe_mode: false, recovery_mode: false, location_name: "Casa privada", latitude: 40.1, longitude: -3.2,
  });
  let server;
  return {
    state,
    url: `http://localhost:${port}`,
    start: () =>
      new Promise((resolve, reject) => {
        server = createServer((req, res) => {
          if (req.headers.authorization !== `Bearer ${HA_TOKEN}`) {
            res.writeHead(401).end(JSON.stringify({ message: "Invalid authentication" }));
            return;
          }
          const route = { "/api/config": config, "/api/states": states }[req.url];
          if (!route) {
            res.writeHead(404).end();
            return;
          }
          res.setHeader("Content-Type", "application/json");
          res.end(JSON.stringify(route()));
        });
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
