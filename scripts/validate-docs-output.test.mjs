import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);
const script = path.join(path.dirname(fileURLToPath(import.meta.url)), "validate-docs-output.mjs");
const manifest = {
  outputRoot: ".eirctl/outputs/docs",
  html: { entry: "html/index.html" },
  pdf: { entry: "pdf/index.pdf" }
};

async function fixture() {
  const directory = await mkdtemp(path.join(os.tmpdir(), "eirctl-docs-validator-"));
  const output = path.join(directory, "output");
  const manifestPath = path.join(directory, "manifest.json");
  await writeFile(manifestPath, JSON.stringify(manifest));
  await mkdir(path.join(output, "html"), { recursive: true });
  await mkdir(path.join(output, "pdf"), { recursive: true });
  return { directory, output, manifestPath };
}

async function runValidator(manifestPath, output) {
  return execFileAsync(process.execPath, [script, "--manifest", manifestPath, "--output-root", output]);
}

async function withFixture(callback) {
  const value = await fixture();
  try {
    await callback(value);
  } finally {
    await rm(value.directory, { recursive: true, force: true });
  }
}

test("accepts complete valid output", async () => {
  await withFixture(async ({ manifestPath, output }) => {
    await writeFile(path.join(output, "html", "index.html"), "<!doctype html><html><img src=\"images/logo.svg\"></html>");
    await mkdir(path.join(output, "html", "images"));
    await writeFile(path.join(output, "html", "images", "logo.svg"), "<svg></svg>");
    await writeFile(path.join(output, "pdf", "index.pdf"), "%PDF-1.7\n");
    await assert.doesNotReject(runValidator(manifestPath, output));
  });
});

test("fails when a required output is missing", async () => {
  await withFixture(async ({ manifestPath, output }) => {
    await assert.rejects(runValidator(manifestPath, output), /Missing HTML entry/);
  });
});

test("fails when HTML or PDF signatures are invalid", async (t) => {
  await t.test("HTML", async () => {
    await withFixture(async ({ manifestPath, output }) => {
      await writeFile(path.join(output, "html", "index.html"), "not html");
      await writeFile(path.join(output, "pdf", "index.pdf"), "%PDF-1.7\n");
      await assert.rejects(runValidator(manifestPath, output), /Invalid HTML signature/);
    });
  });

  await t.test("PDF", async () => {
    await withFixture(async ({ manifestPath, output }) => {
      await writeFile(path.join(output, "html", "index.html"), "<!doctype html><html></html>");
      await writeFile(path.join(output, "pdf", "index.pdf"), "not a PDF");
      await assert.rejects(runValidator(manifestPath, output), /Invalid PDF signature/);
    });
  });
});

test("fails when generated HTML has an unresolved local reference", async () => {
  await withFixture(async ({ manifestPath, output }) => {
    await writeFile(path.join(output, "html", "index.html"), "<!doctype html><html><img src=\"images/missing.svg\"></html>");
    await writeFile(path.join(output, "pdf", "index.pdf"), "%PDF-1.7\n");
    await assert.rejects(runValidator(manifestPath, output), /Unresolved generated local reference/);
  });
});
