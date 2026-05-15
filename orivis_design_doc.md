# Orivis / 观澜：多环境可用性观测平台设计文档

> 项目暂名：**Orivis / 观澜**  
> 定位：面向多环境、多区域、多探针的服务可用性观测平台  
> 当前版本：V0.1 设计草案

---

## 1. 背景

Uptime Kuma 这类工具非常适合单实例、单环境的可用性监控，但在企业内部多环境、多机房、多网络区域的场景下，会遇到几个明显问题：

1. 缺少原生的多 Agent / 多 Probe 模型。
2. 不方便区分生产、测试、预发、内网、外网、区域机房等不同环境。
3. 无法很好地表达“从不同区域看同一个服务”的可用性差异。
4. 告警容易重复，缺少多探针结果聚合和降噪。
5. 配置更多依赖 UI 操作，不利于 GitOps / 配置即代码。
6. 高可用、多数据库、多部署方式支持不足。

因此，Orivis 的目标不是简单复制 Uptime Kuma，而是构建一个适合企业内部使用的“多环境可用性观测平台”。

---

## 2. 目标

### 2.1 核心目标

1. 支持多 Agent 分布式探测。
2. Agent 支持多种运行环境，优先支持 Docker 和本机运行。
3. 主服务支持多种数据库后端。
4. 首版 UI 不引入复杂前端构建工具，采用服务端模板渲染。
5. 支持 HTTP、TCP、Ping、DNS、TLS 证书等基础监测。
6. 支持环境、区域、探针、监控项的统一建模。
7. 支持多探针结果聚合，避免单点误判。
8. 支持基础告警、状态页和历史结果查询。

### 2.2 非目标

V1 阶段不追求以下能力：

1. 不做完整 Prometheus 替代品。
2. 不做完整 APM / Trace 系统。
3. 不做复杂前端 SPA。
4. 不做复杂低代码看板。
5. 不做大规模多租户 SaaS。
6. 不做 Kubernetes 强依赖。
7. 不做复杂机器学习异常检测。

---

## 3. 产品定位

Orivis 关注的是“服务是否可用、从哪里看不可用、不可用是否真实、应该如何通知”。

它更像是：

```text
Uptime Kuma + 分布式探针 + 多环境模型 + 告警聚合 + 状态页 + GitOps 配置
```

与传统监控平台的区别：

| 类型 | 关注点 | Orivis 是否覆盖 |
|---|---|---|
| Uptime Kuma | 单实例可用性监控 | 覆盖，并增强多环境 |
| Prometheus | 指标采集与查询 | 不替代，只做轻量指标展示 |
| Grafana | 看板展示 | 不替代 |
| Jaeger / Tempo | Trace 调用链 | 不替代 |
| Zabbix | 主机与基础设施监控 | 部分覆盖 |
| 自研 Agent 平台 | 多环境探针 | 核心能力 |

---

## 4. 总体架构

```text
┌─────────────────────────────────────────────────────────┐
│                       Orivis UI                         │
│              Server-side Template Rendering             │
└──────────────────────────┬──────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────┐
│                    Orivis Server                        │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │ Monitor API  │  │ Agent API    │  │ Alert Engine │   │
│  └──────────────┘  └──────────────┘  └──────────────┘   │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │ Scheduler    │  │ Status Page  │  │ Config Sync  │   │
│  └──────────────┘  └──────────────┘  └──────────────┘   │
│                                                         │
└──────────────────────────┬──────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────┐
│                    Storage Layer                        │
│        SQLite / PostgreSQL / MySQL / Future DB          │
└──────────────────────────┬──────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
┌───────▼───────┐  ┌───────▼───────┐  ┌───────▼───────┐
│ Orivis Agent  │  │ Orivis Agent  │  │ Orivis Agent  │
│ Docker Env    │  │ Host Env      │  │ Future Env    │
└───────┬───────┘  └───────┬───────┘  └───────┬───────┘
        │                  │                  │
        ▼                  ▼                  ▼
  HTTP/TCP/Ping       Local Process       Container/Service
  DNS/TLS Check       Script Check        Runtime Check
```

