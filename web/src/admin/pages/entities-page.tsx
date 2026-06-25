import { useState, useEffect, useCallback } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Plus, FileText, Loader2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogTrigger, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { EntityDetailModal } from "@/admin/pages/entity-detail";
import { authHeaders } from "@/lib/auth";

const API_BASE = "http://localhost:8080/api/v1";

interface EntityItem {
  id: string;  // UUID, portable
  code: string;
  pro_name: string;
  title: string;
  state: number;
  state_text: string;
  serial_num: string;
  cur_act: string;
  draft_name: string;
  created_at: string;
}

interface ProcessOption {
  alias: string;
  name: string;
}

type TabKey = "all" | "active" | "completed";

const TAB_MAP: Record<TabKey, string | undefined> = {
  all: undefined,
  active: "1",
  completed: "100",
};

export function EntitiesPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [entities, setEntities] = useState<EntityItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<TabKey>("all");
  const [createOpen, setCreateOpen] = useState(false);
  const [createTitle, setCreateTitle] = useState("");
  const [createProcess, setCreateProcess] = useState("OP");
  const [processes, setProcesses] = useState<ProcessOption[]>([]);
  const [creating, setCreating] = useState(false);
  const [loadingProcesses, setLoadingProcesses] = useState(false);
  const [selectedEntityId, setSelectedEntityId] = useState<string | null>(null);

  const fetchEntities = useCallback(async (state?: string) => {
    setLoading(true);
    try {
      const params = state ? `?state=${state}` : "";
      const res = await fetch(`${API_BASE}/entities${params}`, { headers: { ...authHeaders() } });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setEntities(data.entities ?? []);
    } catch (err) {
      console.error("Failed to fetch entities:", err);
      setEntities([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchEntities(TAB_MAP[tab]);
  }, [tab, fetchEntities]);

  const handleCreate = async () => {
    if (!createTitle.trim()) return;
    setCreating(true);
    try {
      const res = await fetch(`${API_BASE}/entities`, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ pro_alias: createProcess, title: createTitle.trim() }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setCreateOpen(false);
      setCreateTitle("");
      setCreateProcess("OP");
      fetchEntities(TAB_MAP[tab]);
    } catch (err) {
      console.error("Failed to create entity:", err);
    } finally {
      setCreating(false);
    }
  };

  const fetchProcesses = useCallback(async () => {
    setLoadingProcesses(true);
    try {
      const res = await fetch(`${API_BASE}/processes`, { headers: { ...authHeaders() } });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const list: ProcessOption[] = (data.processes ?? []).map((p: { alias: string; name: string }) => ({
        alias: p.alias,
        name: p.name,
      }));
      setProcesses(list);
      if (list.length > 0 && !list.find((p) => p.alias === createProcess)) {
        setCreateProcess(list[0].alias);
      }
    } catch (err) {
      console.error("Failed to fetch processes:", err);
    } finally {
      setLoadingProcesses(false);
    }
  }, [createProcess]);

  const stateBadge = (state: number, stateText: string) => {
    if (state === 100) return <Badge variant="secondary">{t("entities.status.completed")}</Badge>;
    if (state === 0) return <Badge variant="outline">{t("entities.status.draft")}</Badge>;
    return <Badge>{t("entities.status.processing")}</Badge>;
  };

  const tabs: { key: TabKey; label: string }[] = [
    { key: "all", label: t("entities.all") },
    { key: "active", label: t("entities.active") },
    { key: "completed", label: t("entities.completed") },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">{t("entities.page.title")}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{t("entities.page.description")}</p>
        </div>
        <Dialog open={createOpen} onOpenChange={(open) => {
          setCreateOpen(open);
          if (open) fetchProcesses();
        }}>
          <DialogTrigger render={<Button><Plus className="h-4 w-4" />{t("entities.create.title")}</Button>} />
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("entities.create.title")}</DialogTitle>
              <DialogDescription>{t("entities.create.placeholder")}</DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <div>
                <p className="text-sm font-medium mb-1">流程类型</p>
                <Select value={createProcess} onValueChange={setCreateProcess} disabled={loadingProcesses}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择流程" />
                  </SelectTrigger>
                  <SelectContent>
                    {loadingProcesses ? (
                      <SelectItem value="__loading" disabled>加载中...</SelectItem>
                    ) : processes.length === 0 ? (
                      <SelectItem value="OP">OP申请单</SelectItem>
                    ) : (
                      processes.map((p) => (
                        <SelectItem key={p.alias} value={p.alias}>{p.name} ({p.alias})</SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <p className="text-sm font-medium mb-1">工单标题</p>
                <Input
                  placeholder={t("entities.create.placeholder")}
                  value={createTitle}
                  onChange={(e) => setCreateTitle(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") handleCreate(); }}
                  autoFocus
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setCreateOpen(false)}>{t("common.cancel")}</Button>
              <Button onClick={handleCreate} disabled={creating || !createTitle.trim()}>
                {creating ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                {t("common.confirm")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 rounded-xl bg-muted/50 p-1 w-fit">
        {tabs.map((tabItem) => (
          <button
            key={tabItem.key}
            onClick={() => setTab(tabItem.key)}
            className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
              tab === tabItem.key
                ? "bg-white text-foreground shadow-xs"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            {tabItem.label}
          </button>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("entities.page.title")}</CardTitle>
          <CardDescription>
            {loading ? t("common.loading") : `${entities.length} 条工单`}
          </CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {loading ? (
            <div className="flex items-center justify-center py-16">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : entities.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
              <FileText className="h-10 w-10 mb-3 opacity-40" />
              <p className="text-sm">{t("common.none")}</p>
            </div>
          ) : (
            <Table>
              <TableHead>
                <TableRow>
                  <TableHeaderCell>{t("entities.table.name")}</TableHeaderCell>
                  <TableHeaderCell>{t("entities.table.serialNum")}</TableHeaderCell>
                  <TableHeaderCell>{t("entities.table.currentAct")}</TableHeaderCell>
                  <TableHeaderCell>{t("entities.table.status")}</TableHeaderCell>
                  <TableHeaderCell>{t("entities.table.draftName")}</TableHeaderCell>
                  <TableHeaderCell>{t("entities.table.createdAt")}</TableHeaderCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {entities.map((e) => (
                  <TableRow
                    key={e.id}
                    className="cursor-pointer"
                    onClick={() => setSelectedEntityId(e.id)}
                  >
                    <TableCell className="font-medium">{e.title}</TableCell>
                    <TableCell className="text-muted-foreground font-mono text-xs">{e.serial_num || e.code}</TableCell>
                    <TableCell>{e.cur_act || "-"}</TableCell>
                    <TableCell>{stateBadge(e.state, e.state_text)}</TableCell>
                    <TableCell className="text-muted-foreground">{e.draft_name}</TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {e.created_at ? new Date(e.created_at).toLocaleString("zh-CN") : "-"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Entity Detail Modal */}
      <EntityDetailModal
        entityId={selectedEntityId}
        onClose={() => { setSelectedEntityId(null); fetchEntities(TAB_MAP[tab]); }}
      />
    </div>
  );
}
