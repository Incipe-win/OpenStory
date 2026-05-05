"use client";

import { FeedCard } from "@/components/feed/feed-card";
import type { Work } from "@/lib/types/work";

interface FeedListProps {
  works: Work[];
}

export function FeedList({ works }: FeedListProps) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      {works.map((work) => (
        <FeedCard key={work.id} work={work} />
      ))}
    </div>
  );
}
