// Impresora simulada por IPP, con la forma de respuesta observada en una Brother
// DCP-L3550CDW real: Get-Printer-Attributes en /ipp/print, sin credenciales.
// Nombre, ubicación, UUID y número de serie son inventados.
import { createServer } from "node:http";

export const PRINTER_NAME = "impresora-simulada";
export const PRINTER_LOCATION = "Despacho simulado";
export const PRINTER_SERIAL = "SIM-SERIE-0002";

// Codificación de IPP (RFC 8010): etiqueta, nombre y valor con sus longitudes.
function attr(tag, name, value) {
  const n = Buffer.from(name);
  const v = Buffer.isBuffer(value) ? value : typeof value === "number" ? int(value) : Buffer.from(value);
  const head = Buffer.alloc(1 + 2);
  head.writeUInt8(tag, 0);
  head.writeUInt16BE(n.length, 1);
  const len = Buffer.alloc(2);
  len.writeUInt16BE(v.length);
  return Buffer.concat([head, n, len, v]);
}

function int(n) {
  const b = Buffer.alloc(4);
  b.writeInt32BE(n);
  return b;
}

// Un atributo con varios valores: el nombre solo va en el primero.
const many = (tag, name, values) => Buffer.concat(values.map((v, i) => attr(tag, i === 0 ? name : "", v)));

function textLang(lang, text) {
  const l = Buffer.from(lang);
  const t = Buffer.from(text);
  const b = Buffer.alloc(4 + l.length + t.length);
  b.writeUInt16BE(l.length, 0);
  l.copy(b, 2);
  b.writeUInt16BE(t.length, 2 + l.length);
  t.copy(b, 4 + l.length);
  return b;
}

export function fakePrinter(port) {
  // printerState: 3 en reposo, 4 imprimiendo, 5 detenida. levels: negro, cian, magenta y amarillo.
  const state = { printerState: 3, reasons: ["none"], levels: [90, 75, 60, 45], accepting: true, ippStatus: 0, requests: 0 };
  const reply = (requestId) => {
    const head = Buffer.alloc(8);
    head.writeUInt8(2, 0);
    head.writeUInt16BE(state.ippStatus, 2);
    head.writeUInt32BE(requestId, 4);
    return Buffer.concat([
      head,
      Buffer.from([0x01]),
      attr(0x47, "attributes-charset", "utf-8"),
      attr(0x48, "attributes-natural-language", "en"),
      Buffer.from([0x04]),
      attr(0x45, "printer-uri-supported", `ipp://192.0.2.40:${port}/ipp/print`),
      attr(0x42, "printer-name", PRINTER_NAME),
      attr(0x41, "printer-location", PRINTER_LOCATION),
      attr(0x35, "printer-info", textLang("es", "Impresora del despacho simulado")),
      attr(0x45, "printer-uuid", "urn:uuid:00000000-0000-4000-8000-000000000002"),
      attr(0x41, "printer-device-id", `MFG:Brother;CMD:PJL,PCL,URF;SN:${PRINTER_SERIAL};`),
      attr(0x41, "printer-make-and-model", "Brother DCP-L3550CDW series"),
      attr(0x23, "printer-state", state.printerState),
      many(0x44, "printer-state-reasons", state.reasons),
      attr(0x22, "printer-is-accepting-jobs", Buffer.from([state.accepting ? 1 : 0])),
      attr(0x35, "printer-alert-description", textLang("es", "Espera")),
      many(0x42, "marker-colors", ["#000000", "#00FFFF", "#FF00FF", "#FFFF00"]),
      many(0x21, "marker-high-levels", [100, 100, 100, 100]),
      many(0x21, "marker-levels", state.levels),
      many(0x21, "marker-low-levels", [10, 10, 10, 10]),
      many(0x42, "marker-names", ["BK", "C", "M", "Y"]),
      many(0x44, "marker-types", ["toner", "toner", "toner", "toner"]),
      many(0x42, "printer-firmware-name", ["MAIN", "SUB1"]),
      many(0x41, "printer-firmware-string-version", ["ZA2206202233:93D6", "1.59"]),
      Buffer.from([0x03]),
    ]);
  };
  let server;
  const sockets = new Set();
  return {
    state,
    url: `ipp://127.0.0.1:${port}`,
    host: `127.0.0.1:${port}`,
    start: () =>
      new Promise((resolve, reject) => {
        server = createServer((req, res) => {
          const chunks = [];
          req.on("data", (c) => chunks.push(c));
          req.on("end", () => {
            const body = Buffer.concat(chunks);
            // Solo Get-Printer-Attributes (0x000B) por POST con application/ipp.
            if (req.method !== "POST" || req.url !== "/ipp/print" || req.headers["content-type"] !== "application/ipp" || body.length < 8 || body.readUInt16BE(2) !== 0x000b) {
              res.writeHead(400, { "Content-Type": "text/plain" });
              res.end("solo Get-Printer-Attributes");
              return;
            }
            state.requests++;
            res.writeHead(200, { "Content-Type": "application/ipp" });
            res.end(reply(body.readUInt32BE(4)));
          });
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
