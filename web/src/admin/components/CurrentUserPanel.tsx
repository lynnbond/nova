import { LogOut } from "lucide-react";
import { useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";
import { useAuth } from "@/lib/auth";

function getInitial(name: string): string {
  if (!name) return "?";
  return name.charAt(0).toUpperCase();
}

export function CurrentUserPanel() {
  const { t } = useI18n();
  const auth = useAuth();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

  const displayName = auth.user?.name ?? t("currentUser.defaultName");
  const initial = getInitial(auth.user?.name ?? displayName);

  const handleLogout = () => {
    auth.logout();
    navigate({ to: "/login" });
  };

  return (
    <div ref={containerRef} className="relative border-t border-border/70 p-3">
      <button
        type="button"
        onClick={() => setOpen((c) => !c)}
        className="flex w-full items-center gap-3 rounded-2xl px-3 py-2.5 text-left text-sm font-medium text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground"
      >
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
          {initial}
        </div>
        <div className="min-w-0 flex-1">
          <div className="truncate">{displayName}</div>
          {auth.user?.dept && (
            <div className="truncate text-xs text-muted-foreground/70">{auth.user.dept}</div>
          )}
        </div>
      </button>
      {open ? (
        <div className="absolute bottom-full left-3 z-30 mb-3 w-[min(18rem,calc(100vw-2rem))] rounded-2xl border border-border/70 bg-background p-3 shadow-lg">
          <Button
            variant="outline"
            className="h-9 w-full justify-center text-sm"
            onClick={handleLogout}
          >
            <LogOut className="mr-2 h-4 w-4" />
            <span>{t("currentUser.logout")}</span>
          </Button>
        </div>
      ) : null}
    </div>
  );
}
