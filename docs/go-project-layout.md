# Go 项目目录规范

## 一、官方推荐布局

```
project-root/
├── cmd/                  # 可执行文件入口
│   └── myapp/
│       └── main.go       # 这里只有 func main()，极简
│
├── internal/             # 私有包（Go 编译器强制外部不可引用）
│   ├── app/              # 应用级编排（wire up 依赖）
│   ├── handler/          # HTTP/gRPC handler
│   ├── service/          # 业务逻辑层
│   ├── repository/       # 数据访问层（DB 操作）
│   └── middleware/        # HTTP 中间件
│
├── pkg/                  # 公开包（可被外部项目 import）
│   ├── auth/             # 认证工具
│   └── httputil/         # HTTP 工具函数
│
├── api/                  # API 协议定义
│   ├── openapi.yaml      # OpenAPI 规范
│   └── proto/            # protobuf 定义
│
├── configs/              # 配置文件模板
│   └── config.toml.example
│
├── scripts/              # 构建/部署/维护脚本
│   └── migrate.sh
│
├── docs/                 # 文档
│   └── architecture.md
│
├── test/                 # 外部集成测试（黑盒测试）
│   └── integration_test.go
│
├── web/                  # Web 前端资源（如果项目包含 UI）
│   ├── src/
│   └── package.json
│
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── .github/              # CI/CD 配置
    └── workflows/
```

---

## 二、核心规则

### 1. `cmd/` — 只放 `func main()`

每个可执行程序一个子目录，`main.go` 里只做三件事：
- 解析配置（flag/env）
- 组装依赖（wire / 手动）
- 启动服务

```go
// cmd/myapp/main.go
func main() {
    cfg := config.Load()
    db := sql.Open(cfg.DSN)
    svc := service.New(db)
    srv := handler.New(svc)
    log.Fatal(srv.Serve(cfg.Addr))
}
```

**不要**在 `cmd/` 里放业务逻辑，它只是一个薄薄的入口。

### 2. `internal/` — 核心业务代码

Go 编译器会强制阻止外部项目 `import` `internal` 包里的东西。这是 Go 的原生模块隔离机制。

```
internal/
├── app/           # 应用组装（wire、依赖注入）
├── handler/       # HTTP 控制器（解析请求、调用 service、返回响应）
├── service/       # 业务逻辑（事务边界、领域规则）
├── repository/    # 数据访问（SQL、Redis、外部 API 调用）
├── middleware/    # HTTP 中间件（auth、logging、recovery）
└── model/         # 领域模型 / DTO
```

**分层调用方向**：handler → service → repository

```
handler (请求/响应格式)
   ↓
service (业务规则、事务)
   ↓
repository (数据存取)
```

### 3. `pkg/` — 可复用的公开库

如果包可以被其他项目独立使用，放这里。例如：
- 自定义 `auth.JWT` 工具
- `uuid` 生成器
- HTTP 客户端封装

**判断标准**：如果你不确定要不要公开，放 `internal/`。后续需要公开时再挪到 `pkg/`。

---

## 三、包命名规范

| 规范 | 示例 | 说明 |
|------|------|------|
| 全小写 | `service`、`handler` | Go 惯例，不用下划线、驼峰 |
| 单数名词 | `user` 不是 `users` | Go package 名不用复数 |
| 简短 | `httputil` 不是 `http_utility` | 2~8 个字符最佳 |
| 避免 `common`、`utils`、`lib` | 要按职责分 | 见下面说明 |

### 不要用 `utils`、`common`、`helper`

坏味道：
```
pkg/utils/string.go
pkg/utils/http.go
pkg/utils/time.go
```

好的做法是按能力拆分：
```
pkg/strutil/   # 字符串工具
pkg/httputil/  # HTTP 工具
pkg/timeutil/  # 时间工具
```

---

## 四、文件组织

### 4.1 按职责分文件

一个包内按功能拆文件，而不是按类型：

```
service/
├── user.go          # 用户相关的业务逻辑
├── order.go         # 订单相关的业务逻辑
├── notification.go  # 通知相关的业务逻辑
└── service.go       # Service 结构体定义、构造函数
```

而不是：
```
service/
├── types.go         # 所有 struct
├── handler.go       # 所有方法
└── errors.go        # 所有错误
```

### 4.2 `internal/app/app.go`

对于单体应用，可以有一个 `internal/app` 包负责所有应用级的编排：

```
internal/app/
├── app.go           # App 结构体、New()、Serve()
├── routes.go        # 路由注册
├── handlers.go      # HTTP handler
├── middleware.go    # 中间件
├── seed.go          # 种子数据
└── config.go        # 配置结构体
```

这个模式就是 nova 目前用的。

---

## 五、特例与增补

### 5.1 Monorepo 多项目

```
project/
├── apps/
│   ├── api/         # cmd/myapp 的替代组织方式
│   │   └── main.go
│   └── worker/      # 后台 worker
│       └── main.go
├── libs/            # internal/ 的替代组织方式
│   └── auth/
└── packages/        # pkg/ 的替代组织方式
    └── go-auth/
```

### 5.2 领域驱动设计 (DDD)

```
internal/
├── domain/          # 领域实体 + 值对象 + 领域服务
│   ├── user/
│   │   ├── user.go          # 实体
│   │   ├── repository.go    # 接口定义
│   │   └── service.go       # 领域服务
│   └── order/
├── application/     # 应用服务（用例编排）
└── infrastructure/  # 基础设施实现
    ├── persistence/ # repository 的 SQL 实现
    └── http/        # 外部 API 客户端
```

### 5.3 项目根目录直接放核心包

如果你的项目是一个**库（library）**而不是**应用**，核心代码直接放根目录：

```
project/
├── types.go         # 核心类型
├── engine.go        # 引擎核心
├── store.go         # 存储接口
├── sqlite/          # 存储实现
│   └── store.go
├── cmd/
│   └── servertool/
│       └── main.go
├── internal/
│   └── app/
│       └── app.go
```

这就是 nova 的模式：核心引擎直接暴露在根目录作为 PDK。

---

## 六、实战：nova 当前结构分析

```
nova/
├── builder.go           # 流程构建器 (PDK 核心)
├── engine.go            # 工作流引擎 (PDK 核心)
├── types.go             # 领域类型 (PDK 核心)
├── store.go             # Store 接口 (PDK 核心)
├── seal.go              # 其他核心功能
├── notification.go      # 通知机制
│
├── internal/
│   ├── app/             # HTTP 应用层
│   │   ├── app.go       # Server + handler 编排
│   │   ├── routes.go    # (已内联到 app.go)
│   │   ├── seed.go      # 种子数据
│   │   ├── webdist/     # 前端构建产物 (embed)
│   │   └── ...
│   └── iam/             # IAM 客户端
│
├── sqlite/              # Store 的 SQLite 实现
│   └── store.go
│
├── cmd/
│   └── nova-server/
│       └── main.go
│
├── web/                 # React 前端
├── docs/                # 文档
├── scripts/             # 工具脚本
├── go.mod
└── Makefile
```

这个结构合理：PDK 核心放根目录、应用层在 `internal/`、存储实现在独立包。符合标准 Go 布局。
