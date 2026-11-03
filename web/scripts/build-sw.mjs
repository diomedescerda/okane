import { createHash } from "node:crypto";
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const web = dirname(dirname(fileURLToPath(import.meta.url)));
const dist = join(web, "dist");
const files = [
  "index.html",
  "manifest.webmanifest",
  "favicon-32.png",
  "apple-touch-icon.png",
  ...["assets", "brand", "icons"].flatMap((dir) =>
    readdirSync(join(dist, dir))
      .filter((file) => /\.(js|css|webp|png)$/.test(file))
      .map((file) => `${dir}/${file}`),
  ),
];
const template = readFileSync(join(web, "scripts/service-worker.js"), "utf8");
const hash = createHash("sha256").update(template);
for (const file of files)
  hash.update(file).update(readFileSync(join(dist, file)));
const version = hash.digest("hex").slice(0, 16);
const urls = files.map((file) => (file === "index.html" ? "/" : `/${file}`));
writeFileSync(
  join(dist, "sw.js"),
  template
    .replace("__CACHE_VERSION__", version)
    .replace("__PRECACHE_URLS__", JSON.stringify(urls)),
);
console.log(`PWA shell: ${urls.length} files, version ${version}`);
