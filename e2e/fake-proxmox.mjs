// Proxmox VE simulado, con la forma de respuesta de la API oficial.
import { createServer } from "node:http";

export const TOKEN = "talaia@pve!dashboard=00000000-prueba";
const GIB = 1024 ** 3;

const node = (name, cpu, maxcpu, mem, maxmem, disk, maxdisk) => ({
  id: `node/${name}`, type: "node", node: name, status: "online",
  cpu, maxcpu, mem: mem * GIB, maxmem: maxmem * GIB, disk: disk * GIB, maxdisk: maxdisk * GIB,
});
const guest = (type, vmid, name, nodeName, status, cpu, maxcpu, mem, maxmem) => ({
  id: `${type}/${vmid}`, type, vmid, name, node: nodeName, status, template: 0,
  cpu, maxcpu, mem: mem * GIB, maxmem: maxmem * GIB,
});

export function fakeProxmox(port) {
  const state = { nodeDown: false };
  const resources = () => [
    node("nodo1", 0.12, 8, 6.2, 16, 40, 240),
    node("nodo2", 0.34, 16, 28.9, 32, 120, 480),
    state.nodeDown
      ? { id: "node/nodo3", type: "node", node: "nodo3", status: "offline", maxcpu: 4, maxmem: 8 * GIB }
      : node("nodo3", 0.05, 4, 2.1, 8, 12, 120),
    { id: "storage/nodo1/local", type: "storage", node: "nodo1", status: "available", disk: GIB, maxdisk: 2 * GIB },
    guest("qemu", 100, "domotica", "nodo1", "running", 0.08, 2, 1.6, 4),
    guest("qemu", 101, "pruebas", "nodo1", "stopped", 0, 2, 0, 4),
    guest("lxc", 200, "mensajeria", "nodo1", "running", 0.01, 1, 0.1, 0.5),
    guest("qemu", 110, "contenedores", "nodo2", "running", 0.41, 8, 19.2, 24),
    guest("lxc", 210, "panel", "nodo2", "running", 0.02, 1, 0.2, 1),
    { ...guest("qemu", 900, "plantilla", "nodo2", "stopped", 0, 2, 0, 4), template: 1 },
    guest("lxc", 220, "vpn", "nodo3", state.nodeDown ? "unknown" : "running", 0, 1, 0.1, 0.5),
  ];
  const routes = {
    "/api2/json/version": () => ({ version: "9.0.0-simulado", release: "9.0" }),
    "/api2/json/cluster/status": () => [
      { type: "cluster", id: "cluster", name: "laboratorio", nodes: 3, quorate: 1, version: 3 },
      { type: "node", id: "node/nodo1", name: "nodo1", online: 1, local: 1, nodeid: 1, ip: "192.0.2.11", level: "" },
    ],
    "/api2/json/cluster/resources": resources,
  };
  let server;
  return {
    state,
    url: `http://localhost:${port}`,
    start: () =>
      new Promise((resolve, reject) => {
        server = createServer((req, res) => {
          if (req.headers.authorization !== `PVEAPIToken=${TOKEN}`) {
            res.writeHead(401).end();
            return;
          }
          const route = routes[req.url];
          if (!route) {
            res.writeHead(404).end();
            return;
          }
          res.setHeader("Content-Type", "application/json");
          res.end(JSON.stringify({ data: route() }));
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
