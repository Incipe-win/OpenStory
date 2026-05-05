"use client";

import { Dialog } from "@/components/ui/dialog";
import { CyberButton } from "@/components/ui/button";

interface ConfirmationDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  message: string;
  confirmLabel?: string;
  destructive?: boolean;
  loading?: boolean;
}

export function ConfirmationDialog({
  open,
  onClose,
  onConfirm,
  title,
  message,
  confirmLabel = "Confirm",
  destructive,
  loading,
}: ConfirmationDialogProps) {
  return (
    <Dialog open={open} onClose={onClose} title={title}>
      <p className="text-sm text-muted-foreground mb-6 font-mono">
        &gt; {message}
      </p>
      <div className="flex justify-end gap-3">
        <CyberButton variant="ghost" size="sm" onClick={onClose} disabled={loading}>
          Cancel
        </CyberButton>
        <CyberButton
          variant={destructive ? "secondary" : "glitch"}
          size="sm"
          onClick={onConfirm}
          loading={loading}
        >
          {confirmLabel}
        </CyberButton>
      </div>
    </Dialog>
  );
}
