import Alpine from "alpinejs";
import {
  buildEnvironmentDisplayTree,
  buildEnvironmentTree,
  cloneConfigValue,
  ConfigTypeMemory,
  deepMerge,
  formatEnvironmentTreeLabel,
} from "./config-value.js";
import "./main.css";

const API_ROOT = "/config-center/api";

class ConfigTreeEditor {
  constructor(root, value, onChange, onError, typeMemory) {
    this.root = root;
    this.value = value;
    this.onChange = onChange;
    this.onError = onError;
    this.typeMemory = typeMemory;
  }

  render(value) {
    this.value = value;
    this.root.replaceChildren();
    const rootNode = document.createElement("div");
    rootNode.className = "config-root";
    this.renderCollection(rootNode, [], value, true);
    this.root.append(rootNode);
  }

  renderCollection(container, path, value, isRoot = false) {
    const entries = Array.isArray(value) ? value.map((item, index) => [index, item]) : Object.entries(value);
    const collection = document.createElement("div");
    collection.className = isRoot ? "config-collection" : "config-collection config-collection-nested";
    collection.dataset.depth = String(path.length % 6);

    if (entries.length === 0) {
      const empty = document.createElement("div");
      empty.className = "config-empty";
      empty.textContent = Array.isArray(value) ? "数组为空" : "对象暂无字段";
      collection.append(empty);
    }

    entries.forEach(([key, child], index) => {
      collection.append(this.renderNode(path, key, child, Array.isArray(value), index, entries.length));
    });

    const addButton = this.button(Array.isArray(value) ? "＋ 添加数组项" : "＋ 添加字段", "config-add");
    addButton.addEventListener("click", () => {
      const target = this.at(path);
      if (Array.isArray(target)) {
        target.push("");
      } else {
        const key = this.nextKey(target);
        target[key] = "";
      }
      this.commit();
    });
    collection.append(addButton);
    container.append(collection);
  }

  renderNode(parentPath, key, value, parentIsArray, index, length) {
    const path = [...parentPath, key];
    const wrapper = document.createElement("div");
    wrapper.className = "config-node";
    const row = document.createElement("div");
    row.className = "config-row";

    if (parentIsArray) {
      const indexLabel = document.createElement("span");
      indexLabel.className = "config-index";
      indexLabel.textContent = String(Number(key) + 1);
      row.append(indexLabel);
    } else {
      const keyInput = document.createElement("input");
      keyInput.className = "config-key";
      keyInput.value = key;
      keyInput.maxLength = 128;
      keyInput.setAttribute("aria-label", "配置键名");
      keyInput.addEventListener("change", () => this.renameKey(parentPath, key, keyInput.value, keyInput));
      row.append(keyInput);
    }

    const typeOptions = [
      ["object", "对象"], ["array", "数组"], ["string", "字符串"],
      ["bool", "布尔值"], ["number", "数字"],
    ].map(([optionValue, label]) => ({ value: optionValue, label }));
    const typeDropdown = this.dropdown({
      value: valueType(value),
      options: typeOptions,
      className: "config-type-dropdown",
      ariaLabel: "配置值类型",
      onChange: (nextType) => {
        const previousType = valueType(this.at(path));
        if (previousType === nextType) return;
        this.typeMemory.remember(path, previousType, this.at(path));
        this.replace(path, this.typeMemory.restore(path, nextType, defaultValue(nextType)));
        this.commit();
      },
    });
    row.append(typeDropdown);

    row.append(this.renderValueInput(path, value));

    const actions = document.createElement("div");
    actions.className = "config-actions";
    if (parentIsArray) {
      const up = this.button("↑", "config-action", "上移");
      up.disabled = index === 0;
      up.addEventListener("click", () => this.moveArrayItem(parentPath, index, index - 1));
      const down = this.button("↓", "config-action", "下移");
      down.disabled = index === length - 1;
      down.addEventListener("click", () => this.moveArrayItem(parentPath, index, index + 1));
      actions.append(up, down);
    }
    const remove = this.button("×", "config-action config-remove", "删除");
    remove.addEventListener("click", () => {
      const parent = this.at(parentPath);
      if (Array.isArray(parent)) parent.splice(Number(key), 1);
      else delete parent[key];
      this.commit();
    });
    actions.append(remove);
    row.append(actions);
    wrapper.append(row);

    if (isCollection(value)) {
      const nested = document.createElement("div");
      nested.className = "config-nested";
      nested.dataset.depth = String(path.length % 6);
      this.renderCollection(nested, path, value);
      wrapper.append(nested);
    }
    return wrapper;
  }

