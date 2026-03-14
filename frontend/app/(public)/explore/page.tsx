"use client";
import { useState, useEffect, useRef, useCallback } from "react";
import { Search, Filter } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { MediaCard } from "@/components/media/MediaCard";
import { Navbar } from "@/components/layout/Navbar";
import { publicApi } from "@/lib/api";
import type { Media } from "@/lib/types";
import { useDebounce } from "@/lib/hooks";

const TYPES = ["", "image", "video", "gif"] as const;
const TYPE_LABELS: Record<string, string> = { "": "All", image: "Photos", video: "Videos", gif: "GIFs" };

export default function ExplorePage() {
  const [items, setItems] = useState<Media[]>([]);
  const [cursor, setCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(true);
  const [loading, setLoading] = useState(false);
  const [query, setQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState<string>("");
  const [isSearching, setIsSearching] = useState(false);
  const [searchResults, setSearchResults] = useState<Media[]>([]);
  const debouncedQuery = useDebounce(query, 400);
  const loaderRef = useRef<HTMLDivElement | null>(null);

  const loadMore = useCallback(async (reset = false) => {
    if (loading || (!hasMore && !reset)) return;
    setLoading(true);
    try {
      const { data } = await publicApi.explore({
        cursor: reset ? undefined : cursor,
        page_size: 20,
        type: typeFilter || undefined,
      });
      const newItems: Media[] = data.data || [];
      setItems((prev) => reset ? newItems : [...prev, ...newItems]);
      setCursor(data.next_cursor);
      setHasMore(!!data.next_cursor);
    } catch {
      setHasMore(false);
    } finally {
      setLoading(false);
    }
  }, [cursor, hasMore, loading, typeFilter]);

  // Initial load + type filter change
  useEffect(() => {
    setItems([]);
    setCursor(undefined);
    setHasMore(true);
    loadMore(true);
  }, [typeFilter]);

  // Infinite scroll
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => { if (entries[0].isIntersecting && hasMore && !loading) loadMore(); },
      { threshold: 0.1 }
    );
    if (loaderRef.current) observer.observe(loaderRef.current);
    return () => observer.disconnect();
  }, [hasMore, loading, loadMore]);

  // Search
  useEffect(() => {
    if (!debouncedQuery.trim()) {
      setIsSearching(false);
      setSearchResults([]);
      return;
    }
    setIsSearching(true);
    publicApi.search({ q: debouncedQuery, page_size: 20 })
      .then(({ data }) => setSearchResults(data.data || []))
      .catch(() => setSearchResults([]))
      .finally(() => setIsSearching(false));
  }, [debouncedQuery]);

  const displayItems = query.trim() ? searchResults : items;

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="container mx-auto py-8 px-4">
        {/* Header */}
        <div className="mb-6">
          <h1 className="text-3xl font-bold mb-4">Explore</h1>

          {/* Search */}
          <div className="relative mb-4 max-w-md">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search photos and videos..."
              className="pl-9"
            />
          </div>

          {/* Filters */}
          <div className="flex gap-2">
            {TYPES.map((t) => (
              <Button
                key={t}
                variant={typeFilter === t ? "default" : "outline"}
                size="sm"
                onClick={() => setTypeFilter(t)}
              >
                {TYPE_LABELS[t]}
              </Button>
            ))}
          </div>
        </div>

        {/* Grid */}
        {displayItems.length === 0 && !loading ? (
          <div className="text-center py-20 text-muted-foreground">
            {query.trim() ? "No results found" : "No media yet. Be the first to upload!"}
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
            {displayItems.map((item) => (
              <MediaCard key={item.id} media={item} />
            ))}
          </div>
        )}

        {/* Infinite scroll trigger */}
        {!query.trim() && (
          <div ref={loaderRef} className="mt-8 flex justify-center">
            {loading && <p className="text-muted-foreground text-sm">Loading...</p>}
            {!hasMore && items.length > 0 && (
              <p className="text-muted-foreground text-sm">You&apos;ve seen everything!</p>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
