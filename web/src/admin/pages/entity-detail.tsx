import { useState, useEffect, useCallback } from "react";
import {
  FileText, Clock, CheckCircle2, Circle, Loader2,
  SendHorizonal, Activity, ListTodo, History, MessageSquare,
  X,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/table";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogClose,
} from "@/components/ui/dialog";
import { authHeaders } from "@/lib/auth";

const API_BASE = "http://localhost:8080/api/v1";

interface Task {
  id: string;  // UUID
  act_name: string;
  act_title: string;
  state: number;
  state_text: string;
  handlers: string;
}

interface Todo {
  id: string;  // UUID
  task_id: string;
  entity_id: string;
  act_title: string;
  entity_title: string;
  pro_name: string;
  handler_uid: string;
  handler_name: string;
  accepted: boolean;
}

interface NextAct {
  act_id: string;  // UUID
  act_name: string;
  act_title: string;
}

interface LogEntry {
  id: number;  // rowid
  act_title: string;
  handler_name: string;
  content: string;
  arrive_at: string;
  finish_at: string;
}

interface Opinion {
  id: number;
  act_title: string;
  handler_name: string;
  content: string;
  written_at: string;
}

interface EntityDetail {
  id: string;  // UUID
  code: string;
  pro_name: string;
  title: string;
  state: number;
  state_text: string;
  serial_num: string;
  cur_act: string;
  draft_name: string;
  send_at: string;
  over_at: string;
  created_at: string;
  tasks: Task[];
  todos: Todo[];
  next_acts: NextAct[];
  logs: LogEntry[];
  opinions: Opinion[];
}

interface Props {
  entityId: string | null;
  onClose: () => void;
}

