"use client";

import { useState } from "react";
import { PageContainer } from "@/components/layout/page-container";
import { Topbar } from "@/components/layout/topbar";
import { AssetCard } from "@/components/assets/asset-card";
import { AssetPreview } from "@/components/assets/asset-preview";
import { AssetTypeFilter } from "@/components/assets/asset-type-filter";
import { LoadingState } from "@/components/shared/loading-state";
import { EmptyState } from "@/components/shared/empty-state";
import { useProjects } from "@/lib/hooks/use-projects";
import { useProjectAssets, useSubmitAssetToFeed } from "@/lib/hooks/use-assets";
import type { Asset } from "@/lib/types/asset";
import { useToast } from "@/components/ui/toast";
import { apiErrorMessage } from "@/lib/api-errors";
import { FolderOpen } from "lucide-react";

export default function AssetsPage() {
  const [filter, setFilter] = useState("all");
  const [previewAsset, setPreviewAsset] = useState<Asset | null>(null);
  const { data: projectsData } = useProjects(1);
  const projectList = projectsData?.data || [];
  const submitAsset = useSubmitAssetToFeed();
  const toast = useToast();

  const handleSubmitToFeed = (asset: Asset) => {
    submitAsset.mutate(asset.id, {
      onSuccess: () => {
        toast.success("Asset submitted", "It is now pending moderation review.");
      },
      onError: (error) => {
        toast.error("Submit failed", apiErrorMessage(error, "Unable to submit asset."));
      },
    });
  };

  return (
    <>
      <Topbar title="Assets" />
      <PageContainer>
        <div className="flex items-center justify-between mb-6">
          <p className="text-xs font-mono text-muted-foreground">
            &gt; Media asset library
          </p>
          <AssetTypeFilter activeFilter={filter} onFilterChange={setFilter} />
        </div>

        {projectList.length === 0 ? (
          <EmptyState
            title="No assets"
            description="Create a project and upload assets to get started"
            icon={<FolderOpen className="h-12 w-12" />}
          />
        ) : (
          <div className="space-y-8">
            {projectList.map((project) => (
              <AssetProjectSection
                key={project.id}
                projectId={project.id}
                projectName={project.name}
                filter={filter}
                onPreview={setPreviewAsset}
                onSubmitToFeed={handleSubmitToFeed}
                submittingAssetId={submitAsset.variables}
                submitLoading={submitAsset.isPending}
              />
            ))}
          </div>
        )}

        <AssetPreview
          asset={previewAsset}
          open={!!previewAsset}
          onClose={() => setPreviewAsset(null)}
        />
      </PageContainer>
    </>
  );
}

function AssetProjectSection({
  projectId,
  projectName,
  filter,
  onPreview,
  onSubmitToFeed,
  submittingAssetId,
  submitLoading,
}: {
  projectId: string;
  projectName: string;
  filter: string;
  onPreview: (asset: Asset) => void;
  onSubmitToFeed: (asset: Asset) => void;
  submittingAssetId?: string;
  submitLoading: boolean;
}) {
  const { data, isLoading } = useProjectAssets(projectId, 1);

  if (isLoading) return <LoadingState message={`Loading ${projectName}...`} />;
  if (!data || data.data.length === 0) return null;

  const filtered =
    filter === "all"
      ? data.data
      : data.data.filter((a) => a.type.toLowerCase() === filter);
  if (filtered.length === 0) return null;

  return (
    <div>
      <h3 className="text-sm font-heading font-semibold uppercase tracking-wider text-foreground mb-3">
        {projectName}
      </h3>
      <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
        {filtered.map((asset) => (
          <AssetCard
            key={asset.id}
            asset={asset}
            onClick={() => onPreview(asset)}
            onSubmitToFeed={onSubmitToFeed}
            submitLoading={submitLoading && submittingAssetId === asset.id}
          />
        ))}
      </div>
    </div>
  );
}