  renderValueInput(path, value) {
    const type = valueType(value);
    if (type === "object" || type === "array") {
      const summary = document.createElement("span");
      summary.className = "config-summary";
      const size = type === "array" ? value.length : Object.keys(value).length;
      summary.textContent = `${size} ${type === "array" ? "项" : "个字段"}`;
      return summary;
    }
    if (type === "bool") {
      return this.dropdown({
        value: String(value),
        options: [
          { value: "true", label: "true" },
          { value: "false", label: "false" },
        ],
        className: "config-value-dropdown",
        ariaLabel: "布尔值",
        onChange: (nextValue) => {
          this.replace(path, nextValue === "true");
          this.commit();
        },
      });
    }
    const input = document.createElement(type === "string" && String(value).length > 80 ? "textarea" : "input");
    input.className = "config-value";
    if (type === "number") {
      input.type = "number";
      input.step = "any";
    } else {
      input.type = "text";
      input.maxLength = 65536;
    }
    input.value = String(value);
    input.addEventListener("input", () => {
      if (type === "string") {
        this.replace(path, input.value);
        this.onChange(cloneConfigValue(this.value), false);
      }
    });
    input.addEventListener("change", () => {
      if (type === "number") {
        const parsed = Number(input.value);
        if (!Number.isFinite(parsed)) {
          this.onError("请输入有效的数字");
          input.value = String(this.at(path));
          return;
        }
        if (Number.isInteger(parsed) && !Number.isSafeInteger(parsed)) {
          this.onError("整数部分超出了 JavaScript 可安全表示的范围");
          input.value = String(this.at(path));
          return;
        }
        this.replace(path, parsed);
      } else {
        this.replace(path, input.value);
      }
      this.commit(false);
    });
    return input;
  }

  renameKey(parentPath, oldKey, proposedKey, input) {
    const key = proposedKey.trim();
    const parent = this.at(parentPath);
    if (!key || key.length > 128) {
      this.onError("键名不能为空且不能超过 128 个字符");
      input.value = oldKey;
      return;
    }
    if (key !== oldKey && Object.hasOwn(parent, key)) {
      this.onError(`键名「${key}」已存在`);
      input.value = oldKey;
      return;
    }
    if (key === oldKey) return;
    const entries = Object.entries(parent).map(([entryKey, entryValue]) => entryKey === oldKey ? [key, entryValue] : [entryKey, entryValue]);
    Object.keys(parent).forEach((entryKey) => delete parent[entryKey]);
    entries.forEach(([entryKey, entryValue]) => { parent[entryKey] = entryValue; });
    this.commit();
  }

  moveArrayItem(parentPath, from, to) {
    const parent = this.at(parentPath);
    if (to < 0 || to >= parent.length) return;
    const [item] = parent.splice(from, 1);
    parent.splice(to, 0, item);
    this.commit();
  }

  at(path) {
    return path.reduce((current, segment) => current[segment], this.value);
  }

  replace(path, value) {
    const parent = this.at(path.slice(0, -1));
    parent[path.at(-1)] = value;
  }

  commit(render = true) {
    const snapshot = cloneConfigValue(this.value);
    this.value = snapshot;
    this.onChange(snapshot, render);
    if (render) this.render(snapshot);
  }

  nextKey(object) {
    let counter = 1;
    while (Object.hasOwn(object, `new_key_${counter}`)) counter += 1;
    return `new_key_${counter}`;
  }

