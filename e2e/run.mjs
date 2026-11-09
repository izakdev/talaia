// Pruebas de extremo a extremo. Requiere el binario compilado: make build.
// Los identificadores E2E-xx están descritos en docs/proyecto/04-plan-de-pruebas.md.
import assert from "node:assert/strict";
import { execFile, spawn } from "node:child_process";
import { existsSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { promisify } from "node:util";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";
import { fakeAdGuard, PASSWORD, USER } from "./fake-adguard.mjs";
import { fakeHomeAssistant, HA_TOKEN } from "./fake-homeassistant.mjs";
import { fakeOPNsense, OPN_KEY, OPN_SECRET } from "./fake-opnsense.mjs";
import { fakePortainer, PORTAINER_TOKEN } from "./fake-portainer.mjs";
import { fakeWGDashboard, WGDASHBOARD_KEY } from "./fake-wgdashboard.mjs";
import { fakeProxmox, TOKEN } from "./fake-proxmox.mjs";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const screenshotsDir = process.argv.includes("--screenshots") ? join(root, "docs/usuario/img") : null;
const BASE = "http://127.0.0.1:18080";
const ADMIN = { user: "admin", password: "una-clave-bien-larga" };

const adguard = fakeAdGuard(13000);
const proxmox = fakeProxmox(13001);
const homeAssistant = fakeHomeAssistant(13002);
const opnsense = fakeOPNsense(13003);
const portainer = fakePortainer(13004);
const wgdashboard = fakeWGDashboard(13005);
const dataDir = mkdtempSync(join(tmpdir(), "talaia-e2e-"));
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitForServer(url = BASE) {
  for (let i = 0; i < 50; i++) {
    try {
      if ((await fetch(`${url}/healthz`)).ok) return;
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

const widget = (name) => page.getByTestId(`widget-${name}`);
const toEditor = async () => {
  await page.getByRole("link", { name: "Panel", exact: true }).click();
  await page.getByRole("heading", { name: "Panel", exact: true }).waitFor();
};
const saveEditor = async () => {
  await page.getByRole("button", { name: "Guardar panel" }).click();
  await page.getByRole("status").getByText("Sin cambios").waitFor();
};
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
    assert.equal(await page.locator(".catalog button").count(), 6);
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
  ["E2E-37", "La atalaya resume el peor estado y lleva al bloque", async () => {
    const sky = page.locator(".lookout-sky");
    await sky.getByText("El servicio está en orden").waitFor();
    assert.equal(await sky.getAttribute("data-watch"), "good");
    assert.equal(await page.locator(".service.tone-good").count(), 1);
    adguard.state.protection = false;
    await sky.getByText("Necesita atención:").waitFor();
    assert.equal(await sky.getAttribute("data-watch"), "warn");
    assert.equal(await page.locator(".service.tone-warn").count(), 1);
    adguard.state.protection = true;
    await adguard.stop();
    await page.locator('.lookout-sky[data-watch="bad"]').waitFor();
    assert.equal(await page.locator(".service.tone-bad").count(), 1);
    await sky.getByRole("link", { name: "AdGuard principal" }).click();
    await page.waitForFunction(() => document.activeElement?.id?.startsWith("servicio-"));
    assert.match(await page.evaluate(() => document.activeElement.innerText), /AdGuard principal/);
    await adguard.start();
    await sky.getByText("El servicio está en orden").waitFor();
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
    assert.equal(await list.locator("li").count(), 11);
    assert.equal(await page.getByLabel(/resumen de luces/).isChecked(), true);
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
    const state = block.locator(".service-state");
    await state.getByText("En marcha").waitFor();
    await block.getByText("412,5 W").waitFor();
    await block.getByText(/1\.?204,3 kWh/).waitFor();
    const resources = await block.locator(".ha-section", { hasText: "Recursos" }).innerText();
    assert.match(resources, /CPU\s+12 %/);
    assert.match(resources, /RAM\s+25 %/);
    assert.match(resources, /Disco\s+25 %/);
    assert.match(resources, /Equipo encendido hace 3 días/);
    const summary = await block.locator(".ha-section", { hasText: "Resumen" }).innerText();
    assert.match(summary, /Luces encendidas\s+1\/1/);
    assert.match(summary, /Interruptores activos\s+1\/1/);
    assert.match(summary, /Automatizaciones activas\s+1\/1/);
    assert.match(summary, /Actualizaciones pendientes\s+1/);
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
    await state.getByText("En marcha").waitFor();
    homeAssistant.state.starting = true;
    await state.getByText("Arrancando").waitFor();
    homeAssistant.state.starting = false;
    await state.getByText("En marcha").waitFor();
  }],
  ["E2E-22", "Solo las entidades elegidas salen por la API pública", async () => {
    let body = await publicDashboard();
    assert.match(body, /Potencia de casa/);
    assert.doesNotMatch(body, /Persona privada|person\.|Casa privada|latitude|Cerradura privada|lock\.|nombre-privado/);
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
  ["E2E-23", "Enlace a cada servicio desde su bloque", async () => {
    await toDashboard();
    const adguardLink = page.locator(".service", { hasText: "AdGuard principal" }).locator(".service-link");
    assert.equal(await adguardLink.getAttribute("href"), adguard.url);
    assert.equal(await adguardLink.getAttribute("target"), "_blank");
    assert.match(await adguardLink.getAttribute("rel"), /noopener/);
    await toAdmin();
    await row("Domótica").getByRole("button", { name: "Editar" }).click();
    const link = page.getByLabel(/Enlace para abrir/);
    await link.fill("javascript:alert(1)");
    await page.getByRole("button", { name: "Guardar" }).click();
    await page.getByRole("alert").getByText("El enlace debe empezar").waitFor();
    await link.fill("https://domotica.example/inicio");
    await page.getByRole("button", { name: "Guardar" }).click();
    await page.locator("dialog").waitFor({ state: "detached" });
    await toDashboard();
    await page
      .locator(".service", { hasText: "Domótica" })
      .locator('.service-link[href="https://domotica.example/inicio"]')
      .waitFor();
    assert.doesNotMatch(await publicDashboard(), /localhost:13002/);
    await toAdmin();
  }],
  ["E2E-24", "Editor: arrastrar, controles de teclado, descartar y guardar", async () => {
    await toEditor();
    await page.getByRole("status").getByText("Sin cambios").waitFor();
    assert.equal(await page.locator(".editor-widget").count(), 3);
    await shot(page, "15-editor-panel");
    await widget("Domótica").locator(".widget-handle").scrollIntoViewIfNeeded();
    const before = await widget("Domótica").boundingBox();
    const handle = await widget("Domótica").locator(".widget-handle").boundingBox();
    await page.mouse.move(handle.x + 20, handle.y + 10);
    await page.mouse.down();
    await page.mouse.move(handle.x + 200, handle.y + 12, { steps: 8 });
    await page.mouse.move(handle.x + 620, handle.y + 12, { steps: 12 });
    await page.mouse.up();
    await page.getByRole("status").getByText("Cambios sin guardar").waitFor();
    await page.waitForFunction(() => document.querySelector(".editor-widget.react-draggable-dragging") === null);
    const after = await widget("Domótica").boundingBox();
    assert.notEqual(Math.round(after.x), Math.round(before.x));
    await page.getByRole("button", { name: "Descartar" }).click();
    await page.getByRole("status").getByText("Sin cambios").waitFor();

    const adguardBox = await widget("AdGuard principal").boundingBox();
    await widget("AdGuard principal").getByRole("button", { name: "Más ancho" }).click();
    await page.getByRole("status").getByText("Cambios sin guardar").waitFor();
    // La rejilla anima el cambio de tamaño.
    await page.waitForFunction(
      (width) => document.querySelector('[data-testid="widget-AdGuard principal"]').getBoundingClientRect().width > width + 50,
      adguardBox.width,
    );
    await widget("AdGuard principal").getByRole("button", { name: "Después" }).click();
    await saveEditor();
    await toDashboard();
    await page.locator(".panel-grid .service", { hasText: "AdGuard principal" }).waitFor();
    const columns = await page.evaluate(() =>
      [...document.querySelectorAll(".panel-grid > div")].map((el) => el.style.gridColumn),
    );
    assert.ok(columns.includes("1 / span 8") || columns.some((c) => c.endsWith("span 8")), String(columns));
    await page.getByRole("link", { name: "Administración" }).click();
  }],
  ["E2E-25", "Varias páginas, servicios fuera del panel y aviso desde otra página", async () => {
    await toEditor();
    await page.getByRole("button", { name: "Añadir página" }).click();
    await page.getByLabel("Nombre de la página").fill("Red");
    await page.getByRole("button", { name: "AdGuard principal" }).click();
    assert.equal(await page.locator(".editor-widget").count(), 1);
    await page.getByRole("button", { name: "Inicio", exact: true }).click();
    await widget("Domótica").getByRole("button", { name: "Quitar" }).click();
    await widget("AdGuard principal").getByRole("button", { name: "Quitar" }).click();
    await shot(page, "16-editor-paginas");
    await saveEditor();
    let body = JSON.parse(await publicDashboard());
    assert.equal(body.pages.length, 2);
    assert.ok(!body.connections.some((c) => c.name === "Domótica"), "una conexión fuera del panel no debe publicarse");
    await toDashboard();
    const tabs = page.locator(".page-tabs");
    await tabs.getByRole("link", { name: "Inicio" }).waitFor();
    assert.equal(await page.locator(".panel-grid .service").count(), 1);
    await tabs.getByRole("link", { name: "Red" }).click();
    await page.locator(".panel-grid .service", { hasText: "AdGuard principal" }).waitFor();
    assert.match(page.url(), /\/p\//);
    await page.reload();
    await page.locator(".panel-grid .service", { hasText: "AdGuard principal" }).waitFor();
    assert.equal(await page.locator(".panel-grid .service", { hasText: "Clúster de casa" }).count(), 0);
    // Un servicio de otra página que falla se ve en la cabecera y su enlace lleva a esa página.
    await tabs.getByRole("link", { name: "Inicio" }).click();
    await adguard.stop();
    const sky = page.locator(".lookout-sky");
    await sky.getByRole("link", { name: "AdGuard principal" }).click();
    await page.locator(".panel-grid .service.tone-bad", { hasText: "AdGuard principal" }).waitFor();
    assert.match(page.url(), /\/p\//);
    await adguard.start();
    await page.locator('.lookout-sky[data-watch="good"]').waitFor();

    // Deja una única página con todo, para los escenarios siguientes.
    await page.getByRole("link", { name: "Administración" }).click();
    await row("Domótica").getByText("Funciona").waitFor();
    await toEditor();
    await page.getByRole("button", { name: "Red", exact: true }).click();
    await page.getByRole("button", { name: "Eliminar página" }).click();
    await page.getByRole("button", { name: "Eliminar página" }).click();
    await page.getByRole("button", { name: "AdGuard principal" }).click();
    await page.getByRole("button", { name: "Domótica" }).click();
    await saveEditor();
    body = JSON.parse(await publicDashboard());
    assert.equal(body.pages.length, 1);
    assert.equal(body.connections.length, 3);
  }],
  ["E2E-26", "Tema y color de acento", async () => {
    await page.getByRole("link", { name: "Ajustes" }).click();
    await page.getByRole("heading", { name: "Ajustes" }).waitFor();
    await page.getByRole("radio", { name: "Oscuro" }).check();
    await page.getByRole("radio", { name: "Turquesa" }).check();
    await page.getByRole("button", { name: "Guardar ajustes" }).click();
    await page.getByRole("status").getByText("Ajustes guardados").waitFor();
    await page.waitForFunction(() => document.documentElement.dataset.theme === "dark");
    const colors = await page.evaluate(() => ({
      accent: getComputedStyle(document.documentElement).getPropertyValue("--beacon").trim(),
      page: getComputedStyle(document.body).backgroundColor,
    }));
    assert.equal(colors.accent, "#2dd4bf");
    assert.equal(colors.page, "rgb(7, 43, 53)");
    await shot(page, "17-ajustes");
    await toDashboard();
    await page.getByText("Protección activa").waitFor();
    await shot(page, "18-tema-oscuro");
    await toAdmin();
    await page.getByRole("link", { name: "Ajustes" }).click();
    await page.getByRole("radio", { name: "Según el sistema" }).check();
    await page.getByRole("radio", { name: "Ámbar" }).check();
    await page.getByRole("button", { name: "Guardar ajustes" }).click();
    await page.getByRole("status").getByText("Ajustes guardados").waitFor();
  }],
  ["E2E-27", "Inicio de sesión opcional para ver el panel", async () => {
    const visitorContext = await browser.newContext({ locale: "es-ES" });
    const visitor = await visitorContext.newPage();
    await visitor.goto(BASE);
    await visitor.getByText("Protección activa").waitFor();

    await page.getByLabel(/Exigir inicio de sesión/).check();
    await page.getByRole("button", { name: "Guardar ajustes" }).click();
    await page.getByRole("status").getByText("Ajustes guardados").waitFor();

    // Quien ya tenía el panel abierto deja de verlo sin recargar.
    await visitor.getByRole("heading", { name: "Entra para ver el panel" }).waitFor();
    assert.equal((await fetch(`${BASE}/api/dashboard`)).status, 401);
    assert.equal((await fetch(`${BASE}/api/dashboard/events`)).status, 401);
    await visitor.getByLabel("Usuario").fill(ADMIN.user);
    await visitor.getByLabel("Contraseña").fill(ADMIN.password);
    await visitor.getByRole("button", { name: "Entrar" }).click();
    await visitor.getByText("Protección activa").waitFor();
    await visitorContext.close();

    await page.getByLabel(/Exigir inicio de sesión/).uncheck();
    await page.getByRole("button", { name: "Guardar ajustes" }).click();
    await page.getByRole("status").getByText("Ajustes guardados").waitFor();
    assert.equal((await fetch(`${BASE}/api/dashboard`)).status, 200);
  }],
  ["E2E-28", "Cambio y recuperación de contraseña", async () => {
    const form = page.locator("form", { hasText: "Contraseña actual" });
    const changed = "contrasena-cambiada-larga";
    await form.getByLabel("Contraseña actual").fill("no-es-la-actual-larga");
    await form.getByLabel(/^Contraseña nueva/).fill(changed);
    await form.getByLabel("Repite la contraseña nueva").fill("otra-distinta-larga");
    await form.getByRole("alert").getByText("no coinciden").waitFor();
    await form.getByLabel("Repite la contraseña nueva").fill(changed);
    await form.getByRole("button", { name: "Cambiar contraseña" }).click();
    await form.getByRole("alert").getByText("no es correcta").waitFor();
    await form.getByLabel("Contraseña actual").fill(ADMIN.password);
    await form.getByRole("button", { name: "Cambiar contraseña" }).click();
    await form.getByRole("status").getByText("Contraseña cambiada").waitFor();

    const { stdout } = await promisify(execFile)(join(root, "bin/talaia"), ["reset-password", "-data", dataDir]);
    const recovered = stdout.match(/Contraseña nueva: (\S+)/)[1];
    assert.match(stdout, /Usuario: admin/);

    // La recuperación revoca las sesiones abiertas.
    await page.goto(`${BASE}/admin`);
    await page.getByRole("heading", { name: "Entra en la administración" }).waitFor();
    await page.getByLabel("Usuario").fill(ADMIN.user);
    await page.getByLabel("Contraseña").fill(changed);
    await page.getByRole("button", { name: "Entrar" }).click();
    await page.getByRole("alert").getByText("Usuario o contraseña incorrectos").waitFor();
    await page.getByLabel("Contraseña").fill(recovered);
    await page.getByRole("button", { name: "Entrar" }).click();
    await page.getByRole("heading", { name: "Conexiones", exact: true }).waitFor();
    ADMIN.password = recovered;
  }],
  ["E2E-29", "Primer arranque protegido con un código de instalación", async () => {
    const dir = mkdtempSync(join(tmpdir(), "talaia-e2e-token-"));
    const tokenFile = join(dir, "setup.token");
    writeFileSync(tokenFile, "codigo-de-prueba\n", { mode: 0o600 });
    const url = "http://127.0.0.1:18081";
    const server = spawn(join(root, "bin/talaia"), ["-listen", "127.0.0.1:18081", "-data", join(dir, "data"), "-setup-token-file", tokenFile], { stdio: "ignore" });
    const installContext = await browser.newContext({ locale: "es-ES" });
    try {
      await waitForServer(url);
      const installer = await installContext.newPage();
      await installer.goto(url);
      await installer.getByLabel("Usuario").fill(ADMIN.user);
      await installer.getByLabel("Contraseña").fill("una-clave-bien-larga");
      await installer.getByLabel(/Código de instalación/).fill("incorrecto");
      await installer.getByRole("button", { name: "Crear administrador" }).click();
      await installer.getByRole("alert").getByText("no es correcto").waitFor();

      await installer.goto(`${url}/?setup=codigo-de-prueba`);
      assert.equal(await installer.getByLabel(/Código de instalación/).inputValue(), "codigo-de-prueba");
      assert.doesNotMatch(installer.url(), /setup=/);
      await installer.getByLabel("Usuario").fill(ADMIN.user);
      await installer.getByLabel("Contraseña").fill("una-clave-bien-larga");
      await installer.getByRole("button", { name: "Crear administrador" }).click();
      await installer.getByRole("heading", { name: "Aún no hay servicios" }).waitFor();
      assert.equal(existsSync(tokenFile), false);
    } finally {
      await installContext.close();
      server.kill("SIGTERM");
      rmSync(dir, { recursive: true, force: true });
    }
  }],
  ["E2E-30", "Alta de OPNsense y privilegios limitados", async () => {
    await page.goto(`${BASE}/admin/connections/new`);
    await page.getByRole("button", { name: /OPNsense/ }).click();
    await page.getByLabel("Nombre", { exact: true }).fill("Cortafuegos");
    await page.getByLabel("URL", { exact: true }).fill(opnsense.url);
    await page.getByLabel(/^Clave de API/).fill(`key=${OPN_KEY}`);
    await page.locator("dialog input[type=number]").fill("5");
    const secret = page.locator("dialog input[type=password]");
    await secret.fill("secreto-incorrecto");
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog p[role=status]").getByText("Credenciales rechazadas").waitFor();
    await secret.fill(`secret=${OPN_SECRET}`);
    opnsense.state.limited = true;
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog p[role=status]").getByText("Conexión correcta").waitFor();
    await page.getByText("Faltan algunos datos").waitFor();
    opnsense.state.limited = false;
    await page.getByRole("button", { name: "Guardar" }).click();
    await row("Cortafuegos").getByText("Funciona").waitFor();
  }],
  ["E2E-31", "OPNsense en el panel: recursos, gateways y tráfico WAN", async () => {
    await toDashboard();
    const block = page.locator(".service", { hasText: "Cortafuegos" });
    const state = block.locator(".service-state");
    await state.getByText("Gateways en línea").waitFor();
    const body = await block.innerText();
    assert.match(body, /CPU\s+10 %/);
    assert.match(body, /RAM\s+25 %/);
    assert.match(body, /Disco\s+25 %/);
    assert.match(body, /OPNsense 26\.7-simulado/);
    await block.locator(".ha-entities li", { hasText: "WAN_V4" }).getByText("En línea, 4,2 ms").waitFor();
    await block.locator(".ha-entities li", { hasText: "WAN_V6" }).getByText("Sin monitorizar").waitFor();
    await block.getByText(/Mbit\/s/).first().waitFor({ timeout: 20000 });
    await shot(block, "19-panel-opnsense");
    opnsense.state.gateway = "loss";
    await state.getByText("Gateway degradado").waitFor();
    opnsense.state.gateway = "down";
    await state.getByText("Gateway caído").waitFor();
    opnsense.state.gateway = "none";
    opnsense.state.monitored = false;
    await state.getByText("Gateways sin monitorizar").waitFor();
    opnsense.state.monitored = true;
    await state.getByText("Gateways en línea").waitFor();
    await toAdmin();
  }],
  ["E2E-32", "Alta de Portainer", async () => {
    await page.goto(`${BASE}/admin/connections/new`);
    await page.getByRole("button", { name: /Portainer/ }).click();
    await page.getByLabel("Nombre", { exact: true }).fill("Contenedores");
    await page.getByLabel("URL", { exact: true }).fill(portainer.url);
    await page.locator("dialog input[type=number]").fill("5");
    const secret = page.locator("dialog input[type=password]");
    await secret.fill("token-incorrecto");
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog p[role=status]").getByText("Credenciales rechazadas").waitFor();
    await secret.fill(PORTAINER_TOKEN);
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog p[role=status]").getByText("Conexión correcta").waitFor();
    await page.getByRole("button", { name: "Guardar" }).click();
    await row("Contenedores").getByText("Funciona").waitFor();
  }],
  ["E2E-33", "Portainer en el panel: entornos, contenedores y estados", async () => {
    await toDashboard();
    const block = page.locator(".service", { hasText: "Contenedores" });
    const state = block.locator(".service-state");
    await state.getByText("Entornos en línea").waitFor();
    const body = await block.innerText();
    assert.match(body, /Entornos\s+2\/2/);
    assert.match(body, /Contenedores en marcha\s+11\/12/);
    assert.match(body, /Stacks\s+7/);
    assert.match(body, /Portainer 2\.39-simulado/);
    await block.locator(".ha-entities li", { hasText: "local" }).getByText("En línea, 7/8 en marcha").waitFor();
    await block.getByText(/Recuento de Portainer hace 2 minutos/).waitFor();
    await shot(block, "20-panel-portainer");
    portainer.state.unhealthy = 2;
    await state.getByText("Contenedores con problemas").waitFor();
    await block.locator(".ha-entities li", { hasText: "local" }).getByText("2 con problemas", { exact: false }).waitFor();
    portainer.state.unhealthy = 0;
    portainer.state.edgeDown = true;
    await state.getByText("Entorno caído").waitFor();
    await block.locator(".ha-entities li", { hasText: "raspberry" }).getByText("Caído", { exact: true }).waitFor();
    portainer.state.edgeDown = false;
    portainer.state.empty = true;
    await state.getByText("Sin entornos Docker").waitFor();
    portainer.state.empty = false;
    await state.getByText("Entornos en línea").waitFor();
    await toAdmin();
  }],
  ["E2E-34", "Alta de WGDashboard", async () => {
    await page.goto(`${BASE}/admin/connections/new`);
    await page.getByRole("button", { name: /WGDashboard/ }).click();
    await page.getByLabel("Nombre", { exact: true }).fill("VPN");
    await page.getByLabel("URL", { exact: true }).fill(wgdashboard.url);
    await page.locator("dialog input[type=number]").fill("5");
    const secret = page.locator("dialog input[type=password]");
    await secret.fill("clave-incorrecta");
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog p[role=status]").getByText("Credenciales rechazadas").waitFor();
    await secret.fill(WGDASHBOARD_KEY);
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog p[role=status]").getByText("Conexión correcta").waitFor();
    await page.getByRole("button", { name: "Guardar" }).click();
    await row("VPN").getByText("Funciona").waitFor();
  }],
  ["E2E-35", "WGDashboard en el panel: túneles, peers y estados", async () => {
    await toDashboard();
    const block = page.locator(".service", { hasText: "VPN" });
    const state = block.locator(".service-state");
    await state.getByText("Túneles activos").waitFor();
    const body = await block.innerText();
    assert.match(body, /Túneles\s+2\/2/);
    assert.match(body, /Peers conectados\s+3\/4/);
    assert.match(body, /Tráfico\s+2,5 GiB/);
    assert.match(body, /Tráfico acumulado: 768 MiB recibidos y 1,8 GiB enviados/);
    assert.match(body, /WGDashboard v4\.3-simulado/);
    await block.locator(".ha-entities li", { hasText: "wg0" }).getByText("Activo, 2/3 peers conectados").waitFor();
    await shot(block, "21-panel-wgdashboard");
    wgdashboard.state.stopped = true;
    await state.getByText("Túnel parado").waitFor();
    await block.locator(".ha-entities li", { hasText: "wg1" }).getByText("Parado", { exact: true }).waitFor();
    assert.match(await block.innerText(), /Túneles\s+1\/2/);
    wgdashboard.state.stopped = false;
    wgdashboard.state.empty = true;
    await state.getByText("Sin túneles").waitFor();
    wgdashboard.state.empty = false;
    await state.getByText("Túneles activos").waitFor();
    await toAdmin();
  }],
  ["E2E-36", "WireGuard en el bloque de OPNsense, opcional y sin nombres de peers", async () => {
    assert.doesNotMatch(await publicDashboard(), /"wireguard"/);
    await row("Cortafuegos").getByRole("button", { name: "Editar" }).click();
    await page.getByLabel(/estado de WireGuard/).check();
    opnsense.state.wireguard = false;
    await page.getByRole("button", { name: "Probar conexión" }).click();
    await page.locator("dialog p[role=status]").getByText("Conexión correcta").waitFor();
    await page.getByText("Faltan algunos datos").waitFor();
    opnsense.state.wireguard = true;
    await page.getByRole("button", { name: "Guardar" }).click();
    await page.locator("dialog").waitFor({ state: "detached" });
    await toDashboard();
    const block = page.locator(".service", { hasText: "Cortafuegos" });
    const state = block.locator(".service-state");
    await block.locator(".ha-entities li", { hasText: "Acceso remoto" }).getByText("Activa, 1/2 peers conectados").waitFor();
    await block.getByText("Tráfico de WireGuard: 512 MiB recibidos y 1,5 GiB enviados").waitFor();
    await state.getByText("Gateways en línea").waitFor();
    await block.getByText(/Mbit\/s/).first().waitFor({ timeout: 20000 });
    await shot(block, "22-panel-opnsense-wireguard");
    const body = await publicDashboard();
    for (const leak of ["peer-simulado", "clave-publica", "198.51.100", "10.77.0", "51777"]) {
      assert.equal(body.includes(leak), false, leak);
    }
    opnsense.state.tunnelDown = true;
    await state.getByText("Túnel WireGuard caído").waitFor();
    await block.locator(".ha-entities li", { hasText: "Acceso remoto" }).getByText("Caída", { exact: true }).waitFor();
    opnsense.state.gateway = "down";
    await state.getByText("Gateway caído").waitFor();
    opnsense.state.gateway = "none";
    opnsense.state.tunnelDown = false;
    await state.getByText("Gateways en línea").waitFor();
    await toAdmin();
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
    for (const leak of [USER, PASSWORD, TOKEN, HA_TOKEN, OPN_KEY, OPN_SECRET, "localhost:13002", "192.0.2.11", "192.0.2.50", "cortafuegos.simulado", "proceso-simulado", "pppoe1", PORTAINER_TOKEN, "192.0.2.60", "docker.simulado", "instancia-simulada", WGDASHBOARD_KEY, "clave-privada-simulada", "clave-publica-simulada", "10.66.0", "51666", "wg-simulado", "vpn.simulado", "192.0.2.53", "base_url", "link_url", "secret"]) {
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
  await opnsense.start();
  await portainer.start();
  await wgdashboard.start();
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
  await opnsense.stop().catch(() => {});
  await portainer.stop().catch(() => {});
  await wgdashboard.stop().catch(() => {});
  rmSync(dataDir, { recursive: true, force: true });
}
if (failed) process.exit(1);
console.log(`${scenarios.length} escenarios correctos`);
