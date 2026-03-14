"use client";
import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/lib/store";
import { Navbar } from "@/components/layout/Navbar";
import { Uploader } from "@/components/upload/Uploader";

export default function UploadPage() {
  const { user, isLoading } = useAuthStore();
  const router = useRouter();

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login");
    }
  }, [user, isLoading, router]);

  if (isLoading) return null;
  if (!user) return null;

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="container mx-auto py-8 px-4">
        <div className="mb-8">
          <h1 className="text-3xl font-bold">Upload</h1>
          <p className="text-muted-foreground mt-1">Share photos, videos, and GIFs with anyone</p>
        </div>
        <Uploader />
      </main>
    </div>
  );
}
