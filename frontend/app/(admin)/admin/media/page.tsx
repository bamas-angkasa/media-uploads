"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Trash2, ExternalLink, Search, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Navbar } from "@/components/layout/Navbar";
import { adminApi } from "@/lib/api";
import { useAuthStore } from "@/lib/store";
import { formatBytes, formatNumber } from "@/lib/utils";
import { toast } from "sonner";
import Link from "next/link";
import { formatDistanceToNow } from "date-fns";

export default function AdminMediaPage() {
  const { user, isLoading } = useAuthStore();
  const router = useRouter();
  const [items, setItems] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");

  useEffect(() => {
    if (!isLoading && (!user || user.role !== "admin")) router.push("/");
  }, [user, isLoading, router]);

  useEffect(() => {
    if (user?.role === "admin") loadMedia();
  }, [user, page, statusFilter]);

  const loadMedia = async () => {
    setLoading(true);
    try {
      const { data } = await adminApi.listMedia({
        page, page_size: 20,
        q: search || undefined,
        status: statusFilter || undefined,
      });
      setItems(data.data || []);
      setTotal(data.total || 0);
    } catch {
      toast.error("Failed to load media");
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("Permanently delete this file?")) return;
    try {
      await adminApi.deleteMedia(id);
      setItems((prev) => prev.filter((i) => i.id !== id));
      toast.success("Deleted");
    } catch {
      toast.error("Failed to delete");
    }
  };

  if (isLoading || !user || user.role !== "admin") return null;

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="container mx-auto py-8 px-4">
        <h1 className="text-3xl font-bold mb-6">Media Management</h1>

        {/* Filters */}
        <div className="flex gap-3 mb-4">
          <div className="relative flex-1 max-w-sm">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && loadMedia()}
              placeholder="Search title or short code..."
              className="pl-9"
            />
          </div>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="rounded-md border border-input bg-background px-3 py-2 text-sm"
          >
            <option value="">All status</option>
            <option value="ready">Ready</option>
            <option value="processing">Processing</option>
            <option value="failed">Failed</option>
          </select>
          <Button onClick={loadMedia} variant="outline">Search</Button>
        </div>

        <p className="text-sm text-muted-foreground mb-4">{total} total files</p>

        {loading ? (
          <div className="flex justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="rounded-lg border overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-muted/50">
                <tr>
                  <th className="text-left p-3 font-medium">File</th>
                  <th className="text-left p-3 font-medium">User</th>
                  <th className="text-left p-3 font-medium">Status</th>
                  <th className="text-right p-3 font-medium">Views</th>
                  <th className="text-right p-3 font-medium">Size</th>
                  <th className="text-right p-3 font-medium">Date</th>
                  <th className="p-3"></th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {items.map((item) => (
                  <tr key={item.id} className="hover:bg-muted/20">
                    <td className="p-3">
                      <div>
                        <p className="font-medium truncate max-w-[200px]">{item.title || "—"}</p>
                        <p className="text-xs text-muted-foreground font-mono">{item.short_code}</p>
                      </div>
                    </td>
                    <td className="p-3">
                      <div>
                        <p className="font-medium">{item.username}</p>
                        <p className="text-xs text-muted-foreground">{item.email}</p>
                      </div>
                    </td>
                    <td className="p-3">
                      <Badge variant={item.status === "ready" ? "default" : item.status === "failed" ? "destructive" : "secondary"}>
                        {item.status}
                      </Badge>
                    </td>
                    <td className="p-3 text-right">{formatNumber(item.view_count)}</td>
                    <td className="p-3 text-right">{formatBytes(item.file_size)}</td>
                    <td className="p-3 text-right text-muted-foreground text-xs">
                      {formatDistanceToNow(new Date(item.created_at), { addSuffix: true })}
                    </td>
                    <td className="p-3">
                      <div className="flex gap-1 justify-end">
                        <Button variant="ghost" size="icon" className="h-7 w-7" asChild>
                          <Link href={`/i/${item.short_code}`} target="_blank">
                            <ExternalLink className="h-3.5 w-3.5" />
                          </Link>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-destructive hover:text-destructive"
                          onClick={() => handleDelete(item.id)}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {items.length === 0 && (
              <div className="text-center py-10 text-muted-foreground">No files found</div>
            )}
          </div>
        )}

        {/* Pagination */}
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
