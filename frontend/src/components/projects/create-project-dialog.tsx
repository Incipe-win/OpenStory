"use client";

import { useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { CyberInput } from "@/components/ui/input";
import { CyberButton } from "@/components/ui/button";
import { useCreateProject } from "@/lib/hooks/use-projects";

interface CreateProjectDialogProps {
  open: boolean;
  onClose: () => void;
}

export function CreateProjectDialog({ open, onClose }: CreateProjectDialogProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const createProject = useCreateProject();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createProject.mutate(
      { name, description: description || undefined },
      { onSuccess: () => { setName(""); setDescription(""); onClose(); } }
    );
  };

  return (
    <Dialog open={open} onClose={onClose} title="Create Project">
      <form onSubmit={handleSubmit} className="space-y-4">
        <CyberInput
          id="name"
          label="Project Name"
          placeholder="my-video-project"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
        <CyberInput
          id="description"
          label="Description"
          prefix=">"
          placeholder="A short description..."
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <div className="flex justify-end gap-3 pt-2">
          <CyberButton variant="ghost" type="button" onClick={onClose}>
            Cancel
          </CyberButton>
          <CyberButton variant="glitch" type="submit" loading={createProject.isPending} disabled={!name.trim()}>
            Create
          </CyberButton>
        </div>
      </form>
    </Dialog>
  );
}
