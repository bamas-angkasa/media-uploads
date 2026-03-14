"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Users, FileImage, HardDrive, AlertTriangle, TrendingUp, Upload } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Navbar } from "@/components/layout/Navbar";
import { adminApi } from "@/lib/api";
import { useAuthStore } from "@/lib/store";
import type { PlatformStats } from "@/lib/types";

function StatCard({ title, value, icon: Icon, description }: {
  title: string;
  value: string | number;
  icon: React.ElementType;
  description?: string;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value}</div>
        {description && <p className="text-xs text-muted-foreground mt-1">{description}</p>}
      </CardContent>
    </Card>
  );
}

export default function AdminDashboardPage() {
  const { user, isLoading } = useAuthStore();
  const router = useRouter();
  const [stats, setStats] = useState<PlatformStats | null>(null);

  useEffect(() => {
    if (!isLoading) {
      if (!user || user.role !== "admin") {
        router.push("/");
      }
    }
  }, [user, isLoading, router]);

  useEffect(() => {
    if (user?.role === "admin") {
      adminApi.getStats().then(({ data }) => setStats(data.data));
    }
  }, [user]);

  if (isLoading || !user || user.role !== "admin") return null;

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="container mx-auto py-8 px-4">
        <h1 className="text-3xl font-bold mb-6">Admin Dashboard</h1>

        {stats ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-8">
            <StatCard title="Total Media" value={stats.total_media.toLocaleString()} icon={FileImage} description={`+${stats.media_last_24h} in last 24h`} />
            <StatCard title="Total Users" value={stats.total_users.toLocaleString()} icon={Users} description={`+${stats.users_last_7d} this week`} />
            <StatCard title="Storage Used" value={`${stats.total_storage_gb.toFixed(2)} GB`} icon={HardDrive} />
            <StatCard title="Pending Reports" value={stats.pending_reports} icon={AlertTriangle} description="Awaiting review" />
            <StatCard title="Uploads (24h)" value={stats.media_last_24h} icon={Upload} />
            <StatCard title="New Users (7d)" value={stats.users_last_7d} icon={TrendingUp} />
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-8">
            {[...Array(6)].map((_, i) => (
              <Card key={i}>
                <CardContent className="pt-6">
                  <div className="h-8 bg-muted rounded animate-pulse" />
                </CardContent>
              </Card>
            ))}
          </div>
        )}

        {/* Quick links */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          {[
            { href: "/admin/media", label: "Manage Media", icon: FileImage },
            { href: "/admin/users", label: "Manage Users", icon: Users },
            { href: "/admin/reports", label: "Review Reports", icon: AlertTriangle },
          ].map(({ href, label, icon: Icon }) => (
            <a
              key={href}
              href={href}
              className="flex items-center gap-3 p-4 rounded-lg border hover:bg-muted/50 transition-colors"
            >
              <Icon className="h-5 w-5 text-muted-foreground" />
              <span className="font-medium">{label}</span>
            </a>
          ))}
        </div>
      </main>
    </div>
  );
}
