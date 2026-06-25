import { useState, useEffect, useCallback } from "react";
import { Plus, FileText, Loader2, Pencil, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogTrigger, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { authHeaders } from "@/lib/auth";

const API_BASE = "http://localhost:8080/api/v1";

interface UserItem {
  id: string;
  uid: string;
  name: string;
  email: string;
  phone: string;
  dept: string;
  state: "active" | "disabled";
  created_at: string;
  updated_at: string;
}

interface UserFormData {
  uid: string;
  name: string;
  email: string;
  phone: string;
  dept: string;
  state: "active" | "disabled";
}

const emptyForm: UserFormData = {
  uid: "",
  name: "",
  email: "",
  phone: "",
  dept: "",
  state: "active",
};

export function UsersPage() {
  const { t } = useI18n();

  const [users, setUsers] = useState<UserItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [form, setForm] = useState<UserFormData>(emptyForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const fetchUsers = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/users`, { headers: { ...authHeaders() } });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const mapped = (data.users ?? []).map((u: Record<string, unknown>) => ({
        ...u,
        state: u.state === 0 || u.state === "0" ? "active" : "disabled",
        id: u.id ?? u.ID,
        uid: u.uid ?? u.UID,
        name: u.name ?? u.Name,
        dept: u.dept ?? u.Dept,
        email: u.email ?? u.Email,
        phone: u.phone ?? u.Phone,
      }));
      setUsers(mapped);
    } catch (err) {
      console.error("Failed to fetch users:", err);
      setUsers([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const openCreate = () => {
    setForm(emptyForm);
    setCreateOpen(true);
  };

  const openEdit = (user: UserItem) => {
    setForm({
      uid: user.uid,
      name: user.name,
      email: user.email,
      phone: user.phone,
      dept: user.dept,
      state: user.state,
    });
    setEditingId(user.id);
    setEditOpen(true);
  };

  const openDelete = (id: string) => {
    setDeletingId(id);
    setDeleteOpen(true);
  };

  const handleSave = async (isEdit: boolean) => {
    if (!form.uid.trim() || !form.name.trim()) return;
    setSaving(true);
    try {
      const url = isEdit
        ? `${API_BASE}/users/${editingId}`
        : `${API_BASE}/users`;
      const method = isEdit ? "PUT" : "POST";
      const body = {
        ...form,
        state: form.state === "active" ? 0 : 1,
      };
      const res = await fetch(url, {
        method,
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      if (isEdit) {
        setEditOpen(false);
      } else {
        setCreateOpen(false);
      }
      setForm(emptyForm);
      setEditingId(null);
      fetchUsers();
    } catch (err) {
      console.error("Failed to save user:", err);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deletingId) return;
    setSaving(true);
    try {
      const res = await fetch(`${API_BASE}/users/${deletingId}`, {
        method: "DELETE",
        headers: { ...authHeaders() },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setDeleteOpen(false);
      setDeletingId(null);
      fetchUsers();
    } catch (err) {
      console.error("Failed to delete user:", err);
    } finally {
      setSaving(false);
    }
  };

  const stateBadge = (state: string) => {
    if (state === "active") {
      return <Badge variant="outline" className="border-emerald-500 text-emerald-600">{t("users.state.active")}</Badge>;
    }
    return <Badge variant="outline" className="border-red-500 text-red-600">{t("users.state.disabled")}</Badge>;
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">{t("users.page.title")}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{t("users.page.description")}</p>
        </div>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger render={<Button onClick={openCreate}><Plus className="h-4 w-4" />{t("users.create.title")}</Button>} />
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("users.create.title")}</DialogTitle>
              <DialogDescription>{t("users.page.description")}</DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-2">
              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t("users.field.uid")}</label>
                <Input
                  placeholder={t("users.field.uid")}
                  value={form.uid}
                  onChange={(e) => setForm({ ...form, uid: e.target.value })}
                  autoFocus
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t("users.field.name")}</label>
                <Input
                  placeholder={t("users.field.name")}
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t("users.field.dept")}</label>
                <Input
                  placeholder={t("users.field.dept")}
                  value={form.dept}
                  onChange={(e) => setForm({ ...form, dept: e.target.value })}
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t("users.field.email")}</label>
                <Input
                  placeholder={t("users.field.email")}
                  type="email"
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t("users.field.phone")}</label>
                <Input
                  placeholder={t("users.field.phone")}
                  value={form.phone}
                  onChange={(e) => setForm({ ...form, phone: e.target.value })}
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t("users.field.state")}</label>
                <Select
                  value={form.state}
                  onValueChange={(v: "active" | "disabled") => setForm({ ...form, state: v })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="active">{t("users.state.active")}</SelectItem>
                      <SelectItem value="disabled">{t("users.state.disabled")}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setCreateOpen(false)}>{t("common.cancel")}</Button>
              <Button onClick={() => handleSave(false)} disabled={saving || !form.uid.trim() || !form.name.trim()}>
                {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                {t("common.confirm")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("users.page.title")}</CardTitle>
          <CardDescription>
            {loading ? t("common.loading") : `${users.length} 个用户`}
          </CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {loading ? (
            <div className="flex items-center justify-center py-16">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : users.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
              <FileText className="h-10 w-10 mb-3 opacity-40" />
              <p className="text-sm">{t("common.none")}</p>
            </div>
          ) : (
            <Table>
              <TableHead>
                <TableRow>
                  <TableHeaderCell>{t("users.table.uid")}</TableHeaderCell>
                  <TableHeaderCell>{t("users.table.name")}</TableHeaderCell>
                  <TableHeaderCell>{t("users.table.dept")}</TableHeaderCell>
                  <TableHeaderCell>{t("users.table.email")}</TableHeaderCell>
                  <TableHeaderCell>{t("users.table.phone")}</TableHeaderCell>
                  <TableHeaderCell>{t("users.table.state")}</TableHeaderCell>
                  <TableHeaderCell>{t("users.table.actions")}</TableHeaderCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {users.map((u) => (
                  <TableRow
                    key={u.id}
                    className="cursor-pointer"
                    onClick={() => openEdit(u)}
                  >
                    <TableCell className="font-medium font-mono text-xs">{u.uid}</TableCell>
                    <TableCell>{u.name}</TableCell>
                    <TableCell className="text-muted-foreground">{u.dept || "-"}</TableCell>
                    <TableCell className="text-muted-foreground">{u.email || "-"}</TableCell>
                    <TableCell className="text-muted-foreground">{u.phone || "-"}</TableCell>
                    <TableCell>{stateBadge(u.state)}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => openEdit(u)}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => openDelete(u.id)}
                          className="text-red-500 hover:text-red-600 hover:bg-red-50"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Edit Dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("users.edit.title")}</DialogTitle>
            <DialogDescription>{t("users.edit.title")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t("users.field.uid")}</label>
              <Input
                placeholder={t("users.field.uid")}
                value={form.uid}
                onChange={(e) => setForm({ ...form, uid: e.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t("users.field.name")}</label>
              <Input
                placeholder={t("users.field.name")}
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t("users.field.dept")}</label>
              <Input
                placeholder={t("users.field.dept")}
                value={form.dept}
                onChange={(e) => setForm({ ...form, dept: e.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t("users.field.email")}</label>
              <Input
                placeholder={t("users.field.email")}
                type="email"
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t("users.field.phone")}</label>
              <Input
                placeholder={t("users.field.phone")}
                value={form.phone}
                onChange={(e) => setForm({ ...form, phone: e.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t("users.field.state")}</label>
              <Select
                value={form.state}
                onValueChange={(v: "active" | "disabled") => setForm({ ...form, state: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="active">{t("users.state.active")}</SelectItem>
                    <SelectItem value="disabled">{t("users.state.disabled")}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>{t("common.cancel")}</Button>
            <Button onClick={() => handleSave(true)} disabled={saving || !form.uid.trim() || !form.name.trim()}>
              {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("users.delete.confirm")}</DialogTitle>
            <DialogDescription>{t("users.delete.confirm")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>{t("common.cancel")}</Button>
            <Button variant="destructive" onClick={handleDelete} disabled={saving}>
              {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {t("common.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
