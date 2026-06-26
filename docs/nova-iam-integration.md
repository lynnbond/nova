# Nova × IAM Server 集成方案

## 一、现状分析

### Nova 当前认证
- 简陋的 JWT 自签发（硬编码 secret fallback）
- 本地用户表（sqlite），seed 时创建 admin/zhangsan/lisi/wangwu
- 无 SSO、无组织架构、无 RBAC

### IAM Server 能力
- 统一认证中心（密码/验证码/飞书/Google OAuth）
- 多租户 + 应用隔离
- RBAC + 资源级权限
- 内网 API 供微服务调用（无需用户认证）

---

## 二、架构方案

```
┌─────────────────────────────────────────────────┐
│                  浏览器                           │
│  ┌─────────┐     ┌───────────────────────────┐   │
│  │Nova 前端 │     │IAM SSO 登录页              │   │
│  │(React)   │     │(test-iam.lingyiwanwu.net) │   │
│  └────┬────┘     └────────────┬──────────────┘   │
│       │                       │                   │
└───────┼───────────────────────┼───────────────────┘
        │ ① 跳转登录             │
        ├──────────────────────>│
        │                       │ ② 用户认证
        │ ③ 回调 + token        │
        │<──────────────────────┤
        │ ④ 携带 token 请求      │
        │    nova API           │
        ▼                       ▼
┌──────────────────────────────────────────────────┐
│              Nova 后端 (Go)                        │
│                                                   │
│  ┌─────────────┐   ┌──────────────────────────┐   │
│  │IAM 中间件     │   │用户同步服务                │   │
│  │• token 验证   │   │• 定时拉取 IAM 用户列表     │   │
│  │• 用户自动创建  │   │• 增量同步                 │   │
│  └─────────────┘   └───────────┬──────────────┘   │
│                                │                  │
│                                ▼                  │
│  ┌──────────────────────────────────────────────┐ │
│  │              IAM Client                       │ │
│  │  /iam/internal/v2/users → 用户列表            │ │
│  │  /iam/api/v2/users/self → 用户详情+角色        │ │
│  └──────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────┘
```

---

## 三、SSO 登录对接

### 3.1 前端改造

#### 登录页替换
- 删除现有 `/login` 页的本地登录表单
- 页面加载时调用 `GET /iam/api/v2/login/iam/url?redirect_uri=<nova callback>`
- 将用户重定向到 IAM 登录页

#### 回调页（新增 `/auth/iam/callback`）
- URL 带回 `token` 参数（短期 token）
- 用短期 token 调用 `POST /iam/api/v2/user/csrf-token` → 获取长期 CSRF Token
- 用 CSRF Token 获取用户信息 `GET /iam/api/v2/users/self`
- **将 CSRF Token 发到 nova 后端** `POST /api/v1/auth/iam`
- nova 后端验证 token，创建/更新本地用户，签发 nova JWT
- 前端保存 nova JWT，跳转到首页

```
登录流程：

用户访问 nova 首页
  → 未登录，跳转 /login
  → /login 页检测到没有 nova token
  → 请求 IAM 登录 URL
  → 重定向到 IAM SSO 页
  → 用户输入密码 / 飞书扫码
  → IAM 回调到 nova /auth/iam/callback?token=xxx
  → 前端用 xxx 换取 IAM CSRF Token
  → 前端 POST /api/v1/auth/iam { csrf_token: "..." }
  → nova 后端验证通过，签发 nova JWT
  → 前端保存 nova JWT，进入首页
```

#### 退出登录
- 调用 `POST /iam/api/v2/logout?returnUrl=<nova>`（清除 IAM 会话）
- 前端清除 nova JWT，重定向到 nova 登录页

### 3.2 后端新增

#### `POST /api/v1/auth/iam`
```go
func handleAuthIAM(w, r) {
    // 1. 解析请求体 { csrf_token: "..." }
    // 2. 用 csrf_token 调 IAM GET /iam/api/v2/users/self
    // 3. 获取用户信息（userId, userName, realName, email, orgId, orgName, roles）
    // 4. 在 nova 本地用户表查找或创建用户
    //    - 用 IAM userId 作为 nova 的 UID
    //    - 同步更新 name, email, dept (从 orgName)
    // 5. 签发 nova JWT，返回 { token, user }
}
```

