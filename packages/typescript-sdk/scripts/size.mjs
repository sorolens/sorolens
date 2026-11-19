import { build } from "esbuild";
import { gzipSync } from "node:zlib";
import { statSync } from "node:fs";

const result = await build({
  entryPoints: ["src/index.ts"],
  bundle: true,
  minify: true,
  format: "esm",
  platform: "neutral",
  target: "es2022",
  write: false,
});
const code = result.outputFiles[0].contents;
const gzip = gzipSync(code);

const treeShaken = await build({
  stdin: {
    contents: 'export { verifyWebhook } from "./src/webhooks.ts";',
    resolveDir: process.cwd(),
    loader: "ts",
  },
  bundle: true,
  minify: true,
  format: "esm",
  platform: "neutral",
  target: "es2022",
  write: false,
});
const shaken = treeShaken.outputFiles[0].contents;
const text = Buffer.from(shaken).toString("utf8");
const ok = !text.includes("SorolensClient") && !text.includes("listContracts");

console.log(`full bundle:  ${(code.length / 1024).toFixed(2)} kB min, ${(gzip.length / 1024).toFixed(2)} kB gzip`);
console.log(`verifyWebhook only: ${(shaken.length / 1024).toFixed(2)} kB min`);
console.log(`tree-shaking: ${ok ? "ok (client code dropped)" : "FAILED (client code retained)"}`);
console.log(`sanity: dist/esm/index.js is ${(statSync("dist/esm/index.js").size / 1024).toFixed(2)} kB on disk`);
if (!ok) process.exitCode = 1;
