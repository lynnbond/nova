# Pandora Nova Framework — UI & 实体属性完整分析报告

> 源码路径：`/tmp/nova-migration/framework-src/`（Pandora 框架层）
> `/tmp/nova-migration/nova4-src/`（Nova4 引擎核心）
> `/tmp/nova-migration/nova-ddl-all.sql`（完整 DDL）
> 作者：LiYan（岩哥）
> 分析日期：2026-06-24

---

## 一、Pandora Nova 的全貌

```
┌────────────────────────────────────────────────────────────────┐
│                    业务层（你的业务表单）                           │
│  Channel Class（实体业务交互类，Java 全限定类名）                   │
├────────────────────────────────────────────────────────────────┤
│              Pandora Nova Framework（UI/框架适配层）               │
│  Nova4FrameController  ─  Servlet 控制器                         │
│  Nova4ToModelFactory   ─  Nova4→Nova3.x 模型转换适配器             │
│  NovaRequest           ─  HTTP 请求封装（参数解析+实体定位）         │
│  NovaResponse          ─  HTTP 响应封装（状态+数据+Wax）           │
│  PreSubmitAction       ─  预提交逻辑                              │
│  SubmitAction          ─  提交逻辑                                │
│  AddEntityAction       ─  新增实体                                │
│  UpdateEntityAction    ─  更新实体                                │
│  NovaHookReceiverImpl  ─  钩子接收器                              │
│  PandoraUserNova4HandlerSource ─ 处理人数据源                      │
├────────────────────────────────────────────────────────────────┤
│                    Nova4 工作流引擎核心                             │
│  Nova4Engine / HandlerEngine / TaskManager / ...                │
│  EntityTaskStater / EntityChildrenStater                        │
│  Nova4HandlerChooser / CandidateChooser                         │
│  NovaEntity / NovaWormhole / TargetInfo / Nova4SubmitInfo       │
├────────────────────────────────────────────────────────────────┤
│                        数据库层                                   │
│  MySQL（nova 库），13+ 张核心表                                   │
└────────────────────────────────────────────────────────────────┘
```

---

## 二、Pandora Nova 前端交互模型（核心 UI 功能）

### 2.1 前端路由与页面入口

Pandora Nova 通过 `NovaServletController` 接口定义所有前端可调用的操作，通过 HTTP 参数 `act` 分派：

| act 动作 | 对应方法 | 说明 |
|---------|---------|------|
| `preSubmit` | `preSubmit()` | 预提交（校验表单数据完备性，决定下一步） |
| `submit` | `submit()` | 正式提交 |
| `addEntity` | `addEntity()` | 新增保存实体表单 |
| `updateEntity` | `updateEntity()` | 更新实体 |
| `doNewDoc` | `doNewDoc()` | 打开新流程表单 |
| `doOpenDocByCode` | `doOpenDocByCode()` | 根据 code 打开实体 |
| `doOpenDocByEntityId` | `doOpenDocByEntityId()` | 根据 entityId 打开实体 |
| `doOpenDocBySerialNum` | `doOpenDocBySerialNum()` | 根据流水号打开实体 |
| `getOpinionList` | `getOpinionList()` | 获取处理意见列表 |
| `addOpinion` | `addOpinion()` | 新增意见/留言 |
| `getHandleLogs` | `getHandleLogs()` | 获取处理日志 |
| `getHandleLogList` | `getHandleLogList()` | 处理日志列表（兼容版） |
| `setEntityFlag` | `setEntityFlag()` | 设置实体旗标 |
| `getEntityFlag` | `getEntityFlag()` | 获取实体旗标 |
| `doOpenHandleLog` | `doOpenHandleLog()` | 打开处理日志 |
| `doInsertAttention` | `doInsertAttention()` | 关注实体 |
| `doDeleteAttention` | `doDeleteAttention()` | 取消关注实体 |
| `cancelEntity` | `cancelEntity()` | 删除/撤销实体 |
| `doCopyEntity` | `doCopyEntity()` | 拷贝实体生成草稿 |
| `insertAnalysis` | `insertAnalysis()` | 保存行为分析数据 |
| `doSaveUserProAlias` | `doSaveUserProAlias()` | 保存用户对申请说明的 `never_remind_me` |
| `getFlowChartJSON` | `getFlowChartJSON()` | 获取流程图 JSON 数据 |
| `doCountTable` | `doCountTable()` | 执行统计操作 |
| `showFlowReadMe` | `showFlowReadMe()` | 打开申请说明页面 |

