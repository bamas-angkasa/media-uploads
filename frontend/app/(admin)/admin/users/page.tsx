"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Search, Loader2, ShieldOff, ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Navbar } from "@/components/layout/Navbar";
import { adminApi } from "@/lib/api";
import { useAuthStore } from "@/lib/store";
import { formatBytes } from "@/lib/utils";
import { toast } from "sonner";
import { formatDistanceToNow } from "date-fns";

export default function AdminUsersPage() {
  const { user, isLoading } = useAuthStore();
  const router = useRouter();
  const [items, setItems] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  useEffect(() => {
    if (!isLoading && (!user || user.role !== "admin")) router.push("/");
  }, [user, isLoading, router]);

  useEffect(() => {
    if (user?.role === "admin") loadUsers();
  }, [user, page]);

  const loadUsers = async () => {
    setLoading(true);
    try {
      const { data } = await adminApi.listUsers({ page, page_size: 20, q: search || undefined });
      setItems(data.data || []);
      setTotal(data.total || 0);
    } catch {
      toast.error("Failed to load users");
    } finally {
      setLoading(false);
    }
  };

  const toggleActive = async (userId: string, currentlyActive: boolean) => {
    try {
      await adminApi.updateUser(userId, { is_active: !currentlyActive });
      setItems((prev) =>
        prev.map((u) => u.id === userId ? { ...u, is_active: !currentlyActive } : u)
      );
      toast.success(currentlyActive ? "User suspended" : "User activated");
    } catch {
      toast.error("Failed to update user");
    }
  };

  if (isLoading || !user || user.role !== "admin") return null;

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="container mx-auto py-8 px-4">
        <h1 className="text-3xl font-bold mb-6">User Management</h1>

        <div className="flex gap-3 mb-4">
          <div className="relative flex-1 max-w-sm">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && loadUsers()}
              placeholder="Search username or email..."
              className="pl-9"
            />
          </div>
          <Button onClick={loadUsers} variant="outline">Search</Button>
        </div>

        <p className="text-sm text-muted-foreground mb-4">{total} total users</p>

        {loading ? (
          <div className="flex justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="rounded-lg border overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-muted/50">
                <tr>
                  <th className="text-left p-3 font-medium">User</th>
                  <th className="text-left p-3 font-medium">Role</th>
                  <th className="text-left p-3 font-medium">Storage</th>
                  <th className="text-left p-3 font-medium">Status</th>
                  <th className="text-right p-3 font-medium">Joined</th>
                  <th className="p-3"></th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {items.map((u) => (
                  <tr key={u.id} className="hover:bg-muted/20">
                    <td className="p-3">
                      <p className="font-medium">{u.username}</p>
                      <p className="text-xs text-muted-foreground">{u.email}</p>
                    </td>
                    <td className="p-3">
                      <Badge variant={u.role === "admin" ? "default" : "secondary"}>
                        {u.role}
                      </Badge>
                    </td>
                    <td className="p-3">
                      <p className="text-xs">
                        {formatBytes(u.storage_used)} / {formatBytes(u.storage_quota)}
                      </p>
                      <div className="h-1.5 rounded-full bg-secondary mt-1 w-24 overflow-hidden">
                        <div
                          className="h-full bg-primary"
                          style={{ width: `${Math.min(100, (u.storage_used / u.storage_quota) * 100)}%` }}
                        />
                      </div>
                    </td>
                    <td className="p-3">
                      <Badge variant={u.is_active ? "default" : "destructive"}>
                        {u.is_active ? "Active" : "Suspended"}
                      </Badge>
                    </td>
                    <td className="p-3 text-right text-muted-foreground text-xs">
                      {formatDistanceToNow(new Date(u.created_at), { addSuffix: true })}
                    </td>
                    <td className="p-3">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => toggleActive(u.id, u.is_active)}
                        className={u.is_active ? "text-destructive hover:text-destructive" : ""}
                      >
                        {u.is_active
                          ? <><ShieldOff className="h-3.5 w-3.5 mr-1" />Suspend</>
                          : <><ShieldCheck className="h-3.5 w-3.5 mr-1" />Activate</>
                        }
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {items.length === 0 && (
              <div className="text-center py-10 text-muted-foreground">No users found</div>
            )}
          </div>
        )}

        <div className="flex items-center justify-between mt-4">
          <Button variant="outline" onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page === 1}>
            Previous
          </Button>
          <span className="text-sm text-muted-foreground">Page {page}</span>
          <Button variant="outline" onClick={() => setPage((p) => p + 1)} disabled={items.length < 20}>
            Next
          </Button>
        </div>
      </main>
    </div>
  );
}
