"use client";

import { useAuthGuard } from "@/lib/hooks/use-auth";
import { useAuthStore } from "@/lib/stores/auth-store";
import { Sidebar } from "@/components/layout/sidebar";
import { LoadingState } from "@/components/shared/loading-state";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const isAuthed = useAuthGuard();
  const { user } = useAuthStore();
  const router = useRouter();

  useEffect(() => {
    if (user && user.role !== "admin") {
      router.push("/projects");
    }
  }, [user, router]);

  if (!isAuthed || !user) {
    return <LoadingState message="Authenticating..." />;
  }

  if (user.role !== "admin") {
    return null;
  }

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <main className="flex-1 ml-56 overflow-auto">{children}</main>
    </div>
  );
}