  button(text, className, title = "") {
    const button = document.createElement("button");
    button.type = "button";
    button.className = className;
    button.textContent = text;
    if (title) button.title = title;
    return button;
  }

  dropdown({ value, options, className, ariaLabel, onChange }) {
    const wrapper = document.createElement("div");
    wrapper.className = `config-dropdown ${className}`;

    const trigger = this.button("", "config-dropdown-trigger");
    trigger.setAttribute("aria-label", ariaLabel);
    trigger.setAttribute("aria-haspopup", "listbox");
    trigger.setAttribute("aria-expanded", "false");
    const label = document.createElement("span");
    label.className = "config-dropdown-label";
    label.textContent = options.find((option) => option.value === value)?.label ?? value;
    const arrow = document.createElement("span");
    arrow.className = "dropdown-arrow";
    arrow.setAttribute("aria-hidden", "true");
    trigger.append(label, arrow);

    const menu = document.createElement("div");
    menu.className = "config-dropdown-menu";
    menu.setAttribute("role", "listbox");
    menu.hidden = true;

    const close = () => {
      menu.hidden = true;
      trigger.setAttribute("aria-expanded", "false");
      wrapper.classList.remove("config-dropdown-open");
    };
    const open = () => {
      menu.hidden = false;
      trigger.setAttribute("aria-expanded", "true");
      wrapper.classList.add("config-dropdown-open");
    };

    options.forEach((option) => {
      const item = this.button(option.label, "config-dropdown-option");
      item.setAttribute("role", "option");
      item.setAttribute("aria-selected", String(option.value === value));
      if (option.value === value) item.classList.add("config-dropdown-option-selected");
      item.addEventListener("click", () => {
        close();
        if (option.value !== value) onChange(option.value);
      });
      menu.append(item);
    });

    trigger.addEventListener("click", () => menu.hidden ? open() : close());
    wrapper.addEventListener("keydown", (event) => {
      if (event.key !== "Escape") return;
      close();
      trigger.focus();
    });
    wrapper.addEventListener("focusout", (event) => {
      if (!wrapper.contains(event.relatedTarget)) close();
    });
    wrapper.append(trigger, menu);
    return wrapper;
  }
}

