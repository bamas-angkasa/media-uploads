"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { ExternalLink, CheckCircle, XCircle, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Navbar } from "@/components/layout/Navbar";
import { adminApi } from "@/lib/api";
import { useAuthStore } from "@/lib/store";
import { toast } from "sonner";
import Link from "next/link";
import { formatDistanceToNow } from "date-fns";

export default function AdminReportsPage() {
  const { user, isLoading } = useAuthStore();
  const router = useRouter();
  const [items, setItems] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState("pending");

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
    try {
      await adminApi.updateReport(id, action);
      setItems((prev) => prev.filter((r) => r.id !== id));
      toast.success(action === "resolve" ? "Report resolved" : "Report dismissed");
    } catch {
      toast.error("Failed to update report");
    }
  };

  if (isLoading || !user || user.role !== "admin") return null;

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="container mx-auto py-8 px-4">
        <h1 className="text-3xl font-bold mb-6">Reports</h1>

        <div className="flex gap-2 mb-4">
          {["pending", "resolved", "dismissed"].map((s) => (
            <Button
              key={s}
              variant={statusFilter === s ? "default" : "outline"}
              size="sm"
              onClick={() => setStatusFilter(s)}
            >
              {s.charAt(0).toUpperCase() + s.slice(1)}
            </Button>
          ))}
        </div>

        <p className="text-sm text-muted-foreground mb-4">{total} reports</p>

        {loading ? (
          <div className="flex justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="space-y-3">
            {items.map((report) => (
              <div key={report.id} className="rounded-lg border p-4">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <Badge variant="outline">{report.reason}</Badge>
                      <Badge variant={
                        report.status === "pending" ? "secondary" :
                        report.status === "resolved" ? "default" : "outline"
                      }>
                        {report.status}
                      </Badge>
                    </div>
                    <p className="text-sm">
                      Reported by <strong>{report.reporter}</strong> ·{" "}
                      {formatDistanceToNow(new Date(report.created_at), { addSuffix: true })}
                    </p>
                    <p className="text-sm text-muted-foreground mt-1">
                      File:{" "}
                      <Link
                        href={`/i/${report.media_short_code}`}
                        target="_blank"
                        className="text-primary hover:underline font-mono"
                      >
                        {report.media_short_code}
                        <ExternalLink className="inline h-3 w-3 ml-1" />
                      </Link>
                      {report.media_title && ` — ${report.media_title}`}
                    </p>
                  </div>

                  {report.status === "pending" && (
                    <div className="flex gap-2 shrink-0">
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => handleAction(report.id, "resolve")}
                      >
                        <CheckCircle className="h-3.5 w-3.5 mr-1" />
                        Resolve
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleAction(report.id, "dismiss")}
                      >
                        <XCircle className="h-3.5 w-3.5 mr-1" />
                        Dismiss
                      </Button>
                    </div>
                  )}
                </div>
              </div>
            ))}
            {items.length === 0 && (
              <div className="text-center py-10 text-muted-foreground">
                No {statusFilter} reports
              </div>
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
