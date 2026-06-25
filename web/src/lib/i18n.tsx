import { createContext, useContext, useMemo, useState, type ReactNode } from "react";

type TranslationKey =
  | "common.cancel" | "common.close" | "common.confirm" | "common.loading"
  | "common.reload" | "common.save" | "common.search"
  | "common.previousPage" | "common.nextPage" | "common.none" | "common.unknown"
  | "common.loadingCurrent" | "common.notConfigured"

  | "sidebar.platform" | "sidebar.group.overview"
  | "sidebar.group.processManagement" | "sidebar.group.taskManagement"
  | "sidebar.group.system"
  | "sidebar.item.welcome" | "sidebar.item.processes" | "sidebar.item.entities"
  | "sidebar.item.todos" | "sidebar.item.outbox" | "sidebar.item.about"
  | "sidebar.item.users"
  | "sidebar.loadingApp" | "sidebar.unknownApp" | "sidebar.appFallback"

  | "welcome.badge" | "welcome.title.line1" | "welcome.title.line2"
  | "welcome.description"
  | "welcome.tile.stable.title" | "welcome.tile.stable.description"
  | "welcome.tile.focused.title" | "welcome.tile.focused.description"
  | "welcome.tile.quiet.title" | "welcome.tile.quiet.description"
  | "welcome.entryLabel" | "welcome.entryAction"

  | "processes.page.eyebrow" | "processes.page.title" | "processes.page.description"
  | "entities.page.eyebrow" | "entities.page.title" | "entities.page.description"
  | "todos.page.eyebrow" | "todos.page.title" | "todos.page.description"
  | "outbox.page.eyebrow" | "outbox.page.title" | "outbox.page.description"

  | "currentUser.defaultName" | "currentUser.normalUser"
  | "currentUser.logout"
  | "currentUser.section.language" | "language.zh-CN" | "language.en-US"

  | "router.notFound.title" | "router.notFound.description" | "router.notFound.back"

  | "processes.table.name" | "processes.table.status" | "processes.table.created"
  | "processes.status.active" | "processes.status.paused" | "processes.status.completed"
  | "processes.status.failed"
  | "todos.table.title" | "todos.table.process" | "todos.table.priority" | "todos.table.due"
  | "todos.priority.high" | "todos.priority.medium" | "todos.priority.low"
  | "outbox.table.subject" | "outbox.table.recipient" | "outbox.table.scheduled" | "outbox.table.status"
  | "outbox.status.pending" | "outbox.status.sent" | "outbox.status.failed"
  | "entities.table.name" | "entities.table.type" | "entities.table.status" | "entities.table.updated"
  | "entities.all" | "entities.active" | "entities.completed"
  | "entities.table.code" | "entities.table.serialNum" | "entities.table.currentAct" | "entities.table.draftName" | "entities.table.createdAt"
  | "entities.create.title" | "entities.create.placeholder" | "entities.create.success"
  | "entities.detail.title" | "entities.detail.basicInfo" | "entities.detail.tasks" | "entities.detail.todos" | "entities.detail.logs"
  | "entities.detail.submit" | "entities.detail.submitSuccess"
  | "entities.status.draft" | "entities.status.processing" | "entities.status.completed"
  | "todos.table.entityTitle" | "todos.table.actName" | "todos.table.actions"
  | "todos.accept" | "todos.process" | "todos.acceptSuccess" | "todos.processSuccess"
  | "outbox.table.entityTitle" | "outbox.table.currentAct" | "outbox.table.finishedAt"
  | "entity.logs.empty"
  | "users.page.title" | "users.page.description"
  | "users.create.title" | "users.edit.title"
  | "users.table.uid" | "users.table.name" | "users.table.dept" | "users.table.email" | "users.table.phone" | "users.table.state" | "users.table.actions"
  | "users.field.uid" | "users.field.name" | "users.field.dept" | "users.field.email" | "users.field.phone" | "users.field.state"
  | "users.state.active" | "users.state.disabled"
  | "users.delete.confirm";

