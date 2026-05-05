"use client";

import { useState } from "react";
import { useLogin } from "@/lib/hooks/use-auth";
import { CyberInput } from "@/components/ui/input";
import { CyberButton } from "@/components/ui/button";

export function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const login = useLogin();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    login.mutate({ email, password });
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      <CyberInput
        id="email"
        label="Email"
        prefix=">"
        type="email"
        placeholder="user@openstory.io"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
        autoComplete="email"
      />

      <CyberInput
        id="password"
        label="Password"
        prefix="$"
        type="password"
        placeholder="••••••••"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
        autoComplete="current-password"
      />

      {login.error && (
        <p className="text-xs font-mono text-destructive chromatic-aberration">
          // ERROR: {login.error instanceof Error ? login.error.message : "Login failed"}
        </p>
      )}

      <CyberButton
        type="submit"
        variant="glitch"
        size="lg"
        className="w-full"
        loading={login.isPending}
      >
        Authenticate
      </CyberButton>
    </form>
  );
}
