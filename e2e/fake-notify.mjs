// Receptor de avisos simulado: hace de API de bots de Telegram, de servidor ntfy
// (publicación en JSON) y de webhook. Guarda cada mensaje para las aserciones.
import { createServer } from "node:http";

export const TELEGRAM_TOKEN = "123456:token-de-bot-de-prueba";
// Un token con formato válido que el simulado rechaza, como Telegram con un token revocado.
export const BAD_TELEGRAM_TOKEN = "0:malo";
export const NTFY_TOPIC = "talaia-avisos-de-prueba";
export const NTFY_TOKEN = "tk_token_de_prueba";
export const WEBHOOK_PATH = "/hook/ruta-secreta-de-prueba";

export function fakeNotify(port) {
  const messages = [];
  const handler = (req, res) => {
    let raw = "";
    req.on("data", (chunk) => (raw += chunk));
    req.on("end", () => {
      if (req.method !== "POST") return res.writeHead(404).end();
      let body = null;
      try {
        body = JSON.parse(raw);
      } catch {
        return res.writeHead(400).end();
      }
      const bot = req.url.match(/^\/bot([^/]+)\/sendMessage$/);
      if (bot) {
        if (decodeURIComponent(bot[1]) !== TELEGRAM_TOKEN) {
          res.writeHead(401, { "Content-Type": "application/json" });
          return res.end(JSON.stringify({ ok: false, error_code: 401, description: "Unauthorized" }));
        }
        messages.push({ kind: "telegram", chatId: body.chat_id, text: body.text, body });
      } else if (req.url === "/") {
        messages.push({ kind: "ntfy", auth: req.headers.authorization ?? "", text: body.message, body });
      } else if (req.url === WEBHOOK_PATH) {
        messages.push({ kind: "webhook", text: body.text, body });
      } else {
        return res.writeHead(404).end();
      }
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ ok: true }));
    });
  };
  let server;
  return {
    messages,
    url: `http://localhost:${port}`,
    clear: () => messages.splice(0),
    // Espera a que llegue un mensaje de ese tipo cuyo texto cumpla la condición.
    waitFor: async (kind, match, timeout = 30000) => {
      const deadline = Date.now() + timeout;
      while (Date.now() < deadline) {
        const found = messages.find((m) => m.kind === kind && match(m.text ?? ""));
        if (found) return found;
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
      throw new Error(`no ha llegado ningún aviso ${kind}; recibidos: ${JSON.stringify(messages.map((m) => [m.kind, m.text]))}`);
    },
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
