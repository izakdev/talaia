// Pruebas de extremo a extremo. Requiere el binario compilado: make build.
// Los identificadores E2E-xx están descritos en docs/proyecto/04-plan-de-pruebas.md.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";
import { fakeAdGuard, PASSWORD, USER } from "./fake-adguard.mjs";
import { fakeHomeAssistant, HA_TOKEN } from "./fake-homeassistant.mjs";
import { fakeProxmox, TOKEN } from "./fake-proxmox.mjs";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const screenshotsDir = process.argv.includes("--screenshots") ? join(root, "docs/usuario/img") : null;
const BASE = "http://127.0.0.1:18080";
const ADMIN = { user: "admin", password: "una-clave-bien-larga" };

const adguard = fakeAdGuard(13000);
const proxmox = fakeProxmox(13001);
const homeAssistant = fakeHomeAssistant(13002);
const dataDir = mkdtempSync(join(tmpdir(), "talaia-e2e-"));
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitForServer() {
  for (let i = 0; i < 50; i++) {
    try {
      if ((await fetch(`${BASE}/healthz`)).ok) return;
    } catch {
      await sleep(100);
    }
  }
  throw new Error("Talaia no ha arrancado");
}

async function shot(page, name) {
  if (!screenshotsDir) return;
  // Deja terminar la animación inicial del haz de luz.
  await sleep(1700);
  await page.screenshot({ path: join(screenshotsDir, `${name}.png`) });
}

const publicDashboard = async () => (await fetch(`${BASE}/api/dashboard`)).text();

const talaia = spawn(join(root, "bin/talaia"), ["-listen", "127.0.0.1:18080", "-data", dataDir], { stdio: "ignore" });
const browser = await chromium.launch();
const context = await browser.newContext({ locale: "es-ES", viewport: { width: 1280, height: 800 }, colorScheme: "light" });
const page = await context.newPage();
page.setDefaultTimeout(15000);
const consoleErrors = [];
page.on("pageerror", (error) => consoleErrors.push(error.message));
page.on("console", (message) => {
  // Los rechazos provocados a propósito se registran como recursos con error.
  if (message.type() === "error" && !message.text().includes("Failed to load resource")) consoleErrors.push(message.text());
});

const row = (name) => page.locator(".connections li", { hasText: name });
const openAddAdGuard = async () => {
  await page.getByRole("link", { name: "Añadir conexión" }).click();
  await page.getByRole("button", { name: /AdGuard/ }).click();
};
const fillAdGuard = async (password) => {
  await page.getByLabel("Nombre", { exact: true }).fill("AdGuard principal");
  await page.getByLabel("URL", { exact: true }).fill(adguard.url);
  await page.getByLabel("Usuario de AdGuard", { exact: true }).fill(USER);
  await page.locator("dialog input[type=password]").fill(password);
  await page.locator("dialog input[type=number]").fill("5");
};
const toDashboard = () => page.getByRole("link", { name: "Ver el panel" }).click();
const toAdmin = () => page.getByRole("link", { name: "Administración" }).click();