---

## 5. 核心概念模型

### 5.1 Environment：环境

环境用于区分服务所在的业务或部署阶段。

示例：

```text
prod
staging
test
dev
internal
external
```

字段建议：

| 字段 | 说明 |
|---|---|
| id | 环境 ID |
| name | 环境名称 |
| code | 环境编码 |
| description | 描述 |
| enabled | 是否启用 |

---

### 5.2 Region：区域

区域用于表达物理位置、云区域、机房或网络区域。

示例：

```text
guangdong
wulumuqi
tianyi-guangdong
tianyi-xinjiang
public-internet
```

字段建议：

| 字段 | 说明 |
|---|---|
| id | 区域 ID |
| name | 区域名称 |
| code | 区域编码 |
| description | 描述 |
| enabled | 是否启用 |

---

### 5.3 Agent：探针节点

Agent 是真正执行检测任务的节点。

一个 Agent 至少归属一个区域，可以关联多个环境。

示例：

```text
agent-guangdong-01
agent-wulumuqi-01
agent-test-lab-01
```

字段建议：

| 字段 | 说明 |
|---|---|
| id | Agent ID |
| name | Agent 名称 |
| token_hash | Agent Token 哈希 |
| region_id | 所属区域 |
| environment_ids | 可执行的环境范围 |
| runtime_type | 运行类型，如 docker、host |
| version | Agent 版本 |
| last_seen_at | 最后心跳时间 |
| status | online / offline / disabled |

---

### 5.4 Monitor：监控项

Monitor 是一个被检测对象。

示例：

```text
权益中台 API 健康检查
GitLab 首页
PostgreSQL 端口
Redis 端口
供应商接口
```

字段建议：

| 字段 | 说明 |
|---|---|
| id | 监控项 ID |
| name | 名称 |
| type | http / tcp / ping / dns / tls / script |
| target | 目标地址 |
| environment_id | 所属环境 |
| enabled | 是否启用 |
| interval_seconds | 检测间隔 |
| timeout_seconds | 超时时间 |
| retry_count | 失败重试次数 |
| aggregation_policy | 聚合策略 |

---

### 5.5 Probe Result：探测结果

每次 Agent 执行检测后，都会产生一条探测结果。

字段建议：

| 字段 | 说明 |
|---|---|
| id | 结果 ID |
| monitor_id | 监控项 ID |
| agent_id | Agent ID |
| region_id | 区域 ID |
| environment_id | 环境 ID |
| status | up / down / degraded / unknown |
| latency_ms | 延迟 |
| error_message | 错误信息 |
| checked_at | 检测时间 |
| raw_detail | 原始详情 JSON |

---

## 6. Agent 设计

### 6.1 Agent 定位

Agent 只做三件事：

```text
1. 从 Server 拉取任务
2. 在本地环境执行检测
3. 将检测结果上报给 Server
```

Agent 不负责复杂业务状态判断，也不保存长期历史数据。

---

### 6.2 Agent 运行模式

V1 优先支持两种运行模式：

```text
1. Docker 模式
2. Host 本机模式
```

后续可扩展：

```text
3. Docker Swarm 模式
4. Kubernetes 模式
5. SSH Remote 模式
6. Windows Service 模式
```

---

### 6.3 Docker 模式

Docker 模式用于 Agent 运行在容器内的场景。

适用场景：

1. Docker Compose 环境。
2. Docker Swarm 环境。
3. 统一容器化部署。
4. 不希望直接污染宿主机。

基础配置示例：