### 2.2 关键业务场景流程

#### 场景一：新建流程
```
用户点击"发起" → doNewDoc → 打开表单页 → 填写内容 → save/addEntity
→ 填写完毕 → preSubmit（校验）→ 可选：选择下一环节/处理人
→ submit → 正式提交
```

#### 场景二：处理待办
```
用户点击待办 → doOpenDocByCode → 打开实体详情/编辑页
→ 填写审批意见 → preSubmit → submit
```

#### 场景三：查看已办
```
用户点击已办 → doOpenDocByCode → 只读查看（getOpinionList + getHandleLogs）
```

### 2.3 响应格式（JSON）

所有操作通过 `super.out3(data, status, statusText, wax)` 输出 JSON：

```json
// 通用格式
{
  "status": 1,           // 1=成功，其他=错误码
  "statusText": null,    // 成功时为 null，失败时显示错误信息
  "wax": "hmac_token",   // SealingWax HMAC 签名
  "data": { ... }        // 具体数据
}
```

**各种场景的数据结构：**

#### 新增实体成功
```json
{
  "entity_id": 12345,
  "entity_code": "OP20260624001",
  "entity_title": "OP申请单 #001",
  "task_id": 67890,
  "status": 1,
  "statusText": null,
  "wax": "..."
}
```

#### 更新实体成功
```json
{
  "entity_title": "OP申请单 #001（已更新）",
  "status": 1,
  "statusText": null,
  "wax": "..."
}
```

#### 预提交——需要选择下一环节
```json
{
  "status": 100,  // FEEDBACK_NEXT_ACT_PENDING
  "statusText": "请选择下一步操作",
  "wax": "...",
  "actOptions": [
    {
      "activity": { ... },  // NovaActivity 对象
      "link": { ... }       // NovaLink 对象（含路由方向）
    }
  ]
}
```

#### 预提交——需要选择下一处理人
```json
{
  "status": 101,  // FEEDBACK_NEXT_HANDLER_PENDING
  "statusText": "请选择处理人",
  "wax": "...",
  "act": { ... },           // 目标活动
  "ths": [
    [
      { "uid": "zhangsan", "name": "张三", ... },
      { "uid": "lisi", "name": "李四", ... }
    ]
  ],
  "node": { ... }           // Handler 树形结构
}
```

#### 打开实体（文档详情/编辑页）
```json
{
  "status": 1,
  "wax": "...",
  "document": {
    "process": { ... },    // NovaProcess
    "entity": { ... },     // NovaEntity
    "curHandler": { ... }, // 当前处理人
    "curTask": { ... },    // 当前环节任务
    "activity": { ... },   // 当前活动
    "todoTaskList": [ ... ], // 待办任务列表
    "mode": "new|edit|view" // 页面模式
  }
}
```

### 2.4 实体旗标（Entity Flag）

这是一个可扩展的旗标系统，允许用户对实体设置自定义标记（如：已标记、星标等），通过 `setEntityFlag` / `getEntityFlag` 操作。设计为可横向扩展。

### 2.5 关注系统（Attention）

用户可关注实体，关注后当实体发生变化（被处理、流转等）会通知关注者。`CHANGE_FLAG` 标识变化状态：
- `0` = 未改变
- `1` = 已改变且已知悉
- `2` = 已改变且未知悉

有历史表 `NOVA_WFM_ENTITY_ATTENTION_HIST` 归档关注记录。

### 2.6 意见系统（Opinion）

