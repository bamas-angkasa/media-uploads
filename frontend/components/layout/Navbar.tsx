"use client";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Image, Upload, LayoutDashboard, Shield, LogOut, LogIn } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/lib/store";
import { authApi } from "@/lib/api";
import { clearAccessToken } from "@/lib/auth";
import { toast } from "sonner";

export function Navbar() {
  const { user, clearAuth } = useAuthStore();
  const router = useRouter();

  const handleLogout = async () => {
    try {
      await authApi.logout();
    } catch {}
    clearAccessToken();
    clearAuth();
    router.push("/login");
    toast.success("Logged out");
  };

  return (
    <header className="sticky top-0 z-40 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container flex h-14 items-center gap-4 px-4">
        <Link href="/explore" className="flex items-center gap-2 font-semibold text-lg">
          <Image className="h-5 w-5" />
          <span>MediaShare</span>
        </Link>

        <nav className="flex-1 flex items-center gap-2">
          <Button variant="ghost" size="sm" asChild>
            <Link href="/explore">Explore</Link>
          </Button>
          {user && (
            <>
              <Button variant="ghost" size="sm" asChild>
                <Link href="/upload">
                  <Upload className="h-4 w-4 mr-1" />
                  Upload
                </Link>
              </Button>
              <Button variant="ghost" size="sm" asChild>
                <Link href="/dashboard">
                  <LayoutDashboard className="h-4 w-4 mr-1" />
                  My Files
                </Link>
              </Button>
              {user.role === "admin" && (
                <Button variant="ghost" size="sm" asChild>
                  <Link href="/admin">
                    <Shield className="h-4 w-4 mr-1" />
                    Admin
                  </Link>
                </Button>
              )}
            </>
          )}
        </nav>

        <div className="flex items-center gap-2">
          {user ? (
            <>
              <span className="text-sm text-muted-foreground hidden sm:block">
                {user.username}
              </span>
              <Button variant="ghost" size="icon" onClick={handleLogout} title="Logout">
                <LogOut className="h-4 w-4" />
              </Button>
            </>
          ) : (
            <>
              <Button variant="ghost" size="sm" asChild>
                <Link href="/login">
                  <LogIn className="h-4 w-4 mr-1" />
                  Login
                </Link>
              </Button>
              <Button size="sm" asChild>
                <Link href="/register">Register</Link>
              </Button>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