```yaml
services:
  orivis-agent:
    image: orivis/agent:latest
    container_name: orivis-agent
    restart: always
    environment:
      ORIVIS_SERVER_URL: "https://orivis.example.com"
      ORIVIS_AGENT_TOKEN: "agent-token"
      ORIVIS_AGENT_NAME: "agent-guangdong-01"
      ORIVIS_REGION: "guangdong"
      ORIVIS_RUNTIME: "docker"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

Docker 模式能力：

| 能力 | V1 是否支持 |
|---|---|
| HTTP 检测 | 支持 |
| TCP 检测 | 支持 |
| DNS 检测 | 支持 |
| TLS 检测 | 支持 |
| Ping 检测 | 视容器权限而定 |
| Docker 容器状态检测 | 支持，可选挂载 docker.sock |
| Docker Compose 服务检测 | 后续支持 |
| Docker Swarm Service 检测 | 后续支持 |

注意：

1. 默认不要求挂载 docker.sock。
2. 只有需要检测 Docker 容器、服务状态时才挂载 docker.sock。
3. docker.sock 权限较高，需要在文档中明确安全风险。

---

### 6.4 Host 本机模式

Host 模式用于 Agent 直接运行在宿主机上。

适用场景：

1. 需要检测本机进程。
2. 需要执行本机脚本。
3. 需要 ICMP Ping。
4. 需要读取本机端口、systemd、磁盘等信息。
5. 不希望通过容器部署 Agent。

systemd 示例：

```ini
[Unit]
Description=Orivis Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/orivis-agent \
  --server-url=https://orivis.example.com \
  --token=agent-token \
  --name=agent-wulumuqi-01 \
  --region=wulumuqi \
  --runtime=host
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Host 模式能力：

| 能力 | V1 是否支持 |
|---|---|
| HTTP 检测 | 支持 |
| TCP 检测 | 支持 |
| DNS 检测 | 支持 |
| TLS 检测 | 支持 |
| Ping 检测 | 支持 |
| 本机进程检测 | 后续支持 |
| systemd 服务检测 | 后续支持 |
| Shell Script 检测 | 支持，但默认关闭 |

---

### 6.5 Agent 内部结构

```text
orivis-agent
├── bootstrap
├── config
├── auth
├── scheduler
├── task_pull_client
├── result_push_client
├── checker
│   ├── http_checker
│   ├── tcp_checker
│   ├── ping_checker
│   ├── dns_checker
│   ├── tls_checker
│   ├── script_checker
│   └── docker_checker
├── runtime
│   ├── docker_runtime
│   ├── host_runtime
│   └── noop_runtime
└── telemetry
```

---

### 6.6 Agent 与 Server 通信

V1 建议使用 HTTP Pull + HTTP Push。

原因：

1. 实现简单。
2. 对网络环境要求低。
3. Agent 只需要主动访问 Server，不需要 Server 反连 Agent。
4. 适合内网、防火墙、NAT、多区域场景。

通信流程：

```text
Agent 启动
  ↓
注册 / 鉴权
  ↓
定时心跳
  ↓
拉取任务列表
  ↓
本地执行检测
  ↓
上报检测结果
  ↓
循环执行
```

接口示例：

```text
POST /api/agent/register
POST /api/agent/heartbeat
GET  /api/agent/tasks
POST /api/agent/results
```

后续版本可以支持：

```text
gRPC Stream
WebSocket
NATS
MQTT
```

但 V1 不建议引入这些复杂依赖。

---

### 6.7 Agent 安全设计

1. Agent 使用 Token 与 Server 鉴权。
2. Server 不保存明文 Token，只保存哈希。
3. Agent Token 支持吊销与轮换。
4. Agent 可绑定允许执行的环境和区域。
5. Script 检测默认关闭。
6. Docker socket 检测能力默认关闭。
7. Server 下发任务时需要校验 Agent 是否有权限执行。

---

## 7. 检测类型设计

### 7.1 HTTP 检测

支持能力：

1. GET / POST / HEAD。
2. 自定义 Header。
3. 自定义 Body。
4. 状态码断言。
5. 响应内容断言。
6. JSON Path 断言。
7. 超时控制。
8. TLS 校验开关。
9. 跟随重定向开关。

示例：

```yaml
type: http
url: https://example.com/q/health
method: GET
timeout: 5s
assert:
  status: 200
  json_path: $.status
  equals: UP
```

---