每个实体可以有多个意见（审批意见/留言）。意见支持父子结构（回复关系）：
- `PARENT_ID` = 父意见 ID（可回复某条意见）
- 关键字段：`OPINION_CONTENT`（内容，最大 4000 字符）、`ACT_TITLE`（反范式冗余活动标题）

### 2.7 处理日志（Handle Log）

按步骤+任务粒度记录处理轨迹。每个处理记录包含：
- 到达时间、受理时间、完成时间
- 处理人和处理人名称
- 日志信息
- 客户端 IP（insert 和 update 分别记录）
- 客户端类型（0=web, 1=email, 2=java, 3=public）

### 2.8 步骤日志（Step Log）

以步骤为粒度记录流转过程（与处理日志不同，偏流程引擎内部记录）：
- `CUR_ACT_ID` → `DEST_ACT_ID`（从当前活动到目标活动的转向）
- `LINK_TYPE`（链接类型）
- 冗余存储目标活动的名称、标题、类型

### 2.9 流程阅读说明（Process Instructions）

每个流程（按 PRO_ALIAS）可维护一份 HTML/Markdown 文本说明，用户发起流程前可查看。并有 `never_remind_me` 机制（通过 `doSaveUserProAlias` 设置）。

### 2.10 行为分析

`insertAnalysis` 将用户操作数据写入 `NOVA_ANALYSIS_SUBMIT_TIME` 表，记录操作类型、耗时、流程别名、当前环节、目标环节、处理人等，用于统计分析（类似计时审计）。

---

## 三、实体模型（Entity）—— 核心数据对象

### 3.1 数据库表：NOVA_WFM_ENGINE_ENTITIES

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | int(11) PK | 主键，自增 |
| `CODE` | varchar(255) UNIQUE | **唯一编码**（业务标识，UUID 或业务编码） |
| `PRO_ID` | int(11) FK | 流程 ID（关联 NOVA_WFM_ENGINE_PROCESS） |
| `PRO_VER` | int(11) | 流程版本号 |
| `ENTITY_TITLE` | varchar(200) | **实体标题**（文档标题） |
| `URGENCY` | int(11) | 紧急程度（预留，未定义具体枚举） |
| `DRAFT_UID` | varchar(50) | **拟稿人 UID** |
| `DRAFT_NAME` | varchar(50) | 拟稿人名称（冗余） |
| `DRAFT_DEPT_ID` | varchar(50) | 拟稿人部门 ID |
| `DRAFT_DEPT_NAME` | varchar(200) | 拟稿人部门名称（冗余） |
| `SEND_TIME` | datetime | 提交时间 |
| `OVER_TIME` | datetime | 完结时间 |
| `DEL_FLAG` | int(11) | 删除标识：0=缺省/正常，1=已删除 |
| `STATE` | int(11) | **处理状态**：0=未提交(草稿)，1=已提交(处理中)，100=已完结 |
| `PAR_ENTITY_FLAG` | int(11) | 父实体存在标识：0=无，1=有 |
| `SUB_ENTITY_FLAG` | int(11) | 子实体存在标识：0=无，1=有 |
| `PAR_ENTITY_ID` | int(11) | 父实体 ID |
| `SERIAL_NUM` | varchar(50) UNIQUE | **流水号**（业务流水号，如 OP20260624001） |

### 3.2 实体状态枚举（EntityStateEnum）

| 值 | 名称 | 说明 |
|----|------|------|
| 0 | DRAFT | 草稿（未提交） |
| 1 | SUBMIT | 已提交（处理中） |
| 100 | OVER | 已完结 |

> 原 nova4 源码中 `EntityStateEnum` 还定义了 `BACK`（退回）、`CANCEL`（撤销）等，但 DDL 中只有 `STATE` 字段用 0/1/100。退回和撤销由引擎内部状态管理而非实体状态字段。

### 3.3 实体视图（EntityView）—— 不可变对象模式

