import { Link } from "@tanstack/react-router";
import { useMemo } from "react";

import { CurrentUserPanel } from "@/admin/components/CurrentUserPanel";
import { NotificationBell } from "@/components/notification-bell";
import { groups, type NavGroup } from "@/admin/menu-access";
import { useI18n, type TranslationKey } from "@/lib/i18n";
import { cn } from "@/lib/utils";

export function Sidebar() {
  const { t } = useI18n();

  return (
    <div className="flex h-screen w-72 flex-col border-r border-border/70 bg-[radial-gradient(circle_at_top,_rgba(54,111,255,0.1),_transparent_35%),linear-gradient(180deg,rgba(248,250,252,0.95),rgba(255,255,255,0.98))]">
      <div className="border-b border-border/70 px-6 py-5">
        <div className="text-xs font-semibold uppercase tracking-[0.24em] text-primary/70">
          {t("sidebar.platform")}
        </div>
        <div className="mt-3 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="text-xl font-semibold tracking-tight">
              {t("sidebar.unknownApp")}
            </div>
          </div>
          <NotificationBell />
        </div>
      </div>
      <nav className="flex-1 space-y-6 overflow-y-auto px-4 py-5">
        {groups.map((group) => (
          <div key={group.titleKey}>
            <div className="mb-2 px-2 text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">
              {t(group.titleKey as TranslationKey)}
            </div>
            <div className="space-y-1">
              {group.items.map((item) => (
                <Link
                  key={item.labelKey}
                  to={item.to}
                  className={cn(
                    "flex items-center gap-3 rounded-2xl px-3 py-2.5 text-sm font-medium transition-colors",
                    "text-muted-foreground hover:bg-muted/80 hover:text-foreground",
                  )}
                  activeProps={{
                    className:
                      "flex items-center gap-3 rounded-2xl px-3 py-2.5 text-sm font-medium transition-colors bg-primary text-primary-foreground shadow-sm hover:bg-primary hover:text-primary-foreground",
                  }}
                >
                  <item.icon className="h-4 w-4" />
                  {t(item.labelKey as TranslationKey)}
                </Link>
              ))}
            </div>
          </div>
        ))}
      </nav>
      <CurrentUserPanel />
    </div>
  );
}