### 7.2 TCP 检测

支持能力：

1. 主机 + 端口连通性。
2. 连接超时。
3. 可选发送 payload。
4. 可选读取响应。

适合：

```text
PostgreSQL
Redis
Kafka
Git SSH
自定义 TCP 服务
```

---

### 7.3 Ping 检测

支持能力：

1. ICMP Ping。
2. 延迟统计。
3. 丢包统计。

注意：

1. Docker 容器内执行 Ping 可能需要额外权限。
2. V1 可以在 Docker 模式下降级为 TCP Ping 或 HTTP Ping。

---

### 7.4 DNS 检测

支持能力：

1. A / AAAA / CNAME / TXT / MX 查询。
2. 指定 DNS Server。
3. 断言解析结果。
4. 解析耗时统计。

---

### 7.5 TLS 证书检测

支持能力：

1. 证书是否过期。
2. 剩余有效天数。
3. 证书域名匹配。
4. 证书链校验。
5. 到期前告警。

---

### 7.6 Script 检测

Script 检测用于特殊场景，不作为默认能力开放。

支持原则：

1. 默认关闭。
2. 仅 Host 模式支持。
3. 需要在 Agent 本地白名单目录中配置脚本。
4. Server 只下发脚本 ID，不直接下发任意脚本内容。
5. 防止 Server 被攻破后远程执行任意命令。

示例：

```text
/etc/orivis/scripts/check-local-service.sh
```

---

### 7.7 Docker 检测

Docker 检测用于容器环境。

V1 可支持：

1. 容器是否运行。
2. 容器重启次数。
3. 容器健康检查状态。

后续支持：

1. Docker Compose 服务状态。
2. Docker Swarm Service 副本状态。
3. 服务副本数异常告警。
4. 容器日志错误采样。

---

## 8. 多探针聚合策略

单个服务可以由多个 Agent 同时检测。

例如：

```text
权益中台 API
├── 广东 Agent 检测
├── 乌鲁木齐 Agent 检测
└── 公网 Agent 检测
```

聚合策略：

| 策略 | 说明 |
|---|---|
| all_down | 所有探针失败才判定失败 |
| any_down | 任一探针失败就判定失败 |
| majority_down | 多数探针失败才判定失败 |
| region_required | 指定关键区域失败即失败 |
| weighted | 按探针权重计算 |

V1 推荐先实现：

```text
all_down
any_down
majority_down
```

状态聚合结果：

```text
UP：整体可用
DOWN：整体不可用
DEGRADED：部分区域异常
UNKNOWN：数据不足或探针离线
```

---

## 9. 告警设计

### 9.1 告警触发

告警不应该直接由单条 Probe Result 触发，而应该由聚合后的 Monitor State 触发。

流程：

```text
Probe Result
  ↓
Result Aggregator
  ↓
Monitor State
  ↓
Alert Rule
  ↓
Alert Event
  ↓
Notification
```

---

### 9.2 告警降噪

V1 需要支持基础降噪：

1. 连续失败 N 次才告警。
2. 恢复后发送恢复通知。
3. 相同监控项在一定时间内不重复告警。
4. Agent 离线和服务失败分开处理。
5. 多探针失败合并成一条告警。

---

### 9.3 通知渠道

V1 支持：

1. Webhook。
2. Email。
3. 企业微信 / 微信机器人。
4. 短信接口预留。

---

## 10. 主服务数据库设计

### 10.1 多数据库支持目标

主服务数据库需要支持多种后端，便于不同部署场景使用。

V1 建议支持：

```text
SQLite
PostgreSQL
MySQL / MariaDB
```

推荐使用方式：

| 场景 | 推荐数据库 |
|---|---|
| 单机体验 / 开发 | SQLite |
| 企业生产 | PostgreSQL |
| 已有 MySQL 环境 | MySQL / MariaDB |
| 大量历史结果 | PostgreSQL + TimescaleDB 可选 |

---

### 10.2 数据访问层原则

不建议在核心代码里到处写数据库方言判断。

