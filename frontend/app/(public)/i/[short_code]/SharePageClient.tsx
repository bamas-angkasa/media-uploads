"use client";
import { useEffect, useState } from "react";
import Image from "next/image";
import { Download, Copy, Flag, Eye, Calendar, FileImage, Video } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Navbar } from "@/components/layout/Navbar";
import { publicApi } from "@/lib/api";
import { formatBytes, formatNumber } from "@/lib/utils";
import { useAuthStore } from "@/lib/store";
import type { Media } from "@/lib/types";
import { toast } from "sonner";
import { formatDistanceToNow } from "date-fns";

const REPORT_REASONS = [
  "Inappropriate content",
  "NSFW / Adult content",
  "Spam",
  "Copyright violation",
  "Other",
];

export function SharePageClient({ media }: { media: Media }) {
  const { user } = useAuthStore();
  const [reportReason, setReportReason] = useState(REPORT_REASONS[0]);
  const [isReporting, setIsReporting] = useState(false);
  const shareUrl = typeof window !== "undefined"
    ? `${window.location.origin}/i/${media.short_code}`
    : `/i/${media.short_code}`;

  useEffect(() => {
    publicApi.recordView(media.short_code).catch(() => {});
  }, [media.short_code]);

  const handleDownload = async () => {
    try {
      const { data } = await publicApi.download(media.short_code);
      window.open(data.download_url, "_blank");
    } catch {
      toast.error("Failed to get download link");
    }
  };

  const handleCopyLink = () => {
    navigator.clipboard.writeText(shareUrl);
    toast.success("Link copied!");
  };

  const handleReport = async () => {
    if (!user) { toast.error("You must be logged in to report"); return; }
    setIsReporting(true);
    try {
      await publicApi.report(media.short_code, reportReason);
      toast.success("Report submitted. Thank you!");
    } catch {
      toast.error("Failed to submit report");
    } finally {
      setIsReporting(false);
    }
  };

  const isVideo = media.type === "video";
  const embedCode = isVideo
    ? `<video src="${shareUrl}" controls></video>`
    : `<img src="${media.thumbnail_url}" alt="${media.title || ""}" />`;

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="container mx-auto py-8 px-4 max-w-4xl">
        {/* Media preview */}
        <div className="rounded-xl overflow-hidden border bg-black mb-6">
          {isVideo ? (
            <video
              src={media.thumbnail_url || ""}
              controls
              className="w-full max-h-[600px] object-contain"
              poster={media.thumbnail_url}
            />
          ) : media.thumbnail_url ? (
            <div className="relative flex justify-center bg-black">
              <Image
                src={media.thumbnail_url}
                alt={media.title || "Media"}
                width={media.width || 1200}
                height={media.height || 800}
                className="max-h-[600px] w-auto object-contain"
                priority
              />
            </div>
          ) : (
            <div className="flex h-60 items-center justify-center text-muted-foreground">
              {isVideo ? <Video className="h-16 w-16" /> : <FileImage className="h-16 w-16" />}
            </div>
          )}
        </div>

        <div className="grid gap-6 md:grid-cols-3">
          {/* Main info */}
          <div className="md:col-span-2 space-y-4">
            <div>
              <h1 className="text-2xl font-bold">{media.title || media.short_code}</h1>
              {media.description && (
                <p className="text-muted-foreground mt-2">{media.description}</p>
              )}
            </div>

            {media.tags && media.tags.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {media.tags.map((tag) => (
                  <Badge key={tag} variant="secondary">{tag}</Badge>
                ))}
              </div>
            )}

            {/* Stats */}
            <div className="flex gap-4 text-sm text-muted-foreground">
              <span className="flex items-center gap-1">
                <Eye className="h-4 w-4" />
                {formatNumber(media.view_count)} views
              </span>
              <span className="flex items-center gap-1">
                <Download className="h-4 w-4" />
                {formatNumber(media.download_count)} downloads
              </span>
              <span className="flex items-center gap-1">
                <Calendar className="h-4 w-4" />
                {formatDistanceToNow(new Date(media.created_at), { addSuffix: true })}
              </span>
            </div>

            {/* Embed code */}
            <div className="space-y-2">
              <Label className="text-sm">Embed code</Label>
              <div className="flex gap-2">
                <Input readOnly value={embedCode} className="font-mono text-xs" />
                <Button variant="outline" size="sm" onClick={() => {
                  navigator.clipboard.writeText(embedCode);
                  toast.success("Embed code copied!");
                }}>
                  Copy
                </Button>
              </div>
            </div>
          </div>

          {/* Actions sidebar */}
          <div className="space-y-3">
            <Button onClick={handleDownload} className="w-full">
              <Download className="mr-2 h-4 w-4" />
              Download
            </Button>
            <Button variant="outline" onClick={handleCopyLink} className="w-full">
              <Copy className="mr-2 h-4 w-4" />
              Copy link
            </Button>

            {/* Share URL */}
            <div className="space-y-1">
              <Label className="text-xs text-muted-foreground">Share link</Label>
              <Input readOnly value={shareUrl} className="text-xs font-mono" />
            </div>

            {/* File info */}
            <div className="rounded-lg border p-3 space-y-1 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Size</span>
                <span>{formatBytes(media.file_size)}</span>
              </div>
              {media.width && media.height && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Dimensions</span>
                  <span>{media.width} × {media.height}</span>
                </div>
              )}
              <div className="flex justify-between">
                <span className="text-muted-foreground">Type</span>
                <Badge variant="outline" className="text-xs">{media.type.toUpperCase()}</Badge>
              </div>
            </div>

            {/* Report */}
            <Dialog>
              <DialogTrigger asChild>
                <Button variant="ghost" size="sm" className="w-full text-muted-foreground">
                  <Flag className="mr-2 h-4 w-4" />
                  Report
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Report content</DialogTitle>
                  <DialogDescription>Help us keep MediaShare safe</DialogDescription>
                </DialogHeader>
                <div className="space-y-3">
                  <div className="space-y-2">
                    {REPORT_REASONS.map((reason) => (
                      <label key={reason} className="flex items-center gap-2 cursor-pointer">
                        <input
                          type="radio"
                          name="reason"
                          value={reason}
                          checked={reportReason === reason}
                          onChange={() => setReportReason(reason)}
                        />
                        <span className="text-sm">{reason}</span>
                      </label>
                    ))}
                  </div>
                  <Button onClick={handleReport} disabled={isReporting} className="w-full">
                    {isReporting ? "Submitting..." : "Submit report"}
                  </Button>
                </div>
              </DialogContent>
            </Dialog>
          </div>
        </div>
      </main>
    </div>
  );
}
