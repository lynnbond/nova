import { Outlet } from "@tanstack/react-router";

import { Sidebar } from "@/admin/components/Sidebar";

export function AppLayout() {
  return (
    <div className="flex min-h-screen overflow-hidden bg-[linear-gradient(180deg,#f5f8ff_0%,#fbfcfe_22%,#f8fafc_100%)]">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-y-auto">
        <main className="p-8 pb-16">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
