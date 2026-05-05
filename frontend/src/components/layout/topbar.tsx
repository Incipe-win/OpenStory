"use client";

import { useAuthStore } from "@/lib/stores/auth-store";
import { formatCredits } from "@/lib/utils/format";
import { useCredits } from "@/lib/hooks/use-credits";
import { Zap } from "lucide-react";

interface TopbarProps {
  title?: string;
  actions?: React.ReactNode;
}

export function Topbar({ title, actions }: TopbarProps) {
  const { user } = useAuthStore();
  const { data: account } = useCredits();

  return (
    <header className="h-16 border-b border-border bg-card/50 backdrop-blur-sm flex items-center justify-between px-6 sticky top-0 z-30">
      <div>
        {title && (
          <h1 className="text-sm font-heading font-semibold uppercase tracking-wider text-foreground">
            {title}
          </h1>
        )}
      </div>

      <div className="flex items-center gap-4">
        {actions}

        {account && (
          <div className="flex items-center gap-2 px-3 py-1.5 bg-muted/50 border border-border cyber-chamfer-xs">
            <Zap className="h-3.5 w-3.5 text-accent" />
            <span className="text-sm font-mono text-accent">
              {formatCredits(account.balance)}
            </span>
          </div>
        )}

        <div className="flex items-center gap-2">
          <div className="w-8 h-8 bg-accent/20 border border-accent/30 cyber-chamfer-xs flex items-center justify-center">
            <span className="text-xs font-mono text-accent">
              {user?.username?.[0]?.toUpperCase() || "?"}
            </span>
          </div>
        </div>
      </div>
    </header>
  );
}
