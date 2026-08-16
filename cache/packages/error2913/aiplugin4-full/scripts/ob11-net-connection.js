// ==UserScript==
// @name         ob11网络连接依赖
// @author       错误&白鱼
// @version      2.3.5
// @description  为插件提供统一的ob11/milky网络连接依赖管理，支持HTTP和WebSocket。\nOB11 HTTP/WS 与 Milky 分三组 JSON 配置，按协议规范分别探测，支持多账号。\nWebSocket会保持持久连接并接收事件推送。\n提供指令 .net 可以直接调用\n在其他插件中使用方法: globalThis.net.callApi(epId, method, data=null)\nepId为骰子账号QQ:12345，method为方法，如get_login_info，data为参数。\n方法可参见https://github.com/botuniverse/onebot-11/blob/master/api/public.md#%E5%85%AC%E5%BC%80-api
// @timestamp    1786636716
// @license      MIT
// @homepageURL  https://github.com/error2913/sealdice-plugin-ob11-net-connection/
// @updateUrl    https://raw.gitmirror.com/error2913/sealdice-plugin-ob11-net-connection/main/dist/ob11网络连接依赖.js
// @updateUrl    https://raw.githubusercontent.com/error2913/sealdice-plugin-ob11-net-connection/main/dist/ob11网络连接依赖.js
// ==/UserScript==

