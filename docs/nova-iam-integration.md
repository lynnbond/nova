# Nova × IAM Server 集成方案（精简版）

> 仅 SSO 登录对接，不涉及用户数据同步

---

## 一、架构

```
┌──────────────────────────────────────────────────┐
│                    浏览器                          │
│  ┌─────────┐      ┌─────────────────────────┐    │
│  │Nova 前端 │      │IAM SSO 登录页             │    │
│  │(React)   │      │test-iam.lingyiwanwu.net │    │
│  └────┬────┘      └───────────┬─────────────┘    │
│       │                       │                    │
└───────┼───────────────────────┼────────────────────┘
        │ ① 未登录，跳转 IAM     │
        ├──────────────────────→│
        │                       │ ② 用户认证
        │ ③ 回调 + 短期 token    │
        │←──────────────────────┤
        │ ④ 用短期 token 换      │
        │   CSRF Token          │
        │ ⑤ POST /api/v1/auth/iam
        │   { csrf_token }
        ▼                       ▼
┌──────────────────────────────────────────────────┐
│              Nova 后端 (Go)                        │
│                                                   │
│  /api/v1/auth/iam:                                 │
│  1. 用 CSRF Token 调 IAM /users/self 验证          │
│  2. 查本地用户表，不存在则自动创建                   │
│  3. 签发 nova JWT 返回                              │
│                                                   │
│  后续请求：nova JWT 认证（不变）                     │
└──────────────────────────────────────────────────┘
```

---

## 二、SSO 登录流程

### 2.1 前端流程

```
用户访问 nova 首页
  → 没有 nova JWT，跳转 /login
  → /login 页 GET /iam/api/v2/login/iam/url?redirect_uri=<nova回调>
  → 重定向到 IAM SSO 页
  → 用户输入密码 / 飞书扫码
  → IAM 回调到 nova /auth/callback?token=<短期token>
  → 前端用短期 token POST /iam/api/v2/user/csrf-token → 拿 CSRF Token
  → 前端 POST /api/v1/auth/iam { csrf_token }
  → nova 后端返回 { token, user }
  → 前端保存 nova JWT，进入首页
```

### 2.2 后端 `/api/v1/auth/iam`

```
请求: POST /api/v1/auth/iam
Body: { "csrf_token": "..." }

处理:
  1. 用 csrf_token 调 IAM: GET /iam/api/v2/users/self
     Header: Csrf-Token: <token>
     
  2. IAM 返回:
     {
       "userId": "550e8400-...",
       "userName": "zhangsan",
       "realName": "张三",
       "email": "zhangsan@01.ai",
       "orgName": "研发部"
     }
     
  3. 查 nova 本地用户表 WHERE uid = IAM userId
     - 存在: 更新 name/email/dept
     - 不存在: CreateUser { UID: IAM.userId, Name: IAM.realName, ... }
     
  4. 签发 nova JWT（使用现有 jwtSecret），过期时间沿用现有配置
     
  5. 返回 { token, user }
```

---

## 三、代码变更

### 3.1 新增文件

**`internal/iam/client.go`** — IAM HTTP 客户端
```go
package iam

type Client struct {
    BaseURL string  // https://test-iam.lingyiwanwu.net
}

// GetUserByToken 验证 CSRF Token 并返回用户信息
func (c *Client) GetUserByToken(ctx, csrfToken) (*User, error)

// GET /iam/api/v2/users/self
// Header: Csrf-Token: <token>

type User struct {
    UserID   string `json:"userId"`
    UserName string `json:"userName"`
    RealName string `json:"realName"`
    Email    string `json:"email"`
    OrgName  string `json:"orgName"`
    Status   string `json:"status"`
}
```

### 3.2 修改文件

| 文件 | 变更 |
|------|------|
| `internal/app/app.go` | 新增路由 + handler `handleAuthIAM` |
| `internal/app/app.go` | Config 新增 `IAMBaseURL string` |
| `web/src/admin/pages/login-page.tsx` | 改为跳转 IAM SSO |
| `web/src/admin/pages/auth-callback.tsx` | **新建** IAM 回调处理 |
| `web/src/lib/auth.tsx` | 对接 `/api/v1/auth/iam` |
| `cmd/nova-server/main.go` | 支持 `IAM_BASE_URL` env |

### 3.3 Route 注册

```go
// 新增路由（无需 JWT 认证）
a.mux.HandleFunc("POST /api/v1/auth/iam", a.handleAuthIAM)
```

---

## 四、IAM 依赖的 API

| 接口 | 用途 | 调用方 |
|------|------|--------|
| `GET /iam/api/v2/login/iam/url?redirect_uri=xxx` | 获取 IAM 登录 URL | 前端 |
| `POST /iam/api/v2/user/csrf-token` Header: `Csrf-Token` | 短期 token → CSRF Token | 前端 |
| `GET /iam/api/v2/users/self` Header: `Csrf-Token` | 验证 token + 获取用户信息 | 后端 |
| `POST /iam/api/v2/logout` | 退出 IAM 登录 | 前端 |

---

## 五、实施步骤

| Phase | 内容 | 工作量 |
|-------|------|--------|
| 1 | IAM Client: `GetUserByToken` | ~50 行 |
| 2 | 后端: `POST /api/v1/auth/iam` handler | ~80 行 |
| 3 | 前端: 改造 login 页 + 新增 callback 页 | ~150 行 |
| 4 | 退出登录 + 边界处理 | ~50 行 |
