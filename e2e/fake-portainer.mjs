// Portainer simulado, con la forma de respuesta observada en un Portainer 2.39 real.
import { createServer } from "node:http";

export const PORTAINER_TOKEN = "token-de-portainer-de-prueba";

export function fakePortainer(port) {
  const state = { edgeDown: false, unhealthy: 0, empty: false };
  const endpoints = () =>
    state.empty
      ? []
      : [
          {
            Id: 3,
            Name: "local",
            Type: 1,
            Status: 1,
            URL: "unix:///var/run/docker.sock",
            PublicURL: "docker.simulado",
            EdgeKey: "",
            Snapshots: [
              {
                Time: Math.floor(Date.now() / 1000) - 120,
                DockerVersion: "29.9.2",
                ContainerCount: 8,
                RunningContainerCount: 7,
                StoppedContainerCount: 0,
                HealthyContainerCount: 1,
                UnhealthyContainerCount: state.unhealthy,
                StackCount: 6,
              },
            ],
          },
          {
            Id: 5,
            Name: "raspberry",
            Type: 2,
            Status: state.edgeDown ? 2 : 1,
            URL: "tcp://192.0.2.60:9001",
            Snapshots: [
              {
                Time: Math.floor(Date.now() / 1000) - 60,
                DockerVersion: "29.5.0",
                ContainerCount: 4,
                RunningContainerCount: 4,
                StoppedContainerCount: 0,
                HealthyContainerCount: 1,
                UnhealthyContainerCount: 0,
                StackCount: 1,
              },
            ],
          },
        ];
  const handler = (req, res) => {
    res.setHeader("Content-Type", "application/json");
    if (req.method === "GET" && req.url === "/api/status") {
      res.end(JSON.stringify({ Version: "2.39-simulado", InstanceID: "instancia-simulada" }));
    } else if (req.method === "GET" && req.url === "/api/endpoints") {
      if (req.headers["x-api-key"] !== PORTAINER_TOKEN) {
        res.writeHead(401).end(JSON.stringify({ message: "Invalid API key" }));
        return;
      }
      res.end(JSON.stringify(endpoints()));
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
