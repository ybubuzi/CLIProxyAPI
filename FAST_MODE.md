# Codex Fast Mode 使用说明（force-upstream-websocket 版）

本版本基于 [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) v8.0.44，新增 **`codex.force-upstream-websocket`** 配置项，解决 HTTP/SSE 客户端无法使用 Fast mode 的问题（对应 [issue #4586](https://github.com/router-for-me/CLIProxyAPI/issues/4586)）。

---

## 原理：Fast 生效需要两个条件

```
Fast 生效 = ① tier=priority 信号  +  ② WebSocket 上游传输通道
```

| 条件 | 由什么控制 |
|---|---|
| ① tier 信号 | 请求体 `service_tier: priority`（来源：服务器注入 **或** 客户端自己发） |
| ② WS 通道 | 凭证的 `websockets` 属性 **或** 本版本新增的 `force-upstream-websocket` |

官方 CLI（ChatGPT 订阅模式）只在 WebSocket 传输上携带 `X-Codex-Routing-Hint: model=...;tier=priority`，上游对 HTTP/SSE 通道的 priority 信号不生效。官方 CPA 的 WS 上游只对"下游也是 WebSocket"的连接开放（`/v1/ws`），普通 HTTP 客户端（Codex CLI、magpie、OpenCode 等）永远走 SSE——**这就是 fast 不生效的原因**。

本版本的 `force-upstream-websocket` 打破了这个限制：HTTP/SSE 客户端也能升级到 WS 上游。

---

## 开启 Fast

三步，改完存盘即生效（CPA 支持配置热重载，无需重启）。

### 第 1 步：凭证开启 WebSocket

在管理面板（CPAMP / CPAMC）的凭证编辑页打开 websockets 开关；或直接在 auth JSON 文件（`auth-dir` 下对应账号的 `.json`）里加：

```json
{
  "websockets": true
}
```

> 注意：这是 `codexWebsocketsEnabled()` 判定的**前提条件**，不开启它，后面两步全部无效。

### 第 2 步：服务器开启 force-upstream-websocket

`config.yaml`（legacy 根级写法）：

```yaml
codex:
  force-upstream-websocket: true
```

v8 布局写法（`config-version: 8` 的文件）：

```yaml
upstream:
  codex:
    force-upstream-websocket: true
```

等价旧路径：`oauth.providers.codex.force-upstream-websocket`。

### 第 3 步：注入 tier 信号（二选一）

**方案 A：服务器统一注入**（所有 gpt-* codex 请求自动带 priority）——legacy 根级写法：

```yaml
payload:
  override:
    - models:
        - name: "gpt-*"
          protocol: "codex"
      params:
        "service_tier": "priority"
```

> v8 布局写法：`requests.payload.override`（同样的规则内容）。

**方案 B：客户端自己发**（更灵活，按需控制）——客户端请求体里带 `"service_tier": "priority"`（Codex 风格 `"fast"` 也可以，CPA 会归一化为 priority）。Codex CLI 在 `~/.codex/config.toml` 里设：

```toml
service_tier = "fast"    # 或 "priority"，两者等价
```

方案 B 无需服务器 payload 配置。

---

## 关闭 Fast

删除第 3 步的注入即可：

- 方案 A：删除 `payload.override` 配置块（或注释掉），存盘热重载即生效
- 方案 B：客户端把 `service_tier` 改回 `"default"` 或删掉该行

第 1、2 步的 WebSocket 通道配置**无需动**——它们只是传输通道选择，没有 tier 信号时不会产生 fast。保留 WS 通道还能减少上游对 SSE 的限流敏感度。

**想连 WS 通道也一并关掉**：把凭证的 `websockets` 设为 `false`（面板开关或 auth JSON），即可回到纯 HTTP/SSE 上游。

---

## 开关速查表

| 想要的效果 | payload 注入（或客户端 tier） | 凭证 websockets | force-upstream-websocket |
|---|---|---|---|
| Fast 开（服务器统一） | ✅ 需要 | ✅ true | ✅ true |
| Fast 开（客户端按需） | 不配 | ✅ true | ✅ true |
| Fast 关 | ❌ 删除 | 任意 | 任意 |
| 完全回到官方行为 | ❌ 删除 | ❌ false | ❌ false（或不配） |

---

## 如何验证 Fast 是否生效

**重要：不要看响应里的 `service_tier` 字段。** OpenAI 官方已确认（[openai/codex#14204](https://github.com/openai/codex/issues/14204)）：ChatGPT 订阅模式下 Fast 由服务端路由处理，`response.service_tier` 回显 `"default"` **不代表 Fast 没生效**，这个字段不是端到端可验证的。

**方法一：看请求日志（确认信号已送达）**

```yaml
request-log: true
```

发一个 codex 请求后，在 `auth-dir/logs/` 里最新的 `v1-responses-*.log` 检查：

```
Upstream Transport: websocket                              ← 通道正确
X-Codex-Routing-Hint: model=<模型名>;tier=priority         ← tier 信号送达
```

验证完把 `request-log` 关回 `false`。

**方法二：TTFT 实测（确认速度提升）**

同账号同模型，交替发"带 tier=priority hint"与"不带 tier"的请求各 3 次，比较首 token 延迟（TTFT）。Fast 生效时 priority 组平均快 30%+ 且方差显著更小（实测参考：1.08s vs 1.72s）。

---

## FAQ

**Q: 响应里 `service_tier` 一直是 `"default"`，是不是没生效？**
不是。这是上游的正常回显行为，验证请用上面两种方法。官方原话："seeing `response.service_tier = \"default\"` does not mean Fast was ignored."

**Q: Fast 消耗什么？**
订阅额度 2.5×（Standard 速率）。已购 credits / Enterprise 按量付费为 2×。对订阅用户：Pro/Plus/ProLite 均可用，无套餐门槛（Ultrafast 才限 Pro $500/Enterprise/Edu）。

**Q: 凭证的 websockets=false 时，force-upstream-websocket 还有用吗？**
没有。`codexWebsocketsEnabled(auth)` 是前提条件，凭证关闭 websockets 会直接短路整个 WS 路径（包括 force 分支），一律回退 HTTP/SSE。

**Q: 为什么 body 里有 `service_tier` 还要 Routing-Hint 头？**
CPA 的 tier→hint 映射会自动完成（`applyCodexRoutingHint`）：body 有 tier，hint 自动带 tier。原生 CLI 的形态是"body 不带 tier、只走 hint"，两者实测等效，本版本同时发送 body 字段和 hint 头，不影响结果。

**Q: 支持哪些模型？**
官方支持列表：GPT-6.1 Sol / GPT-6.1 Terra / GPT-6 Astra / GPT-6 Sol / GPT-6 Luna（GPT-5.6 为 1.5× 加速）。详见 [官方 Speed 文档](https://learn.chatgpt.com/docs/agent-configuration/speed)。

---

## 本版本改动明细

相对上游 v8.0.44，共 4 文件 +151 行（commit `bf3dbc7`，分支 `feat/force-upstream-websocket`）：

| 文件 | 改动 |
|---|---|
| `internal/config/config_types.go` | `CodexConfig` 新增 `ForceUpstreamWebsocket` 字段 |
| `internal/config/config_v8.go` | 注册 v8 路径 `upstream.codex.force-upstream-websocket` |
| `internal/runtime/executor/codex_websockets_executor.go` | `CodexAutoExecutor` 路由条件：WS 下游 **或** force 开启 → WS 上游 |
| `internal/runtime/executor/codex_force_websocket_test.go` | 新增 6 组路由测试 + nil 安全测试 |

默认 `force-upstream-websocket: false`，不配置时行为与官方完全一致。

**构建注意**：Linux 版必须 `CGO_ENABLED=1`（插件宿主依赖 cgo），否则所有插件无法加载。官方 release 的 no-plugin 变体不支持本功能所需的插件生态。
