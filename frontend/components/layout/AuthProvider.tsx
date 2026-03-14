"use client";
import { useEffect } from "react";
import { authApi } from "@/lib/api";
import { setAccessToken } from "@/lib/auth";
import { useAuthStore } from "@/lib/store";

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const { setAuth, setLoading } = useAuthStore();

  useEffect(() => {
    // On mount, try to refresh the access token using the HttpOnly cookie
    const init = async () => {
      try {
        const { data } = await authApi.refresh();
        setAccessToken(data.access_token);
        const { data: meData } = await authApi.me();
        setAuth(meData.user, data.access_token);
      } catch {
        // Not logged in — that's fine
      } finally {
        setLoading(false);
      }
    };
    init();
  }, [setAuth, setLoading]);

  return <>{children}</>;
}
