import { Activity, ArrowRight, CheckCircle, Clock, FileText, Inbox, ListTodo, Users } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { useI18n } from "@/lib/i18n";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export function WelcomePage() {
  const { t } = useI18n();

  const stats = [
    { icon: Activity, label: "活跃流程", value: "4", color: "text-primary" },
    { icon: FileText, label: "今日工单", value: "12", color: "text-primary" },
    { icon: Clock, label: "待审批", value: "3", color: "text-primary" },
    { icon: CheckCircle, label: "本月完成", value: "28", color: "text-primary" },
  ];

  const recentProcesses = [
    { name: "采购审批流程", status: "运行中", statusColor: "default" as const },
    { name: "报销流程", status: "运行中", statusColor: "default" as const },
    { name: "请假审批", status: "已暂停", statusColor: "secondary" as const },
    { name: "合同签署", status: "已完成", statusColor: "secondary" as const },
  ];

  const quickActions = [
    { icon: FileText, label: "新建流程", to: "/processes" },
    { icon: Inbox, label: "创建工单", to: "/entities" },
    { icon: ListTodo, label: "待办事项", to: "/todos" },
  ];

  return (
    <div className="space-y-8">
      {/* Brand Header */}
      <div className="relative overflow-hidden rounded-3xl border border-border/60 bg-[linear-gradient(135deg,rgba(248,250,252,0.98),rgba(255,255,255,0.95)_42%,rgba(240,245,255,0.92)_100%)] px-8 py-10 shadow-sm">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_18%_22%,rgba(54,111,255,0.06),transparent_38%),radial-gradient(circle_at_82%_58%,rgba(54,111,255,0.04),transparent_42%)]" />
        <div className="relative">
          <div className="text-[11px] font-semibold uppercase tracking-[0.28em] text-primary/60">
            Nova Workflow Engine
          </div>
          <div className="mt-4 flex items-end gap-6">
            <div>
              <div className="text-4xl font-semibold tracking-tight sm:text-5xl">
                <span className="text-primary">N</span>ova
              </div>
              <p className="mt-3 max-w-lg text-sm leading-7 text-muted-foreground">
                Go 原生的轻量工作流引擎。状态机驱动、并发汇聚、可嵌入式 PDK。
              </p>
            </div>
            <div className="hidden items-center gap-2 rounded-full border border-border/60 bg-white/70 px-4 py-2 text-xs text-muted-foreground backdrop-blur sm:flex">
              <div className="h-2 w-2 rounded-full bg-green-500" />
              引擎运行中
            </div>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <Card key={stat.label} size="sm">
            <CardContent className="flex items-center gap-4 px-4 py-4">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/5">
                <stat.icon className={`h-5 w-5 ${stat.color}`} />
              </div>
              <div>
                <div className="text-2xl font-semibold tracking-tight">{stat.value}</div>
                <div className="text-xs text-muted-foreground">{stat.label}</div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Quick Actions + Recent */}
      <div className="grid gap-6 lg:grid-cols-[1fr_360px]">
        {/* Recent Processes */}
        <Card>
          <CardHeader className="border-b border-border/50">
            <CardTitle>最近流程</CardTitle>
            <CardDescription>当前系统中的活跃流程定义</CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            <div className="divide-y divide-border/50">
              {recentProcesses.map((p) => (
                <div key={p.name} className="flex items-center justify-between px-4 py-3.5">
                  <div className="flex items-center gap-3">
                    <Activity className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm font-medium">{p.name}</span>
                  </div>
                  <Badge variant={p.statusColor}>{p.status}</Badge>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Quick Actions */}
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>快捷操作</CardTitle>
              <CardDescription>常用功能快速入口</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              {quickActions.map((action) => (
                <Link
                  key={action.label}
                  to={action.to}
                  className="flex items-center gap-3 rounded-xl border border-border/60 px-4 py-3 text-sm font-medium text-foreground transition-colors hover:bg-muted/60"
                >
                  <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/5">
                    <action.icon className="h-4 w-4 text-primary" />
                  </div>
                  <span className="flex-1">{action.label}</span>
                  <ArrowRight className="h-4 w-4 text-muted-foreground" />
                </Link>
              ))}
            </CardContent>
          </Card>

          {/* Engine Info */}
          <Card size="sm">
            <CardContent className="px-4 py-4">
              <div className="flex items-center gap-3">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/5">
                  <Users className="h-4 w-4 text-primary" />
                </div>
                <div className="text-sm">
                  <div className="font-medium">Nova v0.1.0</div>
                  <div className="text-xs text-muted-foreground">Go 1.21+ · SQLite 存储</div>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