const zhCN: Record<string, string> = {
  "common.cancel": "取消",
  "common.close": "关闭",
  "common.confirm": "确认",
  "common.loading": "加载中...",
  "common.reload": "重新加载",
  "common.save": "保存",
  "common.search": "查询",
  "common.previousPage": "上一页",
  "common.nextPage": "下一页",
  "common.none": "无",
  "common.unknown": "未知",
  "common.loadingCurrent": "正在读取...",
  "common.notConfigured": "暂未提供",

  "sidebar.platform": "Nova Platform",
  "sidebar.group.overview": "概览",
  "sidebar.group.processManagement": "流程管理",
  "sidebar.group.taskManagement": "任务管理",
  "sidebar.group.system": "系统",
  "sidebar.item.welcome": "欢迎页",
  "sidebar.item.processes": "流程定义",
  "sidebar.item.entities": "工单管理",
  "sidebar.item.todos": "待办事项",
  "sidebar.item.outbox": "发件箱",
  "sidebar.item.about": "关于",
  "sidebar.item.users": "用户管理",
  "sidebar.loadingApp": "正在读取...",
  "sidebar.unknownApp": "Nova",
  "sidebar.appFallback": "工作流引擎",

  "welcome.badge": "Nova Platform",
  "welcome.title.line1": "Nova workflow,",
  "welcome.title.line2": "a calmer place to start.",
  "welcome.description": "欢迎回来。这里不需要立刻给你抛出一整屏系统细节，只留一块干净、安静、可继续工作的起点。",
  "welcome.tile.stable.title": "STABLE",
  "welcome.tile.stable.description": "流程定义与状态一致，引擎在后台可靠运行。",
  "welcome.tile.focused.title": "FOCUSED",
  "welcome.tile.focused.description": "把注意力留给你接下来要处理的事，而不是首页本身。",
  "welcome.tile.quiet.title": "QUIET",
  "welcome.tile.quiet.description": "少一点说明，多一点呼吸感。",
  "welcome.entryLabel": "Entry Point",
  "welcome.entryAction": "准备好了就从左侧继续。",

  "processes.page.eyebrow": "流程管理",
  "processes.page.title": "流程定义",
  "processes.page.description": "查看和管理工作流流程定义及其当前状态。",
  "entities.page.eyebrow": "工单管理",
  "entities.page.title": "工单列表",
  "entities.page.description": "查看和管理所有流程工单实例。",
  "todos.page.eyebrow": "任务管理",
  "todos.page.title": "待办事项",
  "todos.page.description": "查看和处理当前用户待处理的审批和任务。",
  "outbox.page.eyebrow": "消息管理",
  "outbox.page.title": "发件箱",
  "outbox.page.description": "查看已发送和待发送的消息与通知。",

  "currentUser.defaultName": "Nova 用户",
  "currentUser.normalUser": "普通用户",
  "currentUser.logout": "退出",
  "currentUser.section.language": "语言",
  "language.zh-CN": "简体中文",
  "language.en-US": "English",

  "router.notFound.title": "页面不存在",
  "router.notFound.description": "当前管理页面尚未定义或路由尚未迁移完成。",
  "router.notFound.back": "返回控制台",

  "processes.table.name": "流程名称",
  "processes.table.status": "状态",
  "processes.table.created": "创建时间",
  "processes.status.active": "运行中",
  "processes.status.paused": "已暂停",
  "processes.status.completed": "已完成",
  "processes.status.failed": "失败",
  "todos.table.title": "标题",
  "todos.table.process": "所属流程",
  "todos.table.priority": "优先级",
  "todos.table.due": "截止日期",
  "todos.priority.high": "高",
  "todos.priority.medium": "中",
  "todos.priority.low": "低",
  "outbox.table.subject": "主题",
  "outbox.table.recipient": "收件人",
  "outbox.table.scheduled": "计划时间",
  "outbox.table.status": "状态",
  "outbox.status.pending": "待发送",
  "outbox.status.sent": "已发送",
  "outbox.status.failed": "发送失败",
  "entities.table.name": "工单名称",
  "entities.table.type": "类型",
  "entities.table.status": "状态",
  "entities.table.updated": "更新时间",

  "entities.all": "全部",
  "entities.active": "处理中",
  "entities.completed": "已完结",
  "entities.table.code": "流水号",
  "entities.table.serialNum": "流水号",
  "entities.table.currentAct": "当前环节",
  "entities.table.draftName": "起草人",
  "entities.table.createdAt": "创建时间",
  "entities.create.title": "新建工单",
  "entities.create.placeholder": "请输入工单标题",
  "entities.create.success": "工单创建成功",
  "entities.detail.title": "工单详情",
  "entities.detail.basicInfo": "基本信息",
  "entities.detail.tasks": "任务列表",
  "entities.detail.todos": "当前待办",
  "entities.detail.logs": "处理日志",
  "entities.detail.submit": "提交流转",
  "entities.detail.submitSuccess": "流转提交成功",
  "entities.status.draft": "草稿",
  "entities.status.processing": "处理中",
  "entities.status.completed": "已完结",
  "todos.table.entityTitle": "工单标题",
  "todos.table.actName": "环节名称",
  "todos.table.actions": "操作",
  "todos.accept": "受理",
  "todos.process": "处理",
  "todos.acceptSuccess": "受理成功",
  "todos.processSuccess": "处理成功",
  "outbox.table.entityTitle": "工单标题",
  "outbox.table.currentAct": "当前环节",
  "outbox.table.finishedAt": "处理时间",
  "entity.logs.empty": "暂无处理记录",
  "users.page.title": "用户管理",
  "users.page.description": "管理平台用户及其权限。",
  "users.create.title": "新建用户",
  "users.edit.title": "编辑用户",
  "users.table.uid": "登录名",
  "users.table.name": "姓名",
  "users.table.dept": "部门",
  "users.table.email": "邮箱",
  "users.table.phone": "手机号",
  "users.table.state": "状态",
  "users.table.actions": "操作",
  "users.field.uid": "登录名",
  "users.field.name": "姓名",
  "users.field.dept": "部门",
  "users.field.email": "邮箱",
  "users.field.phone": "手机号",
  "users.field.state": "状态",
  "users.state.active": "正常",
  "users.state.disabled": "停用",
  "users.delete.confirm": "确定要删除该用户吗？",
};

type TranslationParams = Record<string, string | number>;
type I18nContextValue = {
  t: (key: TranslationKey, replacements?: TranslationParams) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

function translate(key: TranslationKey, replacements?: TranslationParams) {
  let text = zhCN[key] ?? key;
  if (replacements) {
    Object.entries(replacements).forEach(([name, value]) => {
      text = text.replace(new RegExp(`\\{${name}\\}`, "g"), String(value));
    });
  }
  return text;
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const value = useMemo<I18nContextValue>(
    () => ({
      t: (key, replacements) => translate(key, replacements),
    }),
    [],
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error("useI18n must be used within I18nProvider.");
  }
  return context;
}

export type { TranslationKey };
