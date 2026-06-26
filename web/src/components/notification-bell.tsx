import { useState, useEffect, useCallback, useRef } from "react";
import { Bell, CheckCheck, Loader2, FileText, Undo2, Mail } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import { authHeaders, useAuth } from "@/lib/auth";

const API_BASE = import.meta.env.VITE_API_BASE ?? "/api/v1";

interface NotificationItem {
  id: string;
  entity_id: string;
  notif_type: string;
  title: string;
  content: string;
  target_uid: string;
  read: boolean;
  created_at: string;
}

interface NotifResponse {
  notifications: NotificationItem[];
  total: number;
  unread: number;
  page: number;
  limit: number;
}

const NOTIF_ICONS: Record<string, typeof Bell> = {
  todo_created: Mail,
  entity_returned: Undo2,
  entity_completed: FileText,
};

export function NotificationBell() {
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const [notifs, setNotifs] = useState<NotificationItem[]>([]);
  const [unread, setUnread] = useState(0);
  const [loading, setLoading] = useState(false);
  const [markingAll, setMarkingAll] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const fetchNotifs = useCallback(async () => {
    if (!user) return;
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/notifications?limit=10`, {
        headers: { ...authHeaders() },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: NotifResponse = await res.json();
      setNotifs(data.notifications ?? []);
      setUnread(data.unread ?? 0);
    } catch (err) {
      console.error("Failed to fetch notifications:", err);
    } finally {
      setLoading(false);
    }
  }, [user]);

  useEffect(() => {
    fetchNotifs();
    // Poll every 30 seconds
    const interval = setInterval(fetchNotifs, 30000);
    return () => clearInterval(interval);
  }, [fetchNotifs]);

  // Close on outside click
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    if (open) document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  const handleMarkAllRead = async () => {
    setMarkingAll(true);
    try {
      const res = await fetch(`${API_BASE}/notifications/read-all`, {
        method: "POST",
        headers: { ...authHeaders() },
      });
      if (res.ok) {
        setNotifs((prev) => prev.map((n) => ({ ...n, read: true })));
        setUnread(0);
      }
    } catch (err) {
      console.error("Failed to mark all read:", err);
    } finally {
      setMarkingAll(false);
    }
  };

  const handleMarkRead = async (id: string) => {
    try {
      await fetch(`${API_BASE}/notifications/${id}/read`, {
        method: "POST",
        headers: { ...authHeaders() },
      });
      setNotifs((prev) =>
        prev.map((n) => (n.id === id ? { ...n, read: true } : n))
      );
      setUnread((prev) => Math.max(0, prev - 1));
    } catch (err) {
      console.error("Failed to mark read:", err);
    }
  };

  if (!user) return null;

  const IconMap = NOTIF_ICONS;

  return (
    <div ref={ref} className="relative">
      <Button
        variant="ghost"
        size="icon-sm"
        className="relative"
        onClick={() => setOpen(!open)}
      >
        <Bell className="h-4 w-4" />
        {unread > 0 && (
          <span className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold text-white leading-none">
            {unread > 99 ? "99+" : unread}
          </span>
        )}
      </Button>

      {open && (
        <div className="absolute right-0 top-full mt-1 z-50 w-80 rounded-xl border border-border/60 bg-white shadow-lg">
          {/* Header */}
          <div className="flex items-center justify-between border-b px-4 py-2.5">
            <span className="text-sm font-semibold">通知</span>
            {unread > 0 && (
              <Button
                variant="ghost"
                size="xs"
                onClick={handleMarkAllRead}
                disabled={markingAll}
              >
                {markingAll ? (
                  <Loader2 className="h-3 w-3 animate-spin mr-1" />
                ) : (
                  <CheckCheck className="h-3 w-3 mr-1" />
                )}
                全部已读
              </Button>
            )}
          </div>

          {/* List */}
          <ScrollArea className="max-h-80">
            {loading && notifs.length === 0 ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
              </div>
            ) : notifs.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                <Bell className="h-8 w-8 mb-2 opacity-30" />
                <p className="text-xs">暂无通知</p>
              </div>
            ) : (
              <div className="divide-y divide-border/40">
                {notifs.map((n) => {
                  const Icon = IconMap[n.notif_type] || Bell;
                  return (
                    <button
                      key={n.id}
                      className={cn(
                        "w-full text-left px-4 py-3 transition-colors hover:bg-muted/30",
                        !n.read && "bg-blue-50/50"
                      )}
                      onClick={() => handleMarkRead(n.id)}
                    >
                      <div className="flex gap-2.5">
                        <div
                          className={cn(
                            "mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full",
                            n.read
                              ? "bg-muted/50 text-muted-foreground"
                              : "bg-primary/10 text-primary"
                          )}
                        >
                          <Icon className="h-3.5 w-3.5" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <p
                            className={cn(
                              "text-xs font-medium truncate",
                              !n.read && "font-semibold"
                            )}
                          >
                            {n.title}
                          </p>
                          <p className="mt-0.5 text-[11px] text-muted-foreground line-clamp-2">
                            {n.content}
                          </p>
                          <p className="mt-1 text-[10px] text-muted-foreground/60">
                            {n.created_at
                              ? new Date(n.created_at).toLocaleString("zh-CN")
                              : ""}
                          </p>
                        </div>
                        {!n.read && (
                          <div className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                        )}
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </ScrollArea>
        </div>
      )}
    </div>
  );
}
