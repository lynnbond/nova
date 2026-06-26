import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from "react";

const API_BASE = import.meta.env.VITE_API_BASE ?? "/api/v1";
const IAM_BASE_URL = import.meta.env.VITE_IAM_BASE_URL ?? "";
const TOKEN_KEY = "nova_token";

export interface AuthUser {
  uid: string;
  name: string;
  dept: string;
}

interface AuthContextValue {
  user: AuthUser | null;
  token: string | null;
  loading: boolean;
  isAuthenticated: boolean;
  iamEnabled: boolean;
  login: (uid: string, password: string) => Promise<void>;
  logout: () => void;
  iamLogin: () => void;
  authCallback: (csrfToken: string) => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

/** Read token from localStorage directly -- used by existing pages that don't use the context. */
export function getStoredToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

/** Build auth headers object from stored token. */
export function authHeaders(): Record<string, string> {
  const token = getStoredToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

/** Helper: fetch with auth headers baked in. */
export async function apiFetch(path: string, options: RequestInit = {}): Promise<Response> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string> | undefined),
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
  return fetch(`${API_BASE}${path}`, { ...options, headers });
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [token, setToken] = useState<string | null>(() => getStoredToken());
  const [loading, setLoading] = useState(true);

  // On mount, try to restore session from stored token
  useEffect(() => {
    const stored = getStoredToken();
    if (!stored) {
      setLoading(false);
      return;
    }
    setToken(stored);
    fetch(`${API_BASE}/me`, {
      headers: { Authorization: `Bearer ${stored}` },
    })
      .then((res) => {
        if (!res.ok) throw new Error("Session expired");
        return res.json();
      })
      .then((data) => {
        const u = data.user ?? data;
        setUser({ uid: u.uid, name: u.name, dept: u.dept });
      })
      .catch(() => {
        // Token invalid / session expired -- clear
        localStorage.removeItem(TOKEN_KEY);
        setToken(null);
        setUser(null);
      })
      .finally(() => setLoading(false));
  }, []);

  const login = useCallback(async (uid: string, password: string) => {
    const res = await fetch(`${API_BASE}/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ uid, password }),
    });
    if (!res.ok) {
      let msg = "登录失败";
      try {
        const body = await res.json();
        msg = body.error || msg;
      } catch {}
      throw new Error(msg);
    }
    const data = await res.json();
    const tokenVal: string = data.token;
    const userVal: AuthUser = data.user ?? { uid, name: uid, dept: "" };

    localStorage.setItem(TOKEN_KEY, tokenVal);
    setToken(tokenVal);
    setUser(userVal);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    setToken(null);
    setUser(null);
  }, []);

  const iamLogin = useCallback(async () => {
    const origin = window.location.origin;
    const redirectUri = encodeURIComponent(`${origin}/auth/callback`);
    try {
      const res = await fetch(`${IAM_BASE_URL}/iam/api/v2/login/iam/url?redirect_uri=${redirectUri}`);
      if (!res.ok) throw new Error("HTTP " + res.status);
      const data = await res.json();
      if (data.url) {
        window.location.href = data.url;
      }
    } catch (err) {
      console.error("IAM login failed:", err);
      throw new Error("无法获取IAM登录地址");
    }
  }, []);

  const authCallback = useCallback(async (csrfToken: string) => {
    const res = await fetch(`${API_BASE}/auth/iam`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ csrf_token: csrfToken }),
    });
    if (!res.ok) {
      let msg = "IAM 登录失败";
      try { const body = await res.json(); msg = body.error || msg; } catch {}
      throw new Error(msg);
    }
    const data = await res.json();
    localStorage.setItem(TOKEN_KEY, data.token);
    setToken(data.token);
    setUser(data.user);
  }, []);

  const value: AuthContextValue = {
    user,
    token,
    loading,
    isAuthenticated: !!token && !!user,
    iamEnabled: !!IAM_BASE_URL,
    login,
    logout,
    iamLogin,
    authCallback,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return context;
}
