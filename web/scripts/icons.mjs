// Copia los iconos de simple-icons y los propios a dist/icons, con un índice para el selector.
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const web = join(dirname(fileURLToPath(import.meta.url)), "..");
const source = join(web, "node_modules", "simple-icons");
const out = join(web, "dist", "icons");

// Iconos que simple-icons no tiene: título y color de cada uno.
const own = {
  "talaia-nut": { title: "Network UPS Tools", hex: "16A34A" },
};

const titleTag = /<title>.*?<\/title>/s;
mkdirSync(out, { recursive: true });
const index = [];
for (const icon of JSON.parse(readFileSync(join(source, "data", "simple-icons.json"), "utf8"))) {
  const svg = readFileSync(join(source, "icons", `${icon.slug}.svg`), "utf8");
  writeFileSync(join(out, `${icon.slug}.svg`), svg.replace(titleTag, "").replace(' role="img"', ""));
  index.push({ slug: icon.slug, title: icon.title, hex: icon.hex });
}
for (const file of readdirSync(join(web, "icons"))) {
  const slug = file.replace(/\.svg$/, "");
  if (!own[slug]) throw new Error(`Falta el título y el color de ${file}`);
  writeFileSync(join(out, file), readFileSync(join(web, "icons", file)));
  index.push({ slug, ...own[slug] });
}
index.sort((a, b) => a.title.localeCompare(b.title, "en"));
writeFileSync(join(out, "index.json"), JSON.stringify(index));
console.log(`${index.length} iconos en ${out}`);
