import { useState, useCallback } from "react";
import { Plus, GripVertical, X, Settings2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export interface FieldDef {
  id: string;
  label: string;
  type: "text" | "textarea" | "number" | "select" | "checkbox" | "date" | "ai_text";
  required: boolean;
  default?: string;
  options?: string[];
  placeholder?: string;
  ai_hint?: string;
  order: number;
}

interface Props {
  fields: FieldDef[];
  onChange: (fields: FieldDef[]) => void;
}

let fieldSeed = 0;
function newFieldId() { return `f_${++fieldSeed}`; }

const FIELD_META: Record<string, { label: string; color: string; bg: string }> = {
  text:     { label: "文本",     color: "text-blue-600",     bg: "bg-blue-50" },
  textarea: { label: "多行文本", color: "text-indigo-600",   bg: "bg-indigo-50" },
  number:   { label: "数字",     color: "text-emerald-600",  bg: "bg-emerald-50" },
  select:   { label: "下拉选择", color: "text-violet-600",   bg: "bg-violet-50" },
  checkbox: { label: "多选",     color: "text-orange-600",   bg: "bg-orange-50" },
  date:     { label: "日期",     color: "text-cyan-600",     bg: "bg-cyan-50" },
  ai_text:  { label: "AI 填充",  color: "text-pink-600",    bg: "bg-pink-50" },
};

const FIELD_TYPES = Object.keys(FIELD_META);

export function FormEditor({ fields, onChange }: Props) {
  const [selectedIdx, setSelectedIdx] = useState<number | null>(null);

  const addField = useCallback((type: string) => {
    const newField: FieldDef = {
      id: newFieldId(),
      label: FIELD_META[type]?.label || "新字段",
      type: type as FieldDef["type"],
      required: false,
      order: fields.length,
    };
    onChange([...fields, newField]);
    setSelectedIdx(fields.length);
  }, [fields, onChange]);

  const updateField = useCallback((idx: number, updates: Partial<FieldDef>) => {
    const next = [...fields];
    next[idx] = { ...next[idx], ...updates };
    onChange(next);
  }, [fields, onChange]);

  const removeField = useCallback((idx: number) => {
    const next = fields.filter((_, i) => i !== idx).map((f, i) => ({ ...f, order: i }));
    onChange(next);
    setSelectedIdx(null);
  }, [fields, onChange]);

  const moveField = useCallback((idx: number, dir: -1 | 1) => {
    const to = idx + dir;
    if (to < 0 || to >= fields.length) return;
    const next = [...fields];
    [next[idx], next[to]] = [next[to], next[idx]];
    onChange(next.map((f, i) => ({ ...f, order: i })));
    setSelectedIdx(to);
  }, [fields, onChange]);

  const selected = selectedIdx !== null ? fields[selectedIdx] : null;

  return (
    <div className="flex h-full gap-0">
      {/* Left: field palette + list */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Add field toolbar */}
        <div className="flex flex-wrap gap-1.5 border-b border-border/60 px-3 py-2">
          {FIELD_TYPES.map((type) => {
            const meta = FIELD_META[type];
            return (
              <button
                key={type}
                type="button"
                onClick={() => addField(type)}
                className={cn("flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-medium transition-colors", meta.bg, meta.color, "hover:opacity-80")}
              >
                <Plus className="h-3 w-3" />{meta.label}
              </button>
            );
          })}
        </div>

        {/* Field list */}
        <div className="flex-1 overflow-y-auto p-3 space-y-1.5">
          {fields.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Settings2 className="h-8 w-8 mb-2 opacity-30" />
              <p className="text-xs">点击上方按钮添加表单字段</p>
            </div>
          ) : fields.map((field, idx) => {
            const meta = FIELD_META[field.type] || FIELD_META.text;
            return (
              <div
                key={field.id}
                className={cn(
                  "flex items-center gap-2 rounded-lg border px-3 py-2 cursor-pointer transition-colors",
                  selectedIdx === idx
                    ? "border-primary bg-primary/5"
                    : "border-border/60 hover:border-primary/30"
                )}
                onClick={() => setSelectedIdx(idx)}
              >
                <div className="flex flex-col gap-0.5 opacity-30">
                  <button type="button" onClick={(e) => { e.stopPropagation(); moveField(idx, -1); }} className="h-2 w-3 hover:text-foreground">▲</button>
                  <button type="button" onClick={(e) => { e.stopPropagation(); moveField(idx, 1); }} className="h-2 w-3 hover:text-foreground">▼</button>
                </div>
                <div className={cn("flex h-6 w-6 items-center justify-center rounded text-[10px] font-bold", meta.bg, meta.color)}>
                  {meta.label.charAt(0)}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-xs font-medium truncate">{field.label}</p>
                  <p className="text-[10px] text-muted-foreground">{meta.label}{field.required ? " · 必填" : ""}</p>
                </div>
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); removeField(idx); }}
                  className="text-muted-foreground hover:text-destructive opacity-0 group-hover:opacity-100"
                >
                  <X className="h-3 w-3" />
                </button>
              </div>
            );
          })}
        </div>
      </div>

      {/* Right: property panel */}
      {selected && (
        <div className="w-64 shrink-0 border-l border-border/60 bg-white overflow-y-auto p-4 space-y-4">
          <div className="flex items-center justify-between">
            <h4 className="text-xs font-semibold">字段属性</h4>
            <button type="button" onClick={() => setSelectedIdx(null)} className="text-muted-foreground hover:text-foreground">
              <X className="h-3.5 w-3.5" />
            </button>
          </div>

          <Field label="字段标识">
            <Input
              value={selected.id}
              onChange={(e) => updateField(selectedIdx!, { id: e.target.value })}
              className="h-7 text-[11px] font-mono"
            />
          </Field>

          <Field label="标签">
            <Input
              value={selected.label}
              onChange={(e) => updateField(selectedIdx!, { label: e.target.value })}
              className="h-7 text-xs"
            />
          </Field>

          <Field label="字段类型">
            <select
              value={selected.type}
              onChange={(e) => updateField(selectedIdx!, { type: e.target.value as FieldDef["type"] })}
              className="h-7 w-full rounded-md border border-input bg-background px-2 text-xs"
            >
              {FIELD_TYPES.map((t) => (
                <option key={t} value={t}>{FIELD_META[t].label}</option>
              ))}
            </select>
          </Field>

          <Field label="占位文字">
            <Input
              value={selected.placeholder || ""}
              onChange={(e) => updateField(selectedIdx!, { placeholder: e.target.value })}
              className="h-7 text-xs"
            />
          </Field>

          <Field label="默认值">
            <Input
              value={selected.default || ""}
              onChange={(e) => updateField(selectedIdx!, { default: e.target.value })}
              className="h-7 text-xs"
            />
          </Field>

          {(selected.type === "select" || selected.type === "checkbox") && (
            <Field label="选项（逗号分隔）">
              <Input
                value={(selected.options || []).join(", ")}
                onChange={(e) => updateField(selectedIdx!, { options: e.target.value.split(",").map(s => s.trim()).filter(Boolean) })}
                className="h-7 text-xs"
                placeholder="选项A, 选项B, 选项C"
              />
            </Field>
          )}

          {selected.type === "ai_text" && (
            <Field label="AI 提示（AI Hint）">
              <textarea
                value={selected.ai_hint || ""}
                onChange={(e) => updateField(selectedIdx!, { ai_hint: e.target.value })}
                className="w-full rounded-md border border-input bg-background p-2 text-xs min-h-[60px] resize-y"
                placeholder="例如：根据工单标题自动生成摘要"
              />
            </Field>
          )}

          <label className="flex items-center gap-2 text-xs">
            <input
              type="checkbox"
              checked={selected.required}
              onChange={(e) => updateField(selectedIdx!, { required: e.target.checked })}
              className="rounded"
            />
            必填字段
          </label>
        </div>
      )}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <p className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">{label}</p>
      {children}
    </div>
  );
}
