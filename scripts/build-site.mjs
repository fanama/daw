// Builds the browser app as static files for Freebuff Sites. The Go server in
// cmd/fanamix is only the local launcher and is not part of this output.
import { copyFile, mkdir, readdir, rm, stat } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const WEB = join(ROOT, "web");
const OUT = join(ROOT, "dist-site");
const MAX_ASSET_BYTES = 9_512_842;
const MAX_ASSETS = 1_000;

function fail(message) {
  console.error(`site build: ${message}`);
  process.exit(1);
}

function go(args, options = {}) {
  const result = spawnSync("go", args, { cwd: ROOT, stdio: "inherit", ...options });
  if (result.error) fail(`cannot run Go: ${result.error.message}`);
  if (result.status !== 0) process.exit(result.status ?? 1);
  return result;
}

await rm(OUT, { recursive: true, force: true });
await mkdir(OUT, { recursive: true });
go(["build", "-trimpath", "-ldflags=-s -w", "-o", join(OUT, "fanamix.wasm"), "./webapp"], {
  env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
});

const goroot = spawnSync("go", ["env", "GOROOT"], { cwd: ROOT, encoding: "utf8" });
if (goroot.error) fail(`cannot find GOROOT: ${goroot.error.message}`);
if (goroot.status !== 0) process.exit(goroot.status ?? 1);

await copyFile(join(goroot.stdout.trim(), "lib", "wasm", "wasm_exec.js"), join(OUT, "wasm_exec.js"));

let assets = 0;
for (const entry of await readdir(WEB, { withFileTypes: true })) {
  if (!entry.isFile() || entry.name === "fanamix.wasm" || entry.name === "wasm_exec.js") continue;
  await copyFile(join(WEB, entry.name), join(OUT, entry.name));
  assets += 1;
}

const wasm = await stat(join(OUT, "fanamix.wasm"));
if (wasm.size >= MAX_ASSET_BYTES) {
  fail(`fanamix.wasm is ${wasm.size} bytes; Freebuff Sites allows less than ${MAX_ASSET_BYTES}`);
}
assets += 2;
if (assets > MAX_ASSETS) fail(`${assets} files; Freebuff Sites allows at most ${MAX_ASSETS}`);

console.log(`site build: ${OUT} (${assets} files, wasm ${(wasm.size / 1024 / 1024).toFixed(1)} MiB)`);
