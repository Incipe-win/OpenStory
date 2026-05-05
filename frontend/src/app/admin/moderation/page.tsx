"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { adminApi, type ModerationRecord, type ReviewInput } from "@/lib/api/admin";
import { PageContainer } from "@/components/layout/page-container";
import { Topbar } from "@/components/layout/topbar";
import { CyberCard } from "@/components/ui/card";
import { CyberButton } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/badge";
import { Tabs } from "@/components/ui/tabs";
import { ConfirmationDialog } from "@/components/shared/confirmation-dialog";
import { LoadingState } from "@/components/shared/loading-state";
import { ErrorState } from "@/components/shared/error-state";
import { EmptyState } from "@/components/shared/empty-state";
import { formatDate } from "@/lib/utils/format";
import { Shield, Check, X, Eye } from "lucide-react";
import Link from "next/link";

export default function ModerationPage() {
  const [filter, setFilter] = useState("pending");
  const [reviewItem, setReviewItem] = useState<ModerationRecord | null>(null);
  const [reviewAction, setReviewAction] = useState<"approved" | "rejected" | null>(null);
  const queryClient = useQueryClient();

  const { data: items, isLoading, isError, refetch } = useQuery({
    queryKey: ["admin", "moderation", filter],
    queryFn: () => adminApi.listModeration(filter),
  });

  const reviewMutation = useMutation({
    mutationFn: ({ workId, input }: { workId: string; input: ReviewInput }) =>
      adminApi.reviewWork(workId, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "moderation"] });
      setReviewItem(null);
      setReviewAction(null);
    },
  });

  const handleReview = () => {
    if (!reviewItem || !reviewAction) return;
    reviewMutation.mutate({
      workId: reviewItem.target_id,
      input: { status: reviewAction, reason: "" },
    });
  };

  const tabs = [
    { id: "pending", label: "Pending" },
    { id: "approved", label: "Approved" },
    { id: "rejected", label: "Rejected" },
  ];

  return (
    <>
      <Topbar title="Moderation" />
      <PageContainer>
        <div className="flex items-center gap-3 mb-6">
          <Shield className="h-5 w-5 text-accent-secondary" />
          <h1 className="text-lg font-heading font-bold uppercase tracking-wide text-foreground">
            Content Moderation
          </h1>
        </div>

        <Tabs tabs={tabs} activeTab={filter} onTabChange={setFilter} className="mb-6" />

        {isLoading && <LoadingState />}
        {isError && <ErrorState onRetry={() => refetch()} />}
        {items && items.length === 0 && (
          <EmptyState
            title={`No ${filter} items`}
            description="All caught up on moderation"
            icon={<Shield className="h-12 w-12" />}
          />
        )}

        {items && items.length > 0 && (
          <div className="space-y-3">
            {items.map((item) => (
              <CyberCard key={item.id} className="p-5">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div>
                      <p className="text-sm font-mono text-foreground">
                        {item.target_type} / {item.target_id.slice(0, 12)}...
                      </p>
                      <p className="text-xs text-muted-foreground mt-1">
                        User: {item.user_id.slice(0, 12)}... / Provider: {item.provider}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {formatDate(item.created_at)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <StatusBadge status={item.status} />
                    <Link href={`/works/${item.target_id}`}>
                      <CyberButton variant="ghost" size="sm">
                        <Eye className="h-4 w-4 mr-1" />
                        View
                      </CyberButton>
                    </Link>
                    {item.status === "pending" && (
                      <>
                        <CyberButton
                          variant="glitch"
                          size="sm"
                          onClick={() => { setReviewItem(item); setReviewAction("approved"); }}
                        >
                          <Check className="h-4 w-4 mr-1" />
                          Approve
                        </CyberButton>
                        <CyberButton
                          variant="secondary"
                          size="sm"
                          onClick={() => { setReviewItem(item); setReviewAction("rejected"); }}
                        >
                          <X className="h-4 w-4 mr-1" />
                          Reject
                        </CyberButton>
                      </>
                    )}
                  </div>
                </div>
              </CyberCard>
            ))}
          </div>
        )}

        <ConfirmationDialog
          open={!!reviewItem && !!reviewAction}
          onClose={() => { setReviewItem(null); setReviewAction(null); }}
          onConfirm={handleReview}
          title={reviewAction === "approved" ? "Approve Work" : "Reject Work"}
          message={
            reviewAction === "approved"
              ? "This work will be published to the feed."
              : "This work will be rejected and hidden from the feed."
          }
          confirmLabel={reviewAction === "approved" ? "Approve" : "Reject"}
          destructive={reviewAction === "rejected"}
          loading={reviewMutation.isPending}
        />
      </PageContainer>
    </>
  );
}
