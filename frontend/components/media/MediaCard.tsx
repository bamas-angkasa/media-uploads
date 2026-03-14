import Link from "next/link";
import Image from "next/image";
import { Eye, Download, Play, FileImage, Video } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { formatNumber, formatBytes } from "@/lib/utils";
import type { Media } from "@/lib/types";

interface MediaCardProps {
  media: Media;
  showUser?: boolean;
}

export function MediaCard({ media }: MediaCardProps) {
  const isVideo = media.type === "video";

  return (
    <Link href={`/i/${media.short_code}`} className="group block">
      <div className="overflow-hidden rounded-lg border bg-card transition-shadow hover:shadow-md">
        {/* Thumbnail */}
        <div className="relative aspect-video bg-muted overflow-hidden">
          {media.thumbnail_url ? (
            <Image
              src={media.thumbnail_url}
              alt={media.title || "Media"}
              fill
              className="object-cover transition-transform group-hover:scale-105"
              sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw"
            />
          ) : (
            <div className="flex h-full items-center justify-center text-muted-foreground">
              {isVideo ? <Video className="h-12 w-12" /> : <FileImage className="h-12 w-12" />}
            </div>
          )}
          {isVideo && (
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="rounded-full bg-black/50 p-3 backdrop-blur-sm">
                <Play className="h-6 w-6 text-white fill-white" />
              </div>
            </div>
          )}
          <div className="absolute top-2 right-2">
            <Badge variant="secondary" className="text-xs">
              {media.type.toUpperCase()}
            </Badge>
          </div>
        </div>

        {/* Info */}
        <div className="p-3">
          <h3 className="font-medium text-sm line-clamp-1 mb-1">
            {media.title || media.short_code}
          </h3>
          {media.tags && media.tags.length > 0 && (
            <div className="flex flex-wrap gap-1 mb-2">
              {media.tags.slice(0, 3).map((tag) => (
                <Badge key={tag} variant="outline" className="text-xs px-1.5 py-0">
                  {tag}
                </Badge>
              ))}
            </div>
          )}
          <div className="flex items-center gap-3 text-xs text-muted-foreground">
            <span className="flex items-center gap-1">
              <Eye className="h-3 w-3" />
              {formatNumber(media.view_count)}
            </span>
            <span className="flex items-center gap-1">
              <Download className="h-3 w-3" />
              {formatNumber(media.download_count)}
            </span>
            <span className="ml-auto">{formatBytes(media.file_size)}</span>
          </div>
        </div>
      </div>
    </Link>
  );
}