#### IAM Client 模块（新增文件）
```go
// internal/iam/client.go
type IAMClient struct {
    BaseURL    string  // https://test-iam.lingyiwanwu.net
}

// VerifyToken 验证 CSRF Token 并返回用户信息
func (c *IAMClient) GetUserByToken(ctx, csrfToken) (*IAMUser, error)
// GET /iam/api/v2/users/self
// Header: Csrf-Token: <token>

type IAMUser struct {
    UserID   string
    UserName string
    RealName string
    Email    string
    OrgID    string
    OrgName  string
    Roles    []RoleInfo
}
```

#### IAM 配置项
```go
// Config 新增字段
type Config struct {
    // ... 现有字段
    IAMBaseURL  string // IAM 服务地址
}
```

---

## 四、用户同步

### 4.1 同步方式

| 场景 | 方式 | 说明 |
|------|------|------|
| 用户登录时 | 按需同步 | 用户第一次登录时自动在 nova 创建用户 |
| 定时全量同步 | Cron/后台线程 | 每 5 分钟通过内网 API 拉取全量用户，更新本地表 |
| 手动触发 | Admin API | `POST /api/v1/sync/users` |

### 4.2 内网 API 调用

IAM 内网 API `GET /iam/internal/v2/users` 直接返回所有用户列表：
```json
{
  "total": 100,
  "page": 1,
  "size": 100,
  "data": [
    {
      "userId": "550e8400-...",
      "userName": "zhangsan",
      "realName": "张三",
      "status": "enabled",
      "email": "zhangsan@01.ai",
      "phone": "13800138000",
      "orgId": "org-xxx",
      "orgName": "研发部",
      "roles": [{ "roleId": "...", "roleName": "普通用户" }]
    }
  ]
}
```

### 4.3 nova 用户表映射

| nova.User 字段 | IAM 来源 | 说明 |
|----------------|---------|------|
| ID | 本地 UUID | nova 自生成 |
| UID | `userId` | 用 IAM userId 作为唯一标识 |
| Name | `realName` | 真实姓名 |
| Email | `email` | |
| Phone | `phone` | |
| Dept | `orgName` | 所在部门 |
| PasswordHash | 留空 | 密码由 IAM 管理，nova 不存 |

### 4.4 Handler 解析对接

nova 的 `HandlerResolver` 对接 IAM 角色/组织查询：
- 按角色查用户：`GET /iam/internal/v2/roles/:name/users`
- 按组织查用户：`GET /iam/internal/v2/orgs/:orgUuid/users`
- 用户权限校验：`GET /iam/internal/v2/users/:userId`（含角色 + 权限树）

---

## 五、代码变更清单

| 文件 | 变更 |
|------|------|
| `internal/app/app.go` | 新增 `/api/v1/auth/iam` 路由 + handler |
| `internal/app/app.go` | Config 新增 `IAMBaseURL` |
| `internal/iam/client.go` | **新文件** IAM Client：GetUserByToken、SyncUsers |
| `internal/iam/types.go` | **新文件** IAM 响应数据结构 |
| `web/src/admin/pages/login-page.tsx` | 改造为跳转 IAM SSO |
| `web/src/admin/pages/auth-callback.tsx` | **新文件** IAM 回调处理页 |
| `web/src/lib/auth.tsx` | 对接新的登录流程 |
| `cmd/nova-server/main.go` | 支持 `IAM_BASE_URL` 环境变量 |

---

## 六、实施步骤

1. **Phase 1: IAM Client 库** — 实现 `GetUserByToken`、`SyncUsers`
2. **Phase 2: 后端认证** — 新增 `/api/v1/auth/iam`，token 验证 + 自动创建用户
3. **Phase 3: 前端 SSO** — 替换登录页，新增回调页
4. **Phase 4: 用户定时同步** — 后台 goroutine 定期拉取
5. **Phase 5: Handler 对接** — HandlerResolver 对接 IAM 组织/角色接口