实体对外暴露 `EntityView` 为不可变对象（只读），内部通过 `EntityStore` 操作 `EntityPojo` 持久化。`EntityView` 包含 `_pojo` 引用指向原始 POJO 用于后续的字段拷贝转换。

### 3.4 实体创建参数（EntityBean）

创建实体最少需提供：
- `code` — 实体编码
- `proId` — 流程 ID
- `proVerNum` — 流程版本号
- `proVerId` — 流程版本 ID（从 ProVer 表获取）
- `title` — 实体标题（可选）

---

## 四、流程模型（Process）

### 4.1 流程表：NOVA_WFM_ENGINE_PROCESS

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | int(11) PK | 主键 |
| `PRO_NAME` | varchar(100) | 流程名称（如「OP 申请单」） |
| `PRO_ALIAS` | varchar(10) UNIQUE | **流程别名**（4 位大写英文字母，如 `OP`、`LEAVE`） |
| `IS_VALID` | tinyint(1) | 是否有效 |
| `PRO_OWNER` | varchar(100) | 流程所有者 |
| `EMAIL_NAME` | varchar(100) | 系统邮件发件人名称 |
| `EMAIL_ADDRESS` | varchar(100) | 系统邮件发件人地址 |
| `PROJECT_ID` | varchar(100) | 项目 ID |
| `PRO_CLASS` | varchar(100) | 流程分类 |
| `PRO_DESC` | varchar(1000) | 流程概述 |
| `ONLINE_VER` | int(11) | 线上版本（当前生效版本） |
| `CHANNEL_CLASS` | varchar(1000) | **管道类名**（实体业务交互类，完整 Java 类名） |
| `SN_DATE` | datetime | 编号时间（用于流水号生成策略） |
| `SN_NUM` | int(11) | 编号流水（用于生成流水号） |
| `BUTTON_SHIELD` | varchar(50) | 按钮屏蔽：F=流程图（`F` 开头） |
| `UDC_CLASS` | varchar(1000) | UDC 用户自定义控制器完整类名 |
| `MOBILE_LEVEL` | int(11) | 移动端级别：0=不支持，10=只读浏览，20=只读审批，30=可编辑，40=可编辑处理 |
| `META` | varchar(100) | 元数据 |
| `TIME_LIMIT` | int(11) | 超时时间（小时） |

### 4.2 流程版本表：NOVA_WFM_ENGINE_PRO_VER

| 字段 | 说明 |
|------|------|
| `ID` | 主键 |
| `PRO_ID` | 流程 ID |
| `PRO_VER` | 版本号（递增） |
| `IS_RELEASE` | 是否已发布：0=未发布，1=已发布 |

> 流程的运行时定义通过 `PRO_ID` + `PRO_VER` 定位到 `ProVer` 表，获得 `ID` 作为 `proVerId`。

---

## 五、活动/节点模型（Activity / Node）—— 核心设计

### 5.1 活动表：NOVA_WFM_ENGINE_ACTIVITY

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | int(11) PK | 主键 |
| `PRO_ID` | int(11) FK | 流程 ID |
| `ACT_NAME` | varchar(100) UNIQUE | **活动标识**（英文数字，发布后不可变，如 `draft`、`approve`） |
| `ACT_TITLE` | varchar(100) | **活动标题**（中文显示名，如「起草申请」「审批」） |
| `ACT_DESC` | varchar(1000) | 活动描述 |
| `ACT_VER` | int(11) DEFAULT 1 | 活动版本 |
| `ACT_TYPE` | int(11) DEFAULT 1 | **活动类型**（详见下方） |
| `ACT_SUB_TYPE` | int(11) DEFAULT 0 | **活动子类型**（组合 ACT_TYPE 使用） |
| `IS_FORCE_OPINION` | int(11) DEFAULT 0 | **是否强制填写意见**：0=不强制，1=强制 |
| `EDIT_FLAG` | varchar(8) DEFAULT 'VIEW' | **可编辑标识**：EDIT=可编辑，VIEW=只读 |
| `DECISION_FILTER` | varchar(100) DEFAULT 'none' | **决策意见**：`活动名称?0/1|...` 格式，问号后 0/1 为强制意见标识，管道符分割多个决策意见 |
| `BUTTON_SHIELD` | varchar(50) DEFAULT 'none' | **按钮屏蔽**：S=保存，D=删除，C=撤单，O=意见，L=日志（逗号分隔） |
| `FUNCTION_SWITCH` | varchar(50) DEFAULT 'none' | **功能开关**：E=email 审批 |
| `BUTTON_UDN` | varchar(200) DEFAULT 'none' | **自定义按钮名**：`按钮英文名=自定义名称|...` |
| `MOBILE_LEVEL` | int(11) DEFAULT 0 | 移动端级别 |
| `TIME_LIMIT` | int(11) | 超时时间（小时） |
| `WAIT_ACTS` | varchar(128) | **等待参数**：需要等待完结的活动名称集合（逗号分隔） |

