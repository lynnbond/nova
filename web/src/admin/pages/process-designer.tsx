import { useState, useEffect, useCallback, type ComponentProps } from "react";
import { useParams, useNavigate } from "@tanstack/react-router";
import {
  ArrowLeft, CirclePlay, CircleStop, FileEdit, GitFork, GitMerge,
  Hourglass, Plus, X, Save, Send, Loader2,
} from "lucide-react";
import {
  ReactFlow, Background, Controls, MiniMap, Handle, Position,
  useNodesState, useEdgesState, addEdge,
  type Node, type Edge, type Connection, type NodeProps,
  MarkerType,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { authHeaders } from "@/lib/auth";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080/api/v1";

/* ─── Types ─────────────────────────────────────────── */
type ActTypeLabel = "START" | "MANUAL" | "ROUTER" | "CONVERGE" | "WAIT" | "END";

interface FlowNodeData {
  label: string;
  actName: string;
  type: ActTypeLabel;
  handler?: string;
  policy?: "single" | "exclusive" | "concurrent";
  groups?: string;
  requireOpinion?: boolean;
}

interface ActDef {
  name: string;
  title: string;
  type: number;
  policy?: number;
  handler?: string;
  groups?: string;
}

interface LinkDef {
  from: string;
  to: string;
}

const nodeTypeMeta: Record<ActTypeLabel, { label: string; color: string; bg: string; border: string; icon: typeof CirclePlay }> = {
  START:    { label: "开始", color: "text-emerald-600", bg: "bg-emerald-50", border: "border-emerald-300", icon: CirclePlay },
  MANUAL:   { label: "人工", color: "text-blue-600", bg: "bg-blue-50", border: "border-blue-300", icon: FileEdit },
  ROUTER:   { label: "并发", color: "text-violet-600", bg: "bg-violet-50", border: "border-violet-300", icon: GitFork },
  CONVERGE: { label: "汇聚", color: "text-orange-600", bg: "bg-orange-50", border: "border-orange-300", icon: GitMerge },
  WAIT:     { label: "等待", color: "text-amber-600", bg: "bg-amber-50", border: "border-amber-300", icon: Hourglass },
  END:      { label: "结束", color: "text-rose-600", bg: "bg-rose-50", border: "border-rose-300", icon: CircleStop },
};

const ACT_TYPE_MAP: Record<string, number> = {
  START: 0, MANUAL: 1, ROUTER: 3, CONVERGE: 4, WAIT: 5, TAIL: 6, END: 100,
};
const POLICY_MAP: Record<string, number | undefined> = {
  single: 10, exclusive: 21, concurrent: 22,
};
const REV_POLICY: Record<number, string> = { 10: "single", 21: "exclusive", 22: "concurrent" };

/* ─── Custom Node ──────────────────────────────────── */
function FlowNode({ data, selected }: NodeProps<FlowNodeData>) {
  const meta = nodeTypeMeta[data.type];
  const Icon = meta.icon;
  const isTerminal = data.type === "START" || data.type === "END";

  if (isTerminal) {
    return (
      <div className="relative flex flex-col items-center">
        {data.type !== "START" && (
          <Handle type="target" position={Position.Top} className="!h-1.5 !w-1.5 !border !border-slate-300 !bg-white" />
        )}
        {data.type !== "END" && (
          <Handle type="source" position={Position.Bottom} className="!h-1.5 !w-1.5 !border !border-slate-300 !bg-white" />
        )}
        <div className={cn(
          "flex h-6 w-6 items-center justify-center rounded-full ring-1 transition-all",
          meta.bg, meta.border.replace("border-", "ring-"),
          selected && "ring-2 ring-primary/50 scale-110",
        )}>
          <Icon className={cn("h-3.5 w-3.5", meta.color)} />
        </div>
        <span className="mt-1 text-[8px] font-semibold text-muted-foreground">{meta.label}</span>
      </div>
    );
  }
  return (
    <div
      className={cn(
        "relative rounded-md border px-1.5 py-1 text-center shadow-xs transition-all",
        meta.bg, meta.border,
        selected && "ring-1 ring-primary/30 shadow-md scale-105",
      )}
      style={{ minWidth: 38 }}
    >
      {data.type !== "START" && (
        <Handle type="target" position={Position.Top} className="!h-1.5 !w-1.5 !border !border-slate-300 !bg-white" />
      )}
      {data.type !== "END" && (
        <Handle type="source" position={Position.Bottom} className="!h-1.5 !w-1.5 !border !border-slate-300 !bg-white" />
      )}
      <div className={cn("mx-auto flex h-4 w-4 items-center justify-center rounded-md", meta.bg, "ring-1", meta.border.replace("border-", "ring-"))}>
        <Icon className={cn("h-2.5 w-2.5", meta.color)} />
      </div>
      <div className="mt-0.5 text-[9px] font-semibold leading-tight">{data.label}</div>
      {data.handler && (
        <div className="mt-[1px] text-[7px] leading-tight text-muted-foreground">{data.handler}</div>
      )}
      <div className={cn("mx-auto mt-[1px] inline-block rounded-full px-1.5 py-[1px] text-[7px] font-medium leading-tight", meta.bg, meta.color)}>
        {meta.label}
      </div>
    </div>
  );
}

const nodeTypes = { flowNode: FlowNode };

/* ─── Helpers ──────────────────────────────────────── */
let seed = 0;
function nid() { return `n_${++seed}`; }
function eid() { return `e_${++seed}`; }

function makeNodesAndEdges(acts: ActDef[], links: LinkDef[]): [Node<FlowNodeData>[], Edge[]] {
  seed = 0;
  const actNameToType: Record<string, ActTypeLabel> = {
    "start": "START", "end": "END",
  };
  const nodes: Node<FlowNodeData>[] = [];
  const nodeMap: Record<string, string> = {}; // actName → nodeId
  const typeOrder: ActTypeLabel[] = ["START", "MANUAL", "ROUTER", "CONVERGE", "WAIT", "TAIL", "END"];

  // Convert type number to label
  const typeNumToLabel: Record<number, ActTypeLabel> = {
    0: "START", 1: "MANUAL", 3: "ROUTER", 4: "CONVERGE", 5: "WAIT", 6: "TAIL", 100: "END",
  };

  acts.forEach((a, i) => {
    const type = actNameToType[a.name] || typeNumToLabel[a.type] || "MANUAL";
    actNameToType[a.name] = type;
    const nodeId = nid();
    nodeMap[a.name] = nodeId;
    nodes.push({
      id: nodeId,
      type: "flowNode",
      position: { x: (i * 140) % 420, y: Math.floor(i / 3) * 100 + 40 },
      data: {
        label: a.title,
        actName: a.name,
        type,
        handler: a.handler || "",
        policy: a.policy ? (REV_POLICY[a.policy] as any) || "single" : "single",
        groups: a.groups || "",
      },
    });
  });

  const edges: Edge[] = links.map((l) => {
    const src = nodeMap[l.from];
    const dst = nodeMap[l.to];
    if (!src || !dst) return null as any;
    return {
      id: eid(),
      source: src,
      target: dst,
      type: "smoothstep",
      animated: true,
      markerEnd: { type: MarkerType.ArrowClosed, color: "#94a3b8" },
      style: { stroke: "#94a3b8", strokeWidth: 2 },
    };
  }).filter(Boolean);

  return [nodes, edges];
}

/* ─── Page Component ──────────────────────────────── */
export function ProcessDesignerPage() {
  const { alias } = useParams({ strict: false });
  const navigate = useNavigate();
  const [processName, setProcessName] = useState("新流程");
  const [version, setVersion] = useState(1);
  const [publishState, setPublishState] = useState<"draft" | "published">("draft");
  const [proAlias, setProAlias] = useState(alias || "new");
  const [loading, setLoading] = useState(alias !== "new");
  const [saving, setSaving] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [msg, setMsg] = useState("");

  const [nodes, setNodes, onNodesChange] = useNodesState<FlowNodeData>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [selectedNode, setSelectedNode] = useState<Node<FlowNodeData> | null>(null);

  // Load existing process design from API
  useEffect(() => {
    if (!alias || alias === "new") {
      setLoading(false);
      return;
    }
    setLoading(true);
    setProAlias(alias);
    fetch(`${API_BASE}/processes/${alias}/design`, { headers: { ...authHeaders() } })
      .then((r) => { if (!r.ok) throw new Error("HTTP " + r.status); return r.json(); })
      .then((data) => {
        const p = data.process;
        setProcessName(p.name);
        setVersion(p.ver);
        setPublishState("draft");
        if (data.acts && data.acts.length > 0) {
          const [ns, es] = makeNodesAndEdges(data.acts, data.links || []);
          setNodes(ns);
          setEdges(es);
        }
      })
      .catch((err) => {
        console.error("Failed to load process design:", err);
        setMsg("加载失败: " + err.message);
      })
      .finally(() => setLoading(false));
  }, [alias, setNodes, setEdges]);

  const onConnect = useCallback(
    (conn: Connection) => setEdges((eds) => addEdge({
      ...conn, type: "smoothstep", animated: true,
      markerEnd: { type: MarkerType.ArrowClosed, color: "#94a3b8" },
      style: { stroke: "#94a3b8", strokeWidth: 2 },
    }, eds)),
    [setEdges],
  );

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedNode(node as Node<FlowNodeData>);
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedNode(null);
  }, []);

  function addNode(type: ActTypeLabel) {
    const count = nodes.filter((n) => n.data.type === type).length + 1;
    const newId = nid();
    const maxY = Math.max(...nodes.map((n) => n.position.y), 0);
    const newNode: Node<FlowNodeData> = {
      id: newId, type: "flowNode",
      position: { x: 0, y: maxY + 40 },
      data: { label: `${nodeTypeMeta[type].label}${count}`, actName: `${type.toLowerCase()}${count}`, type, policy: "single" },
    };
    setNodes((nds) => [...nds, newNode]);
    const lastNode = nodes[nodes.length - 1];
    if (lastNode) {
      setEdges((eds) => [...eds, {
        id: eid(), source: lastNode.id, target: newId,
        type: "smoothstep", animated: true,
        markerEnd: { type: MarkerType.ArrowClosed, color: "#94a3b8" },
        style: { stroke: "#94a3b8", strokeWidth: 2 },
      }]);
    }
    setSelectedNode(newNode);
  }

  function removeSelectedNode() {
    if (!selectedNode) return;
    const data = selectedNode.data;
    if (data.type === "START" || data.type === "END") return;
    setNodes((nds) => nds.filter((n) => n.id !== selectedNode.id));
    setEdges((eds) => eds.filter((e) => e.source !== selectedNode.id && e.target !== selectedNode.id));
    setSelectedNode(null);
  }

  function updateNodeData(updates: Partial<FlowNodeData>) {
    if (!selectedNode) return;
    setNodes((nds) =>
      nds.map((n) => (n.id === selectedNode.id ? { ...n, data: { ...n.data, ...updates } } : n))
    );
    setSelectedNode((prev) => prev ? { ...prev, data: { ...prev.data, ...updates } } : null);
  }

  // Serialize canvas to API format
  function serializeDesign(): { acts: ActDef[]; links: LinkDef[] } {
    // Build nodeId → actName & act data
    const nodeMap: Record<string, { actName: string; data: FlowNodeData }> = {};
    nodes.forEach((n) => {
      nodeMap[n.id] = { actName: n.data.actName, data: n.data };
    });
    const acts: ActDef[] = nodes.map((n) => {
      const d = n.data;
      const typeNum = ACT_TYPE_MAP[d.type] ?? 1;
      return {
        name: d.actName,
        title: d.label,
        type: typeNum,
        policy: POLICY_MAP[d.policy || "single"] || 10,
        handler: d.handler || "",
        groups: d.groups || "",
      };
    });
    const links: LinkDef[] = edges
      .map((e) => {
        const from = nodeMap[e.source]?.actName;
        const to = nodeMap[e.target]?.actName;
        return from && to ? { from, to } : null;
      })
      .filter(Boolean) as LinkDef[];
    return { acts, links };
  }

  async function handleSave() {
    setSaving(true);
    setMsg("");
    try {
      const design = serializeDesign();
      const body = JSON.stringify({ name: processName, ...design });
      const targetAlias = proAlias === "new" ? "" : proAlias;
      const url = targetAlias
        ? `${API_BASE}/processes/${targetAlias}/design`
        : `${API_BASE}/processes/design`;
      const res = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body,
      });
      if (!res.ok) {
        const err = await res.text();
        throw new Error(err);
      }
      const data = await res.json();
      if (data.process) {
        setProAlias(data.process.alias);
        setVersion(data.ver || data.process.ver);
      }
      setMsg("✅ 保存成功");
    } catch (err: any) {
      setMsg("❌ 保存失败: " + (err.message || ""));
      console.error("Save failed:", err);
    } finally {
      setSaving(false);
    }
  }

  async function handlePublish() {
    setPublishing(true);
    setMsg("");
    try {
      const res = await fetch(`${API_BASE}/processes/${proAlias}/publish`, {
        method: "POST",
        headers: { ...authHeaders() },
      });
      if (!res.ok) throw new Error("HTTP " + res.status);
      setPublishState("published");
      setMsg("✅ 发布成功");
    } catch (err: any) {
      setMsg("❌ 发布失败: " + (err.message || ""));
    } finally {
      setPublishing(false);
    }
  }

  const addableTypes: ActTypeLabel[] = ["MANUAL", "ROUTER", "CONVERGE", "WAIT"];

  if (loading) {
    return (
      <div className="flex h-[calc(100vh-5rem)] items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="flex h-[calc(100vh-5rem)] gap-0 overflow-hidden">
      {/* Main: Toolbar + Canvas */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Toolbar */}
        <div className="flex items-center gap-3 border-b border-border/60 bg-white px-5 py-3">
          <button type="button" onClick={() => navigate({ to: "/processes" })} className="text-muted-foreground hover:text-foreground">
            <ArrowLeft className="h-4 w-4" />
          </button>

          <Input
            value={processName}
            onChange={(e) => setProcessName(e.target.value)}
            className="h-8 w-56 border-0 bg-transparent px-0 text-base font-semibold shadow-none focus-visible:ring-0"
          />

          {/* Version */}
          <Badge variant="outline" className="gap-1 text-xs">
            v{version}
            <span className={cn("h-1.5 w-1.5 rounded-full", publishState === "published" ? "bg-green-500" : "bg-amber-500")} />
            {publishState === "published" ? "已发布" : "草稿"}
          </Badge>

          <Badge variant="secondary" className="font-mono text-xs">{proAlias}</Badge>

          {/* Status message */}
          {msg && (
            <span className={cn("text-xs", msg.includes("✅") ? "text-green-600" : "text-red-500")}>{msg}</span>
          )}

          <div className="ml-auto flex gap-1.5">
            {addableTypes.map((t) => {
              const meta = nodeTypeMeta[t];
              return (
                <button key={t} type="button" onClick={() => addNode(t)}
                  className={cn("flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors", meta.bg, meta.color, "hover:opacity-80")}
                >
                  <Plus className="h-3 w-3" />{meta.label}
                </button>
              );
            })}
          </div>

          <div className="flex gap-1.5 border-l border-border/60 pl-3">
            <Button size="xs" variant="outline" onClick={handleSave} disabled={saving}>
              {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
              保存
            </Button>
            <Button size="xs" onClick={handlePublish} disabled={publishing || publishState === "published"}>
              {publishing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Send className="h-3.5 w-3.5" />}
              发布
            </Button>
          </div>
        </div>

        {/* React Flow Canvas */}
        <div className="flex-1">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={onNodeClick}
            onPaneClick={onPaneClick}
            nodeTypes={nodeTypes}
            fitView
            minZoom={0.3}
            maxZoom={2}
            defaultEdgeOptions={{
              type: "smoothstep",
              animated: true,
              markerEnd: { type: MarkerType.ArrowClosed, color: "#94a3b8" },
              style: { stroke: "#94a3b8", strokeWidth: 2 },
            }}
          >
            <Background color="#e2e8f0" gap={20} size={1} />
            <Controls showInteractive={false} className="!rounded-lg !border !border-border/60 !shadow-sm" />
            <MiniMap
              nodeColor={(n) => nodeTypeMeta[(n.data as FlowNodeData).type]?.border?.replace("border-", "#") || "#e2e8f0"}
              maskColor="rgba(0,0,0,0.06)"
              className="!rounded-xl !border !border-border/60 !shadow-sm"
            />
          </ReactFlow>
        </div>
      </div>

      {/* Right: Property Panel */}
      <div className="w-80 shrink-0 border-l border-border/60 bg-white overflow-y-auto">
        {selectedNode ? (
          <div className="p-5 space-y-5">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold">节点属性</h3>
              <div className="flex gap-1">
                {selectedNode.data.type !== "START" && selectedNode.data.type !== "END" && (
                  <button type="button" onClick={removeSelectedNode} className="text-muted-foreground hover:text-destructive">
                    <X className="h-4 w-4" />
                  </button>
                )}
              </div>
            </div>

            {/* Type Badge */}
            <div className={cn("flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium", nodeTypeMeta[selectedNode.data.type].bg, nodeTypeMeta[selectedNode.data.type].color)}>
              {(() => { const I = nodeTypeMeta[selectedNode.data.type].icon; return <I className="h-4 w-4" />; })()}
              {nodeTypeMeta[selectedNode.data.type].label}
            </div>

            {/* Name */}
            <Field label="标识">
              <Input value={selectedNode.data.actName} onChange={(e) => updateNodeData({ actName: e.target.value })} className="h-8 text-xs" />
            </Field>

            {/* Title */}
            <Field label="标题">
              <Input value={selectedNode.data.label} onChange={(e) => updateNodeData({ label: e.target.value })} className="h-8 text-xs" />
            </Field>

            {/* Handler (MANUAL only) */}
            {selectedNode.data.type === "MANUAL" && (
              <>
                <Field label="处理人">
                  <Input value={selectedNode.data.handler || ""} onChange={(e) => updateNodeData({ handler: e.target.value })} placeholder="角色/部门/用户" className="h-8 text-xs" />
                </Field>

                <Field label="处理策略">
                  <select
                    value={selectedNode.data.policy || "single"}
                    onChange={(e) => updateNodeData({ policy: e.target.value as any })}
                    className="flex h-8 w-full items-center rounded-lg border border-border/60 bg-background px-2.5 text-xs shadow-xs outline-none focus:border-primary/40 focus:ring-3 focus:ring-primary/10"
                  >
                    <option value="single">单人处理</option>
                    <option value="exclusive">多人互斥</option>
                    <option value="concurrent">多人并行</option>
                  </select>
                </Field>

                <Field label="候选组">
                  <Input value={selectedNode.data.groups || ""} onChange={(e) => updateNodeData({ groups: e.target.value })} placeholder="role:op,dept:ops" className="h-8 text-xs font-mono" />
                  <p className="text-[10px] text-muted-foreground mt-1">格式：type:id，逗号分隔</p>
                </Field>

                <label className="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer">
                  <input type="checkbox" checked={selectedNode.data.requireOpinion || false} onChange={(e) => updateNodeData({ requireOpinion: e.target.checked })} className="rounded border-border/60" />
                  需要填写处理意见
                </label>
              </>
            )}

            {/* ROUTER properties */}
            {selectedNode.data.type === "ROUTER" && (
              <Field label="分支数量">
                <p className="text-xs text-muted-foreground">通过连线自由创建分支</p>
              </Field>
            )}

            {/* CONVERGE / WAIT properties */}
            {selectedNode.data.type === "CONVERGE" && (
              <Field label="汇聚来源">
                <p className="text-xs text-muted-foreground">连接到需要汇聚的节点</p>
              </Field>
            )}

            <div className="pt-2">
              <Button size="sm" className="w-full text-xs">应用</Button>
            </div>
          </div>
        ) : (
          <div className="flex h-full items-center justify-center text-center text-xs text-muted-foreground px-6">
            点击画布中的节点编辑属性<br />
            拖拽节点下方的圆点可创建连线
          </div>
        )}
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <label className="text-xs font-medium text-muted-foreground">{label}</label>
      {children}
    </div>
  );
}
