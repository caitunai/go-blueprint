export function cloneConfigValue(value) {
  if (Array.isArray(value)) {
    return value.map((item) => cloneConfigValue(item));
  }
  if (isPlainConfigObject(value)) {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [key, cloneConfigValue(item)]),
    );
  }
  return value;
}

export function deepMerge(base, override) {
  if (!isPlainConfigObject(base) || !isPlainConfigObject(override)) {
    return cloneConfigValue(override);
  }

  const result = cloneConfigValue(base);
  Object.entries(override).forEach(([key, value]) => {
    result[key] = Object.hasOwn(result, key)
      ? deepMerge(result[key], value)
      : cloneConfigValue(value);
  });
  return result;
}

export function buildEnvironmentTree(environments) {
  const knownIDs = new Set(environments.map((environment) => environment.id));
  const children = new Map();
  environments.forEach((environment) => {
    const parentID = knownIDs.has(environment.parent_id) ? environment.parent_id : 0;
    const siblings = children.get(parentID) ?? [];
    siblings.push(environment);
    children.set(parentID, siblings);
  });
  children.forEach((siblings) => {
    siblings.sort((left, right) => left.name.localeCompare(right.name, "zh-CN"));
  });

  const rows = [];
  const visited = new Set();
  const appendChildren = (parentID, ancestorContinuations = []) => {
    const siblings = children.get(parentID) ?? [];
    siblings.forEach((environment, index) => {
      if (visited.has(environment.id)) return;
      visited.add(environment.id);
      const isLast = index === siblings.length - 1;
      rows.push({
        environment,
        depth: ancestorContinuations.length,
        ancestorContinuations,
        isLast,
        hasChildren: (children.get(environment.id)?.length ?? 0) > 0,
        hasPrevious: rows.length > 0,
      });
      appendChildren(environment.id, [...ancestorContinuations, !isLast]);
    });
  };

  appendChildren(0);
  return rows;
}

export function buildEnvironmentDisplayTree(environments) {
  if (environments.length === 0) return [];

  const environmentRows = buildEnvironmentTree(environments).map((row) => ({
    ...row,
    depth: row.depth + 1,
    ancestorContinuations: [false, ...row.ancestorContinuations],
    hasPrevious: true,
  }));

  return [
    {
      environment: {
        id: "config-root",
        name: "配置根环境",
        slug: "",
        parent_id: 0,
      },
      depth: 0,
      ancestorContinuations: [],
      isLast: true,
      hasChildren: true,
      hasPrevious: false,
      virtual: true,
    },
    ...environmentRows,
  ];
}

export function formatEnvironmentTreeLabel(row) {
  const ancestorLines = row.ancestorContinuations
    .slice(0, -1)
    .map((continues) => continues ? "│ " : "  ")
    .join("");
  const branch = row.depth === 0 ? "● " : row.isLast ? "└─● " : "├─● ";
  return `${ancestorLines}${branch}${row.environment.name}（${row.environment.slug}）`;
}

export class ConfigTypeMemory {
  constructor() {
    this.values = new Map();
  }

  clear() {
    this.values.clear();
  }

  remember(path, type, value) {
    const pathKey = JSON.stringify(path);
    const rememberedTypes = this.values.get(pathKey) ?? new Map();
    rememberedTypes.set(type, cloneConfigValue(value));
    this.values.set(pathKey, rememberedTypes);
  }

  restore(path, type, fallback) {
    const rememberedTypes = this.values.get(JSON.stringify(path));
    return rememberedTypes?.has(type)
      ? cloneConfigValue(rememberedTypes.get(type))
      : cloneConfigValue(fallback);
  }
}

function isPlainConfigObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