### 5.2 活动类型（ACT_TYPE）

| 值 | 说明 |
|----|------|
| 1 | **START** / 开始节点 |
| 2 | **MANUAL** / 人工节点（需要人处理） |
| 3 | **ROUTER** / 路由交互节点 |
| 4 | **CONVERGE** / 汇聚节点 |
| 5 | **WAIT** / 等待节点（异步等待外部事件） |
| 6 | **END** / 结束节点 |
| 7 | **SPLITTER** / 分流节点（Nova4 新增，nova3.x 不存在） |

### 5.3 子类型（ACT_SUB_TYPE）—— 仅 ACT_TYPE=3（路由交互）时有效

| 值 | 说明 |
|----|------|
| 1 | 决策分支（用户选择走向） |
| 2 | 条件分支（系统按条件自动路由） |
| 3 | 并发活动（多路同时前进） |

### 5.4 可编辑标识（EDIT_FLAG）

| 值 | 说明 |
|----|------|
| `EDIT` | 表单可编辑 |
| `VIEW` | 表单只读 |

### 5.5 活动链接表：NOVA_WFM_ENGINE_ACT_LINK

| 字段 | 说明 |
|------|------|
| `LINK_ID` | 主键 |
| `PRO_VER_ID` | 流程版本 ID |
| `ACT_ID` | 活动 ID（当前活动） |
| `PREV_ACT_ID` | 上一活动 ID（前驱活动） |
| `LINK_TITLE` | 链接标题（路由标签，如「同意」「驳回」） |
| `LINK_TYPE` | **链接类型**：1=前进路径，2=回退路径 |

> 注意：链接是单向的，从 `PREV_ACT_ID` → `ACT_ID`。LINK_TYPE 区分前进和回退，这是实现「退回/驳回」操作的关键机制。

### 5.6 运行时活动属性（RtAct 对象）

运行时活动 `RtAct` 包含活动定义的所有属性，并通过 `get_pojo()` 返回对应 POJO 用于字段拷贝到 `NovaActivityModel`。

`NovaActivityModel` 扩展属性：
- `handlerRule`: `NovaHandlerRuleModel` — 处理人规则
- `decisionFilterList`: `NovaDecisionFilterModel[]` — 决策意见列表
- `buttonUdnMap`: `Map<String,String>` — 自定义按钮名映射
- `process`: `NovaProcess` — 所属流程的引用

### 5.7 决策过滤器格式

`DECISION_FILTER` 字段存储格式示例：
```
同意=批准?0|驳回=不同意?1
```
解析后为 `NovaDecisionFilterModel` 列表：
- `name` = 决策名称（如「同意」）
- `mapping` = 映射值（如「批准」）
- `isForce` = 是否强制（0/1）

---

## 六、处理人模型（Handler）

### 6.1 处理人属性（Nova4Handler）

| 属性 | 说明 |
|------|------|
| `uid` | 唯一 ID |
| `name` | 名称 |
| `orgCode` | 部门编号 |
| `orgName` | 部门名称 |
| `orgFullName` | 部门全称 |
| `handlerType` | 处理人类型（枚举） |

