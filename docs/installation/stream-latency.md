# 流式首字：采样、按渠道灰度与边缘代理

**首字指客户端读到的首个非空正文增量**（空白字符也是正文），而不是收到 HTTP 200、`: PING`、Chat 的空角色帧或 Responses 的 `response.created`。工具参数和单独的推理增量不算正文。非流式接口仍返回原协议的完整结果。

## 启用和回滚

- **观测**：在网关进程设置 `RELAY_STREAM_TIMING_PERCENT=1` 并滚动重启，即采样约 1% 的文本流请求。默认 `0` 不采样；隔离的测试实例可设 `100`。不用打开 `DEBUG`。
- **减少 Chat 的回写等待**：通过现有管理员渠道更新接口，在目标渠道的 **`setting` JSON** 中合并 `"eager_chat_text": true`；不是 `other_settings`，不要用单字段对象覆盖该渠道已有的代理、系统提示词等配置。仅影响该渠道后续请求；删掉此字段或设回 `false` 即回滚，默认仍用旧路径。
- 仅对同格式 OpenAI Chat 的单选项普通正文立即发送。待发角色帧仍按原顺序输出；混合/终止/usage 帧、音频、强制格式、思考转正文及跨协议转换不走该快捷路径。正文就算是最后一帧也只发一次，usage、工具用量和结束事件仍按原规则处理。

## 如何读日志

按 request ID 查 `relay_stream_timing`：一个请求的每次路由尝试单独一条，按 `attempt` 排序。数值是**微秒**；`-1` 表示未观察到、协议/传输未覆盖或阶段不可比，不能当成 0。分段采样不替换原有 FRT、看板数据或选路指标。日志不包含正文、请求头、完整 URL 或密钥。

| 字段 | 含义 |
| --- | --- |
| `ingress_to_dispatch_us` | 进入 relay 中间件到调用上游 HTTP 客户端；首试含鉴权、读取请求、选渠、价格/预扣和协议适配。后续尝试还含之前的等待。 |
| `attempt_to_dispatch_us` | 单次尝试开始到发上游，适合看重试本身。 |
| `dispatch_to_conn_us` / `conn_reused` | 到首次获取上游连接的时间（含池等待/建连）；`1` 复用、`0` 新建、`-1` 未观测。 |
| `dispatch_to_headers_us` | 到收到 HTTP 响应头；并非上游首字。 |
| `dispatch_to_text_us` | 到公共 SSE 扫描器读到首个正文增量，包含网络、上游排队及生成。 |
| `text_to_flush_us` | 从读到上游正文到本机完成首个下游正文 Flush。 |
| `ingress_to_flush_us` | 网关内的总耗时；不含客户端到边缘的传输和客户端渲染。 |

正文识别覆盖共享 SSE 路径上的 OpenAI Chat、Responses `output_text.delta`、Claude 文本块、Gemini 非 thought 的 text part。对只发终态而无正文增量的流、使用厂商专用传输的渠道，不推测缺失的首字。若配置把推理重写成正文，应按实际下游格式看，不能直接与原生正文样本混用。`status` 是该次 relay 尝试的结果状态，不是流内每个事件的状态。

## 对照测量

从同一用户侧探针交替请求「直连上游」及「中转域名」；在中转机上交替请求「同一上游」及「本机网关」。固定上游渠道/模型、最终 prompt、缓存状态、推理参数、并发和输出上限。读取 SSE **正文**并继续读到正常终态，避免因探针取消而改变结算。分别统计首试/重试、冷/热连接的 P50 和 P95，以及错误率与 usage；不要把 `curl -w '%{time_starttransfer}'` 当首字。公网探针和真实上游可能产生费用，不要镜像真实用户请求做测试。

- 只有 `dispatch_to_conn_us` 在新连接时高：检查机房线路、出口代理、DNS、TCP/TLS。默认已有共享连接池；不要为了测试关闭证书校验。只有热连接在高并发时仍排队，才按渠道试现有 `http2_connection_shards`，并对照连接复用率。
- 请求头很快但 `dispatch_to_text_us` 很慢：先用相同模型、输入直连同一上游测。预填充/推理等待不能通过发空帧消除。
- `text_to_flush_us` 低、本机流正常而公网成批到达：逐跳检查反向代理、CDN、客户端 SDK 缓冲。有些代理忽略 `X-Accel-Buffering: no`。例如在 **已有的文本 API Nginx location** 内按需合并：

  ```nginx
  proxy_http_version 1.1;
  proxy_set_header Connection "";
  proxy_buffering off;
  proxy_cache off;
  gzip off;
  ```

  保留已有的 `proxy_pass`、认证、可信转发头、超时和请求体大小限制，不新建会绕过这些规则的 location，不套用到 WebSocket 或全站静态资源。CDN 对 SSE 不缓存、不合并响应；修改前后均用正文到达时间验收。
- 只有大输入的 `ingress_to_dispatch_us` 高：核对本地磁盘 spool、token 估算和媒体读取。磁盘选模现使用短生命周期的只读文件映射，不省略完整 JSON 校验，也不丢弃可供重试的请求体。
- 短输入也慢：看网关到 Redis/数据库的延迟、连接池等待、赠金查询执行计划。赠金余额和到期条件在同一条查询以数据库时间判定，现有用户+状态索引可用，不以陈旧缓存授权或放宽预扣。
