import assert from "node:assert/strict";
import test from "node:test";

import {
  buildEnvironmentTree,
  buildEnvironmentDisplayTree,
  cloneConfigValue,
  ConfigTypeMemory,
  deepMerge,
  formatEnvironmentTreeLabel,
} from "./config-value.js";

test("cloneConfigValue clones Alpine-style proxies", () => {
  const reactiveConfig = new Proxy({
    service: new Proxy({ host: "base", port: 8080 }, {}),
    regions: new Proxy(["cn", "us"], {}),
    enabled: true,
  }, {});

  const cloned = cloneConfigValue(reactiveConfig);

  assert.deepEqual(cloned, {
    service: { host: "base", port: 8080 },
    regions: ["cn", "us"],
    enabled: true,
  });
  assert.notEqual(cloned, reactiveConfig);
  assert.notEqual(cloned.service, reactiveConfig.service);
  assert.notEqual(cloned.regions, reactiveConfig.regions);
});

test("deepMerge accepts proxies and replaces arrays and scalar values", () => {
  const inherited = new Proxy({
    service: { host: "base", port: 8080 },
    regions: ["cn", "us"],
    enabled: true,
  }, {});
  const draft = new Proxy({
    service: new Proxy({ host: "production" }, {}),
    regions: new Proxy(["cn"], {}),
    enabled: false,
  }, {});

  assert.deepEqual(deepMerge(inherited, draft), {
    service: { host: "production", port: 8080 },
    regions: ["cn"],
    enabled: false,
  });
});

test("buildEnvironmentTree describes branch depth and continuation lines", () => {
  const rows = buildEnvironmentTree([
    { id: 4, parent_id: 1, name: "qa", slug: "qa" },
    { id: 3, parent_id: 2, name: "production", slug: "prod" },
    { id: 1, parent_id: 0, name: "base", slug: "base" },
    { id: 2, parent_id: 1, name: "development", slug: "dev" },
    { id: 5, parent_id: 0, name: "standalone", slug: "standalone" },
  ]);

  assert.deepEqual(
    rows.map((row) => ({
      id: row.environment.id,
      depth: row.depth,
      guides: row.ancestorContinuations,
      isLast: row.isLast,
      hasChildren: row.hasChildren,
      hasPrevious: row.hasPrevious,
    })),
    [
      { id: 1, depth: 0, guides: [], isLast: false, hasChildren: true, hasPrevious: false },
      { id: 2, depth: 1, guides: [true], isLast: false, hasChildren: true, hasPrevious: true },
      { id: 3, depth: 2, guides: [true, true], isLast: true, hasChildren: false, hasPrevious: true },
      { id: 4, depth: 1, guides: [true], isLast: true, hasChildren: false, hasPrevious: true },
      { id: 5, depth: 0, guides: [], isLast: true, hasChildren: false, hasPrevious: true },
    ],
  );

  assert.deepEqual(rows.map(formatEnvironmentTreeLabel), [
    "● base（base）",
    "├─● development（dev）",
    "│ └─● production（prod）",
    "└─● qa（qa）",
    "● standalone（standalone）",
  ]);
});

test("ConfigTypeMemory restores the last value used by each type", () => {
  const memory = new ConfigTypeMemory();
  const path = ["service", "limits"];
  const objectValue = { retries: 3, regions: ["cn", "us"] };

  memory.remember(path, "object", objectValue);
  memory.remember(path, "string", "temporary override");

  assert.deepEqual(memory.restore(path, "object", {}), objectValue);
  assert.equal(memory.restore(path, "string", ""), "temporary override");
  assert.equal(memory.restore(path, "bool", false), false);

  const restoredObject = memory.restore(path, "object", {});
  restoredObject.regions.push("eu");
  assert.deepEqual(memory.restore(path, "object", {}), objectValue);

  memory.clear();
  assert.deepEqual(memory.restore(path, "object", {}), {});
});

test("buildEnvironmentDisplayTree adds a read-only root above real environments", () => {
  const rows = buildEnvironmentDisplayTree([
    { id: 1, parent_id: 0, name: "base", slug: "base" },
    { id: 2, parent_id: 1, name: "development", slug: "dev" },
    { id: 3, parent_id: 0, name: "production", slug: "prod" },
  ]);

  assert.deepEqual(
    rows.map((row) => ({
      id: row.environment.id,
      depth: row.depth,
      guides: row.ancestorContinuations,
      virtual: row.virtual ?? false,
    })),
    [
      { id: "config-root", depth: 0, guides: [], virtual: true },
      { id: 1, depth: 1, guides: [false], virtual: false },
      { id: 2, depth: 2, guides: [false, true], virtual: false },
      { id: 3, depth: 1, guides: [false], virtual: false },
    ],
  );
});