### 6.2 处理人类型（HandlerEnum）

| 枚举 | 说明 | 映射到 Nova3.x |
|------|------|---------------|
| `HUMAN` | 真实存在的人 | HandlerType.HUMAN |
| `GHOST` | 曾经存在的人（已离职） | HandlerType.GHOST |
| `AVATAR` | 组织机构外真实存在的人（外部人员） | HandlerType.AVATAR |
| `ANGEL` | 从未存在的人（虚拟角色） | HandlerType.ANGEL |
| `ROBOT` | 机器人 | HandlerType.ANGEL（nova3.x 不存在） |
| `NPC` | 系统角色 | HandlerType.ANGEL（nova3.x 不存在） |
| `GOD` | 神（超级管理员） | HandlerType.GODDESS |

### 6.3 化身处理人（AVATAR Handler）

`NOVA_WFM_AVATAR_HANDLER` 表存储不在组织机构内但真实存在的外部人员。字段包含 UID、名称、部门信息。便于在审批流程中选择外部人员作为处理人。

### 6.4 备份处理人（Backup Handler）

`NOVA_WFM_BACKUP_HANDLER` 表让处理人自定义备份人：
- `AUTH_UID` → 授权人
- `AUTH_PRO_ID` → 授权流程（-1000=全部流程）
- `BACKUP_UID` / `BACKUP_NAME` → 备份处理人

---

## 七、处理人规则（Handler Rule）

### 7.1 规则表：NOVA_WFM_HANDLER_RULE

| 字段 | 说明 |
|------|------|
| `ACT_ID` | 活动 ID（PK） |
| `DEPT_ID` | 部门 ID（纵向部门关系） |
| `TEAM_ID` | 团队 ID（横向合作关系，支持多个逗号分隔） |
| `ROLE_ID` | 角色 ID |
| `TASK_ACT_ID` | 环节活动 ID（运行时通过活动 ID 从任务表/任务处理人表取实际处理人） |
| `TASK_ACT_NAME` | 环节活动 Name（取代活动 ID） |
| `BASE_ON` | **处理人筛选基准**（见下方） |
| `HANDLE_TYPE` | **处理类型**（见下方） |
| `IS_HANDLER_SEL` | **处理人可选性**：0=不可选，1=可选 |

### 7.2 BASE_ON（处理人筛选基准）

| 值 | 名称 | 说明 |
|----|------|------|
| 0 | CHANNEL_BASED | 管道自定义（由业务代码决定） |
| 1 | DEPT_BASED | 按部门 |
| 2 | TEAM_BASED | 按团队 |
| 3 | ROLE_BASED | 按角色 |
| 4 | TASK_BASED | 按环节（通过 TASK_ACT_ID 指定另一个活动上的处理人） |

### 7.3 HANDLE_TYPE（处理类型/策略）

| 值 | 名称 | 说明 |
|----|------|------|
| 0 | NONE | 无人处理（自动通过） |
| 10 | SINGLE | 单人处理 |
| 20 | COUNTERSIGN | 多人会签（都要处理） |
| 21 | EXCLUSIVE | 多人排他（一人处理即可） |

### 7.4 处理人选择树（HandlerNode）

当 `IS_HANDLER_SEL=1` 时，前端展示处理人选择界面。数据结构为树形结构 `HandlerNode`（通过 `NovaHandlerNodeFactory.toRootHandlerNode()` 构建），支持按拼音排序。

---

## 八、任务模型（Task & Todo）

### 8.1 环节任务（RunTask）—— 运行时

| 属性 | 说明 |
|------|------|
| `id` | 任务 ID |
| `stepId` | 步骤 ID |
| `actId` | 活动 ID |
| `actName` | 活动名称（冗余） |
| `actTitle` | 活动标题（冗余） |
| `entityId` | 实体 ID |
| `startTime` | 开始时间 |
| `endTime` | 结束时间 |
| `state` | **任务状态** |
| `converge` | **汇聚参数**（存储将汇聚过来的活动主键） |
| `waiting` | **等待参数**（需要等待完结的活动名称集合，逗号分隔） |
| `creator` | 创建者（NovaOper，谁创建了这个任务） |

