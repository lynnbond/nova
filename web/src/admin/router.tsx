import { Link, Navigate, Outlet, createRootRoute, createRoute, createRouter, useLocation } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/lib/i18n";
import { useAuth } from "@/lib/auth";
import { AppLayout } from "@/admin/layouts/AppLayout";
import { WelcomePage } from "@/admin/pages/welcome-page";
import { AuthCallbackPage } from "@/admin/pages/auth-callback";
import { LoginPage } from "@/admin/pages/login-page";
import { ProcessesPage } from "@/admin/pages/processes-page";
import { ProcessDesignerPage } from "@/admin/pages/process-designer";
import { ProcessNewPage } from "@/admin/pages/process-new";
import { EntitiesPage } from "@/admin/pages/entities-page";
import { TodosPage } from "@/admin/pages/todos-page";
import { OutboxPage } from "@/admin/pages/outbox-page";
import { UsersPage } from "@/admin/pages/users-page";

/* ── Protected layout: checks auth, redirects to /login ── */
function ProtectedLayout() {
  const auth = useAuth();
  const location = useLocation();

  if (auth.loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[#f5f7fb]">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!auth.isAuthenticated) {
    // Only redirect if we're not already on the login page
    if (location.pathname !== "/login" && location.pathname !== "/auth/callback") {
      const redirect = encodeURIComponent(location.pathname);
      return <Navigate to={`/login?redirect=${redirect}`} />;
    }
  }

  return <Outlet />;
}

/* ── Auth-free login layout ── */
function LoginLayout() {
  return (
    <div className="flex min-h-screen overflow-hidden bg-[#f5f7fb]">
      <Outlet />
    </div>
  );
}

/* ── Redirect for "/" ── */
function IndexRedirect() {
  return <Navigate to="/welcome" />;
}

/* ── 404 page ── */
function NotFoundPage() {
  const { t } = useI18n();
  return (
    <Card className="mx-auto mt-10 max-w-3xl">
      <CardHeader>
        <CardTitle>{t("router.notFound.title")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm text-muted-foreground">
        <p>{t("router.notFound.description")}</p>
        <Link to="/welcome" className="text-primary underline-offset-4 hover:underline">
          {t("router.notFound.back")}
        </Link>
      </CardContent>
    </Card>
  );
}

/* ── Route definitions ── */

// Root route — no component, just a container for children
const rootRoute = createRootRoute();

// Login route (public, no auth needed)
const loginLayoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "login-layout",
  component: LoginLayout,
});
const loginRoute = createRoute({
  getParentRoute: () => loginLayoutRoute,
  path: "/login",
  component: LoginPage,
});
const authCallbackRoute = createRoute({
  getParentRoute: () => loginLayoutRoute,
  path: "/auth/callback",
  component: AuthCallbackPage,
});

// Protected routes (auth required)
const protectedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "protected",
  component: ProtectedLayout,
});

const appLayoutRoute = createRoute({
  getParentRoute: () => protectedRoute,
  id: "app-layout",
  component: AppLayout,
  notFoundComponent: NotFoundPage,
});

const indexRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/", component: IndexRedirect });
const welcomeRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/welcome", component: WelcomePage });
const processesRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/processes", component: ProcessesPage });
const processDesignerRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/processes/$alias", component: ProcessDesignerPage });
const processNewRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/processes/new", component: ProcessNewPage });
const entitiesRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/entities", component: EntitiesPage });
const todosRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/todos", component: TodosPage });
const outboxRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/outbox", component: OutboxPage });
const usersRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/users", component: UsersPage });

const routeTree = rootRoute.addChildren([
  loginLayoutRoute.addChildren([loginRoute, authCallbackRoute]),
  protectedRoute.addChildren([
    appLayoutRoute.addChildren([
      indexRoute, welcomeRoute, processesRoute, processDesignerRoute, processNewRoute,
      entitiesRoute, todosRoute, outboxRoute, usersRoute,
    ]),
  ]),
]);

export const router = createRouter({ routeTree, defaultPreload: "intent" });

declare module "@tanstack/react-router" {
  interface Register { router: typeof router; }
}
