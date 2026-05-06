"use client";

import { createContext, useCallback, useContext, useMemo, useState } from "react";
import { CheckCircle, Info, X, XCircle } from "lucide-react";
import { cn } from "@/lib/utils/cn";

type ToastVariant = "success" | "error" | "info";

interface ToastInput {
  title: string;
  message?: string;
  variant?: ToastVariant;
  durationMs?: number;
}

interface ToastItem extends Required<Omit<ToastInput, "message">> {
  id: string;
  message?: string;
}

interface ToastContextValue {
  show: (toast: ToastInput) => void;
  success: (title: string, message?: string) => void;
  error: (title: string, message?: string) => void;
  info: (title: string, message?: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

const variantStyles: Record<ToastVariant, string> = {
  success: "border-accent/50 text-accent",
  error: "border-destructive/50 text-destructive",
  info: "border-accent-tertiary/50 text-accent-tertiary",
};

const variantIcons = {
  success: CheckCircle,
  error: XCircle,
  info: Info,
};

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const remove = useCallback((id: string) => {
    setToasts((items) => items.filter((item) => item.id !== id));
  }, []);

  const show = useCallback(
    ({ title, message, variant = "info", durationMs = 3200 }: ToastInput) => {
      const id = crypto.randomUUID();
      setToasts((items) => [...items, { id, title, message, variant, durationMs }]);
      window.setTimeout(() => remove(id), durationMs);
    },
    [remove]
  );

  const value = useMemo<ToastContextValue>(
    () => ({
      show,
      success: (title, message) => show({ title, message, variant: "success" }),
      error: (title, message) => show({ title, message, variant: "error" }),
      info: (title, message) => show({ title, message, variant: "info" }),
    }),
    [show]
  );

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="fixed right-4 top-4 z-[10000] flex w-[360px] max-w-[calc(100vw-32px)] flex-col gap-2 pointer-events-none">
        {toasts.map((toast) => {
          const Icon = variantIcons[toast.variant];
          return (
            <div
              key={toast.id}
              className={cn(
                "pointer-events-auto bg-card/95 border cyber-chamfer-sm p-3 shadow-lg backdrop-blur",
                variantStyles[toast.variant]
              )}
            >
              <div className="flex items-start gap-2">
                <Icon className="mt-0.5 h-4 w-4 shrink-0" />
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-mono uppercase tracking-wider text-foreground">
                    {toast.title}
                  </p>
                  {toast.message && (
                    <p className="mt-1 text-xs font-mono text-muted-foreground">
                      {toast.message}
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  aria-label="Dismiss notification"
                  onClick={() => remove(toast.id)}
                  className="text-muted-foreground transition-colors hover:text-foreground"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error("useToast must be used inside ToastProvider");
  }
  return context;
}