export function EntityDetailModal({ entityId, onClose }: Props) {
  const { t } = useI18n();
  const [entity, setEntity] = useState<EntityDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [selectedActId, setSelectedActId] = useState<string | null>(null);
  const [submitMsg, setSubmitMsg] = useState<string | null>(null);
  const [opinionContent, setOpinionContent] = useState("");

  const fetchDetail = useCallback(async () => {
    if (!entityId) return;
    setLoading(true);
    setEntity(null);
    setSubmitMsg(null);
    setSelectedActId(null);
    setOpinionContent("");
    try {
      const res = await fetch(`${API_BASE}/entities/${entityId}`, { headers: { ...authHeaders() } });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setEntity(data.entity ?? null);
    } catch (err) {
      console.error("Failed to fetch entity detail:", err);
    } finally {
      setLoading(false);
    }
  }, [entityId]);

  useEffect(() => {
    fetchDetail();
  }, [fetchDetail]);

  const handleSubmit = async () => {
    if (!entity) return;
    setSubmitting(true);
    setSubmitMsg(null);
    try {
      const body: Record<string, unknown> = {};
      if (selectedActId) {
        body.next_act_id = selectedActId;
      }
      if (opinionContent.trim()) {
        body.opinion_content = opinionContent.trim();
      }
      const res = await fetch(`${API_BASE}/entities/${entity.id}/submit`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setSubmitMsg(t("entities.detail.submitSuccess"));
      setOpinionContent("");
      fetchDetail();
    } catch (err) {
      console.error("Failed to submit:", err);
      setSubmitMsg("提交失败，请重试");
    } finally {
      setSubmitting(false);
    }
  };

  const stateBadge = (state: number) => {
    if (state === 100) return <Badge variant="secondary">{t("entities.status.completed")}</Badge>;
    if (state === 0) return <Badge variant="outline">{t("entities.status.draft")}</Badge>;
    return <Badge>{t("entities.status.processing")}</Badge>;
  };

  const taskStateIcon = (state: number) => {
    if (state === 100) return <CheckCircle2 className="h-4 w-4 text-green-500" />;
    if (state === 0) return <Circle className="h-4 w-4 text-slate-300" />;
    if (state === 1) return <Loader2 className="h-4 w-4 text-amber-500" />;  // CONVERGING
    if (state === 2) return <Clock className="h-4 w-4 text-orange-500" />;   // WAITING
    if (state === 3) return <Circle className="h-4 w-4 text-blue-400" />;    // READY
    return <Loader2 className="h-4 w-4 animate-spin text-blue-500" />;
  };

  return (
    <Dialog open={!!entityId} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent
        className="!max-w-[90vw] !max-h-[90vh] !w-[90vw] !h-[90vh] flex flex-col gap-0 p-0"
        showCloseButton={false}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b px-6 py-3 shrink-0">
          <DialogTitle className="text-base font-semibold flex items-center gap-2">
            <FileText className="h-4 w-4 text-primary" />
            工单详情
            {entity && stateBadge(entity.state)}
          </DialogTitle>
          <DialogClose render={<Button variant="ghost" size="icon-sm"><X className="h-4 w-4" /></Button>} />
        </div>

        {/* Scrollable body */}
        <ScrollArea className="flex-1">
          <div className="p-6 space-y-5">
            {loading ? (
              <div className="flex items-center justify-center py-24">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : !entity ? (
              <div className="py-16 text-center text-muted-foreground">
                <FileText className="h-12 w-12 mx-auto mb-3 opacity-40" />
                <p>工单不存在或已被删除</p>
              </div>
            ) : (
              <>
                {/* Basic Info */}
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2 text-sm">
                      <FileText className="h-4 w-4 text-primary" />
                      基本信息
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="grid grid-cols-3 gap-4 text-sm">
                      <div>
                        <span className="text-muted-foreground text-xs">标题</span>
                        <p className="font-medium">{entity.title}</p>
                      </div>
                      <div>
                        <span className="text-muted-foreground text-xs">流水号</span>
                        <p className="font-mono text-xs">{entity.serial_num || entity.code}</p>
                      </div>
                      <div>
                        <span className="text-muted-foreground text-xs">流程名称</span>
                        <p>{entity.pro_name}</p>
                      </div>
                      <div>
                        <span className="text-muted-foreground text-xs">当前环节</span>
                        <p>{entity.cur_act || "-"}</p>
                      </div>
                      <div>
                        <span className="text-muted-foreground text-xs">起草人</span>
                        <p>{entity.draft_name}</p>
                      </div>
                      <div>
                        <span className="text-muted-foreground text-xs">创建时间</span>
                        <p>{entity.created_at ? new Date(entity.created_at).toLocaleString("zh-CN") : "-"}</p>
                      </div>
                      {entity.send_at ? (
                        <div>
                          <span className="text-muted-foreground text-xs">提交时间</span>
                          <p>{new Date(entity.send_at).toLocaleString("zh-CN")}</p>
                        </div>
                      ) : null}
                      {entity.over_at ? (
                        <div>
                          <span className="text-muted-foreground text-xs">完结时间</span>
                          <p>{new Date(entity.over_at).toLocaleString("zh-CN")}</p>
                        </div>
                      ) : null}
                    </div>
                  </CardContent>
                </Card>

                {/* Tasks */}
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2 text-sm">
                      <Activity className="h-4 w-4 text-primary" />
                      任务轨迹
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="p-0">
                    <Table>
                      <TableHead>
                        <TableRow>
                          <TableHeaderCell>环节</TableHeaderCell>
                          <TableHeaderCell>状态</TableHeaderCell>
                          <TableHeaderCell>处理人</TableHeaderCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {entity.tasks.map((task) => (
                          <TableRow key={task.id}>
                            <TableCell>
                              <div className="flex items-center gap-2">
                                {taskStateIcon(task.state)}
                                <span>{task.act_title || task.act_name}</span>
                              </div>
                            </TableCell>
                            <TableCell>{stateBadge(task.state)}</TableCell>
                            <TableCell className="text-muted-foreground">{task.handlers || "-"}</TableCell>
                          </TableRow>
                        ))}
                        {entity.tasks.length === 0 && (
                          <TableRow>
                            <TableCell colSpan={3} className="text-center text-muted-foreground py-8">暂无</TableCell>
                          </TableRow>
                        )}
                      </TableBody>
                    </Table>
                  </CardContent>
                </Card>

                {/* Current Todos + Submit */}
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2 text-sm">
                      <ListTodo className="h-4 w-4 text-primary" />
                      当前待办
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    {entity.todos.length === 0 ? (
                      <p className="text-sm text-muted-foreground">暂无待办</p>
                    ) : (
                      <div className="space-y-2">
                        {entity.todos.map((todo) => (
                          <div key={todo.id} className="flex items-center justify-between rounded-lg border p-3">
                            <div>
                              <p className="font-medium text-sm">{todo.act_title}</p>
                              <p className="text-xs text-muted-foreground">
                                处理人: {todo.handler_name} ({todo.handler_uid})
                                {todo.accepted ? " · 已受理" : " · 待受理"}
                              </p>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}

                    {/* Opinion input */}
                    {(entity.state === 0 || entity.state === 1 || entity.state === 50) && (
                      <div className="mt-4">
                        <p className="text-sm font-medium mb-2">处理意见</p>
                        <textarea
                          value={opinionContent}
                          onChange={(e) => setOpinionContent(e.target.value)}
                          placeholder="请输入审批意见..."
                          className="w-full rounded-lg border border-slate-200 bg-white p-3 text-sm min-h-[80px] resize-y focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary"
                        />
                      </div>
                    )}

                    {/* Next steps */}
                    {entity.next_acts && entity.next_acts.length > 0 && (
                      <div className="mt-4 space-y-3">
                        <p className="text-sm font-medium">下一步可选环节:</p>
                        <div className="flex flex-wrap gap-2">
                          {entity.next_acts.map((act) => (
                            <button
                              key={act.act_id}
                              onClick={() => setSelectedActId(act.act_id)}
                              className={`rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors ${
                                selectedActId === act.act_id
                                  ? "border-primary bg-primary/10 text-primary"
                                  : "border-slate-200 text-muted-foreground hover:border-primary/50"
                              }`}
                            >
                              {act.act_title || act.act_name}
                            </button>
                          ))}
                        </div>
                        <div className="flex items-center gap-3">
                          <Button onClick={handleSubmit} disabled={submitting}>
                            {submitting ? (
                              <Loader2 className="h-4 w-4 animate-spin mr-1" />
                            ) : (
                              <SendHorizonal className="h-4 w-4 mr-1" />
                            )}
                            提交流转
                          </Button>
                          {submitMsg && (
                            <span className={`text-sm ${submitMsg.includes("成功") ? "text-green-600" : "text-red-500"}`}>
                              {submitMsg}
                            </span>
                          )}
                        </div>
                      </div>
                    )}

                    {/* Auto-send for no next_acts */}
                    {(!entity.next_acts || entity.next_acts.length === 0) && (entity.state === 0 || entity.state === 1 || entity.state === 50) && (
                      <div className="mt-4">
                        <textarea
                          value={opinionContent}
                          onChange={(e) => setOpinionContent(e.target.value)}
                          placeholder="请输入审批意见..."
                          className="w-full rounded-lg border border-slate-200 bg-white p-3 text-sm min-h-[80px] resize-y focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary"
                        />
                        <div className="mt-3 flex items-center gap-3">
                          <Button onClick={handleSubmit} disabled={submitting}>
                            {submitting ? (
                              <Loader2 className="h-4 w-4 animate-spin mr-1" />
                            ) : (
                              <SendHorizonal className="h-4 w-4 mr-1" />
                            )}
                            提交流转
                          </Button>
                          {submitMsg && (
                            <span className={`ml-3 text-sm ${submitMsg.includes("成功") ? "text-green-600" : "text-red-500"}`}>
                              {submitMsg}
                            </span>
                          )}
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>

                {/* Opinions */}
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2 text-sm">
                      <MessageSquare className="h-4 w-4 text-primary" />
                      处理意见
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    {entity.opinions.length === 0 ? (
                      <p className="text-sm text-muted-foreground">暂无意见</p>
                    ) : (
                      <div className="space-y-3">
                        {entity.opinions.map((op) => (
                          <div key={op.id} className="rounded-lg border bg-muted/20 p-3">
                            <div className="flex items-center justify-between mb-2">
                              <div className="flex items-center gap-2">
                                <span className="text-xs font-medium bg-primary/10 text-primary px-2 py-0.5 rounded">
                                  {op.act_title || "意见"}
                                </span>
                                <span className="text-sm font-medium">{op.handler_name}</span>
                              </div>
                              <span className="text-xs text-muted-foreground">
                                {op.written_at ? new Date(op.written_at).toLocaleString("zh-CN") : ""}
                              </span>
                            </div>
                            <div className="text-sm text-slate-700 leading-relaxed whitespace-pre-wrap">
                              {op.content}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </CardContent>
                </Card>

                {/* Logs Timeline */}
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2 text-sm">
                      <History className="h-4 w-4 text-primary" />
                      处理日志
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    {entity.logs.length === 0 ? (
                      <p className="text-sm text-muted-foreground">暂无日志</p>
                    ) : (
                      <div className="relative">
                        <div className="absolute left-[7px] top-2 bottom-2 w-0.5 bg-slate-200" />
                        <div className="space-y-3">
                          {entity.logs.map((log, idx) => (
                            <div key={log.id || idx} className="relative flex gap-4 pl-6">
                              <div className="absolute left-0 top-1.5 h-3.5 w-3.5 rounded-full border-2 border-primary bg-white" />
                              <div className="flex-1 rounded-lg border bg-muted/30 p-3">
                                <div className="flex items-center justify-between">
                                  <p className="text-sm font-medium">{log.act_title}</p>
                                  <Badge variant="outline" className="text-xs">
                                    <Clock className="h-3 w-3 mr-1" />
                                    {log.finish_at
                                      ? new Date(log.finish_at).toLocaleString("zh-CN")
                                      : log.arrive_at
                                        ? new Date(log.arrive_at).toLocaleString("zh-CN")
                                        : "-"}
                                  </Badge>
                                </div>
                                {log.handler_name && (
                                  <p className="mt-1 text-xs text-muted-foreground">
                                    处理人: {log.handler_name}
                                  </p>
                                )}
                                {log.content && (
                                  <div className="mt-2 rounded bg-white px-2.5 py-1.5 text-sm text-slate-700 border">
                                    {log.content}
                                  </div>
                                )}
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </>
            )}
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  );
}
