#!/usr/bin/env bun
/**
 * Generate fuse-baseline.json by running fuse.js with the EXACT same config as
 * src/tui/hooks/useSearch.ts against the fixture corpus + queries.
 */
import Fuse from "fuse.js";
import yaml from "yaml";
import { readFileSync, writeFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

type Entry = {
  id: string;
  ip: string;
  hostname: string;
  aliases: string[];
  enabled: boolean;
};
type Group = { name: string; entries: Entry[]; groups: Group[] };
type HostsFile = { version: number; groups: Group[] };

interface SearchableEntry {
  entry: Entry;
  groupPath: string[];
  groupPathString: string;
}

function flatten(groups: Group[], parent: string[] = []): SearchableEntry[] {
  const out: SearchableEntry[] = [];
  for (const g of groups) {
    const path = [...parent, g.name];
    for (const e of g.entries) {
      out.push({ entry: e, groupPath: path, groupPathString: path.join("/") });
    }
    out.push(...flatten(g.groups, path));
  }
  return out;
}

const here = dirname(fileURLToPath(import.meta.url));
const fixturePath = resolve(here, "../fixtures/corpus.yaml");
const queriesPath = resolve(here, "../fixtures/queries.json");
const outPath = resolve(here, "../fuse-baseline.json");

const hosts = yaml.parse(readFileSync(fixturePath, "utf8")) as HostsFile;
const queries = JSON.parse(readFileSync(queriesPath, "utf8")) as string[];

const items = flatten(hosts.groups);
const fuse = new Fuse(items, {
  keys: [
    { name: "entry.hostname", weight: 2 },
    { name: "entry.aliases", weight: 1.5 },
    { name: "entry.ip", weight: 1 },
    { name: "groupPathString", weight: 0.5 },
  ],
  threshold: 0.3,
  includeScore: true,
  minMatchCharLength: 2,
  ignoreLocation: true,
});

const baseline: Record<string, Array<{ id: string; hostname: string; score: number | undefined }>> = {};
for (const q of queries) {
  const r = fuse.search(q).slice(0, 5);
  baseline[q] = r.map(x => ({ id: x.item.entry.id, hostname: x.item.entry.hostname, score: x.score }));
}

writeFileSync(outPath, JSON.stringify(baseline, null, 2) + "\n");
console.log(`Wrote ${outPath}`);
for (const q of queries) {
  console.log(`\nQ=${q}`);
  for (const r of baseline[q]) console.log(`  ${r.id}  ${r.hostname}  score=${r.score?.toFixed(4)}`);
}