建议拆成：

```text
Store Interface
  ↓
SQLite Store
PostgreSQL Store
MySQL Store
```

目录示例：

```text
server/internal/store
├── store.go
├── sqlite
│   ├── monitor_store.go
│   ├── agent_store.go
│   └── migration
├── postgres
│   ├── monitor_store.go
│   ├── agent_store.go
│   └── migration
└── mysql
    ├── monitor_store.go
    ├── agent_store.go
    └── migration
```

核心接口示例：

```go
type Store interface {
    MonitorStore() MonitorStore
    AgentStore() AgentStore
    ResultStore() ResultStore
    AlertStore() AlertStore
    Transaction(ctx context.Context, fn func(ctx context.Context, tx Store) error) error
}
```

---

### 10.3 迁移策略

每种数据库维护独立 migration。

```text
migrations/
├── sqlite
├── postgres
└── mysql
```

原因：

1. SQLite、PostgreSQL、MySQL 的 SQL 方言差异明显。
2. 时间类型、JSON 类型、索引能力不同。
3. 后续 PostgreSQL 可以扩展 TimescaleDB，不应强行兼容所有数据库。

---

### 10.4 历史结果存储策略

Probe Result 是增长最快的数据。

V1 可以先普通表存储：

```text
probe_results
```

PostgreSQL 场景下，后续可升级为 TimescaleDB hypertable。

数据保留策略：

| 数据类型 | 保留时间 |
|---|---|
| 原始探测结果 | 7 ~ 30 天 |
| 小时聚合数据 | 90 ~ 180 天 |
| 日聚合数据 | 1 ~ 3 年 |
| 告警事件 | 长期保留 |

---

## 11. UI 设计

### 11.1 V1 UI 原则

第一版 UI 不引入前端构建工具。

也就是说，V1 不使用：

```text
Vite
Webpack
React
Vue
Next.js
复杂 Node.js 构建链
```

推荐使用：

```text
Go html/template
静态 CSS
少量原生 JavaScript
HTMX 可选
Alpine.js 可选
```

更稳妥的 V1 方案：

```text
Server-side Rendering + html/template + 静态资源
```

---

### 11.2 UI 模块

V1 页面：

1. 首页概览。
2. 监控项列表。
3. 监控项详情。
4. Agent 列表。
5. 环境管理。
6. 区域管理。
7. 告警事件列表。
8. 状态页管理。
9. 系统设置。

---

### 11.3 首页概览

展示：

1. 总监控项数量。
2. UP 数量。
3. DOWN 数量。
4. DEGRADED 数量。
5. Agent 在线数量。
6. 最近告警。
7. 按环境分组的健康状态。
8. 按区域分组的健康状态。

---

### 11.4 监控项详情

展示：

1. 当前聚合状态。
2. 每个 Agent 的最近检测结果。
3. 最近 24 小时可用率。
4. 最近延迟趋势。
5. 最近失败原因。
6. 告警记录。

V1 可以用简单 SVG / HTML 表格展示，不需要复杂图表库。

---

### 11.5 状态页

状态页可以公开访问，也可以内部访问。

V1 支持：

1. 创建状态页。
2. 选择展示哪些监控项。
3. 展示当前状态。
4. 展示最近事件。
5. 支持自定义标题。

---

## 12. 配置方式

### 12.1 UI 配置

V1 首先支持 UI 配置，降低使用门槛。

---

### 12.2 文件配置 / GitOps

后续支持配置文件同步。

示例：

```yaml
environments:
  - code: prod
    name: 生产环境

regions:
  - code: guangdong
    name: 广东区域

monitors:
  - code: benefits-api-prod
    name: 权益中台 API
    type: http
    environment: prod
    url: https://benefits-open-platform.xjldkj.com/q/health
    interval: 30s
    timeout: 5s
    probes:
      - agent-guangdong-01
      - agent-wulumuqi-01
    aggregation: majority_down
```

也可以后续支持 HCL：

