import { useState, useEffect, useCallback } from "react";
import { CheckCircle, ListTodo, Loader2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { authHeaders, useAuth } from "@/lib/auth";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080/api/v1";

interface TodoItem {
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

export function TodosPage() {
  const { t } = useI18n();
  const { user } = useAuth();
  const [todos, setTodos] = useState<TodoItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [accepting, setAccepting] = useState<string | null>(null);
  const [processing, setProcessing] = useState<string | null>(null);

  const fetchTodos = useCallback(async () => {
    setLoading(true);
    try {
      const handler = user?.uid || "user_001";
      const res = await fetch(`${API_BASE}/todos?handler=${encodeURIComponent(handler)}`, { headers: { ...authHeaders() } });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setTodos(data.todos ?? []);
    } catch (err) {
      console.error("Failed to fetch todos:", err);
      setTodos([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTodos();
  }, [fetchTodos]);

  const handleAccept = async (todoId: number) => {
    setAccepting(todoId);
    try {
      const res = await fetch(`${API_BASE}/todos/${todoId}/accept`, { method: "POST", headers: { ...authHeaders() } });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      fetchTodos();
    } catch (err) {
      console.error("Failed to accept todo:", err);
    } finally {
      setAccepting(null);
    }
  };

  const handleProcess = async (todo: TodoItem) => {
    setProcessing(todo.id);
    try {
      const res = await fetch(`${API_BASE}/entities/${todo.entity_id}/submit`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({}),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      fetchTodos();
    } catch (err) {
      console.error("Failed to process todo:", err);
    } finally {
      setProcessing(null);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight">{t("todos.page.title")}</h2>
        <p className="mt-1 text-sm text-muted-foreground">{t("todos.page.description")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("todos.page.title")}</CardTitle>
          <CardDescription>
            {loading ? t("common.loading") : `${todos.length} 条待办`}
          </CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {loading ? (
            <div className="flex items-center justify-center py-16">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : todos.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
              <CheckCircle className="h-10 w-10 mb-3 opacity-40" />
              <p className="text-sm">暂无待办事项</p>
            </div>
          ) : (
            <Table>
              <TableHead>
                <TableRow>
                  <TableHeaderCell>{t("todos.table.entityTitle")}</TableHeaderCell>
                  <TableHeaderCell>{t("todos.table.actName")}</TableHeaderCell>
                  <TableHeaderCell>{t("todos.table.process")}</TableHeaderCell>
                  <TableHeaderCell>{t("todos.table.actions")}</TableHeaderCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {todos.map((todo) => (
                  <TableRow key={todo.id}>
                    <TableCell className="font-medium">{todo.entity_title}</TableCell>
                    <TableCell>{todo.act_title}</TableCell>
                    <TableCell className="text-muted-foreground">{todo.pro_name}</TableCell>
                    <TableCell>
                      <div className="flex gap-2">
                        {!todo.accepted ? (
                          <Button
                            size="sm"
                            onClick={() => handleAccept(todo.id)}
                            disabled={accepting === todo.id}
                          >
                            {accepting === todo.id ? (
                              <Loader2 className="h-3 w-3 animate-spin" />
                            ) : null}
                            {t("todos.accept")}
                          </Button>
                        ) : (
                          <Button
                            size="sm"
                            onClick={() => handleProcess(todo)}
                            disabled={processing === todo.id}
                          >
                            {processing === todo.id ? (
                              <Loader2 className="h-3 w-3 animate-spin" />
                            ) : null}
                            {t("todos.process")}
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