### 8.2 任务状态（TaskStateEnum）

| 值 | 名称 | 说明 |
|----|------|------|
| 1 | PENDING | 同步汇聚中 |
| 2 | WAITING | 异步等待中 |
| 3 | READY | 已就绪（可生成待办） |
| 4 | TODO | 已生成待办 |
| 5 | PROCESSING | 正在处理 |
| 6 | PAUSING | 暂停 |
| 100 | OVER | 已处理完毕 |

### 8.3 数据库表结构

**运行中任务表** `NOVA_WFM_RUN_TASK`：
- 存放当前正在运行（未完结）的任务
- 流程完结后删除

**已完成任务表** `NOVA_WFM_OVER_TASK`：
- 记录所有已完结的任务（历史归档）
- 记录实际的 `HANDLERS`（处理人姓名串，冗余显示用）
- 记录 `CONVERGE`（汇聚参数）和 `WAITING`（等待参数）

### 8.4 待办（RunTodo）

| 属性 | 说明 |
|------|------|
| `id` | 待办 ID |
| `todoKey` | **唯一 Key**（加密，可用于无需认证的邮件审批） |
| `entityId` / `entityTitle` | 实体 |
| `proId` / `proTitle` | 流程 |
| `actId` / `actTitle` | 活动 |
| `taskId` | 所属任务 |
| `arriveTime` | 到达时间 |
| `handler` | 待办人（NovaOper） |
| `sender` | 提交人（NovaOper） |

**待办表** `NOVA_WFM_RUN_TODO` 额外字段：
- `ACCEPT_TIME` — 受理时间
- `ACCEPT_FLAG` — 受理标识（0=未受理，1=已受理）
- `HANDLER_NAME` — 代办人名称（冗余）
- `SENDER_UID` / `SENDER_NAME` — 提交人
- `PRO_NAME` — 流程名称（冗余）
- `TODO_KEY` — **待办唯一 Key**（用于邮件审批单点登录无密码验证）

**已办表** `NOVA_WFM_ENTITY_DONE`：
- 记录处理人+实体维度的已办
- `FIRST_TIME` — 首次处理时间
- `LAST_TIME` — 最后处理时间
- 处理人+实体 ID = 唯一

### 8.5 环节处理人表（NOVA_WFM_TASK_HANDLER）

| 字段 | 说明 |
|------|------|
| `TASK_ID` | 环节 ID |
| `Uid` | 处理人 UID |
| `Name` | 处理人名称 |
| `OrgCode` | 部门编号 |
| `OrgName` | 部门名称 |
| `OrgFullName` | 部门全称 |

> 任务处理人表记录一个实体在某一环节的实际处理人。与 Handler Rule 配合使用：Handler Rule 定义「谁能处理」，Task Handler 记录「谁在处理/处理过」。

---

## 九、关键设计模式

### 9.1 不可变对象模式（Immutable Object）

`EntityView`、`RunTask`、`RunTodo` 均为不可变对象：
- 构造函数初始化所有属性
- 仅通过 `EntityStore` 代理类进行持久化操作
- `setter` 方法为 `default`（包级）权限，类外部不可见
- 状态更新返回新实例（Immutable 风格）

### 9.2 适配器模式（Adapter）

`Nova4ToModelFactory` 将 Nova4 运行时对象（`RtPro`、`RtAct`、`RtLink`、`RunTask`、`RunTodo`）转换为 Nova3.x 向前兼容的模型（`NovaProcess`、`NovaActivity`、`NovaTask`、`NovaToDo`）。

### 9.3 代理模式（Proxy）

