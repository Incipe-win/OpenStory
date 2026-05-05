"use client";

import { use } from "react";
import { useQuery } from "@tanstack/react-query";
import { worksApi } from "@/lib/api/works";
import { WorkDetail } from "@/components/works/work-detail";
import { LoadingState } from "@/components/shared/loading-state";
import { ErrorState } from "@/components/shared/error-state";
import { ArrowLeft, Zap } from "lucide-react";
import Link from "next/link";

export default function WorkPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const { data: work, isLoading, isError, refetch } = useQuery({
    queryKey: ["works", id],
    queryFn: () => worksApi.get(id),
    enabled: !!id,
  });

  return (
    <div className="min-h-screen bg-background circuit-bg">
      {/* Header */}
      <header className="border-b border-border bg-card/50 backdrop-blur-sm sticky top-0 z-30">
        <div className="max-w-5xl mx-auto px-6 h-16 flex items-center justify-between">
          <Link href="/feed" className="flex items-center gap-2 text-muted-foreground hover:text-accent transition-colors">
            <ArrowLeft className="h-4 w-4" />
            <Zap className="h-4 w-4" />
            <span className="text-xs font-mono">Back to Feed</span>
          </Link>
        </div>
      </header>

      <main className="max-w-5xl mx-auto px-6 py-10">
        {isLoading && <LoadingState message="Loading work..." />}
        {isError && <ErrorState onRetry={() => refetch()} message="Failed to load work" />}
        {work && <WorkDetail work={work} />}
      </main>
    </div>
  );
}
