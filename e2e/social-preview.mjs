// Genera docs/img/social-preview.png a partir de docs/img/social-preview.html.
import { chromium } from "playwright";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const img = join(dirname(fileURLToPath(import.meta.url)), "..", "docs", "img");
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1280, height: 640 } });
await page.goto(pathToFileURL(join(img, "social-preview.html")).href);
await page.evaluate(() => document.fonts.ready);
await page.screenshot({ path: join(img, "social-preview.png") });
await browser.close();
