import { Metadata } from "next";
import { notFound } from "next/navigation";
import Image from "next/image";
import { SharePageClient } from "./SharePageClient";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

async function getMedia(shortCode: string) {
  try {
    const res = await fetch(`${API_URL}/api/public/${shortCode}`, {
      next: { revalidate: 60 },
    });
    if (!res.ok) return null;
    const json = await res.json();
    return json.data;
  } catch {
    return null;
  }
}

export async function generateMetadata({
  params,
}: {
  params: { short_code: string };
}): Promise<Metadata> {
  const media = await getMedia(params.short_code);
  if (!media) return { title: "Not found" };

  const appUrl = process.env.NEXT_PUBLIC_APP_URL || "http://localhost:3000";

  return {
    title: media.title || `File ${media.short_code} — MediaShare`,
    description: media.description || "View and download this file on MediaShare",
    openGraph: {
      title: media.title || "MediaShare",
      description: media.description || "",
      url: `${appUrl}/i/${media.short_code}`,
      images: media.thumbnail_url ? [{ url: media.thumbnail_url }] : [],
      type: "website",
    },
    twitter: {
      card: "summary_large_image",
      title: media.title || "MediaShare",
      images: media.thumbnail_url ? [media.thumbnail_url] : [],
    },
  };
}

export default async function SharePage({
  params,
}: {
  params: { short_code: string };
}) {
  const media = await getMedia(params.short_code);
  if (!media) notFound();

  return <SharePageClient media={media} />;
}