`Nova4EntityTaskProxy` / `Nova4ProcessActivityProxy` / `Nova4ProcessChannelProxy` 将只读方法从 `NovaEntity` 延迟委派到 `TaskStore` / `RtPro`，实现接口隔离。

### 9.4 按钮控制系统

三层按钮控制：

```
流程级 → BUTTON_SHIELD (F=流程图)
   ↓
环节级 → BUTTON_SHIELD (S/D/C/O/L)
   ↓
自定义 → BUTTON_UDN (按钮英文名=自定义名称)
```

### 9.5 流程图数据

`getFlowChartJSON()` 返回流程图 JSON 数据，供前端渲染流程图。在当前 nova4-src 中实现为空（返回 null），实际流程图数据由 Nova3.x 框架的 `NovaBaseJsonController` 基础类提供。

### 9.6 SealingWax（Wax）

所有重要操作响应均包含 `wax`（HMAC 签名令牌），用于防止响应篡改和 CSRF。

---

## 十、与其他系统的差异点（Go 版 Design References）

### 10.1 Pandora 特有的设计（Go 版可参考采纳）

- **@ant 无依赖**：Pandora Nova 框架不依赖任何前端 UI 框架，可适配任何前端
- **按钮控制体系**：三级按钮屏蔽/命名系统（流程→环节→自定义）
- **关注系统**：实体关注 + 状态变更通知
- **意见/日志分离**：Opinion（审批意见）vs HandleLog（处理日志）vs StepLog（流转轨迹）
- **决策过滤器**：环节可以预设决策意见（同意/驳回+强制标识）
- **移动端级别**：每个流程和环节支持配置移动端能力级别

### 10.2 Pandora 中已被标记为废弃的设计（Go 版跳过）

- `NovaFrameworkService` 类标记为 `@Deprecated`（已被 Action 模式替代）
- 所有 Nova3.x 向后兼容的转换（`NovaProcessActivityProxy`、`NovaProcessChannelProxy` 等）是兼容包袱
- `Nova4FrameController` 中 `doOpenDocBy*` 系列方法均返回 null（实际由 Nova3.x 框架处理）

### 10.3 当前 Go 版的差距

当前 Go 版已经实现了核心引擎和实体持久化，但与 Pandora 相比缺少：

1. **UI 方面**：
   - 意见/留言系统（Opinion CRUD + 父子结构）
   - 关注系统（Attention + 变更通知）
   - 处理日志（HandleLog 完整轨迹）
   - 步骤日志（StepLog 流转过程）
   - 实体旗标（Flag 系统）
   - 流程图 JSON API（FlowChartJSON）
   - 行为分析记录（Analysis submit time）
   - 流程申请说明（ProcessInstructions + never_remind_me）

2. **实体属性方面**：
   - 紧急程度（urgency）
   - 父/子流程标识和关联（par_entity_flag/sub_entity_flag/par_entity_id）
   - 流水号（serial_num/SERIAL_NUM）和编号生成策略

3. **任务/待办方面**：
   - 待办已办完整 CRUD
   - TodoKey（邮件审批用加密 key）
   - 受理标识（ACCEPT_FLAG）
   - 任务处理人表（TaskHandler）

4. **处理人模型方面**：
   - 化身处理人（Avatar Handler — 外部人员）
   - 备份处理人（Backup Handler — 授权代理）
   - 处理人拼音排序
   - 处理人选择树（HandlerNode 树形结构）
   - 已办记录（EntityDone — 处理人+实体维度）

5. **流程定义方面**：
   - 版本管理（ProVer）
   - 活动版本（ActVer）
   - 管道类（ChannelClass — 业务交互类）
   - UDC（用户自定义控制器）
   - 流水号生成策略（SN_DATE + SN_NUM）
   - 按钮屏蔽和自定义
   - 决策过滤器
   - 移动端级别
   - 超时时间
   - 邮件配置

6. **前端路由方面**：
   - 实体三种打开方式（byCode/byEntityId/bySerialNum）
   - 发起→暂存→预提交→提交的完整流程
   - 选择下一环节/处理人的交互
