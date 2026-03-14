"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import {
  Eye, Download, Trash2, Pencil, Copy, Plus, FileImage, Video, Loader2
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Navbar } from "@/components/layout/Navbar";
import { mediaApi } from "@/lib/api";
import { useAuthStore } from "@/lib/store";
import { formatBytes, formatNumber } from "@/lib/utils";
import type { Media } from "@/lib/types";
import { toast } from "sonner";
import Link from "next/link";

export default function DashboardPage() {
  const { user, isLoading } = useAuthStore();
  const router = useRouter();
  const [items, setItems] = useState<Media[]>([]);
  const [loading, setLoading] = useState(true);
  const [editItem, setEditItem] = useState<Media | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editTags, setEditTags] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(true);

  useEffect(() => {
    if (!isLoading && !user) router.push("/login");
  }, [user, isLoading, router]);

  useEffect(() => {
    if (!user) return;
    loadMedia();
  }, [user]);

  const loadMedia = async (pageNum = 1, append = false) => {
    setLoading(true);
    try {
      const { data } = await mediaApi.list({ page: pageNum, page_size: 20 });
      const newItems: Media[] = data.data || [];
      setItems((prev) => append ? [...prev, ...newItems] : newItems);
      setHasMore(newItems.length === 20);
    } catch {
      toast.error("Failed to load your files");
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this file? This cannot be undone.")) return;
    try {
      await mediaApi.delete(id);
      setItems((prev) => prev.filter((i) => i.id !== id));
      toast.success("File deleted");
    } catch {
      toast.error("Failed to delete file");
    }
  };

  const openEdit = (item: Media) => {
    setEditItem(item);
    setEditTitle(item.title || "");
    setEditDescription(item.description || "");
    setEditTags((item.tags || []).join(", "));
  };

  const handleSave = async () => {
    if (!editItem) return;
    setIsSaving(true);
    try {
      const { data } = await mediaApi.update(editItem.id, {
        title: editTitle || undefined,
        description: editDescription || undefined,
        tags: editTags ? editTags.split(",").map((t) => t.trim()).filter(Boolean) : [],
      });
      setItems((prev) => prev.map((i) => (i.id === editItem.id ? data.data : i)));
      setEditItem(null);
      toast.success("Updated!");
    } catch {
      toast.error("Failed to update");
    } finally {
      setIsSaving(false);
    }
  };

  if (isLoading) return null;
  if (!user) return null;

  const storagePercent = Math.round((user.storage_used / user.storage_quota) * 100);

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="container mx-auto py-8 px-4">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-3xl font-bold">My Files</h1>
            <p className="text-muted-foreground mt-1">
              {formatBytes(user.storage_used)} / {formatBytes(user.storage_quota)} used
              <span className="ml-2 text-xs">({storagePercent}%)</span>
            </p>
          </div>
          <Button asChild>
            <Link href="/upload">
              <Plus className="mr-2 h-4 w-4" />
              Upload
            </Link>
          </Button>
        </div>

        {/* Storage bar */}
        <div className="mb-6">
          <div className="h-2 rounded-full bg-secondary overflow-hidden">
            <div
              className="h-full bg-primary transition-all"
              style={{ width: `${Math.min(storagePercent, 100)}%` }}
            />
          </div>
        </div>

        {loading && items.length === 0 ? (
          <div className="flex justify-center py-20">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : items.length === 0 ? (
          <div className="text-center py-20">
            <FileImage className="mx-auto h-16 w-16 text-muted-foreground mb-4" />
            <h2 className="text-xl font-semibold mb-2">No files yet</h2>
            <p className="text-muted-foreground mb-4">Upload your first file to get started</p>
            <Button asChild>
              <Link href="/upload">Upload now</Link>
            </Button>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
            {items.map((item) => (
              <div key={item.id} className="group relative rounded-lg border bg-card overflow-hidden">
                {/* Thumbnail */}
                <Link href={`/i/${item.short_code}`}>
                  <div className="aspect-video bg-muted flex items-center justify-center overflow-hidden">
                    {item.thumbnail_url ? (
                      <Image
                        src={item.thumbnail_url}
                        alt={item.title || ""}
                        fill
                        className="object-cover"
                        sizes="(max-width: 768px) 100vw, 25vw"
                      />
                    ) : (
                      item.type === "video"
                        ? <Video className="h-10 w-10 text-muted-foreground" />
                        : <FileImage className="h-10 w-10 text-muted-foreground" />
                    )}
                  </div>
                </Link>

                {/* Status badge */}
                {item.status !== "ready" && (
                  <div className="absolute top-2 left-2">
                    <Badge variant={item.status === "failed" ? "destructive" : "secondary"}>
                      {item.status}
                    </Badge>
                  </div>
                )}

                <div className="p-3">
                  <p className="font-medium text-sm truncate">{item.title || item.short_code}</p>
                  <div className="flex items-center gap-2 text-xs text-muted-foreground mt-1">
                    <span className="flex items-center gap-0.5"><Eye className="h-3 w-3" />{formatNumber(item.view_count)}</span>
                    <span className="flex items-center gap-0.5"><Download className="h-3 w-3" />{formatNumber(item.download_count)}</span>
                    <span className="ml-auto">{formatBytes(item.file_size)}</span>
                  </div>
                </div>

                {/* Actions (show on hover) */}
                <div className="absolute inset-x-0 bottom-0 p-2 bg-gradient-to-t from-black/60 to-transparent opacity-0 group-hover:opacity-100 transition-opacity flex gap-1 justify-end">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 text-white hover:bg-white/20"
                    onClick={() => {
                      navigator.clipboard.writeText(`${window.location.origin}/i/${item.short_code}`);
                      toast.success("Link copied!");
                    }}
                  >
                    <Copy className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 text-white hover:bg-white/20"
                    onClick={() => openEdit(item)}
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 text-white hover:bg-red-500/60"
                    onClick={() => handleDelete(item.id)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}

        {hasMore && !loading && (
          <div className="mt-6 flex justify-center">
            <Button
              variant="outline"
              onClick={() => {
                const next = page + 1;
                setPage(next);
                loadMedia(next, true);
              }}
            >
              Load more
            </Button>
          </div>
        )}
      </main>

      {/* Edit dialog */}
      <Dialog open={!!editItem} onOpenChange={(open) => !open && setEditItem(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit file</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Title</Label>
              <Input value={editTitle} onChange={(e) => setEditTitle(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Description</Label>
              <textarea
                value={editDescription}
                onChange={(e) => setEditDescription(e.target.value)}
                className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              />
            </div>
            <div className="space-y-2">
              <Label>Tags (comma-separated)</Label>
              <Input value={editTags} onChange={(e) => setEditTags(e.target.value)} placeholder="tag1, tag2" />
            </div>
            <div className="flex gap-2">
              <Button onClick={handleSave} disabled={isSaving} className="flex-1">
                {isSaving ? "Saving..." : "Save"}
              </Button>
              <Button variant="outline" onClick={() => setEditItem(null)} className="flex-1">
                Cancel
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