function configCenterApp() {
  let treeEditor = null;
  const typeMemory = new ConfigTypeMemory();
  const toastTimers = new Set();

  return {
    environments: [], selectedID: 0, publishIDs: [], inheritanceChain: [],
    draft: {}, inherited: {}, finalConfig: {}, publishedRelease: null,
    loadingEnvironments: true, loadingConfig: false, saving: false, publishing: false,
    publishedLoading: false, dirty: false, tab: "draft",
    environmentModal: false, environmentSaving: false, parentSelectorOpen: false,
    sidebarHidden: false, confirming: false,
    toasts: [], toastSequence: 0,
    environmentForm: { id: 0, name: "", slug: "", description: "", parent_id: 0 },
    confirmation: {
      open: false, kind: "", title: "", message: "", confirmLabel: "确认", tone: "primary", targetID: 0, targets: [], urlMode: "push",
    },

    async init() {
      try {
        this.sidebarHidden = window.localStorage.getItem("config-center.sidebar-hidden") === "true";
      } catch {
        this.sidebarHidden = false;
      }
      this.$watch("tab", (tab) => {
        if (tab === "draft") this.$nextTick(() => this.renderEditor());
      });
      await this.loadEnvironments();
      if (this.environments.length > 0) {
        const requestedID = this.environmentIDFromURL();
        const initialID = this.environments.some((environment) => environment.id === requestedID)
          ? requestedID
          : this.environments[0].id;
        await this.selectEnvironment(initialID, true, "replace");
      }
    },
    destroy() {
      toastTimers.forEach((timer) => window.clearTimeout(timer));
      toastTimers.clear();
      treeEditor = null;
      typeMemory.clear();
    },
    handleBeforeUnload(event) {
      if (!this.dirty) return;
      event.preventDefault();
      event.returnValue = "";
    },
    async handlePopState() {
      const requestedID = this.environmentIDFromURL();
      if (!this.environments.some((environment) => environment.id === requestedID)) {
        if (this.selectedID) this.syncEnvironmentURL(this.selectedID, "replace");
        return;
      }
      await this.selectEnvironment(requestedID, false, "none");
    },
    setSidebarHidden(hidden) {
      this.sidebarHidden = hidden;
      try {
        window.localStorage.setItem("config-center.sidebar-hidden", String(hidden));
      } catch {
        // The layout still works when browser storage is unavailable.
      }
      this.$nextTick(() => this.renderEditor());
    },

    get selectedEnvironment() { return this.environments.find((item) => item.id === this.selectedID) ?? null; },
    get availableParents() { return this.environments.filter((item) => item.id !== this.environmentForm.id && !this.isDescendant(item.id, this.environmentForm.id)); },
    get availableParentRows() { return buildEnvironmentTree(this.availableParents); },
    get selectedParentLabel() {
      if (!this.environmentForm.parent_id) return "无（创建为根环境）";
      const environment = this.environments.find((item) => item.id === this.environmentForm.parent_id);
      return environment ? `${environment.name}（${environment.slug}）` : "请选择父环境";
    },
    get publishSelectionLabel() { return this.publishIDs.length ? `已选择 ${this.publishIDs.length} 个环境` : "选择环境后可批量发布"; },
    get formattedFinal() { return JSON.stringify(deepMerge(this.inherited, this.draft), null, 2); },
    get formattedPublished() { return this.publishedRelease ? JSON.stringify(this.publishedRelease.config, null, 2) : ""; },
    get environmentTreeRows() { return buildEnvironmentDisplayTree(this.environments); },

    async loadEnvironments() {
      this.loadingEnvironments = true;
      try {
        const data = await api("/environments");
        this.environments = data.environments;
        this.publishIDs = this.publishIDs.filter((id) => this.environments.some((item) => item.id === id));
      } catch (error) {
        this.notify(error.message, "error");
      } finally {
        this.loadingEnvironments = false;
      }
    },

    async selectEnvironment(id, force = false, urlMode = "push") {
      if (id === this.selectedID && !force) return;
      if (this.dirty && !force) {
        this.openConfirmation({
          kind: "discard",
          title: "放弃未保存的修改？",
          message: "切换环境会丢失当前编辑器中尚未保存的覆盖项。",
          confirmLabel: "放弃并切换",
          tone: "danger",
          targetID: id,
          urlMode,
        });
        return;
      }
      this.selectedID = id;
      this.tab = "draft";
      this.publishedRelease = null;
      const loaded = await this.loadDraft();
      if (loaded && urlMode !== "none") this.syncEnvironmentURL(id, urlMode);
    },

    async loadDraft() {
      this.loadingConfig = true;
      typeMemory.clear();
      let loaded = false;
      try {
        const data = await api(`/environments/${this.selectedID}/config`);
        this.draft = cloneConfigValue(data.draft);
        this.inherited = cloneConfigValue(data.inherited);
        this.finalConfig = cloneConfigValue(data.final);
        this.inheritanceChain = data.inheritance_chain;
        this.dirty = false;
        loaded = true;
      } catch (error) {
        this.notify(error.message, "error");
      } finally {
        this.loadingConfig = false;
      }
      if (loaded) {
        await this.$nextTick();
        this.renderEditor();
      }
      return loaded;
    },

    renderEditor() {
      if (!this.$refs.treeEditor || this.tab !== "draft") return;
      if (!treeEditor || treeEditor.root !== this.$refs.treeEditor) {
        treeEditor = new ConfigTreeEditor(
          this.$refs.treeEditor,
          this.draft,
          (value) => { this.draft = value; this.dirty = true; },
          (message) => this.notify(message, "error"),
          typeMemory,
        );
      }
      treeEditor.render(cloneConfigValue(this.draft));
    },

    async saveDraft() {
      if (!this.dirty || this.saving) return;
      this.saving = true;
      try {
        const data = await api(`/environments/${this.selectedID}/config`, { method: "PUT", body: { config: this.draft } });
        this.inherited = cloneConfigValue(data.inherited);
        this.finalConfig = cloneConfigValue(data.final);
        this.inheritanceChain = data.inheritance_chain;
        this.dirty = false;
        this.notify("草稿已保存");
      } catch (error) {
        this.notify(error.message, "error");
      } finally {
        this.saving = false;
      }
    },

    openEnvironmentModal(environment = null) {
      if (environment) {
        this.environmentForm = { id: environment.id, name: environment.name, slug: environment.slug, description: environment.description, parent_id: environment.parent_id };
      } else {
        this.environmentForm = { id: 0, name: "", slug: "", description: "", parent_id: 0 };
      }
      this.parentSelectorOpen = false;
      this.environmentModal = true;
    },
    closeEnvironmentModal() {
      if (this.environmentSaving) return;
      this.parentSelectorOpen = false;
      this.environmentModal = false;
    },
    selectParentEnvironment(id) {
      this.environmentForm.parent_id = id;
      this.parentSelectorOpen = false;
    },

    async saveEnvironment() {
      this.environmentSaving = true;
      const id = this.environmentForm.id;
      try {
        const body = { name: this.environmentForm.name, slug: this.environmentForm.slug, description: this.environmentForm.description, parent_id: Number(this.environmentForm.parent_id) || 0 };
        const data = await api(id ? `/environments/${id}` : "/environments", { method: id ? "PUT" : "POST", body });
        this.environmentModal = false;
        await this.loadEnvironments();
        await this.selectEnvironment(data.environment.id, true);
        this.notify(id ? "环境已更新" : "环境已创建");
      } catch (error) {
        this.notify(error.message, "error");
      } finally {
        this.environmentSaving = false;
      }
    },

    deleteSelectedEnvironment() {
      const environment = this.selectedEnvironment;
      if (!environment) return;
      this.openConfirmation({
        kind: "delete",
        title: `删除环境「${environment.name}」？`,
        message: "该环境及其全部发布记录将被永久删除。存在子环境继承时，服务器会拒绝本次操作。",
        confirmLabel: "确认删除",
        tone: "danger",
        targetID: environment.id,
        targets: [environment],
      });
    },

    async performDeleteEnvironment(environmentID) {
      try {
        await api(`/environments/${environmentID}`, { method: "DELETE" });
        this.selectedID = 0;
        await this.loadEnvironments();
        if (this.environments.length) await this.selectEnvironment(this.environments[0].id, true, "replace");
        else this.syncEnvironmentURL(0, "replace");
        this.notify("环境已删除");
      } catch (error) {
        this.notify(error.message, "error");
      }
    },

    publishSelected() {
      if (this.publishIDs.length === 0) return;
      if (this.dirty) {
        this.notify("请先保存当前环境草稿再发布", "error");
        return;
      }
      const targets = this.environments.filter((environment) => this.publishIDs.includes(environment.id));
      this.openConfirmation({
        kind: "publish",
        title: `发布 ${this.publishIDs.length} 个环境？`,
        message: "将为以下环境创建新的不可变完整配置快照。",
        confirmLabel: "确认发布",
        tone: "primary",
        targets,
      });
    },

    async performPublish() {
      this.publishing = true;
      try {
        const data = await api("/publish", { method: "POST", body: { environment_ids: this.publishIDs } });
        await this.loadEnvironments();
        if (this.tab === "published") await this.loadPublished();
        this.notify(`发布成功，批次 ${data.publication.batch_id}`);
      } catch (error) {
        this.notify(error.message, "error");
      } finally {
        this.publishing = false;
      }
    },

    openConfirmation({ kind, title, message, confirmLabel, tone, targetID = 0, targets = [], urlMode = "push" }) {
      this.confirmation = {
        open: true,
        kind,
        title,
        message,
        confirmLabel,
        tone,
        targetID,
        urlMode,
        targets: targets.map((environment) => ({
          id: environment.id,
          name: environment.name,
          slug: environment.slug,
        })),
      };
      this.$nextTick(() => this.$refs.confirmationButton?.focus());
    },
    closeConfirmation() {
      if (this.confirming) return;
      if (this.confirmation.kind === "discard" && this.confirmation.urlMode === "none" && this.selectedID) {
        this.syncEnvironmentURL(this.selectedID, "replace");
      }
      this.confirmation.open = false;
    },
    async confirmPendingAction() {
      if (this.confirming) return;
      const { kind, targetID, urlMode } = this.confirmation;
      this.confirming = true;
      try {
        if (kind === "delete") await this.performDeleteEnvironment(targetID);
        if (kind === "publish") await this.performPublish();
        if (kind === "discard") await this.selectEnvironment(targetID, true, urlMode);
        this.confirmation.open = false;
      } finally {
        this.confirming = false;
      }
    },

    async openPublishedTab() { this.tab = "published"; await this.loadPublished(); },
    async loadPublished() {
      this.publishedLoading = true;
      try {
        const data = await api(`/environments/${this.selectedID}/published`);
        this.publishedRelease = data.release;
      } catch (error) {
        if (error.status === 404) this.publishedRelease = null;
        else this.notify(error.message, "error");
      } finally {
        this.publishedLoading = false;
      }
    },

    environmentName(id) { return this.environments.find((item) => item.id === id)?.name ?? "未知环境"; },
    environmentTreeLabel(row) { return formatEnvironmentTreeLabel(row); },
    environmentIDFromURL() {
      const value = new URL(window.location.href).searchParams.get("environment");
      const id = Number(value);
      return Number.isSafeInteger(id) && id > 0 ? id : 0;
    },
    syncEnvironmentURL(id, mode = "push") {
      const url = new URL(window.location.href);
      if (id > 0) url.searchParams.set("environment", String(id));
      else url.searchParams.delete("environment");
      if (url.href === window.location.href) return;
      const state = id > 0 ? { environmentID: id } : {};
      if (mode === "replace") window.history.replaceState(state, "", url);
      else window.history.pushState(state, "", url);
    },
    isDescendant(candidateID, ancestorID) {
      if (!ancestorID) return false;
      let current = this.environments.find((item) => item.id === candidateID);
      while (current?.parent_id) {
        if (current.parent_id === ancestorID) return true;
        current = this.environments.find((item) => item.id === current.parent_id);
      }
      return false;
    },
    formatDate(value) { return value ? new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : ""; },
    async copyText(text) {
      try { await navigator.clipboard.writeText(text); this.notify("已复制到剪贴板"); }
      catch { this.notify("复制失败，请检查浏览器权限", "error"); }
    },
    notify(message, type = "success") {
      const id = ++this.toastSequence;
      this.toasts.push({ id, message, type });
      const timer = window.setTimeout(() => {
        this.toasts = this.toasts.filter((toast) => toast.id !== id);
        toastTimers.delete(timer);
      }, 3500);
      toastTimers.add(timer);
    },
  };
}

async function api(path, options = {}) {
  const response = await fetch(`${API_ROOT}${path}`, {
    method: options.method ?? "GET",
    headers: { Accept: "application/json", "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });
  let payload;
  try { payload = await response.json(); }
  catch { payload = { message: "服务返回了无法识别的响应" }; }
  if (!response.ok || payload.status !== 0) {
    const error = new Error(payload.message || `请求失败 (${response.status})`);
    error.status = response.status;
    throw error;
  }
  return payload.data;
}

function isCollection(value) { return Array.isArray(value) || (value !== null && typeof value === "object"); }
function valueType(value) {
  if (Array.isArray(value)) return "array";
  if (value !== null && typeof value === "object") return "object";
  if (typeof value === "boolean") return "bool";
  if (typeof value === "number") return "number";
  return "string";
}
function defaultValue(type) {
  return { object: {}, array: [], string: "", bool: false, number: 0 }[type];
}
window.configCenterApp = configCenterApp;
window.Alpine = Alpine;
Alpine.start();
