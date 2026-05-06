"use client";

import { useState } from "react";
import { FeedList } from "@/components/feed/feed-list";
import { PageContainer } from "@/components/layout/page-container";
import { Topbar } from "@/components/layout/topbar";
import { Pagination } from "@/components/shared/pagination";
import { LoadingState } from "@/components/shared/loading-state";
import { ErrorState } from "@/components/shared/error-state";
import { EmptyState } from "@/components/shared/empty-state";
import { useFeed } from "@/lib/hooks/use-feed";
import { Video } from "lucide-react";

export default function FeedPage() {
  const [page, setPage] = useState(1);
  const { data, isLoading, isError, refetch } = useFeed(page);

  return (
    <>
      <Topbar title="Feed" />
      <PageContainer>
        <div className="mb-6">
          <p className="text-xs font-mono text-muted-foreground">
            &gt; Published works from the OpenStory community
            <span className="inline-block w-2 h-4 bg-accent ml-1 animate-[blink_1s_step-end_infinite]" />
          </p>
        </div>

        {isLoading && <LoadingState message="Loading feed..." />}
        {isError && (
          <ErrorState
            onRetry={() => refetch()}
            message="Failed to load feed"
          />
        )}
        {data && data.data.length === 0 && (
          <EmptyState
            title="No published works yet"
            description="Publish a work to make it appear here"
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
      </PageContainer>
    </>
  );
}