"use strict";
(() => {
  var __defProp = Object.defineProperty;
  var __defProps = Object.defineProperties;
  var __getOwnPropDescs = Object.getOwnPropertyDescriptors;
  var __getOwnPropSymbols = Object.getOwnPropertySymbols;
  var __hasOwnProp = Object.prototype.hasOwnProperty;
  var __propIsEnum = Object.prototype.propertyIsEnumerable;
  var __defNormalProp = (obj, key, value) => key in obj ? __defProp(obj, key, { enumerable: true, configurable: true, writable: true, value }) : obj[key] = value;
  var __spreadValues = (a, b) => {
    for (var prop in b || (b = {}))
      if (__hasOwnProp.call(b, prop))
        __defNormalProp(a, prop, b[prop]);
    if (__getOwnPropSymbols)
      for (var prop of __getOwnPropSymbols(b)) {
        if (__propIsEnum.call(b, prop))
          __defNormalProp(a, prop, b[prop]);
      }
    return a;
  };
  var __spreadProps = (a, b) => __defProps(a, __getOwnPropDescs(b));

  // src/version.ts
  var VERSION = "2.3.5";

  // src/meta.ts
  var NAME = "ob11网络连接依赖";
  var AUTHOR = "错误&白鱼";

  // src/logger.ts
  var Logger = class {
    constructor(name) {
      this.name = name;
    }
    handleLog(force = false, ...data) {
      const logLevel = ConfigManager.logLevel;
      if (logLevel === "永不" && !force) {
        return "";
      } else if (logLevel === "简短" || logLevel === "永不" && force) {
        const s = data.map((item) => `${item}`).join(" ");
        if (s.length > 1e3) {
          return s.substring(0, 500) + "\n...\n" + s.substring(s.length - 500);
        } else {
          return s;
        }
      } else if (logLevel === "详细") {
        return data.map((item) => `${item}`).join(" ");
      } else {
        return "";
      }
    }
    info(...data) {
      const s = this.handleLog(false, ...data);
      if (!s) {
        return;
      }
      console.log(`【${this.name}】: ${s}`);
    }
    warning(...data) {
      const s = this.handleLog(true, ...data);
      if (!s) {
        return;
      }
      console.warn(`【${this.name}】: ${s}`);
    }
    error(...data) {
      const s = this.handleLog(true, ...data);
      if (!s) {
        return;
      }
      console.error(`【${this.name}】: ${s}`);
    }
  };
  var logger = new Logger(NAME);

  // src/utils.ts
  function getErrorMessage(error) {
    return error instanceof Error ? error.message : String(error);
  }
  function hasOwn(obj, key) {
    return Object.prototype.hasOwnProperty.call(obj, key);
  }
  function parseKwargs(kwargs) {
    const data = {};
    for (const { name, value } of kwargs) {
      if (name === "__proto__" || name === "constructor" || name === "prototype") {
        logger.warning(`忽略不安全的参数名: ${name}`);
        continue;
      }
      let parsed = value;
      try {
        parsed = JSON.parse(`[${value}]`)[0];
      } catch (e) {
      }
      data[name] = parsed;
    }
    return data;
  }

  // src/config.ts
  var KEY_OB11_HTTP = "OB11 HTTP 配置";
  var KEY_OB11_WS = "OB11 WS 配置";
  var KEY_MILKY = "Milky 配置";
  var JSON_CONFIG_EXAMPLE = '{"url":"http://127.0.0.1:8091","token":""}';
  var DESC_OB11_HTTP = '模板配置每个元素（面板上每个文本框）为一个 JSON：{"url":"...","token":"..."}，url 填 OB11 HTTP 服务根地址（如 http://127.0.0.1:8091），token 无鉴权可缺省或留空；示例：' + JSON_CONFIG_EXAMPLE + "，可添加多个元素配置多个端点";
  var DESC_OB11_WS = '模板配置每个元素（面板上每个文本框）为一个 JSON：{"url":"...","token":"..."}，url 填 OB11 WebSocket 服务地址（如 ws://127.0.0.1:8081），token 无鉴权可缺省或留空；示例：{"url":"ws://127.0.0.1:8081","token":""}，可添加多个元素配置多个端点';
  var DESC_MILKY = '模板配置每个元素（面板上每个文本框）为一个 JSON：{"url":"...","token":"..."}，url 填 Milky 根地址（HTTP /api 与 WS /event 共用端口与 token，如 http://127.0.0.1:8091），token 无鉴权可缺省或留空；示例：' + JSON_CONFIG_EXAMPLE + "，可添加多个元素配置多个端点";
  var PLATFORM = "QQ";
  var _ConfigManager = class _ConfigManager {
    static registerConfig() {
      this.ext = _ConfigManager.getExt(NAME);
      seal.ext.registerTemplateConfig(
        this.ext,
        KEY_OB11_HTTP,
        [""],
        DESC_OB11_HTTP
      );
      seal.ext.registerTemplateConfig(
        this.ext,
        KEY_OB11_WS,
        [""],
        DESC_OB11_WS
      );
      seal.ext.registerTemplateConfig(
        this.ext,
        KEY_MILKY,
        [""],
        DESC_MILKY
      );
      seal.ext.registerOptionConfig(this.ext, "日志打印方式", "简短", ["永不", "简短", "详细"], "");
      seal.ext.registerOptionConfig(this.ext, "事件处理", "记录非消息", ["忽略", "记录非消息", "记录", "调试"], "设置对WebSocket事件的处理方式");
      seal.ext.registerOptionConfig(this.ext, "事件分发", "并行", ["并行", "串行"], "设置事件回调执行方式：并行同时触发，串行按事件到达顺序依次执行");
    }
    static getCache(key, getFunc) {
      var _a;
      const timestamp = Date.now();
      if (((_a = this.cache) == null ? void 0 : _a[key]) && timestamp - this.cache[key].timestamp < 3e3) {
        return this.cache[key].data;
      }
      const data = getFunc();
      this.cache[key] = {
        timestamp,
        data
      };
      return data;
    }
    /**
     * 解析一组端点配置：模板配置的每个元素（面板上每个文本框）是一个 JSON（{url, token}）。
     * 空元素跳过；非法 JSON 记警告并忽略该元素；token 缺省或为 null 时兜底为空字符串。
     */
    static parseEndpointConfigs(key) {
      const raw = this.getCache(key, () => seal.ext.getTemplateConfig(this.ext, key));
      const lines = Array.isArray(raw) ? raw : [];
      const result = [];
      for (let i = 0; i < lines.length; i++) {
        const text = typeof lines[i] === "string" ? lines[i].trim() : "";
        if (!text) {
          continue;
        }
        try {
          const parsed = JSON.parse(text);
          const items = Array.isArray(parsed) ? parsed : [parsed];
          for (const item of items) {
            if (!item || typeof item !== "object") {
              logger.warning(`配置 ${key} 中存在非对象项，已跳过: ${JSON.stringify(item)}`);
              continue;
            }
            const url = typeof item.url === "string" ? item.url.trim() : "";
            if (!url) {
              logger.warning(`配置 ${key} 中存在缺少 url 的项，已跳过: ${JSON.stringify(item)}`);
              continue;
            }
            result.push({
              url,
              token: typeof item.token === "string" ? item.token : item.token == null ? "" : String(item.token)
            });
          }
        } catch (error) {
          logger.warning(`配置 ${key} 第 ${i + 1} 个元素解析失败（${getErrorMessage(error)}），已忽略；配置内容: ${text}`);
        }
      }
      return result;
    }
    /** OB11 HTTP 组：模板每个元素一个 {url, token} */
    static get ob11Http() {
      return this.parseEndpointConfigs(KEY_OB11_HTTP);
    }
    /** OB11 WS 组：模板每个元素一个 {url, token} */
    static get ob11Ws() {
      return this.parseEndpointConfigs(KEY_OB11_WS);
    }
    /** Milky 组：模板每个元素一个 {url, token}，HTTP /api 与 WS /event 共用地址与 token */
    static get milky() {
      return this.parseEndpointConfigs(KEY_MILKY);
    }
    static get logLevel() {
      return this.getCache("logLevel", () => seal.ext.getOptionConfig(this.ext, "日志打印方式"));
    }
    static get eventLevel() {
      return this.getCache("eventLevel", () => seal.ext.getOptionConfig(this.ext, "事件处理"));
    }
    static get dispatchMode() {
      return this.getCache("dispatchMode", () => seal.ext.getOptionConfig(this.ext, "事件分发"));
    }
    static getExt(name) {
      if (name == NAME && _ConfigManager.ext) {
        return _ConfigManager.ext;
      }
      let ext = seal.ext.find(name);
      if (!ext) {
        ext = seal.ext.new(name, AUTHOR, VERSION);
        seal.ext.register(ext);
      }
      return ext;
    }
  };
  _ConfigManager.cache = {};
  var ConfigManager = _ConfigManager;

  // src/http.ts
  var HTTPManager = class {
    /**
     * 归一化 Milky HTTP 基地址：去掉末尾的 `/api`（Milky API 挂载点），
     * 使「填根地址」和「填带 /api 后缀的地址」都得到同一个基地址，
     * 避免后续拼出 `/api/api/...`。
     * @example
     *   normalizeMilkyBaseUrl('http://h:8080')       -> 'http://h:8080'
     *   normalizeMilkyBaseUrl('http://h:8080/api')   -> 'http://h:8080'
     *   normalizeMilkyBaseUrl('http://h:8080/api/')  -> 'http://h:8080'
     *   normalizeMilkyBaseUrl('http://h:8080/other') -> 'http://h:8080/other'
     */
    static normalizeMilkyBaseUrl(url) {
      const trimmed = url.trim().replace(/\/+$/, "");
      return trimmed.replace(/\/api$/i, "");
    }
    static async init() {
      if (this.initDone) {
        return;
      }
      if (this.initPromise) {
        return this.initPromise;
      }
      this.initPromise = this.doInit().finally(() => {
        this.initPromise = null;
      });
      return this.initPromise;
    }
    static async doInit() {
      this.urlMap = {};
      this.protocolMap = {};
      const ob11Endpoints = ConfigManager.ob11Http;
      for (const endpoint of ob11Endpoints) {
        const baseUrl = endpoint.url.trim().replace(/\/+$/, "");
        if (!baseUrl) {
          continue;
        }
        const fetchUrl = `${baseUrl}/get_login_info`;
        try {
          const data = await this.fetchData(fetchUrl, endpoint.token);
          const userId = data.user_id;
          if (userId === void 0 || userId === null) {
            throw new Error(`get_login_info 响应缺少 user_id（若该地址是 Milky 端点，请配置到「Milky 配置」）: ${JSON.stringify(data)}`);
          }
          const epId = `${PLATFORM}:${userId}`;
          const eps = seal.getEndPoints();
          let found = false;
          for (let j = 0; j < eps.length; j++) {
            if (eps[j].userId === epId) {
              this.urlMap[epId] = {
                url: baseUrl,
                token: endpoint.token
              };
              logger.info(`找到 ${epId} 地址(OB11 HTTP): ${baseUrl} `);
              found = true;
              break;
            }
          }
          if (!found) {
            logger.warning(`未找到对应的端点: ${epId}`);
          }
        } catch (error) {
          logger.error(`OB11 HTTP 探测失败: ${fetchUrl}, 错误: ${getErrorMessage(error)}`);
        }
      }
      const milkyEndpoints = ConfigManager.milky;
      for (const endpoint of milkyEndpoints) {
        const baseUrl = this.normalizeMilkyBaseUrl(endpoint.url);
        if (!baseUrl) {
          continue;
        }
        try {
          const data = await this.fetchMilkyApi(baseUrl, endpoint.token, "get_login_info", {});
          const userId = data.uin;
          if (userId === void 0 || userId === null) {
            throw new Error(`get_login_info 响应缺少 uin: ${JSON.stringify(data)}`);
          }
          const epId = `${PLATFORM}:${userId}`;
          const eps = seal.getEndPoints();
          let found = false;
          for (let j = 0; j < eps.length; j++) {
            if (eps[j].userId === epId) {
              this.urlMap[epId] = {
                url: baseUrl,
                token: endpoint.token
              };
              this.protocolMap[epId] = "milky";
              logger.info(`找到 ${epId} 地址(Milky HTTP): ${baseUrl} `);
              found = true;
              break;
            }
          }
          if (!found) {
            logger.warning(`未找到对应的端点: ${epId}`);
          }
        } catch (error) {
          logger.error(`Milky HTTP 探测失败: ${baseUrl}/api/get_login_info, 错误: ${getErrorMessage(error)}`);
        }
      }
      logger.info("HTTP 初始化完成，http urlMap: ", JSON.stringify(this.urlMap, null, 2));
      this.initDone = true;
    }
    static async fetchData(url, token = "", data = null) {
      var _a;
      const responseData = await this.postJson(url, token, data != null ? data : {});
      if (!responseData || typeof responseData !== "object") {
        throw new Error(`响应格式异常: ${url}，响应: ${JSON.stringify(responseData)}`);
      }
      const status = responseData.status;
      const retcode = responseData.retcode;
      const retcodeNum = typeof retcode === "number" ? retcode : typeof retcode === "string" && retcode.trim() !== "" ? Number(retcode) : NaN;
      const failed = status !== void 0 && status !== "ok" && status !== "async" || status === void 0 && !Number.isNaN(retcodeNum) && retcodeNum !== 0 && retcodeNum !== 1;
      if (failed) {
        const message = responseData.message || responseData.wording || `retcode=${retcode}`;
        throw new Error(`API调用失败: ${url} - ${message}`);
      }
      if (ConfigManager.logLevel === "详细") {
        logger.info(`获取数据成功: ${JSON.stringify((_a = responseData.data) != null ? _a : {}, null, 2)}`);
      } else {
        logger.info(`获取数据成功: ${url}`);
      }
      return hasOwn(responseData, "data") ? responseData.data : {};
    }
    /** 调用 Milky HTTP API：POST {base}/api/{api}，解析 {status, retcode, data} 信封 */
    static async fetchMilkyApi(baseUrl, token = "", api, data = {}) {
      var _a, _b;
      const url = `${this.normalizeMilkyBaseUrl(baseUrl)}/api/${api}`;
      const responseData = await this.postJson(url, token, data != null ? data : {});
      if (!responseData || typeof responseData !== "object" || !hasOwn(responseData, "status")) {
        throw new Error(`Milky 响应格式异常: ${api} (${url})，响应: ${JSON.stringify(responseData)}`);
      }
      if (responseData.status === "ok") {
        if (ConfigManager.logLevel === "详细") {
          logger.info(`Milky API 成功: ${url}，数据: ${JSON.stringify((_a = responseData.data) != null ? _a : {}, null, 2)}`);
        } else {
          logger.info(`Milky API 成功: ${url}`);
        }
        return (_b = responseData.data) != null ? _b : {};
      }
      const detail = responseData.message || "未知错误";
      const isDecodeError = /Buffer doesn't contain required number of bytes|protobuf.*(fail|解码)|decode.*fail/i.test(detail);
      if (isDecodeError) {
        if (api === "get_forwarded_messages") {
          throw new Error(
            `获取合并转发失败：Milky 协议端从 QQ 服务器收到空数据（forward_id 无效、已过期或协议端缓存未同步）。原始错误: ${detail}`
          );
        }
        throw new Error(
          `Milky API 调用失败(${api} @ ${url})：协议端内部解码错误——通常是对端返回了空数据，参数无效/已过期或协议端状态异常，请核对参数并确认协议端状态。原始错误: ${detail}`
        );
      }
      throw new Error(`Milky API 调用失败(retcode=${responseData.retcode}): ${api} @ ${url} - ${detail}`);
    }
    static async postJson(url, token = "", data = null) {
      const headers = {
        "Content-Type": "application/json"
      };
      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
      }
      const options = {
        method: "POST",
        headers,
        body: JSON.stringify(data != null ? data : {})
      };
      let timeoutId;
      const timeoutPromise = new Promise((_, reject) => {
        timeoutId = setTimeout(() => {
          reject(new Error(`请求超时(${this.timeoutMs}ms): ${url}`));
        }, this.timeoutMs);
      });
      try {
        const response = await Promise.race([
          fetch(url, options),
          timeoutPromise
        ]);
        const text = await response.text();
        if (!response.ok) {
          throw new Error(`请求失败! 状态码: ${response.status}
响应体: ${text}`);
        }
        if (!text) {
          logger.info(`响应体为空，但请求成功: ${url}`);
          return {};
        }
        try {
          return JSON.parse(text);
        } catch (e) {
          throw new Error(`解析响应体时出错:${e}
响应体:${text}`);
        }
      } catch (error) {
        logger.error(`获取数据失败: ${getErrorMessage(error)}`);
        throw error;
      } finally {
        if (timeoutId !== void 0) {
          clearTimeout(timeoutId);
        }
      }
    }
  };
  HTTPManager.urlMap = {};
  /** 端点对应的协议：缺省/OB11 时不存在该键，Milky 时值为 'milky' */
  HTTPManager.protocolMap = {};
  HTTPManager.initDone = false;
  HTTPManager.initPromise = null;
  /** HTTP 请求超时（毫秒），可配置 */
  HTTPManager.timeoutMs = 1e4;

  // src/milky.ts
  var MSG_SEQ_BASE = 4294967296;
  var STORAGE_KEY = `${NAME}:milkyConvIndex`;
  var FORWARD_STORAGE_KEY = `${NAME}:milkyForwardIds`;
  var ConversationIndexer = class {
    constructor() {
      this.convToIndex = /* @__PURE__ */ new Map();
      this.indexToConv = /* @__PURE__ */ new Map();
      /** message_id → 合并转发 forward_id（resId），仅记录本插件收到过的合并转发消息 */
      this.forwardByMsgId = /* @__PURE__ */ new Map();
      this.nextIndex = 0;
      this.loaded = false;
      this.FORWARD_MAX = 2e3;
    }
    reset() {
      this.convToIndex.clear();
      this.indexToConv.clear();
      this.forwardByMsgId.clear();
      this.nextIndex = 0;
      this.loaded = false;
    }
    key(scene, peerId) {
      return `${scene}|${peerId}`;
    }
    ensureLoaded() {
      var _a;
      if (this.loaded) {
        return;
      }
      this.loaded = true;
      const ext = ConfigManager.ext;
      try {
        const raw = ext ? ext.storageGet(STORAGE_KEY) : "";
        if (!raw) {
          return;
        }
        const data = JSON.parse(raw);
        const convs = Array.isArray(data == null ? void 0 : data.convs) ? data.convs : [];
        this.nextIndex = Number((_a = data == null ? void 0 : data.next) != null ? _a : convs.length);
        for (let i = 0; i < convs.length; i++) {
          const scene = convs[i][0];
          const peerId = Number(convs[i][1]);
          this.convToIndex.set(this.key(scene, peerId), i);
          this.indexToConv.set(i, { scene, peerId, seq: 0 });
        }
      } catch (e) {
        logger.warning(`加载 Milky 会话索引失败，已重置: ${e}`);
        this.convToIndex.clear();
        this.indexToConv.clear();
        this.nextIndex = 0;
      }
      try {
        const rawForward = ext ? ext.storageGet(FORWARD_STORAGE_KEY) : "";
        if (rawForward) {
          const forwardData = JSON.parse(rawForward);
          const list = Array.isArray(forwardData == null ? void 0 : forwardData.list) ? forwardData.list : [];
          for (const entry of list) {
            this.forwardByMsgId.set(Number(entry[0]), String(entry[1]));
          }
        }
      } catch (e) {
        logger.warning(`加载 Milky 转发 ID 映射失败，已忽略: ${e}`);
        this.forwardByMsgId.clear();
      }
    }
    save() {
      try {
        const ext = ConfigManager.ext;
        if (!ext) {
          return;
        }
        const list = [];
        for (const [key, index] of this.convToIndex) {
          const idx = key.indexOf("|");
          const scene = key.substring(0, idx);
          const peerId = Number(key.substring(idx + 1));
          list[index] = [scene, peerId];
        }
        ext.storageSet(STORAGE_KEY, JSON.stringify({ v: 1, next: this.nextIndex, convs: list }));
      } catch (e) {
        logger.warning(`保存 Milky 会话索引失败: ${e}`);
      }
    }
    /** 记录 message_id 对应的合并转发 forward_id（持久化，仅保存合并转发消息） */
    setForwardId(messageId, forwardId) {
      this.ensureLoaded();
      if (!Number.isSafeInteger(messageId) || messageId < 0 || !forwardId) {
        return;
      }
      const existed = this.forwardByMsgId.get(messageId);
      if (existed === forwardId) {
        return;
      }
      this.forwardByMsgId.set(messageId, String(forwardId));
      while (this.forwardByMsgId.size > this.FORWARD_MAX) {
        const oldest = this.forwardByMsgId.keys().next().value;
        if (oldest === void 0) {
          break;
        }
        this.forwardByMsgId.delete(oldest);
      }
      this.saveForwards();
    }
    /** 由 message_id 还原合并转发的 forward_id */
    getForwardId(messageId) {
      this.ensureLoaded();
      return this.forwardByMsgId.get(messageId);
    }
    saveForwards() {
      try {
        const ext = ConfigManager.ext;
        if (!ext) {
          return;
        }
        const list = [];
        for (const [id, forwardId] of this.forwardByMsgId) {
          list.push([id, forwardId]);
        }
        ext.storageSet(FORWARD_STORAGE_KEY, JSON.stringify({ v: 1, list }));
      } catch (e) {
        logger.warning(`保存 Milky 转发 ID 映射失败: ${e}`);
      }
    }
    ensureIndex(scene, peerId) {
      this.ensureLoaded();
      const key = this.key(scene, peerId);
      const existed = this.convToIndex.get(key);
      if (existed !== void 0) {
        return existed;
      }
      const index = this.nextIndex++;
      this.convToIndex.set(key, index);
      this.indexToConv.set(index, { scene, peerId, seq: 0 });
      this.save();
      return index;
    }
    /** 生成 OB11 message_id（安全整数范围内可逆编码） */
    getMessageId(scene, peerId, seq) {
      if (!Number.isInteger(seq) || seq < 0 || seq >= MSG_SEQ_BASE) {
        throw new Error(`message_seq 超出 2^32 编码范围: scene=${scene} peer=${peerId} seq=${seq}`);
      }
      const index = this.ensureIndex(scene, peerId);
      const id = index * MSG_SEQ_BASE + seq;
      if (!Number.isSafeInteger(id) || id < 0) {
        throw new Error(`message_id 超出安全整数范围: scene=${scene} peer=${peerId} seq=${seq}`);
      }
      return id;
    }
    /** 由 OB11 message_id 还原 Milky 消息引用 */
    getRef(id) {
      this.ensureLoaded();
      if (!Number.isSafeInteger(id) || id < 0) {
        return void 0;
      }
      const index = Math.floor(id / MSG_SEQ_BASE);
      const seq = id % MSG_SEQ_BASE;
      const conv = this.indexToConv.get(index);
      if (!conv) {
        return void 0;
      }
      return { scene: conv.scene, peerId: conv.peerId, seq };
    }
  };
  function requireParam(method, value, paramName) {
    if (value === void 0 || value === null || value === "") {
      throw new Error(`${method} 需要参数 ${paramName}`);
    }
    return value;
  }
  function validateFileUrl(method, url) {
    if (typeof url !== "string" || url.trim() === "") {
      throw new Error(`${method} 返回的下载 URL 为空，请检查 file_id 是否有效`);
    }
    const trimmed = url.trim();
    if (/^[a-z][a-z0-9+.-]*:\/\/\//i.test(trimmed)) {
      throw new Error(`${method} 返回了无效的下载 URL（${trimmed}），请检查 file_id 是否有效`);
    }
    try {
      const parsed = new URL(trimmed);
      if (!parsed.hostname || !/^[a-z0-9.-]+$/i.test(parsed.hostname)) {
        throw new Error("missing host");
      }
    } catch (e) {
      throw new Error(`${method} 返回了无效的下载 URL（${trimmed}），请检查 file_id 是否有效`);
    }
    return trimmed;
  }
  var RAW_PARAM_VALIDATORS = {
    get_group_file_download_url: (data) => {
      requireParam("get_group_file_download_url", data.group_id, "group_id");
      requireParam("get_group_file_download_url", data.file_id, "file_id");
      return data;
    },
    get_private_file_download_url: (data) => {
      var _a;
      requireParam("get_private_file_download_url", data.file_id, "file_id");
      return __spreadProps(__spreadValues({}, data), { file_hash: (_a = data.file_hash) != null ? _a : data.file_id });
    },
    get_group_files: (data) => {
      requireParam("get_group_files", data.group_id, "group_id");
      return data;
    }
  };
  function resolveForwardId(data) {
    var _a, _b;
    const raw = (_b = (_a = data == null ? void 0 : data.message_id) != null ? _a : data == null ? void 0 : data.id) != null ? _b : data == null ? void 0 : data.forward_id;
    if (raw === void 0 || raw === null || raw === "") {
      throw new Error("get_forward_msg 需要 message_id（或 forward_id）");
    }
    if (typeof raw !== "number" && !/^\d+$/.test(String(raw))) {
      return String(raw);
    }
    const id = Number(raw);
    const forwardId = MilkyAdapter.conversations.getForwardId(id);
    if (forwardId) {
      return forwardId;
    }
    const ref = MilkyAdapter.conversations.getRef(id);
    const reason = ref ? `该消息（scene=${ref.scene}, seq=${ref.seq}）不是合并转发` : "该消息未由本插件收到过，缺少 forward_id 记录";
    throw new Error(`无法解析 message_id=${raw} 对应的合并转发 forward_id：${reason}`);
  }
  var MILKY_EXTRA_APIS = /* @__PURE__ */ new Set([
    "get_impl_info",
    "get_user_profile",
    "get_friend_info",
    "get_peer_pins",
    "set_peer_pin",
    "set_avatar",
    "set_nickname",
    "set_bio",
    "get_custom_face_url_list",
    "send_friend_nudge",
    "send_group_nudge",
    "send_profile_like",
    "delete_friend",
    "get_friend_requests",
    "accept_friend_request",
    "reject_friend_request",
    "set_group_avatar",
    "set_group_member_card",
    "set_group_member_special_title",
    "set_group_member_admin",
    "set_group_member_mute",
    "set_group_whole_mute",
    "kick_group_member",
    "get_group_announcements",
    "send_group_announcement",
    "delete_group_announcement",
    "get_group_essence_messages",
    "set_group_essence_message",
    "quit_group",
    "send_group_message_reaction",
    "get_group_notifications",
    "accept_group_request",
    "reject_group_request",
    "accept_group_invitation",
    "reject_group_invitation",
    "upload_private_file",
    "upload_group_file",
    "get_private_file_download_url",
    "get_group_file_download_url",
    "get_group_files",
    "move_group_file",
    "rename_group_file",
    "delete_group_file",
    "persist_group_file",
    "create_group_folder",
    "rename_group_folder",
    "delete_group_folder",
    "mark_message_as_read",
    "get_history_messages",
    "get_resource_temp_url",
    "get_forwarded_messages"
  ]);
  var UNSUPPORTED_MILKY_API_HINTS = {
    send_group_sign: "Milky 协议暂无群打卡(send_group_sign) API",
    set_group_sign: "Milky 协议暂无群打卡(set_group_sign) API",
    get_group_shut_list: "Milky 协议暂无群禁言列表(get_group_shut_list) API",
    send_group_ai_record: "Milky 协议暂无 AI 声聊(send_group_ai_record) API",
    send_private_ai_record: "Milky 协议暂无 AI 声聊(send_private_ai_record) API",
    get_group_honor_info: "Milky 协议暂无群荣誉(get_group_honor_info) API"
  };
  var NAPCAT_UNSUPPORTED = /* @__PURE__ */ new Set([
    "get_rkey",
    "get_rkey_server",
    "nc_get_rkey",
    "nc_get_packet_status",
    "nc_get_user_status",
    "get_clientkey",
    "get_robot_uin_range",
    "set_online_status",
    "set_diy_online_status",
    "get_unidirectional_friend_list",
    "click_inline_keyboard_button",
    "send_group_ark_share",
    "send_ark_share",
    "forward_friend_single_msg",
    "forward_group_single_msg",
    "translate_en2zh",
    "fetch_ptt_text",
    "create_collection",
    "get_collection_list",
    "set_self_longnick",
    "get_recent_contact",
    "_mark_all_as_read",
    "get_profile_like",
    "fetch_custom_face",
    "fetch_custom_face_detail",
    "add_custom_face",
    "delete_custom_face",
    "set_custom_face_desc",
    "fetch_emoji_like",
    "get_emoji_likes",
    "set_input_status",
    "get_group_ignore_add_request",
    "get_group_ignored_notifies",
    "get_guild_list",
    "get_guild_service_profile",
    "send_packet",
    "get_mini_app_ark",
    "get_ai_record",
    "get_ai_characters",
    "create_flash_task",
    "send_flash_msg",
    "get_share_link",
    "download_fileset",
    "get_fileset_info",
    "get_flash_file_list",
    "get_flash_file_url",
    "get_fileset_id",
    "send_online_file",
    "send_online_folder",
    "get_online_file_msg",
    "receive_online_file",
    "refuse_online_file",
    "cancel_online_file",
    "get_group_signed_list",
    "send_qzone_msg",
    "delete_qzone_msg",
    "download_file",
    "check_url_safely",
    "get_file",
    "get_online_clients",
    "set_group_remark",
    "set_group_member_invite_policy",
    "set_group_member_permissions",
    "set_group_new_member_history_visibility",
    "get_doubt_friends_add_request",
    "set_doubt_friends_add_request",
    "get_group_file_system_info",
    "trans_group_file",
    "set_friend_remark",
    "get_group_at_all_remain",
    "ocr_image",
    "set_group_todo",
    "complete_group_todo",
    "cancel_group_todo",
    "set_group_kick_members",
    "set_group_robot_add_option",
    "set_group_add_option",
    "set_group_search",
    "del_group_album_media",
    "set_group_album_media_like",
    "cancel_group_album_media_like",
    "do_group_album_comment",
    "get_group_album_media_list",
    "upload_image_to_qun_album",
    "get_qun_album_list"
  ]);
  function mapFriendRequest(data, approve) {
    var _a;
    const flag = String((_a = data.flag) != null ? _a : "");
    const parts = flag.split("|");
    const uid = parts[0] || "";
    const isFiltered = parts[1] === "true";
    const cached = MilkyAdapter.requestCache[`friend:${uid}`];
    return {
      api: approve ? "accept_friend_request" : "reject_friend_request",
      params: {
        initiator_uid: uid || (cached == null ? void 0 : cached.initiator_uid) || "",
        is_filtered: isFiltered || (cached == null ? void 0 : cached.is_filtered) || false,
        reason: approve ? void 0 : data.reason
      }
    };
  }
  function mapGroupRequest(data, approve) {
    var _a;
    const flag = String((_a = data.flag) != null ? _a : "");
    const parts = flag.split("|");
    const reason = approve ? void 0 : data.reason;
    if (parts[0] === "invitation") {
      return {
        api: approve ? "accept_group_invitation" : "reject_group_invitation",
        params: {
          group_id: Number(data.group_id),
          invitation_seq: Number(parts[1])
        }
      };
    }
    return {
      api: approve ? "accept_group_request" : "reject_group_request",
      params: {
        notification_seq: Number(parts[0]),
        notification_type: parts[1] === "invited_join_request" ? "invited_join_request" : "join_request",
        group_id: Number(data.group_id),
        is_filtered: parts[2] === "true",
        reason
      }
    };
  }
  function mapMarkAsRead(data, requiredScene) {
    var _a, _b;
    const ref = MilkyAdapter.conversations.getRef(Number((_a = data.message_id) != null ? _a : data.msg_id));
    if (!ref) {
      throw new Error(`无法解析 message_id=${(_b = data.message_id) != null ? _b : data.msg_id} 对应的 Milky 消息（仅支持本插件收到/发送过的消息）`);
    }
    if (requiredScene === "group" && ref.scene !== "group") {
      throw new Error("该 message_id 不是群消息");
    }
    if (requiredScene === "friend" && ref.scene === "group") {
      throw new Error("该 message_id 不是私聊消息");
    }
    return {
      api: "mark_message_as_read",
      params: { message_scene: ref.scene, peer_id: ref.peerId, message_seq: ref.seq }
    };
  }
  var ob11ApiMap = {
    send_private_msg: (data) => ({
      api: "send_private_message",
      params: {
        user_id: data.user_id,
        message: MilkyAdapter.convertOutgoingMessage(data.message, { scene: "friend", peerId: data.user_id })
      }
    }),
    send_group_msg: (data) => ({
      api: "send_group_message",
      params: {
        group_id: data.group_id,
        message: MilkyAdapter.convertOutgoingMessage(data.message, { scene: "group", peerId: data.group_id })
      }
    }),
    send_private_message: (data) => ob11ApiMap.send_private_msg(data),
    send_group_message: (data) => ob11ApiMap.send_group_msg(data),
    send_msg: (data) => {
      var _a, _b;
      if (data.message_type === "group") {
        return ob11ApiMap.send_group_msg(data);
      }
      return ob11ApiMap.send_private_msg(__spreadProps(__spreadValues({}, data), {
        user_id: (_b = (_a = data.user_id) != null ? _a : data.target_id) != null ? _b : data.peer_id
      }));
    },
    delete_msg: (data) => {
      const ref = MilkyAdapter.conversations.getRef(Number(data.message_id));
      if (!ref) {
        throw new Error(`无法解析 message_id=${data.message_id} 对应的 Milky 消息（仅支持本插件收到/发送过的消息）`);
      }
      return {
        api: ref.scene === "group" ? "recall_group_message" : "recall_private_message",
        params: ref.scene === "group" ? { group_id: ref.peerId, message_seq: ref.seq } : { user_id: ref.peerId, message_seq: ref.seq }
      };
    },
    recall_private_message: (data) => {
      if (data.message_seq !== void 0) {
        return { api: "recall_private_message", params: { user_id: data.user_id, message_seq: data.message_seq } };
      }
      return ob11ApiMap.delete_msg({ message_id: data.message_id });
    },
    recall_group_message: (data) => {
      if (data.message_seq !== void 0) {
        return { api: "recall_group_message", params: { group_id: data.group_id, message_seq: data.message_seq } };
      }
      return ob11ApiMap.delete_msg({ message_id: data.message_id });
    },
    get_msg: (data) => {
      const ref = MilkyAdapter.conversations.getRef(Number(data.message_id));
      if (!ref) {
        throw new Error(`无法解析 message_id=${data.message_id} 对应的 Milky 消息（仅支持本插件收到/发送过的消息）`);
      }
      return {
        api: "get_message",
        params: { message_scene: ref.scene, peer_id: ref.peerId, message_seq: ref.seq }
      };
    },
    get_message: (data) => {
      if (data.message_scene !== void 0) {
        return { api: "get_message", params: data };
      }
      return ob11ApiMap.get_msg(data);
    },
    get_forward_msg: (data) => ({
      api: "get_forwarded_messages",
      params: { forward_id: resolveForwardId(data) }
    }),
    get_forwarded_messages: (data) => {
      if (data.forward_id !== void 0 && data.forward_id !== null && data.forward_id !== "") {
        return { api: "get_forwarded_messages", params: { forward_id: String(data.forward_id) } };
      }
      return ob11ApiMap.get_forward_msg(data);
    },
    send_group_forward_msg: (data) => {
      var _a;
      return {
        api: "send_group_message",
        params: {
          group_id: data.group_id,
          message: [
            {
              type: "forward",
              data: {
                messages: (Array.isArray(data.messages) ? data.messages : []).map(
                  (node) => MilkyAdapter.convertForwardNode(node, { scene: "group", peerId: data.group_id })
                ),
                title: data.title,
                preview: (_a = data.preview) != null ? _a : data.news,
                summary: data.summary,
                prompt: data.prompt
              }
            }
          ]
        }
      };
    },
    send_private_forward_msg: (data) => {
      var _a;
      return {
        api: "send_private_message",
        params: {
          user_id: data.user_id,
          message: [
            {
              type: "forward",
              data: {
                messages: (Array.isArray(data.messages) ? data.messages : []).map(
                  (node) => MilkyAdapter.convertForwardNode(node, { scene: "friend", peerId: data.user_id })
                ),
                title: data.title,
                preview: (_a = data.preview) != null ? _a : data.news,
                summary: data.summary,
                prompt: data.prompt
              }
            }
          ]
        }
      };
    },
    send_forward_msg: (data) => {
      if (data.group_id !== void 0) {
        return ob11ApiMap.send_group_forward_msg(data);
      }
      if (data.user_id !== void 0) {
        return ob11ApiMap.send_private_forward_msg(data);
      }
      throw new Error("send_forward_msg 需要 group_id 或 user_id");
    },
    set_essence_msg: (data) => {
      const ref = MilkyAdapter.conversations.getRef(Number(data.message_id));
      if (!ref) {
        throw new Error(`无法解析 message_id=${data.message_id} 对应的 Milky 消息（仅支持本插件收到/发送过的消息）`);
      }
      if (ref.scene !== "group") {
        throw new Error("set_essence_msg 仅支持群消息");
      }
      return {
        api: "set_group_essence_message",
        params: { group_id: ref.peerId, message_seq: ref.seq, is_set: true }
      };
    },
    delete_essence_msg: (data) => {
      const ref = MilkyAdapter.conversations.getRef(Number(data.message_id));
      if (!ref) {
        throw new Error(`无法解析 message_id=${data.message_id} 对应的 Milky 消息（仅支持本插件收到/发送过的消息）`);
      }
      if (ref.scene !== "group") {
        throw new Error("delete_essence_msg 仅支持群消息");
      }
      return {
        api: "set_group_essence_message",
        params: { group_id: ref.peerId, message_seq: ref.seq, is_set: false }
      };
    },
    get_essence_msg_list: (data) => {
      var _a, _b;
      return {
        api: "get_group_essence_messages",
        params: {
          group_id: data.group_id,
          page_index: 0,
          page_size: (_b = (_a = data.page_size) != null ? _a : data.limit) != null ? _b : 50
        }
      };
    },
    set_msg_emoji_like: (data) => {
      var _a, _b, _c;
      const ref = MilkyAdapter.conversations.getRef(Number(data.message_id));
      if (!ref) {
        throw new Error(`无法解析 message_id=${data.message_id} 对应的 Milky 消息（仅支持本插件收到/发送过的消息）`);
      }
      if (ref.scene !== "group") {
        throw new Error("set_msg_emoji_like 仅支持群消息");
      }
      const emojiId = String((_b = (_a = data.emoji_id) != null ? _a : data.reaction) != null ? _b : "");
      return {
        api: "send_group_message_reaction",
        params: {
          group_id: ref.peerId,
          message_seq: ref.seq,
          reaction: emojiId,
          reaction_type: /^\d+$/.test(emojiId) ? "face" : "emoji",
          is_add: (_c = data.is_add) != null ? _c : true
        }
      };
    },
    get_group_file_url: (data) => ({
      api: "get_group_file_download_url",
      params: {
        group_id: requireParam("get_group_file_url", data.group_id, "group_id"),
        file_id: requireParam("get_group_file_url", data.file_id, "file_id")
      }
    }),
    get_group_root_files: (data) => ({
      api: "get_group_files",
      params: { group_id: requireParam("get_group_root_files", data.group_id, "group_id") }
    }),
    get_group_files_by_folder: (data) => ({
      api: "get_group_files",
      params: {
        group_id: requireParam("get_group_files_by_folder", data.group_id, "group_id"),
        parent_folder_id: requireParam("get_group_files_by_folder", data.folder_id, "folder_id")
      }
    }),
    upload_group_file: (data) => {
      var _a, _b;
      return {
        api: "upload_group_file",
        params: {
          group_id: requireParam("upload_group_file", data.group_id, "group_id"),
          parent_folder_id: data.folder,
          file_uri: toMilkyUri(requireParam("upload_group_file", (_a = data.file) != null ? _a : data.url, "file/url")),
          file_name: requireParam("upload_group_file", (_b = data.name) != null ? _b : data.file_name, "name/file_name")
        }
      };
    },
    upload_private_file: (data) => {
      var _a, _b;
      return {
        api: "upload_private_file",
        params: {
          user_id: requireParam("upload_private_file", data.user_id, "user_id"),
          file_uri: toMilkyUri(requireParam("upload_private_file", (_a = data.file) != null ? _a : data.url, "file/url")),
          file_name: requireParam("upload_private_file", (_b = data.name) != null ? _b : data.file_name, "name/file_name")
        }
      };
    },
    group_poke: (data) => ({
      api: "send_group_nudge",
      params: { group_id: data.group_id, user_id: data.user_id }
    }),
    friend_poke: (data) => {
      var _a;
      return {
        api: "send_friend_nudge",
        params: { user_id: data.user_id, is_self: (_a = data.is_self) != null ? _a : false }
      };
    },
    delete_friend: (data) => ({
      api: "delete_friend",
      params: { user_id: data.user_id }
    }),
    mark_msg_as_read: (data) => mapMarkAsRead(data),
    mark_group_msg_as_read: (data) => mapMarkAsRead(data, "group"),
    mark_private_msg_as_read: (data) => mapMarkAsRead(data, "friend"),
    get_group_msg_history: (data) => {
      var _a;
      return {
        api: "get_history_messages",
        params: {
          message_scene: "group",
          peer_id: data.group_id,
          start_message_seq: data.message_seq,
          limit: (_a = data.count) != null ? _a : 20
        }
      };
    },
    get_friend_msg_history: (data) => {
      var _a;
      return {
        api: "get_history_messages",
        params: {
          message_scene: "friend",
          peer_id: data.user_id,
          start_message_seq: data.message_seq,
          limit: (_a = data.count) != null ? _a : 20
        }
      };
    },
    get_group_system_msg: (data) => ({
      api: "get_group_notifications",
      params: { group_id: data.group_id }
    }),
    set_group_portrait: (data) => {
      var _a;
      return {
        api: "set_group_avatar",
        params: {
          group_id: requireParam("set_group_portrait", data.group_id, "group_id"),
          image_uri: toMilkyUri(requireParam("set_group_portrait", (_a = data.file) != null ? _a : data.url, "file/url"))
        }
      };
    },
    set_qq_avatar: (data) => {
      var _a;
      return {
        api: "set_avatar",
        params: { image_uri: toMilkyUri(requireParam("set_qq_avatar", (_a = data.file) != null ? _a : data.url, "file/url")) }
      };
    },
    set_qq_profile: (data) => {
      var _a;
      if (data.nickname !== void 0) {
        return { api: "set_nickname", params: { nickname: data.nickname } };
      }
      if (data.personal_note !== void 0 || data.personal_profile !== void 0) {
        return { api: "set_bio", params: { bio: (_a = data.personal_note) != null ? _a : data.personal_profile } };
      }
      throw new Error("set_qq_profile 需要 nickname 或 personal_note/personal_profile");
    },
    _send_group_notice: (data) => ({
      api: "send_group_announcement",
      params: {
        group_id: requireParam("_send_group_notice", data.group_id, "group_id"),
        content: requireParam("_send_group_notice", data.content, "content"),
        image_uri: data.image ? toMilkyUri(String(data.image)) : void 0
      }
    }),
    _get_group_notice: (data) => ({
      api: "get_group_announcements",
      params: { group_id: requireParam("_get_group_notice", data.group_id, "group_id") }
    }),
    _del_group_notice: (data) => {
      var _a;
      return {
        api: "delete_group_announcement",
        params: {
          group_id: requireParam("_del_group_notice", data.group_id, "group_id"),
          announcement_id: requireParam("_del_group_notice", (_a = data.notice_id) != null ? _a : data.announcement_id, "notice_id/announcement_id")
        }
      };
    },
    create_group_file_folder: (data) => ({
      api: "create_group_folder",
      params: {
        group_id: requireParam("create_group_file_folder", data.group_id, "group_id"),
        folder_name: requireParam("create_group_file_folder", data.folder_name, "folder_name")
      }
    }),
    delete_group_file: (data) => ({
      api: "delete_group_file",
      params: {
        group_id: requireParam("delete_group_file", data.group_id, "group_id"),
        file_id: requireParam("delete_group_file", data.file_id, "file_id")
      }
    }),
    delete_group_folder: (data) => ({
      api: "delete_group_folder",
      params: {
        group_id: requireParam("delete_group_folder", data.group_id, "group_id"),
        folder_id: requireParam("delete_group_folder", data.folder_id, "folder_id")
      }
    }),
    move_group_file: (data) => {
      var _a, _b;
      return {
        api: "move_group_file",
        params: {
          group_id: requireParam("move_group_file", data.group_id, "group_id"),
          file_id: requireParam("move_group_file", data.file_id, "file_id"),
          parent_folder_id: (_a = data.parent_folder_id) != null ? _a : "",
          target_folder_id: (_b = data.target_folder_id) != null ? _b : data.folder_id
        }
      };
    },
    rename_group_file: (data) => {
      var _a, _b, _c;
      return {
        api: "rename_group_file",
        params: {
          group_id: requireParam("rename_group_file", data.group_id, "group_id"),
          file_id: requireParam("rename_group_file", data.file_id, "file_id"),
          parent_folder_id: (_a = data.parent_folder_id) != null ? _a : "",
          new_file_name: requireParam("rename_group_file", (_c = (_b = data.new_name) != null ? _b : data.name) != null ? _c : data.new_file_name, "new_name/name")
        }
      };
    },
    get_private_file_url: (data) => {
      var _a;
      return {
        api: "get_private_file_download_url",
        params: {
          user_id: requireParam("get_private_file_url", data.user_id, "user_id"),
          file_id: requireParam("get_private_file_url", data.file_id, "file_id"),
          // OB11 规格只给 file_id；Milky 的 get_private_file_download_url 需要
          // file_hash，且该 file_id 本身就含文件 hash，缺省时透传兜底。
          file_hash: (_a = data.file_hash) != null ? _a : data.file_id,
          is_self_send: data.is_self_send
        }
      };
    },
    send_poke: (data) => {
      if (data.group_id !== void 0) {
        return ob11ApiMap.group_poke(data);
      }
      return ob11ApiMap.friend_poke(data);
    },
    get_group_info_ex: (data) => ({
      api: "get_group_info",
      params: { group_id: data.group_id, no_cache: data.no_cache }
    }),
    get_group_detail_info: (data) => ({
      api: "get_group_info",
      params: { group_id: data.group_id, no_cache: data.no_cache }
    }),
    get_friends_with_category: (data) => ({
      api: "get_friend_list",
      params: { no_cache: data.no_cache }
    }),
    send_like: (data) => {
      var _a;
      return {
        api: "send_profile_like",
        params: { user_id: data.user_id, count: (_a = data.times) != null ? _a : 1 }
      };
    },
    set_group_kick: (data) => {
      var _a;
      return {
        api: "kick_group_member",
        params: {
          group_id: data.group_id,
          user_id: data.user_id,
          reject_add_request: (_a = data.reject_add_request) != null ? _a : false
        }
      };
    },
    set_group_ban: (data) => {
      var _a;
      return {
        api: "set_group_member_mute",
        params: {
          group_id: data.group_id,
          user_id: data.user_id,
          duration: (_a = data.duration) != null ? _a : 30 * 60
        }
      };
    },
    set_group_whole_ban: (data) => {
      var _a;
      return {
        api: "set_group_whole_mute",
        params: { group_id: data.group_id, is_mute: (_a = data.enable) != null ? _a : true }
      };
    },
    set_group_admin: (data) => {
      var _a;
      return {
        api: "set_group_member_admin",
        params: { group_id: data.group_id, user_id: data.user_id, is_set: (_a = data.enable) != null ? _a : true }
      };
    },
    set_group_card: (data) => {
      var _a;
      return {
        api: "set_group_member_card",
        params: { group_id: data.group_id, user_id: data.user_id, card: (_a = data.card) != null ? _a : "" }
      };
    },
    set_group_name: (data) => ({
      api: "set_group_name",
      params: { group_id: data.group_id, new_group_name: data.group_name }
    }),
    set_group_leave: (data) => ({
      api: "quit_group",
      params: { group_id: data.group_id }
    }),
    set_group_special_title: (data) => {
      var _a;
      return {
        api: "set_group_member_special_title",
        params: {
          group_id: data.group_id,
          user_id: data.user_id,
          special_title: (_a = data.special_title) != null ? _a : ""
        }
      };
    },
    set_friend_add_request: (data) => mapFriendRequest(data, data.approve !== false),
    set_group_add_request: (data) => mapGroupRequest(data, data.approve !== false),
    get_login_info: () => ({ api: "get_login_info", params: {} }),
    get_stranger_info: (data) => ({
      api: "get_user_profile",
      params: { user_id: data.user_id, no_cache: data.no_cache }
    }),
    get_friend_list: (data) => ({
      api: "get_friend_list",
      params: { no_cache: data.no_cache }
    }),
    get_group_info: (data) => ({
      api: "get_group_info",
      params: { group_id: data.group_id, no_cache: data.no_cache }
    }),
    get_group_list: (data) => ({
      api: "get_group_list",
      params: { no_cache: data.no_cache }
    }),
    get_group_member_info: (data) => ({
      api: "get_group_member_info",
      params: { group_id: data.group_id, user_id: data.user_id, no_cache: data.no_cache }
    }),
    get_group_member_list: (data) => ({
      api: "get_group_member_list",
      params: { group_id: data.group_id, no_cache: data.no_cache }
    }),
    get_cookies: (data) => ({
      api: "get_cookies",
      params: data.domain ? { domain: data.domain } : {}
    }),
    get_csrf_token: () => ({ api: "get_csrf_token", params: {} })
  };
  function toMilkyUri(file) {
    const value = String(file != null ? file : "");
    if (/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(value)) {
      return value;
    }
    return `file://${value}`;
  }
  function decodeCQParam(value) {
    return value.replace(/&amp;/g, "&").replace(/&#44;/g, ",").replace(/&#91;/g, "[").replace(/&#93;/g, "]");
  }
  function parseCQString(text) {
    var _a;
    const result = [];
    const regex = /\[CQ:([a-z_]+)((?:,[^[\]]*)?)\]/gi;
    let lastIndex = 0;
    let match;
    while ((match = regex.exec(text)) !== null) {
      if (match.index > lastIndex) {
        result.push({ type: "text", data: { text: text.substring(lastIndex, match.index) } });
      }
      const type = match[1].toLowerCase();
      const params = {};
      if (match[2]) {
        const raw = match[2].replace(/^,/, "");
        const pairs = raw.split(",");
        for (const pair of pairs) {
          const idx = pair.indexOf("=");
          if (idx === -1) {
            continue;
          }
          const key = pair.substring(0, idx);
          const value = decodeCQParam(pair.substring(idx + 1));
          params[key] = value;
        }
      }
      const data = __spreadValues({}, params);
      if (type === "at") {
        data.qq = (_a = params.qq) != null ? _a : params.id;
      } else if (type === "face") {
        data.id = params.id;
      } else if (type === "reply") {
        data.id = params.id;
      } else if (type === "json") {
        data.data = params.data;
      }
      result.push({ type, data });
      lastIndex = regex.lastIndex;
    }
    if (lastIndex < text.length) {
      result.push({ type: "text", data: { text: text.substring(lastIndex) } });
    }
    if (result.length === 0) {
      result.push({ type: "text", data: { text } });
    }
    return result;
  }
  var MilkyAdapter = class {
    static reset() {
      this.conversations.reset();
      this.requestCache = {};
    }
    // ---------------- 发送方向：OB11 → Milky ----------------
    static convertOutgoingMessage(message, ctx) {
      const segments = Array.isArray(message) ? message : typeof message === "string" ? parseCQString(message) : [message];
      return segments.map((segment) => this.convertOutgoingSegment(segment, ctx));
    }
    static convertOutgoingSegment(segment, ctx) {
      var _a, _b, _c, _d, _e, _f, _g, _h, _i, _j, _k, _l;
      const data = segment.data || {};
      switch (segment.type) {
        case "text": {
          return { type: "text", data: { text: String((_a = data.text) != null ? _a : "") } };
        }
        case "at": {
          const qq = String((_b = data.qq) != null ? _b : "");
          if (qq === "all") {
            return { type: "mention_all", data: {} };
          }
          return { type: "mention", data: { user_id: Number(qq) } };
        }
        case "face": {
          return {
            type: "face",
            data: { face_id: String((_d = (_c = data.id) != null ? _c : data.face_id) != null ? _d : ""), is_large: !!data.is_large }
          };
        }
        case "reply": {
          const ref = this.conversations.getRef(Number(data.id));
          if (!ref) {
            throw new Error(`无法解析 reply 消息段中的 message_id=${data.id}，Milky 需要原始 message_seq`);
          }
          return { type: "reply", data: { message_seq: ref.seq } };
        }
        case "image": {
          const file = String((_f = (_e = data.file) != null ? _e : data.url) != null ? _f : "");
          if (!file) {
            throw new Error("image 消息段缺少 file/url");
          }
          return {
            type: "image",
            data: {
              uri: toMilkyUri(file),
              sub_type: data.sub_type === "sticker" ? "sticker" : "normal",
              summary: data.summary
            }
          };
        }
        case "record": {
          const file = String((_h = (_g = data.file) != null ? _g : data.url) != null ? _h : "");
          if (!file) {
            throw new Error("record 消息段缺少 file/url");
          }
          return { type: "record", data: { uri: toMilkyUri(file) } };
        }
        case "video": {
          const file = String((_j = (_i = data.file) != null ? _i : data.url) != null ? _j : "");
          if (!file) {
            throw new Error("video 消息段缺少 file/url");
          }
          return {
            type: "video",
            data: {
              uri: toMilkyUri(file),
              thumb_uri: data.thumb ? toMilkyUri(String(data.thumb)) : void 0
            }
          };
        }
        case "forward": {
          const nodes = (_k = data.messages) != null ? _k : data.content;
          if (!Array.isArray(nodes) || nodes.length === 0) {
            throw new Error("forward 消息段需要完整 data.messages（Milky 不支持仅按转发 ID 发送）");
          }
          return {
            type: "forward",
            data: {
              messages: nodes.map((node) => this.convertForwardNode(node, ctx)),
              title: data.title,
              preview: data.preview,
              summary: data.summary,
              prompt: data.prompt
            }
          };
        }
        case "json": {
          const payload = typeof data.data === "string" ? data.data : JSON.stringify((_l = data.data) != null ? _l : {});
          return { type: "light_app", data: { json_payload: payload } };
        }
        case "music": {
          throw new Error("Milky 协议没有音乐(music)消息段，无法发送；请改用 json 卡片或 light_app");
        }
        case "file": {
          throw new Error("Milky 消息发送不支持 file 段，请使用 upload_group_file / upload_private_file API");
        }
        default: {
          throw new Error(`Milky 不支持转换的 OB11 消息段类型: ${segment.type}`);
        }
      }
    }
    /** 把 OB11 合并转发节点（node）转换为 Milky OutgoingForwardedMessage */
    static convertForwardNode(node, ctx) {
      var _a, _b, _c, _d, _e;
      const data = (node == null ? void 0 : node.data) || {};
      const content = (_b = (_a = data.content) != null ? _a : data.message) != null ? _b : [];
      return {
        user_id: Number((_c = data.user_id) != null ? _c : 0),
        sender_name: String((_e = (_d = data.nickname) != null ? _d : data.name) != null ? _e : "未知用户"),
        time: data.time,
        segments: this.convertOutgoingMessage(content, ctx)
      };
    }
    // ---------------- 接收方向：Milky → OB11 ----------------
    static convertIncomingSegments(segments, scene, peerId) {
      return segments.map((segment) => this.convertIncomingSegment(segment, scene, peerId));
    }
    static convertIncomingSegment(segment, scene, peerId) {
      const data = segment.data || {};
      switch (segment.type) {
        case "text": {
          return { type: "text", data: { text: data.text } };
        }
        case "mention": {
          return { type: "at", data: { qq: String(data.user_id), name: data.name } };
        }
        case "mention_all": {
          return { type: "at", data: { qq: "all" } };
        }
        case "face": {
          return {
            type: "face",
            data: { id: data.face_id, is_large: data.is_large }
          };
        }
        case "reply": {
          const id = this.conversations.getMessageId(scene, peerId, data.message_seq);
          return {
            type: "reply",
            data: {
              id: String(id),
              message_seq: data.message_seq,
              sender_id: data.sender_id,
              sender_name: data.sender_name,
              time: data.time,
              segments: this.convertIncomingSegments(data.segments || [], scene, peerId)
            }
          };
        }
        case "image": {
          return {
            type: "image",
            data: {
              file: data.resource_id,
              url: data.temp_url,
              resource_id: data.resource_id,
              width: data.width,
              height: data.height,
              summary: data.summary,
              sub_type: data.sub_type
            }
          };
        }
        case "record": {
          return {
            type: "record",
            data: { file: data.resource_id, url: data.temp_url, duration: data.duration }
          };
        }
        case "video": {
          return {
            type: "video",
            data: {
              file: data.resource_id,
              url: data.temp_url,
              width: data.width,
              height: data.height,
              duration: data.duration
            }
          };
        }
        case "file": {
          return {
            type: "file",
            data: {
              file: data.file_name,
              file_id: data.file_id,
              file_size: data.file_size,
              name: data.file_name,
              size: data.file_size,
              hash: data.file_hash
            }
          };
        }
        case "forward": {
          return {
            type: "forward",
            data: {
              id: data.forward_id,
              forward_id: data.forward_id,
              title: data.title,
              preview: data.preview,
              summary: data.summary
            }
          };
        }
        case "market_face": {
          return { type: "market_face", data };
        }
        case "light_app": {
          return { type: "json", data: { data: data.json_payload, name: data.app_name } };
        }
        case "xml": {
          return { type: "xml", data: { data: data.xml_payload, service_id: data.service_id } };
        }
        case "markdown": {
          return { type: "markdown", data: { content: data.content } };
        }
        default: {
          logger.warning(`Milky 收到未知消息段类型: ${segment.type}`);
          return { type: "text", data: { text: `[Milky不支持的段类型: ${segment.type}]` } };
        }
      }
    }
    static toRawMessage(segments, scene, peerId) {
      return this.convertIncomingSegments(segments, scene, peerId).map((segment) => {
        const data = segment.data || {};
        switch (segment.type) {
          case "text":
            return data.text;
          case "at":
            return `[CQ:at,qq=${data.qq}]`;
          case "face":
            return `[CQ:face,id=${data.id}]`;
          case "image":
            return `[CQ:image,file=${data.file}]`;
          case "reply":
            return `[CQ:reply,id=${data.id}]`;
          default:
            return `[${segment.type}]`;
        }
      }).join("");
    }
    // ---------------- 事件转换：Milky → OB11 ----------------
    /** 消息含合并转发段时，记录 message_id → forward_id，供 get_forward_msg 还原 */
    static rememberForwardId(messageId, segments) {
      for (const segment of segments) {
        if (segment.type === "forward" && segment.data.forward_id) {
          this.conversations.setForwardId(messageId, segment.data.forward_id);
          return;
        }
      }
    }
    static toOneBot11Event(event) {
      var _a, _b, _c, _d, _e, _f, _g, _h, _i;
      const d = event.data || {};
      const base = { time: event.time, self_id: event.self_id };
      switch (event.event_type) {
        case "message_receive": {
          const msg = d;
          const messageId = this.conversations.getMessageId(msg.message_scene, msg.peer_id, msg.message_seq);
          this.rememberForwardId(messageId, msg.segments);
          const message = this.convertIncomingSegments(msg.segments, msg.message_scene, msg.peer_id);
          const rawMessage = this.toRawMessage(msg.segments, msg.message_scene, msg.peer_id);
          if (msg.message_scene === "group") {
            return __spreadProps(__spreadValues({}, base), {
              post_type: "message",
              message_type: "group",
              sub_type: "normal",
              message_id: messageId,
              user_id: msg.sender_id,
              group_id: msg.peer_id,
              message,
              raw_message: rawMessage,
              font: 0,
              anonymous: null,
              sender: {
                user_id: msg.sender_id,
                nickname: ((_a = msg.group_member) == null ? void 0 : _a.nickname) || "",
                card: ((_b = msg.group_member) == null ? void 0 : _b.card) || "",
                role: ((_c = msg.group_member) == null ? void 0 : _c.role) || "member",
                title: ((_d = msg.group_member) == null ? void 0 : _d.title) || ""
              }
            });
          }
          return __spreadProps(__spreadValues({}, base), {
            post_type: "message",
            message_type: "private",
            sub_type: msg.message_scene === "temp" ? "group" : "friend",
            message_id: messageId,
            user_id: msg.sender_id,
            message,
            raw_message: rawMessage,
            font: 0,
            sender: {
              user_id: msg.sender_id,
              nickname: ((_e = msg.friend) == null ? void 0 : _e.nickname) || ""
            }
          });
        }
        case "message_recall": {
          const messageId = this.conversations.getMessageId(d.message_scene, d.peer_id, d.message_seq);
          if (d.message_scene === "group") {
            return __spreadProps(__spreadValues({}, base), {
              post_type: "notice",
              notice_type: "group_recall",
              group_id: d.peer_id,
              user_id: d.sender_id,
              operator_id: d.operator_id,
              message_id: messageId,
              milky_event_type: "message_recall"
            });
          }
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "friend_recall",
            user_id: d.peer_id,
            message_id: messageId,
            operator_id: d.operator_id,
            milky_event_type: "message_recall"
          });
        }
        case "friend_request": {
          const flag = String((_f = d.initiator_uid) != null ? _f : "");
          this.requestCache[`friend:${flag}`] = {
            initiator_uid: d.initiator_uid,
            initiator_id: d.initiator_id,
            comment: d.comment,
            via: d.via,
            is_filtered: false
          };
          return __spreadProps(__spreadValues({}, base), {
            post_type: "request",
            request_type: "friend",
            user_id: d.initiator_id,
            comment: d.comment,
            flag,
            via: d.via,
            milky_event_type: "friend_request"
          });
        }
        case "group_join_request": {
          const flag = `${d.notification_seq}|join_request|${d.is_filtered}`;
          this.requestCache[`group:${d.group_id}|${d.initiator_id}`] = {
            notification_seq: d.notification_seq,
            notification_type: "join_request",
            is_filtered: d.is_filtered
          };
          return __spreadProps(__spreadValues({}, base), {
            post_type: "request",
            request_type: "group",
            sub_type: "add",
            group_id: d.group_id,
            user_id: d.initiator_id,
            comment: d.comment,
            flag,
            is_filtered: d.is_filtered,
            milky_event_type: "group_join_request"
          });
        }
        case "group_invited_join_request": {
          const flag = `${d.notification_seq}|invited_join_request|${d.is_filtered}`;
          this.requestCache[`group:${d.group_id}|${d.initiator_id}`] = {
            notification_seq: d.notification_seq,
            notification_type: "invited_join_request",
            is_filtered: d.is_filtered
          };
          return __spreadProps(__spreadValues({}, base), {
            post_type: "request",
            request_type: "group",
            sub_type: "invite",
            group_id: d.group_id,
            user_id: d.initiator_id,
            comment: "",
            flag,
            target_user_id: d.target_user_id,
            is_filtered: d.is_filtered,
            milky_event_type: "group_invited_join_request"
          });
        }
        case "group_invitation": {
          const flag = `invitation|${d.invitation_seq}`;
          return __spreadProps(__spreadValues({}, base), {
            post_type: "request",
            request_type: "group",
            sub_type: "invite",
            group_id: d.group_id,
            user_id: d.initiator_id,
            comment: "",
            flag,
            source_group_id: d.source_group_id,
            milky_event_type: "group_invitation"
          });
        }
        case "friend_nudge": {
          const user_id = d.is_self_send ? base.self_id : d.user_id;
          const target_id = d.is_self_send ? d.user_id : base.self_id;
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "notify",
            sub_type: "poke",
            user_id,
            target_id,
            display_action: d.display_action,
            display_suffix: d.display_suffix,
            milky_event_type: "friend_nudge"
          });
        }
        case "group_nudge": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "notify",
            sub_type: "poke",
            group_id: d.group_id,
            user_id: d.sender_id,
            target_id: d.receiver_id,
            display_action: d.display_action,
            display_suffix: d.display_suffix,
            milky_event_type: "group_nudge"
          });
        }
        case "friend_file_upload": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "friend_file_upload",
            user_id: d.user_id,
            file: { id: d.file_id, name: d.file_name, size: d.file_size, hash: d.file_hash, is_self: d.is_self },
            milky_event_type: "friend_file_upload"
          });
        }
        case "group_file_upload": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_upload",
            group_id: d.group_id,
            user_id: d.user_id,
            file: { id: d.file_id, name: d.file_name, size: d.file_size, busid: 0 },
            milky_event_type: "group_file_upload"
          });
        }
        case "group_admin_change": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_admin",
            sub_type: d.is_set ? "set" : "unset",
            group_id: d.group_id,
            user_id: d.user_id,
            operator_id: d.operator_id,
            milky_event_type: "group_admin_change"
          });
        }
        case "group_member_increase": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_increase",
            sub_type: d.invitor_id != null ? "invite" : "approve",
            group_id: d.group_id,
            user_id: d.user_id,
            operator_id: (_h = (_g = d.operator_id) != null ? _g : d.invitor_id) != null ? _h : 0,
            milky_event_type: "group_member_increase"
          });
        }
        case "group_member_decrease": {
          const subType = d.user_id === base.self_id ? "kick_me" : d.operator_id != null ? "kick" : "leave";
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_decrease",
            sub_type: subType,
            group_id: d.group_id,
            user_id: d.user_id,
            operator_id: (_i = d.operator_id) != null ? _i : 0,
            milky_event_type: "group_member_decrease"
          });
        }
        case "group_mute": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_ban",
            sub_type: d.duration > 0 ? "ban" : "lift_ban",
            group_id: d.group_id,
            user_id: d.user_id,
            operator_id: d.operator_id,
            duration: d.duration,
            milky_event_type: "group_mute"
          });
        }
        case "group_whole_mute": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_whole_mute",
            group_id: d.group_id,
            user_id: d.operator_id,
            operator_id: d.operator_id,
            is_mute: d.is_mute,
            milky_event_type: "group_whole_mute"
          });
        }
        case "group_disband": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_disband",
            group_id: d.group_id,
            operator_id: d.operator_id,
            milky_event_type: "group_disband"
          });
        }
        case "group_name_change": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_name_change",
            group_id: d.group_id,
            group_name: d.new_group_name,
            operator_id: d.operator_id,
            milky_event_type: "group_name_change"
          });
        }
        case "group_message_reaction": {
          const messageId = this.conversations.getMessageId("group", d.group_id, d.message_seq);
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_message_reaction",
            group_id: d.group_id,
            user_id: d.user_id,
            message_id: messageId,
            message_seq: d.message_seq,
            face_id: d.face_id,
            reaction_type: d.reaction_type,
            is_add: d.is_add,
            milky_event_type: "group_message_reaction"
          });
        }
        case "group_essence_message_change": {
          const messageId = this.conversations.getMessageId("group", d.group_id, d.message_seq);
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "group_essence_message_change",
            group_id: d.group_id,
            message_id: messageId,
            operator_id: d.operator_id,
            is_set: d.is_set,
            milky_event_type: "group_essence_message_change"
          });
        }
        case "peer_pin_change": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "notice",
            notice_type: "peer_pin_change",
            message_scene: d.message_scene,
            peer_id: d.peer_id,
            is_pinned: d.is_pinned,
            milky_event_type: "peer_pin_change"
          });
        }
        case "bot_offline": {
          return __spreadProps(__spreadValues({}, base), {
            post_type: "meta_event",
            meta_event_type: "lifecycle",
            sub_type: "milky_bot_offline",
            reason: d.reason,
            milky_event_type: "bot_offline"
          });
        }
        default: {
          logger.warning(`Milky 未知事件类型，已忽略: ${event.event_type}`);
          return null;
        }
      }
    }
    // ---------------- API 映射 ----------------
    static mapApi(method, data = {}) {
      const mapper = ob11ApiMap[method];
      if (mapper) {
        return mapper(data || {});
      }
      if (MILKY_EXTRA_APIS.has(method)) {
        const validator = RAW_PARAM_VALIDATORS[method];
        const params = validator ? validator(data || {}) : data || {};
        return { api: method, params };
      }
      const hint = UNSUPPORTED_MILKY_API_HINTS[method];
      if (hint) {
        throw new Error(hint);
      }
      if (NAPCAT_UNSUPPORTED.has(method)) {
        throw new Error(`Milky 协议暂无 ${method} 对应 API（NapCat 扩展）`);
      }
      throw new Error(`Milky 不支持该 API: ${method}`);
    }
    static mapApiResponse(method, raw, requestData = {}) {
      var _a, _b, _c, _d, _e, _f, _g;
      if (method === "send_private_msg" || method === "send_private_message") {
        return {
          message_id: this.conversations.getMessageId("friend", requestData.user_id, raw.message_seq),
          time: raw.time
        };
      }
      if (method === "send_group_msg" || method === "send_group_message") {
        return {
          message_id: this.conversations.getMessageId("group", requestData.group_id, raw.message_seq),
          time: raw.time
        };
      }
      if (method === "get_msg" || method === "get_message") {
        return this.toOneBot11Message(raw.message);
      }
      if (method === "get_forward_msg" || method === "get_forwarded_messages") {
        const messages = Array.isArray(raw.messages) ? raw.messages : [];
        return {
          messages: messages.map((item) => ({
            self_id: 0,
            user_id: 0,
            time: item.time,
            message_id: item.message_seq,
            message_seq: item.message_seq,
            real_id: item.message_seq,
            real_seq: String(item.message_seq),
            message_type: "group",
            sender: { user_id: 0, nickname: item.sender_name, card: "" },
            raw_message: this.toRawMessage(item.segments, "group", 0),
            font: 0,
            sub_type: "normal",
            message: this.convertIncomingSegments(item.segments, "group", 0),
            message_format: "array",
            post_type: "message",
            group_id: 0,
            group_name: ""
          }))
        };
      }
      if (method === "get_login_info") {
        return { user_id: raw.uin, nickname: raw.nickname };
      }
      if (method === "get_stranger_info") {
        return {
          user_id: (_a = raw.user_id) != null ? _a : requestData.user_id,
          nickname: raw.nickname,
          sex: raw.sex,
          age: raw.age,
          qid: raw.qid,
          remark: raw.remark,
          level: raw.level,
          country: raw.country,
          city: raw.city,
          school: raw.school
        };
      }
      if (method === "get_friend_list") {
        return (raw.friends || []).map((f) => this.toOneBot11Friend(f));
      }
      if (method === "get_friend_info") {
        return this.toOneBot11Friend(raw.friend || raw);
      }
      if (method === "get_group_info") {
        return this.toOneBot11Group(raw.group || raw);
      }
      if (method === "get_group_list") {
        return (raw.groups || []).map((g) => this.toOneBot11Group(g));
      }
      if (method === "get_group_member_info") {
        return this.toOneBot11Member(raw.member, requestData.group_id);
      }
      if (method === "get_group_member_list") {
        return (raw.members || []).map((m) => this.toOneBot11Member(m, requestData.group_id));
      }
      if (method === "get_essence_msg_list") {
        const rawList = Array.isArray(raw) ? raw : Array.isArray(raw == null ? void 0 : raw.messages) ? raw.messages : Array.isArray(raw == null ? void 0 : raw.essence_messages) ? raw.essence_messages : [];
        const groupId = requestData.group_id;
        return rawList.map((item) => {
          var _a2;
          const gid = (_a2 = item.group_id) != null ? _a2 : groupId;
          return {
            message_id: this.conversations.getMessageId("group", gid, item.message_seq),
            group_id: gid,
            sender_id: item.sender_id,
            sender_nick: item.sender_name,
            operator_id: item.operator_id,
            operator_nick: item.operator_name,
            operator_time: item.operation_time,
            message_time: item.message_time,
            message_seq: item.message_seq,
            content: this.convertIncomingSegments(item.segments || [], "group", gid)
          };
        });
      }
      if (method === "get_group_file_url") {
        const url = validateFileUrl(method, (_c = (_b = raw.download_url) != null ? _b : raw.url) != null ? _c : raw);
        return { url };
      }
      if (method === "get_group_root_files" || method === "get_group_files_by_folder") {
        const files = (raw.files || []).map((f) => {
          var _a2, _b2, _c2, _d2;
          return __spreadProps(__spreadValues({}, f), {
            file_name: (_a2 = f.file_name) != null ? _a2 : f.name,
            name: (_b2 = f.file_name) != null ? _b2 : f.name,
            size: (_c2 = f.file_size) != null ? _c2 : f.size,
            file_size: (_d2 = f.file_size) != null ? _d2 : f.size
          });
        });
        const folders = (raw.folders || []).map((f) => {
          var _a2, _b2;
          return __spreadProps(__spreadValues({}, f), {
            folder_name: (_a2 = f.folder_name) != null ? _a2 : f.name,
            name: (_b2 = f.folder_name) != null ? _b2 : f.name
          });
        });
        return { files, folders };
      }
      if (method === "get_group_msg_history" || method === "get_friend_msg_history") {
        const msgs = Array.isArray(raw) ? raw : Array.isArray(raw == null ? void 0 : raw.messages) ? raw.messages : [];
        return msgs.map((m) => this.toOneBot11Message(m));
      }
      if (method === "get_private_file_url") {
        const url = validateFileUrl(method, (_e = (_d = raw.download_url) != null ? _d : raw.url) != null ? _e : raw);
        return { url };
      }
      if (method === "get_group_info_ex" || method === "get_group_detail_info") {
        return this.toOneBot11Group(raw.group || raw);
      }
      if (method === "get_friends_with_category") {
        const friends = (raw.friends || []).map((f) => this.toOneBot11Friend(f));
        const categoryMap = /* @__PURE__ */ new Map();
        for (const f of friends) {
          const cid = (_f = f.categoryId) != null ? _f : 0;
          const cname = (_g = f.categoryName) != null ? _g : "";
          const key = `${cid}|${cname}`;
          if (!categoryMap.has(key)) {
            categoryMap.set(key, { categoryId: cid, categoryName: cname, categoryMbCount: 0, buddyList: [] });
          }
          categoryMap.get(key).buddyList.push(f);
        }
        const categories = Array.from(categoryMap.values());
        for (const category of categories) {
          category.categoryMbCount = category.buddyList.length;
        }
        return categories;
      }
      return raw;
    }
    /** Milky FriendEntity → OB11User（NapCat OB11UserSchema 形状；未提供的字段不臆造） */
    static toOneBot11Friend(f) {
      var _a, _b;
      return {
        user_id: f.user_id,
        nickname: f.nickname,
        remark: f.remark,
        sex: f.sex,
        qid: f.qid,
        categoryName: (_a = f.category) == null ? void 0 : _a.category_name,
        categoryId: (_b = f.category) == null ? void 0 : _b.category_id
      };
    }
    /** Milky GroupEntity → OB11Group（NapCat OB11GroupSchema 形状；created_time 等为 Milky 扩展字段） */
    static toOneBot11Group(g) {
      var _a;
      return {
        group_id: g.group_id,
        group_name: g.group_name,
        member_count: g.member_count,
        max_member_count: g.max_member_count,
        group_remark: (_a = g.remark) != null ? _a : "",
        // Milky 未提供全员禁言状态字段，按未开启（0）处理，与 NapCat group_all_shut 取值一致
        group_all_shut: 0,
        created_time: g.created_time,
        description: g.description,
        question: g.question,
        announcement: g.announcement
      };
    }
    static toOneBot11Member(member, groupId) {
      var _a, _b, _c;
      return {
        group_id: (_b = (_a = member.group_id) != null ? _a : groupId) != null ? _b : 0,
        user_id: member.user_id,
        nickname: member.nickname,
        card: member.card,
        sex: member.sex,
        age: 0,
        area: "",
        join_time: member.join_time,
        last_sent_time: member.last_sent_time,
        level: String(member.level),
        role: member.role,
        unfriendly: false,
        title: member.title,
        title_expire_time: 0,
        card_changeable: false,
        // NapCat 字段名为 shut_up_timestamp（禁言截止时间戳），Milky 的 shut_up_end_time 同义
        shut_up_timestamp: (_c = member.shut_up_end_time) != null ? _c : void 0
      };
    }
    static toOneBot11Message(msg) {
      var _a, _b, _c, _d, _e;
      const messageId = this.conversations.getMessageId(msg.message_scene, msg.peer_id, msg.message_seq);
      this.rememberForwardId(messageId, msg.segments);
      const message = this.convertIncomingSegments(msg.segments, msg.message_scene, msg.peer_id);
      const rawMessage = this.toRawMessage(msg.segments, msg.message_scene, msg.peer_id);
      const base = {
        message_id: messageId,
        // NapCat GetMsg 返回 message_seq = real_id = message_id
        message_seq: messageId,
        real_id: messageId,
        time: msg.time,
        user_id: msg.sender_id,
        message,
        raw_message: rawMessage,
        font: 0,
        emoji_likes_list: [],
        message_format: "array"
      };
      if (msg.message_scene === "group") {
        return __spreadProps(__spreadValues({}, base), {
          message_type: "group",
          group_id: msg.peer_id,
          sender: {
            user_id: msg.sender_id,
            nickname: ((_a = msg.group_member) == null ? void 0 : _a.nickname) || "",
            card: ((_b = msg.group_member) == null ? void 0 : _b.card) || "",
            role: ((_c = msg.group_member) == null ? void 0 : _c.role) || "member",
            title: ((_d = msg.group_member) == null ? void 0 : _d.title) || ""
          }
        });
      }
      return __spreadProps(__spreadValues({}, base), {
        message_type: "private",
        sub_type: msg.message_scene === "temp" ? "group" : "friend",
        sender: {
          user_id: msg.sender_id,
          nickname: ((_e = msg.friend) == null ? void 0 : _e.nickname) || ""
        }
      });
    }
  };
  MilkyAdapter.conversations = new ConversationIndexer();
  MilkyAdapter.requestCache = {};
  function isMilkyExtraApi(method) {
    return MILKY_EXTRA_APIS.has(method);
  }

  // src/ws.ts
  var WS_OPEN = 1;
  var WS_CONNECTING = 0;
  var EventDispatcher = class {
    constructor(name) {
      /** 串行模式下的回调执行队列 */
      this.queue = Promise.resolve();
      this.name = name;
      this.onEvent = () => {
      };
      this.onMessageEvent = () => {
      };
      this.onNoticeEvent = () => {
      };
      this.onRequestEvent = () => {
      };
      this.onMetaEvent = () => {
      };
    }
  };
  var _WSManager = class _WSManager {
    static async getEventDispatcher(ext, key = "default") {
      if (!this.initDone) {
        await this.init();
      }
      const mapKey = `${ext.name}::${key}`;
      let ed = this.eventDispatcherMap[mapKey];
      if (!ed) {
        ed = new EventDispatcher(ext.name);
        this.eventDispatcherMap[mapKey] = ed;
      }
      return ed;
    }
    // --- 事件分发 ---//
    static dispatchToDispatcher(ed, epId, event) {
      return Promise.resolve().then(() => ed.onEvent(epId, event)).catch((e) => {
        logger.error(`[${ed.name}] 事件处理错误: ${e.message}`);
      }).then(() => {
        switch (event.post_type) {
          case "message": {
            return ed.onMessageEvent(epId, event);
          }
          case "notice": {
            return ed.onNoticeEvent(epId, event);
          }
          case "request": {
            return ed.onRequestEvent(epId, event);
          }
          case "meta_event": {
            return ed.onMetaEvent(epId, event);
          }
          default: {
            return;
          }
        }
      }).catch((e) => {
        logger.error(`[${ed.name}] 分类型事件处理错误: ${e.message}`);
      });
    }
    static emitEvent(epId, event) {
      for (const name of Object.keys(this.eventDispatcherMap)) {
        const ed = this.eventDispatcherMap[name];
        const dispatch = () => this.dispatchToDispatcher(ed, epId, event);
        if (ConfigManager.dispatchMode === "串行") {
          ed.queue = ed.queue.then(dispatch).catch((e) => {
            logger.error(`[${ed.name}] 事件队列处理错误: ${e.message}`);
          });
        } else {
          Promise.resolve().then(dispatch).catch((e) => {
            logger.error(`[${ed.name}] 事件处理错误: ${e.message}`);
          });
        }
      }
    }
    static async init() {
      if (this.initDone) {
        return;
      }
      if (this.initPromise) {
        return this.initPromise;
      }
      this.initPromise = this.doInit().finally(() => {
        this.initPromise = null;
      });
      return this.initPromise;
    }
    static async doInit() {
      this.urlMap = {};
      Object.keys(this.wsConnections).forEach((epId) => {
        const ci = this.wsConnections[epId];
        if (ci) {
          ci.manualClosed = true;
          if (ci.reconnectTimer) {
            clearTimeout(ci.reconnectTimer);
          }
          this.safeClose(ci.ws);
        }
      });
      this.wsConnections = {};
      this.tempCounter = 0;
      const ob11Endpoints = ConfigManager.ob11Ws;
      for (const endpoint of ob11Endpoints) {
        const url = endpoint.url.trim();
        if (!url) {
          continue;
        }
        await this.probeOb11Endpoint(url, endpoint.token);
      }
      const milkyEndpoints = ConfigManager.milky;
      for (const endpoint of milkyEndpoints) {
        const url = endpoint.url.trim();
        if (!url) {
          continue;
        }
        await this.connectMilkyEndpoint(url, endpoint.token);
      }
      logger.info("WS 初始化完成，ws urlMap: ", JSON.stringify(this.urlMap, null, 2));
      this.initDone = true;
    }
    /** OB11 WS 探测：连原地址，发 get_login_info 等待 echo 完成端点映射 */
    static async probeOb11Endpoint(url, token) {
      const tempEpId = `初始化连接#${++this.tempCounter}`;
      let ci;
      try {
        logger.info(`尝试连接(OB11 WS): ${url}`);
        ci = this.createWsConnection(tempEpId, url, token, 0, "ob11");
        ci.manualClosed = true;
        await this.waitForWsOpen(ci, 5e3);
        if (ci.milkyResolved) {
          logger.info(`端点已在探测前完成映射: ${ci.epId} (${url}, 协议: ${ci.protocol})`);
          return;
        }
        const data = await this.callApiByWsInternal(ci, "get_login_info", {}, this.probeTimeoutMs, true);
        if (data === null || data === void 0) {
          throw new Error("get_login_info 返回空数据");
        }
        const epId = `${PLATFORM}:${data.user_id}`;
        const eps = seal.getEndPoints();
        let found = false;
        for (let j = 0; j < eps.length; j++) {
          if (eps[j].userId === epId) {
            found = true;
            break;
          }
        }
        if (!found) {
          this.destroyConnection(ci.epId);
          logger.warning(`未找到对应的端点: ${epId}`);
          return;
        }
        if (ci.milkyResolved && ci.epId !== tempEpId) {
          logger.info(`端点已在探测期间完成映射: ${ci.epId} (${url}, 协议: ${ci.protocol})`);
          return;
        }
        delete this.wsConnections[tempEpId];
        ci.epId = epId;
        ci.protocol = "ob11";
        ci.milkyResolved = true;
        ci.manualClosed = false;
        this.wsConnections[epId] = ci;
        this.urlMap[epId] = { url: ci.url, token };
        logger.info(`找到 ${epId} 地址(OB11 WS): ${url}`);
      } catch (error) {
        if (ci && ci.milkyResolved) {
          logger.info(`端点已在探测期间完成映射: ${ci.epId} (${url}, 协议: ${ci.protocol})`);
          return;
        }
        if (ci) {
          this.destroyConnection(ci.epId);
        } else {
          this.destroyConnection(tempEpId);
        }
        logger.error(`获取登录信息失败: ${url}, 错误: ${getErrorMessage(error)}`);
      }
    }
    /** Milky WS 连接：连 {url}/event，等首个事件用 self_id 完成端点映射 */
    static async connectMilkyEndpoint(url, token) {
      const tempEpId = `初始化连接#${++this.tempCounter}`;
      const connectUrl = this.normalizeMilkyWsUrl(url);
      try {
        logger.info(`尝试连接(Milky WS): ${connectUrl}`);
        const ci = this.createWsConnection(tempEpId, connectUrl, token, 0, "milky");
        ci.manualClosed = true;
        await this.waitForWsOpen(ci, 5e3);
        ci.manualClosed = false;
        try {
          await this.waitForMilkyResolved(ci, this.milkyResolveTimeoutMs);
          logger.info(`找到 ${ci.epId} 地址(Milky WS): ${connectUrl}`);
        } catch (milkyError) {
          this.destroyConnection(tempEpId);
          logger.error(`Milky 端点匹配失败: ${connectUrl}, 错误: ${getErrorMessage(milkyError)}`);
        }
      } catch (error) {
        this.destroyConnection(tempEpId);
        logger.error(`Milky WS 连接失败: ${connectUrl}, 错误: ${getErrorMessage(error)}`);
      }
    }
    /** 等待 WebSocket 打开（轮询 readyState，避免依赖事件监听器叠加） */
    static waitForWsOpen(ci, timeoutMs = 5e3) {
      return new Promise((resolve, reject) => {
        const start = Date.now();
        let settled = false;
        const timer = setInterval(() => {
          if (settled) {
            clearInterval(timer);
            return;
          }
          if (ci.ws.readyState === WS_OPEN) {
            if (ci.connected) {
              settled = true;
              clearInterval(timer);
              resolve();
            }
          } else if (!ci.connected && (ci.ws.readyState === 2 || ci.ws.readyState === 3)) {
            settled = true;
            clearInterval(timer);
            reject(new Error("WebSocket连接失败或已被关闭"));
          } else if (Date.now() - start > timeoutMs) {
            settled = true;
            clearInterval(timer);
            this.safeClose(ci.ws);
            reject(new Error("WebSocket连接超时"));
          }
        }, 100);
      });
    }
    /** 等待 Milky 首个事件完成端点映射（Milky /event 不响应 API 探测请求） */
    static waitForMilkyResolved(ci, timeoutMs = 15e3) {
      return new Promise((resolve, reject) => {
        const start = Date.now();
        const timer = setInterval(() => {
          if (ci.milkyResolved) {
            clearInterval(timer);
            resolve();
            return;
          }
          if (ci.manualClosed || ci.ws.readyState !== WS_OPEN) {
            clearInterval(timer);
            reject(new Error("Milky WebSocket 连接已断开，无法完成端点匹配"));
            return;
          }
          if (Date.now() - start > timeoutMs) {
            clearInterval(timer);
            reject(new Error("等待 Milky 首个事件超时"));
          }
        }, 100);
      });
    }
    /** 把 Milky 连接从临时 ID 转正为真实端点，返回是否成功 */
    static resolveMilkyConnection(ci, epId) {
      const eps = seal.getEndPoints();
      let found = false;
      for (let j = 0; j < eps.length; j++) {
        if (eps[j].userId === epId) {
          found = true;
          break;
        }
      }
      if (!found) {
        logger.warning(`未找到对应的端点: ${epId}，Milky 连接已关闭`);
        this.destroyConnection(ci.epId);
        return false;
      }
      if (ci.epId !== epId) {
        delete this.wsConnections[ci.epId];
      }
      ci.epId = epId;
      ci.protocol = "milky";
      ci.milkyResolved = true;
      ci.manualClosed = false;
      this.wsConnections[epId] = ci;
      this.urlMap[epId] = { url: ci.url, token: ci.token };
      this.protocolMap[epId] = "milky";
      logger.info(`Milky 连接完成端点映射: ${epId}`);
      return true;
    }
    /** 把 OB11 连接从临时 ID 转正为真实端点，返回是否成功（不写 protocolMap：缺省即 OB11） */
    static resolveOb11Connection(ci, epId) {
      const eps = seal.getEndPoints();
      let found = false;
      for (let j = 0; j < eps.length; j++) {
        if (eps[j].userId === epId) {
          found = true;
          break;
        }
      }
      if (!found) {
        logger.warning(`未找到对应的端点: ${epId}，OB11 连接已关闭`);
        this.destroyConnection(ci.epId);
        return false;
      }
      if (ci.epId !== epId) {
        delete this.wsConnections[ci.epId];
      }
      ci.epId = epId;
      ci.protocol = "ob11";
      ci.milkyResolved = true;
      ci.manualClosed = false;
      this.wsConnections[epId] = ci;
      this.urlMap[epId] = { url: ci.url, token: ci.token };
      logger.info(`OB11 连接完成端点映射: ${epId}`);
      return true;
    }
    /** 安全关闭连接（忽略已关闭/关闭中的状态） */
    static safeClose(ws) {
      try {
        if (ws && (ws.readyState === WS_OPEN || ws.readyState === WS_CONNECTING)) {
          ws.close();
        }
      } catch (e) {
        logger.error(`关闭WebSocket时出错: ${getErrorMessage(e)}`);
      }
    }
    /** 清理连接：标记手动关闭、取消重连定时器、关闭并移除 */
    static destroyConnection(epId) {
      const ci = this.wsConnections[epId];
      if (!ci) {
        return;
      }
      ci.manualClosed = true;
      if (ci.reconnectTimer) {
        clearTimeout(ci.reconnectTimer);
      }
      this.safeClose(ci.ws);
      delete this.wsConnections[epId];
    }
    /** 指数退避自动重连：1s、2s、4s……最多 30s */
    static scheduleReconnect(ci) {
      if (ci.reconnectTimer) {
        clearTimeout(ci.reconnectTimer);
      }
      const delay = Math.min(3e4, 1e3 * Math.pow(2, ci.reconnectAttempts));
      ci.reconnectAttempts++;
      logger.warning(`[${ci.epId}] WebSocket连接断开，${delay / 1e3}s 后尝试重连`);
      ci.reconnectTimer = setTimeout(() => {
        ci.reconnectTimer = void 0;
        const current = this.wsConnections[ci.epId];
        if (!current || current.manualClosed || !this.initDone) {
          return;
        }
        if (current.ws.readyState === WS_OPEN) {
          return;
        }
        logger.info(`[${ci.epId}] 正在重连: ${ci.url}`);
        this.wsConnections[ci.epId] = this.createWsConnection(
          ci.epId,
          ci.url,
          ci.token,
          current.reconnectAttempts,
          current.protocol
        );
      }, delay);
    }
    /**
     * 获取事件的简要描述信息，便于日志记录和调试
     * @param {OneBot11.Event} event OneBot 11 标准事件对象
     * @returns {string} 返回格式化后的事件描述字符串，内容根据事件类型自动拼接
     */
    static getEventDescription(event) {
      var _a;
      const { post_type, time, self_id } = event;
      let eventDesc = `时间:${new Date(time * 1e3).toLocaleString()}, 机器人:${self_id}, 事件类型:${post_type}`;
      switch (post_type) {
        case "message": {
          const messageEvent = event;
          const subType = messageEvent.message_type;
          eventDesc += `.${subType}`;
          if (subType === "group") {
            eventDesc += ` (群:${messageEvent.group_id}, 用户:${messageEvent.user_id})`;
          } else if (subType === "private") {
            eventDesc += ` (用户:${messageEvent.user_id})`;
          }
          if (ConfigManager.logLevel === "详细") {
            let msgContent = messageEvent.message;
            if (typeof msgContent === "object") {
              try {
                msgContent = JSON.stringify(msgContent);
              } catch (e) {
                msgContent = "[无法解析的消息对象]";
              }
            }
            eventDesc += ` 消息:${msgContent}`;
          }
          break;
        }
        case "notice": {
          const noticeEvent = event;
          const noticeType = noticeEvent.notice_type;
          eventDesc += `.${noticeType}`;
          if (noticeType === "group_upload") {
            eventDesc += ` (群:${noticeEvent.group_id}, 用户:${noticeEvent.user_id}, 文件:${((_a = noticeEvent.file) == null ? void 0 : _a.name) || "未知"})`;
          } else if (noticeType === "group_admin") {
            eventDesc += ` (群:${noticeEvent.group_id}, 用户:${noticeEvent.user_id}, 操作:${noticeEvent.sub_type})`;
          } else if (noticeType === "group_decrease" || noticeType === "group_increase") {
            eventDesc += ` (群:${noticeEvent.group_id}, 用户:${noticeEvent.user_id}, 操作:${noticeEvent.sub_type})`;
          } else if (noticeType === "group_ban") {
            eventDesc += ` (群:${noticeEvent.group_id}, 用户:${noticeEvent.user_id}, 时长:${noticeEvent.duration}s)`;
          } else if (noticeType === "friend_add") {
            eventDesc += ` (用户:${noticeEvent.user_id})`;
          } else if (noticeType === "group_recall") {
            eventDesc += ` (群:${noticeEvent.group_id}, 用户:${noticeEvent.user_id}, 消息ID:${noticeEvent.message_id})`;
          } else if (noticeType === "friend_recall") {
            eventDesc += ` (用户:${noticeEvent.user_id}, 消息ID:${noticeEvent.message_id})`;
          } else if (noticeType === "notify") {
            eventDesc += `.${noticeEvent.sub_type}`;
            if (noticeEvent.sub_type === "poke") {
              eventDesc += ` (群:${noticeEvent.group_id || "私聊"}, 戳一戳:${noticeEvent.user_id}->${noticeEvent.target_id})`;
            } else if (noticeEvent.sub_type === "lucky_king") {
              eventDesc += ` (群:${noticeEvent.group_id}, 红包王:${noticeEvent.target_id})`;
            } else if (noticeEvent.sub_type === "honor") {
              eventDesc += ` (群:${noticeEvent.group_id}, 群荣誉:${noticeEvent.honor_type}, 用户:${noticeEvent.user_id})`;
            }
          }
          break;
        }
        case "request": {
          const requestEvent = event;
          const requestType = requestEvent.request_type;
          eventDesc += `.${requestType}`;
          if (requestType === "friend") {
            eventDesc += ` (用户:${requestEvent.user_id}, 验证消息:"${requestEvent.comment}")`;
          } else if (requestType === "group") {
            eventDesc += `.${requestEvent.sub_type} (群:${requestEvent.group_id}, 用户:${requestEvent.user_id}, 消息:"${requestEvent.comment}")`;
          }
          break;
        }
        case "meta_event": {
          const metaEvent = event;
          const metaType = metaEvent.meta_event_type;
          eventDesc += `.${metaType}`;
          if (metaType === "lifecycle") {
            eventDesc += ` (子类型:${metaEvent.sub_type})`;
          } else if (metaType === "heartbeat") {
            eventDesc += ` (状态:${JSON.stringify(metaEvent.status)})`;
          }
          break;
        }
        default: {
          eventDesc += " (未知事件类型)";
        }
      }
      return eventDesc;
    }
    static handleEvent(epId, event) {
      switch (ConfigManager.eventLevel) {
        case "忽略": {
          break;
        }
        case "记录非消息": {
          if (event.post_type === "meta_event" || event.post_type === "message") break;
          const eventDesc = this.getEventDescription(event);
          logger.info(`[${epId}] 收到事件: ${eventDesc}`);
          if (ConfigManager.logLevel === "详细") {
            logger.info(`[${epId}] 完整事件数据: ${JSON.stringify(event, null, 2)}`);
          }
          break;
        }
        case "记录": {
          if (event.post_type === "meta_event") break;
          const eventDesc = this.getEventDescription(event);
          logger.info(`[${epId}] 收到事件: ${eventDesc}`);
          if (ConfigManager.logLevel === "详细") {
            logger.info(`[${epId}] 完整事件数据: ${JSON.stringify(event, null, 2)}`);
          }
          break;
        }
        case "调试": {
          const eventDesc = this.getEventDescription(event);
          logger.info(`[${epId}] 收到事件: ${eventDesc}`);
          if (ConfigManager.logLevel === "详细") {
            logger.info(`[${epId}] 完整事件数据: ${JSON.stringify(event, null, 2)}`);
          }
          break;
        }
      }
      this.emitEvent(epId, event);
    }
    /**
     * 创建一个 WebSocket 连接并管理其生命周期。
     * @param {string} epId   端点ID
     * @param {string} wsUrl  WebSocket 服务端地址
     * @param {string} token  Access Token
     * @param {number} reconnectAttempts 已重连次数（用于指数退避）
     * @returns {ConnectionInfo} 返回连接信息对象
     */
    static createWsConnection(epId, wsUrl, token = "", reconnectAttempts = 0, protocol = "unknown") {
      if (this.wsConnections[epId]) {
        logger.info(`${epId} 的WebSocket连接已存在，先关闭旧连接`);
        this.destroyConnection(epId);
      }
      const normalizedUrl = protocol === "milky" ? this.normalizeMilkyWsUrl(wsUrl) : wsUrl.trim();
      let connectionUrl = normalizedUrl;
      if (token) {
        const separator = connectionUrl.includes("?") ? "&" : "?";
        connectionUrl = `${connectionUrl}${separator}access_token=${encodeURIComponent(token)}`;
      }
      const ws = new WebSocket(connectionUrl);
      const ci = {
        epId,
        ws,
        url: normalizedUrl,
        token,
        protocol,
        connected: false,
        manualClosed: false,
        reconnectAttempts,
        reconnectTimer: void 0,
        apiCallbacks: /* @__PURE__ */ new Map(),
        // 初始一律未解析：OB11 等 echo、Milky 等首个事件后才完成端点映射
        milkyResolved: false
      };
      ws.onopen = function() {
        ci.connected = true;
        ci.reconnectAttempts = 0;
        logger.info(`[${ci.epId}] WebSocket连接成功: ${connectionUrl.replace(/access_token=[^&]*/, "access_token=***")}`);
      };
      ws.onmessage = function(event) {
        var _a;
        try {
          const data = JSON.parse(event.data);
          if (hasOwn(data, "echo") && ci.apiCallbacks.has(data.echo)) {
            ci.protocol = "ob11";
            const callback = ci.apiCallbacks.get(data.echo);
            ci.apiCallbacks.delete(data.echo);
            if (!callback) {
              logger.warning(`[${ci.epId}] 收到响应但找不到对应的回调: ${data.echo}`);
              return;
            }
            try {
              if (data.status === "ok") {
                callback.resolve(data.data || {});
              } else {
                callback.reject(new Error(`API调用失败: ${data.message || data.wording || "未知错误"}`));
              }
            } catch (error) {
              logger.error(`[${ci.epId}] 处理回调时发生错误:`, error);
            }
            if (data.status === "ok" && !ci.milkyResolved && ci.epId.startsWith("初始化连接")) {
              const userId = data.data && ((_a = data.data.user_id) != null ? _a : data.data.uin);
              if (userId !== void 0 && userId !== null) {
                _WSManager.resolveOb11Connection(ci, `${PLATFORM}:${userId}`);
              }
            }
          } else if (hasOwn(data, "post_type")) {
            ci.protocol = "ob11";
            if (!ci.milkyResolved && ci.epId.startsWith("初始化连接") && data.self_id !== void 0 && data.self_id !== null) {
              _WSManager.resolveOb11Connection(ci, `${PLATFORM}:${data.self_id}`);
            }
            _WSManager.handleEvent(ci.epId, data);
          } else if (hasOwn(data, "event_type")) {
            const milkyEvent = data;
            if (ci.protocol !== "ob11") {
              ci.protocol = "milky";
            }
            if (!ci.milkyResolved) {
              const epId2 = `${PLATFORM}:${milkyEvent.self_id}`;
              if (!_WSManager.resolveMilkyConnection(ci, epId2)) {
                return;
              }
            }
            const event2 = MilkyAdapter.toOneBot11Event(milkyEvent);
            if (event2) {
              _WSManager.handleEvent(ci.epId, event2);
            } else {
              logger.warning(`[${ci.epId}] Milky 事件未转换为 OB11: ${JSON.stringify(milkyEvent)}`);
            }
          } else if (hasOwn(data, "status")) {
            logger.info(`[${ci.epId}] 收到无echo的API响应: ${JSON.stringify(data)}`);
          } else {
            logger.warning(`[${ci.epId}] 收到未知格式消息: ${JSON.stringify(data)}`);
          }
        } catch (e) {
          logger.error(`[${ci.epId}] 解析WebSocket消息失败: ${getErrorMessage(e)}`);
        }
      };
      ws.onerror = function(event) {
        ci.connected = false;
        const extra = event && event.message ? `，信息: ${event.message}` : "";
        logger.error(`[${ci.epId}] WebSocket错误: readyState=${ws.readyState}${extra}`);
        if (ws.readyState === WS_OPEN || ws.readyState === WS_CONNECTING) {
          try {
            ws.close();
          } catch (e) {
          }
        }
      };
      ws.onclose = function(event) {
        ci.connected = false;
        if (event.code !== 1e3) {
          logger.warning(`[${ci.epId}] WebSocket异常关闭: ${event.code} ${event.reason}`);
        } else {
          logger.info(`[${ci.epId}] WebSocket正常关闭`);
        }
        ci.apiCallbacks.forEach((callback) => {
          callback.reject(new Error("WebSocket连接已关闭"));
        });
        ci.apiCallbacks.clear();
        if (!ci.manualClosed) {
          _WSManager.scheduleReconnect(ci);
        }
      };
      this.wsConnections[epId] = ci;
      return ci;
    }
    /**
     * 归一化 WebSocket 地址：无路径时按 Milky 文档补 `/event`，
     * 已有路径（含 `/event`）则原样保留；不影响 query 参数。
     * @example
     *   normalizeMilkyWsUrl('ws://h:8080')          -> 'ws://h:8080/event'
     *   normalizeMilkyWsUrl('ws://h:8080/')         -> 'ws://h:8080/event'
     *   normalizeMilkyWsUrl('ws://h:8080/event')    -> 'ws://h:8080/event'
     *   normalizeMilkyWsUrl('ws://h:8080/event?x=1')-> 'ws://h:8080/event?x=1'
     *   normalizeMilkyWsUrl('ws://h:8080/foo')      -> 'ws://h:8080/foo'
     */
    static normalizeMilkyWsUrl(url) {
      const trimmed = url.trim().replace(/\/+$/, "");
      const withoutQuery = trimmed.split("?")[0];
      const path = withoutQuery.replace(/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\/[^/]+/, "");
      if (path === "" || path === "/") {
        return `${trimmed}/event`;
      }
      return trimmed;
    }
    /**
     * 通过WebSocket调用OneBot 11 API。
     * @param {string} epId   端点ID（格式如 QQ:12345）
     * @param {string} action API方法名
     * @param {object} params API参数对象
     * @returns {Promise<object>} 返回API响应的data字段
     */
    static async callApiByWs(epId, action, params = {}, timeoutMs = this.apiTimeoutMs, retainCallbackOnTimeout = false) {
      const ci = this.wsConnections[epId];
      if (!ci || !ci.connected || ci.ws.readyState !== WS_OPEN) {
        throw new Error(`WebSocket连接未建立或已断开: ${epId}`);
      }
      return this.callApiByWsInternal(ci, action, params, timeoutMs, retainCallbackOnTimeout);
    }
    /** callApiByWs 的内部实现：直接持有 ConnectionInfo，不依赖按键查表（探测阶段用） */
    static async callApiByWsInternal(ci, action, params = {}, timeoutMs = this.apiTimeoutMs, retainCallbackOnTimeout = false) {
      if (!ci.connected || ci.ws.readyState !== WS_OPEN) {
        throw new Error(`WebSocket连接未建立或已断开: ${ci.epId}`);
      }
      return new Promise((resolve, reject) => {
        const echo = `api_${Date.now()}_${Math.random().toString(36).slice(2, 11)}`;
        const timeoutId = setTimeout(() => {
          if (ci.apiCallbacks.has(echo)) {
            if (!retainCallbackOnTimeout) {
              ci.apiCallbacks.delete(echo);
            }
            reject(new Error(`API调用超时 (${action})`));
          }
        }, timeoutMs);
        ci.apiCallbacks.set(echo, {
          resolve: (data) => {
            clearTimeout(timeoutId);
            resolve(data);
          },
          reject: (error) => {
            clearTimeout(timeoutId);
            reject(error);
          }
        });
        try {
          const requestData = {
            action,
            params,
            echo
          };
          logger.info(`[${ci.epId}] WebSocket发送API请求: ${JSON.stringify(requestData)}`);
          ci.ws.send(JSON.stringify(requestData));
        } catch (error) {
          clearTimeout(timeoutId);
          ci.apiCallbacks.delete(echo);
          reject(new Error(`发送API请求失败: ${getErrorMessage(error)}`));
        }
      });
    }
    /**
     * 关闭WebSocket连接
     * @param {string} epId 骰子的QQ号，如果不提供则关闭所有连接
     */
    static closeWs(epId) {
      if (epId) {
        if (this.wsConnections[epId]) {
          this.destroyConnection(epId);
          logger.info(`已关闭 ${epId} 的WebSocket连接`);
          return 1;
        } else {
          logger.warning(`${epId} 没有WebSocket连接`);
          return 0;
        }
      } else {
        let count = 0;
        Object.keys(this.wsConnections).forEach((id) => {
          this.destroyConnection(id);
          count++;
        });
        logger.info(`已关闭 ${count} 个WebSocket连接`);
        return count;
      }
    }
    /**
     * 获取WebSocket连接状态
     */
    static getWsStatus() {
      const status = {};
      Object.keys(this.wsConnections).forEach((epId) => {
        const ci = this.wsConnections[epId];
        status[epId] = {
          connected: ci.connected,
          url: ci.url,
          protocol: ci.protocol,
          readyState: ci.ws.readyState,
          reconnectAttempts: ci.reconnectAttempts
        };
      });
      return status;
    }
  };
  _WSManager.urlMap = {};
  _WSManager.initDone = false;
  _WSManager.initPromise = null;
  /** WebSocket API 调用超时（毫秒），可配置 */
  _WSManager.apiTimeoutMs = 1e4;
  /** OB11 探测超时：超过后按 Milky 处理（Milky /event 不响应 API 请求） */
  _WSManager.probeTimeoutMs = 3e3;
  /** Milky 等待首个事件完成端点映射的超时 */
  _WSManager.milkyResolveTimeoutMs = 15e3;
  /** 初始化探测连接的临时序号（OB11 组与 Milky 组共用，避免临时键冲突） */
  _WSManager.tempCounter = 0;
  _WSManager.wsConnections = {};
  _WSManager.eventDispatcherMap = {};
  /** 端点对应的协议：缺省/OB11 时不存在该键，Milky 时值为 'milky' */
  _WSManager.protocolMap = {};
  var WSManager = _WSManager;

  // src/net.ts
  var MILKY_TO_OB11_SHIMS = {
    get_impl_info: {
      action: "get_version_info",
      params: () => ({})
    },
    get_friend_info: {
      action: "get_friend_list",
      params: (req) => ({ no_cache: req.no_cache }),
      response: (raw, req) => {
        const list = Array.isArray(raw) ? raw : [];
        const friend = list.find((f) => String(f.user_id) === String(req.user_id));
        if (!friend) {
          throw new Error(`未找到好友: ${req.user_id}`);
        }
        return friend;
      }
    },
    get_user_profile: {
      action: "get_stranger_info",
      params: (req) => ({ user_id: req.user_id, no_cache: req.no_cache })
    },
    get_custom_face_url_list: {
      action: "fetch_custom_face",
      params: (req) => {
        var _a;
        return { count: (_a = req.count) != null ? _a : 48 };
      }
    },
    send_friend_nudge: {
      action: "friend_poke",
      params: (req) => ({ user_id: req.user_id })
    },
    send_group_nudge: {
      action: "group_poke",
      params: (req) => ({ group_id: req.group_id, user_id: req.user_id })
    },
    send_profile_like: {
      action: "send_like",
      params: (req) => {
        var _a;
        return { user_id: req.user_id, times: (_a = req.count) != null ? _a : 1 };
      }
    },
    delete_friend: {
      action: "delete_friend",
      params: (req) => ({ user_id: req.user_id })
    },
    set_group_avatar: {
      action: "set_group_portrait",
      params: (req) => ({ group_id: req.group_id, file: req.image_uri })
    },
    set_avatar: {
      action: "set_qq_avatar",
      params: (req) => ({ file: req.image_uri })
    },
    set_nickname: {
      action: "set_qq_profile",
      params: (req) => ({ nickname: req.nickname })
    },
    set_bio: {
      action: "set_qq_profile",
      params: (req) => ({ personal_note: req.bio })
    },
    set_group_member_card: {
      action: "set_group_card",
      params: (req) => {
        var _a;
        return { group_id: req.group_id, user_id: req.user_id, card: (_a = req.card) != null ? _a : "" };
      }
    },
    set_group_member_special_title: {
      action: "set_group_special_title",
      params: (req) => {
        var _a;
        return { group_id: req.group_id, user_id: req.user_id, special_title: (_a = req.special_title) != null ? _a : "" };
      }
    },
    set_group_member_admin: {
      action: "set_group_admin",
      params: (req) => {
        var _a;
        return { group_id: req.group_id, user_id: req.user_id, enable: (_a = req.is_set) != null ? _a : true };
      }
    },
    set_group_member_mute: {
      action: "set_group_ban",
      params: (req) => {
        var _a;
        return { group_id: req.group_id, user_id: req.user_id, duration: (_a = req.duration) != null ? _a : 30 * 60 };
      }
    },
    set_group_whole_mute: {
      action: "set_group_whole_ban",
      params: (req) => {
        var _a;
        return { group_id: req.group_id, enable: (_a = req.is_mute) != null ? _a : true };
      }
    },
    kick_group_member: {
      action: "set_group_kick",
      params: (req) => {
        var _a;
        return {
          group_id: req.group_id,
          user_id: req.user_id,
          reject_add_request: (_a = req.reject_add_request) != null ? _a : false
        };
      }
    },
    quit_group: {
      action: "set_group_leave",
      params: (req) => ({ group_id: req.group_id })
    },
    get_group_announcements: {
      action: "_get_group_notice",
      params: (req) => ({ group_id: req.group_id })
    },
    send_group_announcement: {
      action: "_send_group_notice",
      params: (req) => {
        var _a;
        return { group_id: req.group_id, content: req.content, image: (_a = req.image_uri) != null ? _a : req.image };
      }
    },
    delete_group_announcement: {
      action: "_del_group_notice",
      params: (req) => {
        var _a;
        return { group_id: req.group_id, notice_id: (_a = req.announcement_id) != null ? _a : req.notice_id };
      }
    },
    get_group_essence_messages: {
      action: "get_essence_msg_list",
      params: (req) => {
        var _a, _b;
        return { group_id: req.group_id, page_size: (_b = (_a = req.page_size) != null ? _a : req.limit) != null ? _b : 50 };
      }
    },
    set_group_essence_message: {
      action: (req) => req.is_set === false ? "delete_essence_msg" : "set_essence_msg",
      params: (req) => {
        var _a;
        return { message_id: (_a = req.message_seq) != null ? _a : req.message_id };
      }
    },
    send_group_message_reaction: {
      action: "set_msg_emoji_like",
      params: (req) => {
        var _a, _b;
        return { message_id: (_a = req.message_seq) != null ? _a : req.message_id, emoji_id: req.reaction, set: (_b = req.is_add) != null ? _b : true };
      }
    },
    mark_message_as_read: {
      action: (req) => req.message_scene === "group" ? "mark_group_msg_as_read" : "mark_private_msg_as_read",
      params: (req) => {
        const params = {};
        if (req.message_scene === "group") {
          params.group_id = req.peer_id;
        } else {
          params.user_id = req.peer_id;
        }
        if (req.message_seq !== void 0) {
          params.message_id = req.message_seq;
        }
        return params;
      }
    },
    get_history_messages: {
      action: (req) => req.message_scene === "group" ? "get_group_msg_history" : "get_friend_msg_history",
      params: (req) => {
        var _a, _b;
        const base = { message_seq: (_a = req.start_message_seq) != null ? _a : void 0, count: (_b = req.limit) != null ? _b : 20 };
        return req.message_scene === "group" ? __spreadValues({ group_id: req.peer_id }, base) : __spreadValues({ user_id: req.peer_id }, base);
      }
    },
    get_group_notifications: {
      action: "get_group_system_msg",
      params: (req) => {
        var _a;
        return { count: (_a = req.limit) != null ? _a : 50 };
      }
    },
    upload_private_file: {
      action: "upload_private_file",
      params: (req) => {
        var _a, _b;
        const file = (_a = req.file_uri) != null ? _a : req.file;
        const name = (_b = req.file_name) != null ? _b : req.name;
        if (!req.user_id) {
          throw new Error("upload_private_file 需要 user_id");
        }
        if (!file) {
          throw new Error("upload_private_file 需要 file/file_uri");
        }
        if (!name) {
          throw new Error("upload_private_file 需要 name/file_name");
        }
        return { user_id: req.user_id, file, name };
      }
    },
    upload_group_file: {
      action: "upload_group_file",
      params: (req) => {
        var _a, _b, _c;
        const file = (_a = req.file_uri) != null ? _a : req.file;
        const name = (_b = req.file_name) != null ? _b : req.name;
        if (!req.group_id) {
          throw new Error("upload_group_file 需要 group_id");
        }
        if (!file) {
          throw new Error("upload_group_file 需要 file/file_uri");
        }
        if (!name) {
          throw new Error("upload_group_file 需要 name/file_name");
        }
        return {
          group_id: req.group_id,
          file,
          name,
          folder: (_c = req.parent_folder_id) != null ? _c : ""
        };
      }
    },
    get_private_file_download_url: {
      action: "get_private_file_url",
      params: (req) => {
        var _a;
        if (!req.file_id) {
          throw new Error("get_private_file_download_url 需要 file_id");
        }
        return {
          file_id: req.file_id,
          // 与 Milky 端点一致：file_id 本身含文件 hash，缺省时透传兜底
          file_hash: (_a = req.file_hash) != null ? _a : req.file_id
        };
      }
    },
    get_group_file_download_url: {
      action: "get_group_file_url",
      params: (req) => {
        if (!req.group_id) {
          throw new Error("get_group_file_download_url 需要 group_id");
        }
        if (!req.file_id) {
          throw new Error("get_group_file_download_url 需要 file_id");
        }
        return { group_id: req.group_id, file_id: req.file_id };
      }
    },
    get_group_files: {
      action: (req) => req.parent_folder_id ? "get_group_files_by_folder" : "get_group_root_files",
      params: (req) => req.parent_folder_id ? { group_id: req.group_id, folder_id: req.parent_folder_id } : { group_id: req.group_id }
    },
    move_group_file: {
      action: "move_group_file",
      params: (req) => {
        var _a, _b;
        return {
          group_id: req.group_id,
          file_id: req.file_id,
          current_parent_directory: (_a = req.parent_folder_id) != null ? _a : "",
          target_parent_directory: (_b = req.target_folder_id) != null ? _b : ""
        };
      }
    },
    rename_group_file: {
      action: "rename_group_file",
      params: (req) => {
        var _a, _b;
        return {
          group_id: req.group_id,
          file_id: req.file_id,
          current_parent_directory: (_a = req.parent_folder_id) != null ? _a : "",
          new_name: (_b = req.new_file_name) != null ? _b : req.new_name
        };
      }
    },
    delete_group_file: {
      action: "delete_group_file",
      params: (req) => ({ group_id: req.group_id, file_id: req.file_id })
    },
    create_group_folder: {
      action: "create_group_file_folder",
      params: (req) => ({ group_id: req.group_id, folder_name: req.folder_name })
    },
    delete_group_folder: {
      action: "delete_group_folder",
      params: (req) => ({ group_id: req.group_id, folder_id: req.folder_id })
    },
    get_forwarded_messages: {
      action: "get_forward_msg",
      params: (req) => {
        var _a;
        return { id: (_a = req.forward_id) != null ? _a : req.id };
      }
    }
  };
  var MILKY_NO_OB11_EQUIVALENT = /* @__PURE__ */ new Set([
    "get_peer_pins",
    "set_peer_pin",
    "get_friend_requests",
    "accept_friend_request",
    "reject_friend_request",
    "accept_group_request",
    "reject_group_request",
    "accept_group_invitation",
    "reject_group_invitation",
    "get_resource_temp_url",
    "persist_group_file",
    "rename_group_folder"
  ]);
  function decodeQueryComponent(value) {
    const normalized = value.replace(/\+/g, " ");
    try {
      return decodeURIComponent(normalized);
    } catch (e) {
      return normalized;
    }
  }
  var _NetworkClient = class _NetworkClient {
    static async init() {
      ConfigManager.cache = {};
      HTTPManager.initDone = false;
      WSManager.initDone = false;
      await Promise.all([
        HTTPManager.init(),
        WSManager.init()
      ]);
    }
    static messageId(input) {
      if (input === null || typeof input !== "object") {
        throw new Error("messageId 需要传入对象参数，如 { scene, id, msgid } 或 { message_id }");
      }
      if (input.message_id !== void 0 && input.message_id !== null && input.message_id !== "") {
        const messageId = Number(input.message_id);
        if (!Number.isSafeInteger(messageId) || messageId < 0) {
          throw new Error(`messageId 反向转换需要合法的 message_id，收到: ${input.message_id}`);
        }
        const ref = MilkyAdapter.conversations.getRef(messageId);
        if (!ref) {
          throw new Error(
            `无法还原 message_id=${input.message_id}：该会话未由插件建立过索引（需先收到/发送过该会话的消息，或先做一次正向转换）`
          );
        }
        return { scene: ref.scene, id: ref.peerId, msgid: ref.seq };
      }
      const scene = input.scene;
      if (scene !== "friend" && scene !== "group" && scene !== "temp") {
        throw new Error(`messageId 正向转换需要 scene 为 friend/group/temp，收到: ${scene}`);
      }
      const peerId = Number(input.id !== void 0 ? input.id : input.peer_id);
      if (!Number.isSafeInteger(peerId) || peerId <= 0) {
        throw new Error(`messageId 正向转换需要合法的 id/peer_id，收到: ${input.id !== void 0 ? input.id : input.peer_id}`);
      }
      const seq = Number(input.msgid !== void 0 ? input.msgid : input.message_seq);
      if (!Number.isSafeInteger(seq) || seq < 0) {
        throw new Error(`messageId 正向转换需要合法的 msgid/message_seq，收到: ${input.msgid !== void 0 ? input.msgid : input.message_seq}`);
      }
      return MilkyAdapter.conversations.getMessageId(scene, peerId, seq);
    }
    static async getEventDispatcher(ext, key = "default") {
      const ws = await WSManager.getEventDispatcher(ext, key);
      logger.info(`插件[${ext.name}] 正在获取 EventDispatcher 实例`);
      return ws;
    }
    /**
     * 调用网络API（统一接口）
     * @param {string} epId 骰子的QQ号，格式如 QQ:12345
     * @param {string} method 调用的方法名，如 get_login_info
     * @param {any} data 调用的方法的参数，默认为null
     * @returns {Promise<any>} 返回API调用结果
     */
    static async callApi(epId, method, data = null) {
      if (method.indexOf("?") !== -1) {
        const parts = method.split("?");
        method = parts[0];
        const query = parts[1];
        const queryParams = {};
        const pairs = query.split("&");
        for (let i = 0; i < pairs.length; i++) {
          if (!pairs[i]) {
            continue;
          }
          const eqIndex = pairs[i].indexOf("=");
          const rawKey = eqIndex >= 0 ? pairs[i].substring(0, eqIndex) : pairs[i];
          const rawValue = eqIndex >= 0 ? pairs[i].substring(eqIndex + 1) : "";
          const key = decodeQueryComponent(rawKey);
          const value = decodeQueryComponent(rawValue);
          if (key === "__proto__" || key === "constructor" || key === "prototype") {
            logger.warning(`忽略不安全的参数名: ${key}`);
            continue;
          }
          queryParams[key] = value;
        }
        if (data === null) {
          data = queryParams;
        } else {
          for (const k in queryParams) {
            data[k] = queryParams[k];
          }
        }
      }
      if (!HTTPManager.initDone) {
        await HTTPManager.init();
      }
      if (!WSManager.initDone) {
        await WSManager.init();
      }
      if (!hasOwn(HTTPManager.urlMap, epId) && !hasOwn(WSManager.urlMap, epId)) {
        logger.error(`未找到网络地址: ${epId}，请检查配置`);
        logger.info(`当前可用的端点: ${Object.keys(HTTPManager.urlMap).concat(Object.keys(WSManager.urlMap)).join(", ")}`);
        throw new Error(`未找到网络地址: ${epId}，请检查配置`);
      }
      const isMilky = HTTPManager.protocolMap[epId] === "milky" || WSManager.protocolMap[epId] === "milky";
      if (!isMilky && isMilkyExtraApi(method)) {
        const req = data || {};
        const shim = MILKY_TO_OB11_SHIMS[method];
        if (!shim) {
          const hint = MILKY_NO_OB11_EQUIVALENT.has(method) ? "（NapCat 源码未核实到等价 action）" : "";
          throw new Error(`Milky 特有 API ${method} 在 OB11/NapCat 端无对应能力${hint}`);
        }
        const action = typeof shim.action === "function" ? shim.action(req) : shim.action;
        const params = shim.params(req);
        logger.info(`[OB11兼容] ${method} -> ${action}，请求参数: ${JSON.stringify(params)}`);
        if (action !== method) {
          const raw = await this.callApi(epId, action, params);
          return shim.response ? shim.response(raw, req) : raw;
        }
        method = action;
        data = params;
      }
      try {
        if (hasOwn(HTTPManager.urlMap, epId)) {
          const { url, token } = HTTPManager.urlMap[epId];
          if (HTTPManager.protocolMap[epId] === "milky") {
            const reqData = data || {};
            const mapped = MilkyAdapter.mapApi(method, reqData);
            logger.info(`[Milky] 请求地址: ${url}/api/${mapped.api}，请求方法: ${method}，请求参数: ${JSON.stringify(mapped.params)}`);
            const raw = await HTTPManager.fetchMilkyApi(url, token, mapped.api, mapped.params);
            return MilkyAdapter.mapApiResponse(method, raw, reqData);
          }
          logger.info(`请求地址: ${url}，请求方法: ${method}，请求参数: ${JSON.stringify(data)}`);
          return await HTTPManager.fetchData(`${url}/${method}`, token, data);
        } else {
          if (WSManager.protocolMap[epId] === "milky") {
            throw new Error(`Milky WebSocket 仅用于事件推送，epId=${epId} 未配置 HTTP 地址，无法调用 API`);
          }
          const { url } = WSManager.urlMap[epId];
          logger.info(`请求地址: ${url}，请求方法: ${method}，请求参数: ${JSON.stringify(data)}`);
          return await WSManager.callApiByWs(epId, method, data || {});
        }
      } catch (error) {
        logger.error(`网络API调用失败: ${getErrorMessage(error)}`);
        throw error;
      }
    }
  };
  /** 兼容旧版本HTTP依赖 */
  _NetworkClient.getData = _NetworkClient.callApi;
  /** 兼容旧版本2.0.1 */
  _NetworkClient.getWs = _NetworkClient.getEventDispatcher;
  var NetworkClient = _NetworkClient;

  // src/index.ts
  function main() {
    ConfigManager.registerConfig();
    const ext = ConfigManager.ext;
    const cmd = seal.ext.newCmdItemInfo();
    cmd.name = "net";
    cmd.help = `ob11/milky网络连接依赖帮助:
.net init 初始化ob11网络连接依赖
.net close [epId] 关闭WebSocket连接，不指定epId则关闭所有
.net status 查看WebSocket连接状态
.net <方法>
.net msgid --scene=<friend/group/temp> --id=<peer_id> --msgid=<message_seq>   Milky消息引用转OB11 message_id
.net msgid --message_id=<ob11_id>   OB11 message_id还原Milky消息引用
--<参数名>=<参数>

插件设置中按三组配置连接：OB11 HTTP 配置、OB11 WS 配置、Milky 配置；
每组为模板配置，每个元素（面板上每个文本框）一个 JSON：{"url":"...","token":"..."}，token 无鉴权可缺省；
Milky 的 HTTP /api 与 WS /event 共用地址与 token，填根地址即可。

示例:
.net get_login_info
.net send_group_msg --group_id=123456 --message=测试消息
.net close QQ:12345
.net close
.net status`;
    cmd.solve = (ctx, msg, cmdArgs) => {
      if (ctx.privilegeLevel < 100) {
        seal.replyToSender(ctx, msg, seal.formatTmpl(ctx, "核心:提示_无权限"));
        return seal.ext.newCmdExecuteResult(true);
      }
      const epId = ctx.endPoint.userId;
      const ret = seal.ext.newCmdExecuteResult(true);
      const method = cmdArgs.getArgN(1);
      switch (method) {
        case "init": {
          NetworkClient.init().then(() => {
            seal.replyToSender(ctx, msg, "ob11网络连接依赖初始化完成");
          }).catch((error) => {
            seal.replyToSender(ctx, msg, `初始化失败: ${error.message}`);
          });
          return ret;
        }
        case "close": {
          const targetEpId = cmdArgs.getArgN(2);
          const count = WSManager.closeWs(targetEpId);
          if (targetEpId) {
            seal.replyToSender(ctx, msg, count > 0 ? `已关闭 ${targetEpId} 的连接` : `${targetEpId} 没有连接`);
          } else {
            seal.replyToSender(ctx, msg, `已关闭 ${count} 个WebSocket连接`);
          }
          return ret;
        }
        case "status": {
          const status = WSManager.getWsStatus();
          const statusText = Object.keys(status).length > 0 ? JSON.stringify(status, null, 2) : "没有WebSocket连接";
          seal.replyToSender(ctx, msg, `WebSocket连接状态:
${statusText}`);
          return ret;
        }
        case "msgid": {
          try {
            const result = NetworkClient.messageId(parseKwargs(cmdArgs.kwargs));
            seal.replyToSender(ctx, msg, JSON.stringify(result, null, 2));
          } catch (error) {
            seal.replyToSender(ctx, msg, `转换失败: ${getErrorMessage(error)}`);
          }
          return ret;
        }
        case "":
        case "help": {
          ret.showHelp = true;
          return ret;
        }
        default: {
          const data = parseKwargs(cmdArgs.kwargs);
          NetworkClient.callApi(epId, method, data).then((result) => {
            seal.replyToSender(ctx, msg, JSON.stringify(result, null, 2));
          }).catch((error) => {
            seal.replyToSender(ctx, msg, `调用失败: ${error.message}`);
          });
          return ret;
        }
      }
    };
    ext.cmdMap["net"] = cmd;
    globalThis.net = NetworkClient;
    const extHTTP = seal.ext.find("HTTP依赖");
    if (!extHTTP) {
      globalThis.http = NetworkClient;
    }
  }
  main();
})();
