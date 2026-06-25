import { Link } from "@tanstack/react-router";
import { useI18n } from "@/lib/i18n";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

const mockProcesses = [
  { alias: "OP", name: "OP申请单", ver: 1, status: "已发布", actCount: 5, created: "2026-06-24" },
  { alias: "LEAVE", name: "请假流程", ver: 2, status: "已发布", actCount: 4, created: "2026-06-20" },
  { alias: "PURCHASE", name: "采购审批", ver: 1, status: "草稿", actCount: 3, created: "2026-06-18" },
];

export function ProcessesPage() {
  const { t } = useI18n();

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">{t("processes.page.title")}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{t("processes.page.description")}</p>
        </div>
        <Link to="/processes/new" className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary px-3 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4"><path d="M5 12h14"/><path d="M12 5v14"/></svg>
          新建流程
        </Link>
      </div>

      <Card>
        <CardHeader className="border-b border-border/50">
          <CardTitle>流程定义列表</CardTitle>
          <CardDescription>共 {mockProcesses.length} 条流程定义</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHead>
              <TableRow>
                <TableHeaderCell>流程名称</TableHeaderCell>
                <TableHeaderCell>标识</TableHeaderCell>
                <TableHeaderCell>版本</TableHeaderCell>
                <TableHeaderCell>节点数</TableHeaderCell>
                <TableHeaderCell>状态</TableHeaderCell>
                <TableHeaderCell>创建时间</TableHeaderCell>
                <TableHeaderCell className="text-right">操作</TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {mockProcesses.map((p) => (
                <TableRow key={p.alias}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell><code className="rounded bg-muted px-1.5 py-0.5 text-xs">{p.alias}</code></TableCell>
                  <TableCell className="text-muted-foreground">v{p.ver}</TableCell>
                  <TableCell className="text-muted-foreground">{p.actCount}</TableCell>
                  <TableCell>
                    <Badge variant={p.status === "已发布" ? "default" : "secondary"}>{p.status}</Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{p.created}</TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-2">
                      <Link to={`/processes/${p.alias}`} className="text-xs font-medium text-primary hover:underline">设计</Link>
                      <span className="text-muted-foreground/40">|</span>
                      <button type="button" className="text-xs font-medium text-muted-foreground hover:text-foreground">删除</button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