```hcl
monitor "benefits-api-prod" {
  name        = "权益中台 API"
  type        = "http"
  environment = "prod"
  url         = "https://benefits-open-platform.xjldkj.com/q/health"

  interval = "30s"
  timeout  = "5s"

  probes = [
    "agent-guangdong-01",
    "agent-wulumuqi-01"
  ]

  aggregation = "majority_down"
}
```

V1 可以先不做 GitOps，但数据模型要预留配置来源字段：

```text
source = ui / file / api
```

---

## 13. 部署设计

### 13.1 单机部署

适合开发和小规模使用。

```text
orivis-server + SQLite
orivis-agent 同机运行
```

---

### 13.2 Docker Compose 部署

适合普通生产环境。

```yaml
services:
  orivis-server:
    image: orivis/server:latest
    restart: always
    ports:
      - "8080:8080"
    environment:
      ORIVIS_DB_DRIVER: postgres
      ORIVIS_DB_DSN: postgres://orivis:orivis@postgres:5432/orivis
    depends_on:
      - postgres

  postgres:
    image: postgres:17
    restart: always
    environment:
      POSTGRES_DB: orivis
      POSTGRES_USER: orivis
      POSTGRES_PASSWORD: orivis
    volumes:
      - postgres-data:/var/lib/postgresql/data

volumes:
  postgres-data:
```

---

### 13.3 Docker Swarm 部署

适合企业内部多节点部署。

特点：

1. Server 可以单副本起步。
2. PostgreSQL 使用外部数据库或固定节点部署。
3. Agent 可部署为 global service。
4. 每台节点一个 Agent。

示例方向：

```yaml
services:
  orivis-agent:
    image: orivis/agent:latest
    deploy:
      mode: global
    environment:
      ORIVIS_SERVER_URL: https://orivis.example.com
      ORIVIS_AGENT_TOKEN: ${ORIVIS_AGENT_TOKEN}
      ORIVIS_RUNTIME: docker
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

---

## 14. 权限设计

V1 可以做简单账号体系。

角色：

| 角色 | 权限 |
|---|---|
| admin | 全部权限 |
| operator | 管理监控项、Agent、告警 |
| viewer | 只读查看 |

后续扩展：

1. OIDC 登录。
2. LDAP 登录。
3. 项目级权限。
4. 环境级权限。
5. 状态页访问控制。

---

## 15. API 设计

### 15.1 管理 API

```text
GET    /api/monitors
POST   /api/monitors
GET    /api/monitors/{id}
PUT    /api/monitors/{id}
DELETE /api/monitors/{id}

GET    /api/agents
POST   /api/agents
GET    /api/agents/{id}
PUT    /api/agents/{id}
DELETE /api/agents/{id}

GET    /api/environments
POST   /api/environments

GET    /api/regions
POST   /api/regions