const scenarios = [
  ["E2E-01", "Primer arranque y creación del administrador", async () => {
    await page.goto(BASE);
    await page.getByRole("heading", { name: "Crea el administrador" }).waitFor();
    await shot(page, "01-primer-arranque");
    await page.getByLabel("Usuario").fill(ADMIN.user);
    await page.getByLabel("Contraseña").fill(ADMIN.password);
    await page.getByRole("button", { name: "Crear administrador" }).click();
    await page.getByRole("heading", { name: "Aún no hay servicios" }).waitFor();
    await shot(page, "02-panel-vacio");
    const state = await (await fetch(`${BASE}/api/state`)).json();
    assert.equal(state.setup_required, false);
  }],
  ["E2E-02", "Cierre de sesión y acceso protegido a la administración", async () => {
    await page.getByRole("link", { name: "Ir a la administración" }).click();
    await page.getByRole("button", { name: "Cerrar sesión" }).click();
    await page.getByRole("heading", { name: "Aún no hay servicios" }).waitFor();
    await page.goto(`${BASE}/admin`);
    await page.getByRole("heading", { name: "Entra en la administración" }).waitFor();
    assert.equal((await fetch(`${BASE}/api/admin/connections`)).status, 401);
  }],
  ["E2E-03", "Inicio de sesión incorrecto y correcto", async () => {
    await page.getByLabel("Usuario").fill(ADMIN.user);
    await page.getByLabel("Contraseña").fill("clave-incorrecta-larga");
    await page.getByRole("button", { name: "Entrar" }).click();
    await page.getByRole("alert").getByText("Usuario o contraseña incorrectos").waitFor();
    await shot(page, "03-inicio-de-sesion");
    await page.getByLabel("Contraseña").fill(ADMIN.password);
    await page.getByRole("button", { name: "Entrar" }).click();
    await page.getByRole("heading", { name: "Conexiones", exact: true }).waitFor();
  }],
  ["E2E-04", "Catálogo de servicios: búsqueda y navegación", async () => {
    await page.getByRole("link", { name: "Añadir conexión" }).click();
    await page.getByRole("heading", { name: "Elige un servicio" }).waitFor();
    assert.equal(await page.locator(".catalog button").count(), 3);
    await shot(page, "04-catalogo");
    const search = page.getByLabel("Buscar servicio");
    await search.fill("maquinas");
    assert.deepEqual(await page.locator(".catalog strong").allInnerTexts(), ["Proxmox"]);
    await search.fill("no-existe");
    await page.getByText("Ningún servicio coincide").waitFor();
    await search.fill("");
    await page.getByRole("button", { name: /AdGuard/ }).click();
    await page.getByRole("heading", { name: "Añadir AdGuard" }).waitFor();
    await page.getByRole("button", { name: "Cancelar" }).click();
    await page.getByRole("heading", { name: "Elige un servicio" }).waitFor();
    await page.goBack();
    await page.getByRole("heading", { name: "Conexiones", exact: true }).waitFor();
  }],
  ["E2E-05", "Probar conexión sin guardar", async () => {
    await openAddAdGuard();
    await fillAdGuard("contrasena-mala");
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog [role=status]").getByText("Credenciales rechazadas").waitFor();
    await page.locator("dialog input[type=password]").fill(PASSWORD);
    await page.locator("dialog [role=status]").waitFor({ state: "detached" });
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog [role=status]").getByText("Conexión correcta").waitFor();
    await shot(page, "05-formulario-adguard");
    await page.getByRole("button", { name: "Cancelar" }).click();
    await page.goBack();
    await page.getByRole("heading", { name: "Conexiones", exact: true }).waitFor();
    await page.getByText("Todavía no hay conexiones").waitFor();
  }],
  ["E2E-06", "Alta de AdGuard y datos en el panel", async () => {
    await openAddAdGuard();
    await fillAdGuard(PASSWORD);
    await page.getByRole("button", { name: "Guardar" }).click();
    await row("AdGuard principal").getByText("Funciona").waitFor();
    await toDashboard();
    await page.getByText("Protección activa").waitFor();
    const figures = await page.locator(".figures").innerText();
    assert.match(figures, /Consultas/);
    assert.match(figures, /16,6 %/);
    await shot(page, "07-panel");
  }],
  ["E2E-07", "Top de dominios opcional y excluido de la API pública", async () => {
    assert.equal(await page.locator(".tops").count(), 0);
    assert.doesNotMatch(await publicDashboard(), /\.example|top_/);
    await toAdmin();
    await row("AdGuard principal").getByRole("button", { name: "Editar" }).click();
    await page.getByLabel(/dominios más consultados/).check();
    await page.getByRole("button", { name: "Guardar" }).click();
    await page.locator("dialog").waitFor({ state: "detached" });
    await toDashboard();
    await page.getByText("anuncios.example").waitFor();
    assert.match(await publicDashboard(), /anuncios\.example/);
    await shot(page, "08-panel-con-dominios");
    await toAdmin();
    await row("AdGuard principal").getByRole("button", { name: "Editar" }).click();
    assert.equal(await page.getByLabel(/dominios más consultados/).isChecked(), true);
    await page.getByLabel(/dominios más consultados/).uncheck();
    await page.getByRole("button", { name: "Guardar" }).click();
    await page.locator("dialog").waitFor({ state: "detached" });
    assert.doesNotMatch(await publicDashboard(), /\.example/);
    await toDashboard();
    await page.getByText("Protección activa").waitFor();
  }],
  ["E2E-08", "Cambio de estado visible en vivo y sin recargar", async () => {
    adguard.state.protection = false;
    await page.getByText("Protección desactivada").waitFor();
    adguard.state.protection = true;
    await page.getByText("Protección activa").waitFor();
  }],
  ["E2E-09", "Caída y recuperación del servicio", async () => {
    await adguard.stop();
    await page.getByText("No se puede conectar con el servicio").waitFor();
    await page.getByText("Último dato").waitFor();
    assert.equal(await page.locator(".figures").count(), 1);
    await shot(page, "09-panel-con-fallos");
    await adguard.start();
    await page.getByText("Protección activa").waitFor();
  }],
  ["E2E-10", "Conexión guardada con un servicio inaccesible", async () => {
    await toAdmin();
    await page.getByRole("link", { name: "Añadir conexión" }).click();
    await page.getByRole("button", { name: /Home Assistant/ }).click();
    await page.getByLabel("Nombre", { exact: true }).fill("Casa");
    await page.getByLabel("URL", { exact: true }).fill("http://127.0.0.1:9");
    await page.locator("dialog input[type=password]").fill("token-de-larga-duracion");
    await page.getByRole("button", { name: "Guardar" }).click();
    await row("Casa").getByText("No se puede conectar").waitFor();
  }],
  ["E2E-11", "Editar y eliminar conexiones", async () => {
    await row("Casa").getByRole("button", { name: "Editar" }).click();
    const secret = page.locator("dialog input[type=password]");
    assert.equal(await secret.evaluate((input) => input.required), false);
    await page.getByLabel("URL", { exact: true }).fill("http://otro.example:8123");
    assert.equal(await secret.evaluate((input) => input.required), true);
    await page.getByRole("button", { name: "Cancelar" }).click();
    await row("Casa").getByRole("button", { name: "Eliminar" }).click();
    await row("Casa").getByText("¿Eliminar Casa?").waitFor();
    await row("Casa").getByRole("button", { name: "Cancelar" }).click();
    await row("Casa").getByRole("button", { name: "Eliminar" }).click();
    await row("Casa").getByRole("button", { name: "Eliminar" }).click();
    await row("Casa").waitFor({ state: "detached" });
  }],
  ["E2E-16", "Alta de Proxmox con validación del token", async () => {
    await page.getByRole("link", { name: "Añadir conexión" }).click();
    await page.getByRole("button", { name: /Proxmox/ }).click();
    await page.getByLabel("Nombre", { exact: true }).fill("Clúster de casa");
    await page.getByLabel("URL", { exact: true }).fill(proxmox.url);
    await page.locator("dialog input[type=number]").fill("5");
    assert.equal(await page.getByLabel(/detalle de cada VM y LXC/).isChecked(), true);
    const secret = page.locator("dialog input[type=password]");
    await secret.fill("sin-formato");
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.getByRole("alert").getByText("El token debe tener el formato").waitFor();
    await secret.fill("talaia@pve!dashboard=incorrecto");
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog [role=status]").getByText("Credenciales rechazadas").waitFor();
    await secret.fill(TOKEN);
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog [role=status]").getByText("Conexión correcta").waitFor();
    await page.getByRole("button", { name: "Guardar" }).click();
    await row("Clúster de casa").getByText("Funciona").waitFor();
    await shot(page, "06-conexiones");
  }],
  ["E2E-17", "Clúster en el panel: agregados, nodos y detalle", async () => {
    await toDashboard();
    const block = page.locator(".service", { hasText: "Clúster de casa" });
    await block.getByText("Clúster correcto").waitFor();
    const figures = await block.locator(".figures").innerText();
    assert.match(figures, /Nodos\s+3\/3/);
    assert.match(figures, /VM en marcha\s+2\/3/); // Sin contar la plantilla.
    assert.match(figures, /LXC en marcha\s+3\/3/);
    assert.equal(await block.locator(".nodes > li").count(), 3);
    assert.equal(await block.getByText("plantilla").count(), 0);
    await shot(page, "11-panel-proxmox");
    const details = block.locator("details", { hasText: "nodo1" });
    assert.equal(await details.evaluate((el) => el.open), false);
    await details.locator("summary").click();
    await details.getByRole("cell", { name: /domotica/ }).waitFor();
    await details.getByRole("cell", { name: "Parada" }).waitFor();
    await shot(page, "12-proxmox-detalle");
    // El detalle sigue abierto aunque lleguen datos nuevos.
    await sleep(6000);
    assert.equal(await details.evaluate((el) => el.open), true);
  }],
  ["E2E-18", "Nodo caído en el clúster", async () => {
    proxmox.state.nodeDown = true;
    const block = page.locator(".service", { hasText: "Clúster de casa" });
    await block.getByText("2 de 3 nodos en línea").waitFor();
    await block.locator(".nodes li", { hasText: "nodo3" }).getByText("Sin conexión").waitFor();
    proxmox.state.nodeDown = false;
    await block.getByText("Clúster correcto").waitFor();
  }],
  ["E2E-19", "Detalle de VM y LXC opcional y excluido de la API pública", async () => {
    assert.match(await publicDashboard(), /domotica/);
    await toAdmin();
    await row("Clúster de casa").getByRole("button", { name: "Editar" }).click();
    await page.getByLabel(/detalle de cada VM y LXC/).uncheck();
    await page.getByRole("button", { name: "Guardar" }).click();
    await page.locator("dialog").waitFor({ state: "detached" });
    const body = await publicDashboard();
    assert.doesNotMatch(body, /domotica|mensajeria|"guests"/);
    await toDashboard();
    const block = page.locator(".service", { hasText: "Clúster de casa" });
    await block.getByText("Clúster correcto").waitFor();
    assert.equal(await block.locator("details").count(), 0);
    assert.equal(await block.locator(".nodes > li").count(), 3);
    await toAdmin();
  }],
  ["E2E-20", "Alta de Home Assistant con selección de entidades", async () => {
    await page.getByRole("link", { name: "Añadir conexión" }).click();
    await page.getByRole("button", { name: /Home Assistant/ }).click();
    await page.getByLabel("Nombre", { exact: true }).fill("Domótica");
    await page.getByLabel("URL", { exact: true }).fill(homeAssistant.url);
    await page.locator("dialog input[type=number]").fill("5");
    const secret = page.locator("dialog input[type=password]");
    await secret.fill("token-incorrecto");
    await page.getByRole("button", { name: "Cargar entidades" }).click();
    await page.getByRole("alert").getByText("Credenciales rechazadas").waitFor();
    await secret.fill(HA_TOKEN);
    await page.getByRole("button", { name: "Cargar entidades" }).click();
    const list = page.locator(".picker-list");
    await list.getByText("Potencia de casa").waitFor();
    assert.equal(await list.locator("li").count(), 7);
    await page.getByRole("radio", { name: "Consumos" }).check();
    assert.equal(await list.locator("li").count(), 2);
    await list.getByLabel(/Potencia de casa/).check();
    await list.getByLabel(/Energía de casa/).check();
    await page.getByRole("radio", { name: "Todas" }).check();
    await page.getByLabel("Buscar entidad").fill("salon");
    assert.equal(await list.locator("li").count(), 1);
    await list.getByLabel(/Luz del salón/).check();
    await page.getByLabel("Buscar entidad").fill("");
    await list.getByLabel(/Sensor de puerta/).check();
    await page.getByLabel("Buscar entidad").fill("");
    await page.getByRole("status").getByText("4 seleccionadas").waitFor();
    await list.evaluate((el) => el.scrollIntoView({ block: "center" }));
    await shot(page, "13-selector-entidades");
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog p[role=status]").getByText("Conexión correcta").waitFor();
    await page.getByRole("button", { name: "Guardar" }).click();
    await row("Domótica").getByText("Funciona").waitFor();
  }],
  ["E2E-21", "Home Assistant en el panel: consumos, entidades y cambios en vivo", async () => {
    await toDashboard();
    const block = page.locator(".service", { hasText: "Domótica" });
    await block.getByText("En marcha").waitFor();
    assert.match(await block.locator(".figures").innerText(), /412,5 W/);
    assert.match(await block.locator(".figures").innerText(), /1\.?204,3 kWh/);
    const light = block.locator(".ha-entities li", { hasText: "Luz del salón" });
    await light.getByText("Encendido").waitFor();
    await shot(block, "14-panel-home-assistant");
    homeAssistant.state.light = "off";
    await light.getByText("Apagado").waitFor();
    homeAssistant.state.light = "on";
    homeAssistant.state.door = "unavailable";
    await block.getByText("Entidades sin datos: 1").waitFor();
    await block.locator(".ha-entities li", { hasText: "Sensor de puerta" }).getByText("No disponible").waitFor();
    homeAssistant.state.door = "off";
    await block.getByText("En marcha").waitFor();
    homeAssistant.state.starting = true;
    await block.getByText("Arrancando").waitFor();
    homeAssistant.state.starting = false;
    await block.getByText("En marcha").waitFor();
  }],
  ["E2E-22", "Solo las entidades elegidas salen por la API pública", async () => {
    let body = await publicDashboard();
    assert.match(body, /Potencia de casa/);
    assert.doesNotMatch(body, /Persona privada|person\.|Casa privada|latitude/);
    await toAdmin();
    await row("Domótica").getByRole("button", { name: "Editar" }).click();
    await page.getByRole("status").getByText("4 seleccionadas").waitFor();
    await page.getByRole("button", { name: "Cargar entidades" }).click();
    const list = page.locator(".picker-list");
    await page.getByRole("radio", { name: "Seleccionadas" }).check();
    assert.equal(await list.locator("li").count(), 4);
    await list.getByLabel(/Potencia de casa/).click();
    await page.getByRole("status").getByText("3 seleccionadas").waitFor();
    await page.getByRole("button", { name: "Guardar" }).click();
    await page.locator("dialog").waitFor({ state: "detached" });
    body = await publicDashboard();
    assert.doesNotMatch(body, /Potencia de casa/);
  }],
  ["E2E-12", "Cambio de idioma recordado", async () => {
    await page.getByRole("button", { name: "English" }).click();
    await page.getByRole("heading", { name: "Connections", exact: true }).waitFor();
    await page.reload();
    await page.getByRole("heading", { name: "Connections", exact: true }).waitFor();
    await page.getByRole("button", { name: "Español" }).click();
    await page.getByRole("heading", { name: "Conexiones", exact: true }).waitFor();
  }],
  ["E2E-13", "Móvil y modo oscuro sin desbordamiento", async () => {
    const mobile = await browser.newContext({
      locale: "es-ES",
      viewport: { width: 375, height: 800 },
      colorScheme: "dark",
      storageState: await context.storageState(),
    });
    const phone = await mobile.newPage();
    for (const path of ["/", "/admin", "/admin/connections/new"]) {
      await phone.goto(BASE + path);
      await phone.locator("h1").first().waitFor({ state: "attached" });
      await phone.waitForLoadState("load");
      assert.equal(await phone.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, path);
    }
    await phone.goto(BASE);
    await phone.getByText("Protección activa").waitFor();
    await shot(phone, "10-movil-oscuro");
    await mobile.close();
  }],
  ["E2E-14", "La API pública no expone configuración ni credenciales", async () => {
    const body = await publicDashboard();
    for (const leak of [USER, PASSWORD, TOKEN, HA_TOKEN, "localhost:13000", "localhost:13001", "localhost:13002", "192.0.2.11", "base_url", "secret"]) {
      assert.equal(body.includes(leak), false, leak);
    }
    const headers = (await fetch(BASE)).headers;
    assert.match(headers.get("content-security-policy"), /default-src 'self'/);
    assert.equal(headers.get("x-content-type-options"), "nosniff");
  }],
  ["E2E-15", "Sin errores de JavaScript durante el recorrido", async () => {
    assert.deepEqual(consoleErrors, []);
  }],
];

let failed = 0;
try {
  await adguard.start();
  await proxmox.start();
  await homeAssistant.start();
  await waitForServer();
  for (const [id, title, run] of scenarios) {
    try {
      await run();
      console.log(`ok    ${id} ${title}`);
    } catch (error) {
      failed++;
      console.error(`FALLO ${id} ${title}\n${String(error).slice(0, 800)}`);
      break; // Cada escenario parte del estado que deja el anterior.
    }
  }
} finally {
  await browser.close();
  talaia.kill("SIGTERM");
  await adguard.stop().catch(() => {});
  await proxmox.stop().catch(() => {});
  await homeAssistant.stop().catch(() => {});
  rmSync(dataDir, { recursive: true, force: true });
}
if (failed) process.exit(1);
console.log(`${scenarios.length} escenarios correctos`);
