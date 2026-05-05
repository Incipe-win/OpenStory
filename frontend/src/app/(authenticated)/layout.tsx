"use client";

import { useAuthGuard } from "@/lib/hooks/use-auth";
import { Sidebar } from "@/components/layout/sidebar";
import { LoadingState } from "@/components/shared/loading-state";

export default function AuthenticatedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const isAuthed = useAuthGuard();

  if (!isAuthed) {
    return <LoadingState message="Authenticating..." />;
  }

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <main className="flex-1 ml-56 overflow-auto">
        {children}
      </main>
    </div>
  );
}
