"use client";

import { useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { CyberInput } from "@/components/ui/input";
import { CyberButton } from "@/components/ui/button";
import { useCreateWorkflow } from "@/lib/hooks/use-workflows";

interface CreateWorkflowDialogProps {
  open: boolean;
  onClose: () => void;
  projectId: string;
}

export function CreateWorkflowDialog({
  open,
  onClose,
  projectId,
}: CreateWorkflowDialogProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const create = useCreateWorkflow();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    create.mutate(
      { projectId, name, description: description || undefined },
      { onSuccess: () => { setName(""); setDescription(""); onClose(); } }
    );
  };

  return (
    <Dialog open={open} onClose={onClose} title="Create Workflow">
      <form onSubmit={handleSubmit} className="space-y-4">
        <CyberInput
          label="Workflow Name"
          placeholder="main-story-pipeline"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
        <CyberInput
          label="Description"
          placeholder="A brief description..."
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <div className="flex justify-end gap-3 pt-2">
          <CyberButton variant="ghost" type="button" onClick={onClose}>
            Cancel
          </CyberButton>
          <CyberButton
            variant="glitch"
            type="submit"
            loading={create.isPending}
            disabled={!name.trim()}
          >
            Create
          </CyberButton>
        </div>
      </form>
    </Dialog>
  );
}