GET    /api/alerts
GET    /api/status-pages
POST   /api/status-pages
```

---

### 15.2 Agent API

```text
POST /api/agent/register
POST /api/agent/heartbeat
GET  /api/agent/tasks
POST /api/agent/results
```

Agent API 和管理 API 建议使用不同鉴权机制。

---

## 16. 技术选型建议

### 16.1 后端语言

建议使用 Go。

原因：

1. Agent 单二进制分发非常适合 Go。
2. Server 和 Agent 可以共享协议模型。
3. 部署简单。
4. 适合基础设施类项目。
5. 对 Docker、网络探测、系统调用支持较好。

---

### 16.2 后端框架

可选：

```text
net/http + chi
Fiber
Huma + chi / stdlib
```

推荐：

```text
chi + Huma
```

原因：

1. chi 简单稳定。
2. Huma 可以生成 OpenAPI。
3. 不会引入过重框架。
4. 适合长期维护 API contract。

---

### 16.3 数据访问

建议：

```text
database/sql + sqlx 或自研轻量 Store
```

如果要保持 SQL 可控，建议不要一开始引入重 ORM。

迁移工具可选：

```text
goose
atlas
tern
自研 migration runner
```

V1 推荐 goose 或自研极简 migration runner。

---

### 16.4 UI

V1 推荐：

```text
Go html/template
静态 CSS
少量 Vanilla JS
HTMX 可选
```

目录示例：

```text
server/web
├── templates
│   ├── layout.html
│   ├── dashboard.html
│   ├── monitors.html
│   ├── monitor_detail.html
│   ├── agents.html
│   └── alerts.html
├── static
│   ├── app.css
│   └── app.js
└── handlers
```

---

## 17. 项目结构建议

```text
orivis
├── cmd
│   ├── orivis-server
│   ├── orivis-agent
│   └── orivis-cli
├── internal
│   ├── server
│   │   ├── api
│   │   ├── web
│   │   ├── app
│   │   ├── store
│   │   ├── scheduler
│   │   ├── aggregator
│   │   ├── alert
│   │   └── config
│   ├── agent
│   │   ├── checker
│   │   ├── runtime
│   │   ├── client
│   │   ├── scheduler
│   │   └── config
│   └── shared
│       ├── model
│       ├── protocol
│       └── errors
├── migrations
│   ├── sqlite
│   ├── postgres
│   └── mysql
├── deployments
│   ├── docker-compose
│   ├── docker-swarm
│   └── systemd
├── docs
└── go.mod
```

---

## 18. V1 里程碑

### Milestone 1：基础骨架

1. Server 启动。
2. SQLite 支持。
3. html/template UI 骨架。
4. Agent 注册。
5. Agent 心跳。

---

### Milestone 2：基础检测

1. HTTP 检测。
2. TCP 检测。
3. DNS 检测。
4. TLS 检测。
5. 结果上报。
6. 监控项详情页。

---

### Milestone 3：多环境多 Agent

1. Environment 管理。
2. Region 管理。
3. Agent 绑定环境和区域。
4. Monitor 绑定多个 Agent。
5. 聚合状态计算。

---

### Milestone 4：告警与状态页

1. 告警规则。
2. 告警事件。
3. Webhook 通知。
4. Email 通知。
5. 状态页。

---

### Milestone 5：生产化增强

1. PostgreSQL 支持。
2. MySQL 支持。
3. Docker 部署文档。
4. systemd 部署文档。
5. Agent Token 轮换。
6. 数据清理任务。

---

## 19. 风险与注意事项

### 19.1 多数据库复杂度

多数据库支持会增加开发和测试成本。

建议策略：

```text
SQLite 用于单机和开发
PostgreSQL 作为主生产目标
MySQL 作为兼容目标
```

不要为了兼容所有数据库牺牲核心设计。

---

### 19.2 Docker Socket 安全风险

挂载 docker.sock 等于给 Agent 很高权限。

建议：

1. 默认不挂载。
2. 明确文档提示风险。
3. Docker 检测能力单独开关。
4. 后续考虑只读代理或独立 docker-proxy。

---

### 19.3 Script 检测安全风险

不要让 Server 直接下发任意脚本内容。

建议：

1. Agent 本地白名单脚本。
2. Server 只引用脚本 ID。
3. 记录每次执行日志。
4. 限制超时和输出大小。

---

### 19.4 前端简单化带来的限制

无构建工具可以降低复杂度，但交互能力有限。

V1 应接受这个限制，把重点放在核心模型和 Agent 能力上。

后续如果 UI 复杂度上来，再迁移到 React / refine / shadcn/ui。

---

## 20. 总结

Orivis 的第一阶段应该专注于解决一个明确问题：

```text
在多环境、多区域、多网络条件下，准确判断服务是否真的可用。
```

V1 不应追求复杂大而全，而应该优先完成：

1. Server + Agent 架构。
2. Docker / Host 两种 Agent 运行环境。
3. SQLite / PostgreSQL / MySQL 多数据库适配。
4. HTTP / TCP / DNS / TLS 基础检测。
5. 多探针聚合判断。
6. 基础告警。
7. 服务端模板 UI。

这样第一版就能形成一个非常清晰、可落地、可长期演进的基础设施项目。

