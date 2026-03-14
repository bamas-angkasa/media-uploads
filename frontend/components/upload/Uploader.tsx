"use client";
import { useState, useCallback } from "react";
import { useDropzone } from "react-dropzone";
import { Upload, X, CheckCircle, AlertCircle, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { uploadApi } from "@/lib/api";
import { cn, formatBytes } from "@/lib/utils";
import type { ProcessingStatus } from "@/lib/types";
import { toast } from "sonner";
import Link from "next/link";

const MAX_IMAGE = 10 * 1024 * 1024; // 10MB
const MAX_VIDEO = 500 * 1024 * 1024; // 500MB
const ALLOWED_TYPES = [
  "image/jpeg", "image/png", "image/webp", "image/gif",
  "video/mp4", "video/webm", "video/quicktime",
];

type Stage = "idle" | "uploading" | "confirming" | "processing" | "done" | "error";

interface UploadState {
  stage: Stage;
  uploadProgress: number;
  processingStatus: ProcessingStatus | null;
  shortCode: string | null;
  error: string | null;
}

export function Uploader() {
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState("");
  const [state, setState] = useState<UploadState>({
    stage: "idle",
    uploadProgress: 0,
    processingStatus: null,
    shortCode: null,
    error: null,
  });

  const onDrop = useCallback((accepted: File[]) => {
    if (accepted.length === 0) return;
    const f = accepted[0];

    if (!ALLOWED_TYPES.includes(f.type)) {
      toast.error("Unsupported file type");
      return;
    }
    const isVideo = f.type.startsWith("video/");
    const max = isVideo ? MAX_VIDEO : MAX_IMAGE;
    if (f.size > max) {
      toast.error(`File too large (max ${isVideo ? "500MB" : "10MB"})`);
      return;
    }
    setFile(f);
    setState({ stage: "idle", uploadProgress: 0, processingStatus: null, shortCode: null, error: null });
  }, []);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    multiple: false,
    disabled: state.stage !== "idle",
  });

  const handleUpload = async () => {
    if (!file) return;

    setState((s) => ({ ...s, stage: "uploading", uploadProgress: 0 }));

    try {
      // Step 1: Get presigned URL
      const { data: signData } = await uploadApi.sign({
        filename: file.name,
        content_type: file.type,
        size_bytes: file.size,
      });

      // Step 2: Upload directly to S3 via XHR (for progress events)
      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            setState((s) => ({ ...s, uploadProgress: Math.round((e.loaded / e.total) * 100) }));
          }
        };
        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) resolve();
          else reject(new Error(`S3 upload failed: ${xhr.status}`));
        };
        xhr.onerror = () => reject(new Error("Network error during upload"));
        xhr.open("PUT", signData.upload_url);
        xhr.setRequestHeader("Content-Type", file.type);
        xhr.send(file);
      });

      setState((s) => ({ ...s, stage: "confirming", uploadProgress: 100 }));

      // Step 3: Confirm upload
      await uploadApi.confirm(signData.media_id);

      setState((s) => ({ ...s, stage: "processing" }));

      // Step 4: Poll via SSE
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const es = new EventSource(`${apiUrl}/api/upload/progress/${signData.media_id}`);

      es.onmessage = (e) => {
        const status: ProcessingStatus = JSON.parse(e.data);
        setState((s) => ({ ...s, processingStatus: status }));
        if (status.status === "ready") {
          es.close();
          setState((s) => ({ ...s, stage: "done", shortCode: signData.short_code }));
          toast.success("Upload complete!");
        } else if (status.status === "failed") {
          es.close();
          setState((s) => ({ ...s, stage: "error", error: status.error || "Processing failed" }));
          toast.error("Processing failed");
        }
      };
      es.onerror = () => {
        es.close();
        // Fallback: assume processing is happening
      };
    } catch (err: any) {
      const msg = err?.response?.data?.error || err?.message || "Upload failed";
      setState((s) => ({ ...s, stage: "error", error: msg }));
      toast.error(msg);
    }
  };

  const reset = () => {
    setFile(null);
    setTitle("");
    setDescription("");
    setTags("");
    setState({ stage: "idle", uploadProgress: 0, processingStatus: null, shortCode: null, error: null });
  };

  return (
    <div className="space-y-6 max-w-2xl mx-auto">
      {/* Dropzone */}
      {!file && (
        <div
          {...getRootProps()}
          className={cn(
            "border-2 border-dashed rounded-xl p-12 text-center cursor-pointer transition-colors",
            isDragActive
              ? "border-primary bg-primary/5"
              : "border-muted-foreground/30 hover:border-primary/50 hover:bg-muted/30"
          )}
        >
          <input {...getInputProps()} />
          <Upload className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
          <p className="text-lg font-medium mb-1">
            {isDragActive ? "Drop it here!" : "Drag & drop or click to upload"}
          </p>
          <p className="text-sm text-muted-foreground">
            Photos up to 10MB · Videos up to 500MB
          </p>
          <p className="text-xs text-muted-foreground mt-1">
            JPG, PNG, WEBP, GIF, MP4, WEBM, MOV
          </p>
        </div>
      )}

      {/* File selected */}
      {file && state.stage === "idle" && (
        <div className="space-y-4">
          <div className="flex items-center gap-3 p-3 rounded-lg border bg-muted/30">
            <div className="flex-1 min-w-0">
              <p className="font-medium text-sm truncate">{file.name}</p>
              <p className="text-xs text-muted-foreground">{formatBytes(file.size)}</p>
            </div>
            <Button variant="ghost" size="icon" onClick={reset}>
              <X className="h-4 w-4" />
            </Button>
          </div>

          <div className="space-y-2">
            <Label htmlFor="title">Title (optional)</Label>
            <Input id="title" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Give your file a title" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="description">Description (optional)</Label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Describe your file..."
              className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="tags">Tags (optional, comma-separated)</Label>
            <Input id="tags" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="nature, photography, travel" />
          </div>

          <Button onClick={handleUpload} className="w-full" size="lg">
            <Upload className="mr-2 h-4 w-4" />
            Upload
          </Button>
        </div>
      )}

      {/* Uploading */}
      {state.stage === "uploading" && (
        <div className="space-y-3 p-4 border rounded-lg">
          <div className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span className="text-sm font-medium">Uploading...</span>
            <span className="ml-auto text-sm text-muted-foreground">{state.uploadProgress}%</span>
          </div>
          <Progress value={state.uploadProgress} />
        </div>
      )}

      {/* Processing */}
      {(state.stage === "confirming" || state.stage === "processing") && (
        <div className="space-y-3 p-4 border rounded-lg">
          <div className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span className="text-sm font-medium">Processing...</span>
            <span className="ml-auto text-sm text-muted-foreground">
              {state.processingStatus?.progress || 0}%
            </span>
          </div>
          <Progress value={state.processingStatus?.progress || 0} />
          <p className="text-xs text-muted-foreground">Generating thumbnails and optimizing your file</p>
        </div>
      )}

      {/* Done */}
      {state.stage === "done" && state.shortCode && (
        <div className="space-y-4 p-4 border rounded-lg bg-green-50 dark:bg-green-950/20">
          <div className="flex items-center gap-2 text-green-600 dark:text-green-400">
            <CheckCircle className="h-5 w-5" />
            <span className="font-medium">Upload complete!</span>
          </div>
          <div className="flex items-center gap-2">
            <Input
              readOnly
              value={`${typeof window !== "undefined" ? window.location.origin : ""}/i/${state.shortCode}`}
              className="font-mono text-sm"
            />
            <Button
              variant="outline"
              onClick={() => {
                navigator.clipboard.writeText(`${window.location.origin}/i/${state.shortCode}`);
                toast.success("Link copied!");
              }}
            >
              Copy
            </Button>
          </div>
          <div className="flex gap-2">
            <Button asChild className="flex-1">
              <Link href={`/i/${state.shortCode}`}>View</Link>
            </Button>
            <Button variant="outline" className="flex-1" onClick={reset}>
              Upload another
            </Button>
          </div>
        </div>
      )}

      {/* Error */}
      {state.stage === "error" && (
        <div className="space-y-3 p-4 border border-destructive rounded-lg bg-destructive/5">
          <div className="flex items-center gap-2 text-destructive">
            <AlertCircle className="h-5 w-5" />
            <span className="font-medium">Upload failed</span>
          </div>
          <p className="text-sm text-muted-foreground">{state.error}</p>
          <Button variant="outline" onClick={reset}>Try again</Button>
        </div>
      )}
    </div>
  );
}
