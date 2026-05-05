"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { cn } from "@/lib/utils/cn";
import { useAuthStore } from "@/lib/stores/auth-store";
import { useCredits } from "@/lib/hooks/use-credits";
import { formatCredits } from "@/lib/utils/format";
import {
  LayoutDashboard,
  FolderKanban,
  ListTodo,
  FolderOpen,
  Video,
  Shield,
  LogOut,
  Zap,
} from "lucide-react";

const navItems = [
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/tasks", label: "Tasks", icon: ListTodo },
  { href: "/assets", label: "Assets", icon: FolderOpen },
  { href: "/feed", label: "Feed", icon: Video },
];

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { user, logout } = useAuthStore();
  const { data: account } = useCredits();

  const handleLogout = () => {
    logout();
    router.push("/login");
  };

  return (
    <aside className="fixed left-0 top-0 bottom-0 w-56 bg-card border-r border-border flex flex-col z-40">
      {/* Logo */}
      <Link
        href="/projects"
        className="flex items-center gap-2 px-5 h-16 border-b border-border"
      >
        <Zap className="h-5 w-5 text-accent" />
        <span className="font-heading font-bold text-sm uppercase tracking-wider text-accent">
          OpenStory
        </span>
      </Link>

      {/* Navigation */}
      <nav className="flex-1 py-4 px-3 space-y-1">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive =
            pathname === item.href || pathname.startsWith(item.href + "/");
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 px-3 py-2.5 text-sm font-mono transition-all duration-150 cyber-chamfer-xs",
                isActive
                  ? "bg-accent/10 text-accent border border-accent/30"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50 border border-transparent"
              )}
            >
              <Icon
                className={cn(
                  "h-4 w-4",
                  isActive ? "text-accent" : "text-muted-foreground"
                )}
              />
              {item.label}
            </Link>
          );
        })}

        {user?.role === "admin" && (
          <Link
            href="/admin/moderation"
            className={cn(
              "flex items-center gap-3 px-3 py-2.5 text-sm font-mono transition-all duration-150 cyber-chamfer-xs",
              pathname.startsWith("/admin")
                ? "bg-accent-secondary/10 text-accent-secondary border border-accent-secondary/30"
                : "text-muted-foreground hover:text-foreground hover:bg-muted/50 border border-transparent"
            )}
          >
            <Shield className="h-4 w-4" />
            Moderation
          </Link>
        )}
      </nav>

      {/* Credits & User */}
      <div className="px-3 py-4 border-t border-border space-y-3">
        {account && (
          <div className="flex items-center gap-2 px-3 py-2 bg-muted/50 cyber-chamfer-xs">
            <Zap className="h-4 w-4 text-accent" />
            <span className="text-sm font-mono text-accent">
              {formatCredits(account.balance)}
            </span>
            <span className="text-xs text-muted-foreground">credits</span>
          </div>
        )}

        <div className="flex items-center gap-3 px-3">
          <div className="flex-1 min-w-0">
            <p className="text-sm font-mono text-foreground truncate">
              {user?.display_name || user?.username || "User"}
            </p>
            <p className="text-xs text-muted-foreground truncate">
              {user?.email}
            </p>
          </div>
          <button
            onClick={handleLogout}
            className="p-1.5 text-muted-foreground hover:text-destructive transition-colors cursor-pointer"
            title="Logout"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </div>
    </aside>
  );
}
