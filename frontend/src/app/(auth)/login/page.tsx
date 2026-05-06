"use client";

import { LoginForm } from "@/components/auth/login-form";
import { Zap } from "lucide-react";

export default function LoginPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background circuit-bg p-4">
      <div className="w-full max-w-md">
        {/* Header */}
        <div className="text-center mb-10">
          <div className="flex items-center justify-center gap-2 mb-4">
            <Zap className="h-8 w-8 text-accent" />
            <h1 className="text-2xl font-heading font-black uppercase tracking-widest text-accent chromatic-aberration">
              OpenStory
            </h1>
          </div>
          <p className="text-sm font-mono text-muted-foreground">
            &gt; AI-Powered Video Creation Platform
            <span className="inline-block w-2 h-4 bg-accent ml-1 animate-[blink_1s_step-end_infinite]" />
          </p>
        </div>

        {/* Login card */}
        <div className="bg-card border border-border cyber-chamfer p-8 shadow-[var(--shadow-neon)]">
          <h2 className="text-sm font-mono uppercase tracking-[0.2em] text-muted-foreground mb-6">
            {"// Authentication Required"}
          </h2>
          <LoginForm />
        </div>

        {/* Footer */}
        <p className="text-center mt-8 text-xs font-mono text-muted-foreground/50">
          Secure connection via JWT // v1.0.0
        </p>
      </div>
    </div>
  );
}
