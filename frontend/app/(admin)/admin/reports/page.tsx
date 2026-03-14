"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { ExternalLink, CheckCircle, XCircle, Loader2, ImageOff, MoreHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Navbar } from "@/components/layout/Navbar";
import { adminApi } from "@/lib/api";
import { useAuthStore } from "@/lib/store";
import { toast } from "sonner";
import Link from "next/link";
import { formatDistanceToNow } from "date-fns";

function StatusBadge({ status }: { status: string }) {
  const variant =
    status === "pending"
      ? "secondary"
      : status === "resolved"
      ? "default"
      : "outline";
  return <Badge variant={variant}>{status}</Badge>;
}

function Thumbnail({ url, title }: { url?: string; title?: string }) {
  const [errored, setErrored] = useState(false);

  if (!url || errored) {
    return (
      <div className="w-16 h-16 rounded-md bg-muted flex items-center justify-center shrink-0">
        <ImageOff className="h-5 w-5 text-muted-foreground" />
      </div>
    );
  }

  return (
    <img
      src={url}
      alt={title ?? "thumbnail"}
      onError={() => setErrored(true)}
      className="w-16 h-16 rounded-md object-cover shrink-0 bg-muted"
    />
  );
}

export default function AdminReportsPage() {
  const { user, isLoading } = useAuthStore();
  const router = useRouter();
  const [items, setItems] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState("pending");
  const [acting, setActing] = useState<string | null>(null);

  useEffect(() => {
    if (!isLoading && (!user || user.role !== "admin")) router.push("/");
  }, [user, isLoading, router]);

  useEffect(() => {
    if (user?.role === "admin") loadReports();
  }, [user, page, statusFilter]);

  const loadReports = async () => {
    setLoading(true);
    try {
      const { data } = await adminApi.listReports({ page, page_size: 20, status: statusFilter });
      setItems(data.data || []);
      setTotal(data.total || 0);
    } catch {
      toast.error("Failed to load reports");
    } finally {
      setLoading(false);
    }
  };

  const handleAction = async (id: string, action: "resolve" | "dismiss") => {
    setActing(id);
    try {
      await adminApi.updateReport(id, action);
      setItems((prev) => prev.filter((r) => r.id !== id));
      toast.success(action === "resolve" ? "Report resolved" : "Report dismissed");
    } catch {
      toast.error("Failed to update report");
    } finally {
      setActing(null);
    }
  };

  if (isLoading || !user || user.role !== "admin") return null;

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="container mx-auto py-8 px-4">
        <h1 className="text-3xl font-bold mb-6">Content Reports</h1>

        {/* Status filter tabs */}
        <div className="flex gap-2 mb-4">
          {["pending", "resolved", "dismissed"].map((s) => (
            <Button
              key={s}
              variant={statusFilter === s ? "default" : "outline"}
              size="sm"
              onClick={() => { setStatusFilter(s); setPage(1); }}
            >
              {s.charAt(0).toUpperCase() + s.slice(1)}
            </Button>
          ))}
        </div>

        <p className="text-sm text-muted-foreground mb-4">{total} report{total !== 1 ? "s" : ""}</p>

        {loading ? (
          <div className="flex justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <>
            {/* Table header */}
            <div className="hidden md:grid grid-cols-[64px_1fr_140px_1fr_120px_160px] gap-4 px-4 py-2 text-xs font-medium text-muted-foreground uppercase tracking-wide border-b mb-1">
              <span>Preview</span>
              <span>Content</span>
              <span>Reporter</span>
              <span>Reason</span>
              <span>Status</span>
              <span className="text-right">Actions</span>
            </div>

            <div className="space-y-2">
              {items.map((report) => (
                <div
                  key={report.id}
                  className="rounded-lg border bg-card p-4 grid grid-cols-1 md:grid-cols-[64px_1fr_140px_1fr_120px_160px] gap-4 items-center"
                >
                  {/* Thumbnail */}
                  <Thumbnail url={report.thumbnail_url} title={report.media_title} />

                  {/* Content: title + ID */}
                  <div className="min-w-0">
                    <p className="font-medium text-sm truncate">
                      {report.media_title ?? <span className="text-muted-foreground italic">Untitled</span>}
                    </p>
                    <Link
                      href={`/i/${report.media_short_code}`}
                      target="_blank"
                      className="inline-flex items-center gap-1 text-xs font-mono text-primary hover:underline mt-0.5"
                    >
                      {report.media_short_code}
                      <ExternalLink className="h-3 w-3" />
                    </Link>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      ID: <span className="font-mono">{report.media_id?.slice(0, 8)}…</span>
                    </p>
                  </div>

                  {/* Reporter */}
                  <div className="text-sm">
                    <span className="font-medium">{report.reporter}</span>
                    <p className="text-xs text-muted-foreground">
                      {formatDistanceToNow(new Date(report.created_at), { addSuffix: true })}
                    </p>
                  </div>

                  {/* Reason */}
                  <p className="text-sm text-muted-foreground line-clamp-2">{report.reason}</p>

                  {/* Status */}
                  <div>
                    <StatusBadge status={report.status} />
                  </div>

                  {/* Actions */}
                  <div className="flex justify-end">
                    {report.status === "pending" ? (
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            size="icon"
                            variant="ghost"
                            disabled={acting === report.id}
                            className="h-8 w-8"
                          >
                            {acting === report.id ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <MoreHorizontal className="h-4 w-4" />
                            )}
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem
                            onClick={() => handleAction(report.id, "resolve")}
                            className="text-destructive focus:text-destructive"
                          >
                            <CheckCircle className="h-4 w-4 mr-2" />
                            Resolve
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            onClick={() => handleAction(report.id, "dismiss")}
                          >
                            <XCircle className="h-4 w-4 mr-2" />
                            Dismiss
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    ) : (
                      <span className="text-xs text-muted-foreground px-2">—</span>
                    )}
                  </div>
                </div>
              ))}

              {items.length === 0 && (
                <div className="text-center py-16 text-muted-foreground">
                  No {statusFilter} reports
                </div>
              )}
            </div>
          </>
        )}

        {/* Pagination */}
        <div className="flex items-center justify-between mt-6">
          <Button
            variant="outline"
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
          >
            Previous
          </Button>
          <span className="text-sm text-muted-foreground">Page {page}</span>
          <Button
            variant="outline"
            onClick={() => setPage((p) => p + 1)}
            disabled={items.length < 20}
          >
            Next
          </Button>
        </div>
      </main>
    </div>
  );
}
