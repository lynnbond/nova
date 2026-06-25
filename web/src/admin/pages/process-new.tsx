import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

export function ProcessNewPage() {
  const navigate = useNavigate();
  const [alias, setAlias] = useState("");
  const [name, setName] = useState("");

  function handleCreate() {
    if (!alias.trim() || !name.trim()) return;
    navigate({ to: "/processes/$alias", params: { alias: alias.trim().toUpperCase() } });
  }

  return (
    <div className="mx-auto mt-12 max-w-lg">
      <Card>
        <CardHeader>
          <CardTitle>新建流程</CardTitle>
          <CardDescription>创建一个新的工作流流程定义</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1">
            <label className="text-xs font-medium text-muted-foreground">流程标识</label>
            <Input value={alias} onChange={(e) => setAlias(e.target.value.toUpperCase())} placeholder="OP" className="font-mono" />
            <p className="text-[11px] text-muted-foreground">4-10 位大写字母，唯一标识</p>
          </div>
          <div className="space-y-1">
            <label className="text-xs font-medium text-muted-foreground">流程名称</label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="OP申请单" />
          </div>
          <div className="flex gap-2 pt-2">
            <Button variant="outline" onClick={() => navigate({ to: "/processes" })}>取消</Button>
            <Button onClick={handleCreate} disabled={!alias.trim() || !name.trim()}>创建并设计</Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
