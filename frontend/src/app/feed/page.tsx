"use client";

import { useState } from "react";
import { FeedList } from "@/components/feed/feed-list";
import { Pagination } from "@/components/shared/pagination";
import { LoadingState } from "@/components/shared/loading-state";
import { ErrorState } from "@/components/shared/error-state";
import { EmptyState } from "@/components/shared/empty-state";
import { useFeed } from "@/lib/hooks/use-feed";
import { Video, Zap } from "lucide-react";
import Link from "next/link";

export default function FeedPage() {
  const [page, setPage] = useState(1);
  const { data, isLoading, isError, refetch } = useFeed(page);

  return (
    <div className="min-h-screen bg-background circuit-bg">
      {/* Header */}
      <header className="border-b border-border bg-card/50 backdrop-blur-sm sticky top-0 z-30">
        <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
          <Link href="/feed" className="flex items-center gap-2">
            <Zap className="h-5 w-5 text-accent" />
            <span className="font-heading font-bold text-sm uppercase tracking-wider text-accent">
              OpenStory
            </span>
          </Link>
          <Link
            href="/login"
            className="text-xs font-mono text-muted-foreground hover:text-accent transition-colors"
          >
            &gt; Sign In
          </Link>
        </div>
      </header>

      {/* Content */}
      <main className="max-w-7xl mx-auto px-6 py-10">
        <div className="mb-8">
          <h1 className="text-3xl font-heading font-black uppercase tracking-widest text-foreground mb-2 chromatic-aberration">
            Feed
          </h1>
          <p className="text-sm font-mono text-muted-foreground">
            &gt; Published works from the OpenStory community
            <span className="inline-block w-2 h-4 bg-accent ml-1 animate-[blink_1s_step-end_infinite]" />
          </p>
        </div>

        {isLoading && <LoadingState message="Loading feed..." />}
        {isError && <ErrorState onRetry={() => refetch()} />}
        {data && data.data.length === 0 && (
          <EmptyState
            title="No published works yet"
            description="Be the first to publish!"
            icon={<Video className="h-12 w-12" />}
          />
        )}
        {data && data.data.length > 0 && (
          <>
            <FeedList works={data.data} />
            <Pagination
              page={data.meta.page}
              pageSize={data.meta.page_size}
              total={data.meta.total}
              onPageChange={setPage}
              className="mt-8"
            />
          </>
        )}
      </main>
    </div>
  );
}
