import { execFileSync } from "node:child_process";
import { mkdirSync, rmSync } from "node:fs";
import { build } from "esbuild";

rmSync("dist", { recursive: true, force: true });
mkdirSync("dist/esm", { recursive: true });
mkdirSync("dist/cjs", { recursive: true });

const common = {
  entryPoints: ["src/index.ts"],
  bundle: true,
  sourcemap: true,
  target: "es2022",
  legalComments: "none",
  // "neutral" keeps the bundle free of Node shims; the code only uses Web APIs.
  platform: "neutral",
  mainFields: ["module", "main"],
};

await build({ ...common, format: "esm", outfile: "dist/esm/index.js" });
await build({ ...common, format: "cjs", outfile: "dist/cjs/index.cjs" });

execFileSync(
  "node",
  [
    "node_modules/typescript/bin/tsc",
    "-p",
    "tsconfig.json",
    "--noEmit",
    "false",
    "--emitDeclarationOnly",
    "--declaration",
    "--outDir",
    "dist/types",
  ],
  { stdio: "inherit" },
);

console.log("built dist/esm/index.js, dist/cjs/index.cjs and dist/types");
