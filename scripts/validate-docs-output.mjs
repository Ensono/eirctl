#!/usr/bin/env node

import { readFile, stat, readdir } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const htmlSignature = /<!doctype\s+html\b|<html\b/i;
const pdfSignature = "%PDF-";

function fail(message) {
  throw new Error(message);
}

function parseArgs(args) {
  const values = {};
  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index];
    if (!argument.startsWith("--")) {
      fail(`Unexpected argument: ${argument}`);
    }
    const value = args[index + 1];
    if (!value || value.startsWith("--")) {
      fail(`Missing value for ${argument}`);
    }
    values[argument.slice(2)] = value;
    index += 1;
  }
  return values;
}

async function requireFile(file, label) {
  let fileStat;
  try {
    fileStat = await stat(file);
  } catch {
    fail(`Missing ${label}: ${file}`);
  }
  if (!fileStat.isFile() || fileStat.size === 0) {
    fail(`Empty or invalid ${label}: ${file}`);
  }
}

async function htmlFiles(directory) {
  const files = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const file = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...await htmlFiles(file));
    } else if (entry.isFile() && /\.html?$/i.test(entry.name)) {
      files.push(file);
    }
  }
  return files;
}

function localReferences(html) {
  const references = [];
  const attribute = /\b(?:href|src)\s*=\s*(["'])(.*?)\1/gi;
  for (const match of html.matchAll(attribute)) {
    const reference = match[2].trim();
    if (!reference || reference.startsWith("#") || /^[a-z][a-z0-9+.-]*:/i.test(reference) || reference.startsWith("//")) {
      continue;
    }
    references.push(reference);
  }
  return references;
}

async function validateLocalReferences(htmlRoot) {
  const root = path.resolve(htmlRoot);
  for (const document of await htmlFiles(root)) {
    const content = await readFile(document, "utf8");
    for (const reference of localReferences(content)) {
      const pathname = reference.split(/[?#]/, 1)[0];
      let decoded;
      try {
        decoded = decodeURIComponent(pathname);
      } catch {
        fail(`Invalid URL encoding in ${document}: ${reference}`);
      }
      const target = path.resolve(path.dirname(document), decoded);
      if (target !== root && !target.startsWith(`${root}${path.sep}`)) {
        fail(`Generated reference escapes HTML output tree in ${document}: ${reference}`);
      }
      try {
        await stat(target);
      } catch {
        fail(`Unresolved generated local reference in ${document}: ${reference}`);
      }
    }
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const manifestPath = path.resolve(args.manifest ?? "scripts/docs-output-manifest.json");
  const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  const repositoryRoot = path.dirname(path.dirname(manifestPath));
  const outputRoot = path.resolve(repositoryRoot, args["output-root"] ?? manifest.outputRoot);
  const htmlEntry = path.resolve(outputRoot, manifest.html.entry);
  const pdfEntry = path.resolve(outputRoot, manifest.pdf.entry);

  await requireFile(htmlEntry, "HTML entry");
  await requireFile(pdfEntry, "PDF entry");

  if (!htmlSignature.test(await readFile(htmlEntry, "utf8"))) {
    fail(`Invalid HTML signature: ${htmlEntry}`);
  }
  if (!(await readFile(pdfEntry)).subarray(0, pdfSignature.length).equals(Buffer.from(pdfSignature))) {
    fail(`Invalid PDF signature: ${pdfEntry}`);
  }

  await validateLocalReferences(path.dirname(htmlEntry));
  console.log(`Validated documentation output: ${htmlEntry} and ${pdfEntry}`);
}

await main().catch((error) => {
  console.error(`Documentation output validation failed: ${error.message}`);
  process.exitCode = 1;
});
