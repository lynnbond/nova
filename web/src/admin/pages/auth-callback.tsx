import { useEffect, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Loader2, AlertCircle } from "lucide-react";
import { useAuth } from "@/lib/auth";

export function AuthCallbackPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!auth.iamEnabled || auth.isAuthenticated) {
      navigate({ to: "/" });
      return;
    }

    const params = new URLSearchParams(window.location.search);
    const shortToken = params.get("token");

    if (!shortToken) {
      setError("未收到认证信息");
      return;
    }

    // Clean up URL
    window.history.replaceState({}, document.title, "/auth/callback");

    // Exchange short token for CSRF token via IAM
    fetch(`${import.meta.env.VITE_IAM_BASE_URL}/iam/api/v2/user/csrf-token`, {
      method: "POST",
      headers: { "Csrf-Token": shortToken },
    })
      .then((res) => {
        if (!res.ok) throw new Error("获取Token失败: HTTP " + res.status);
        return res.json();
      })
      .then((data) => {
        const csrfToken = data.token || data.data?.token;
        if (!csrfToken) throw new Error("响应中缺少Token");
        return auth.authCallback(csrfToken);
      })
      .then(() => {
        navigate({ to: "/" });
      })
      .catch((err) => {
        setError(err.message || "IAM登录失败");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[#f5f7fb]">
        <div className="flex flex-col items-center gap-4 text-center">
          <AlertCircle className="h-12 w-12 text-red-400" />
          <h2 className="text-lg font-semibold text-foreground">登录失败</h2>
          <p className="text-sm text-muted-foreground max-w-xs">{error}</p>
          <button
            onClick={() => window.location.href = "/login"}
            className="text-sm text-primary underline underline-offset-4 hover:text-primary/80"
          >
            返回登录页
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-[#f5f7fb]">
      <div className="flex flex-col items-center gap-4">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        <p className="text-sm text-muted-foreground">正在登录...</p>
      </div>
    </div>
  );
}
