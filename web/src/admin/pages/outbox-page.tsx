import { useState, useEffect, useCallback } from "react";
import { Inbox, Loader2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { authHeaders } from "@/lib/auth";

const API_BASE = "http://localhost:8080/api/v1";

interface OutboxItem {
  id: string;  // UUID
  entity_id: string;
  task_id: string;
  entity_title: string;
  act_title: string;
  pro_name: string;
  handler_uid: string;
  handler_name: string;
  finish_at: string;
}

export function OutboxPage() {
  const { t } = useI18n();
  const [items, setItems] = useState<OutboxItem[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchOutbox = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/outbox?handler=user_001`, { headers: { ...authHeaders() } });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setOutbox(data.outbox ?? []);
    } catch (err) {
      console.error("Failed to fetch outbox:", err);
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchOutbox();
  }, [fetchOutbox]);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight">{t("outbox.page.title")}</h2>
        <p className="mt-1 text-sm text-muted-foreground">{t("outbox.page.description")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("outbox.page.title")}</CardTitle>
          <CardDescription>
            {loading ? t("common.loading") : `${items.length} 条已办记录`}
          </CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {loading ? (
            <div className="flex items-center justify-center py-16">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : items.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
              <Inbox className="h-10 w-10 mb-3 opacity-40" />
              <p className="text-sm">暂无已办记录</p>
            </div>
          ) : (
            <Table>
              <TableHead>
                <TableRow>
                  <TableHeaderCell>{t("outbox.table.entityTitle")}</TableHeaderCell>
                  <TableHeaderCell>{t("outbox.table.currentAct")}</TableHeaderCell>
                  <TableHeaderCell>{t("todos.table.process")}</TableHeaderCell>
                  <TableHeaderCell>{t("outbox.table.finishedAt")}</TableHeaderCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.entity_title}</TableCell>
                    <TableCell>{item.act_title}</TableCell>
                    <TableCell className="text-muted-foreground">{item.pro_name}</TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {item.finish_at ? new Date(item.finish_at).toLocaleString("zh-CN") : "-"}
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
