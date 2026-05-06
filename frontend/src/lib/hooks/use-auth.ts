"use client";

import { useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { authApi } from "@/lib/api/auth";
import { useAuthStore } from "@/lib/stores/auth-store";
import { queryKeys } from "@/lib/hooks/query-keys";
import { getAccessTokenCookie, setAccessTokenCookie } from "@/lib/auth-cookies";
import type { LoginInput, RegisterInput, TokenPair } from "@/lib/types/auth";

function tokenMaxAgeSeconds(tokens: TokenPair) {
  if (tokens.expires_in) return tokens.expires_in;
  if (tokens.expires_at) return Math.max(0, tokens.expires_at - Date.now() / 1000);
  return 86400;
}

export function useLogin() {
  const { setTokens, setUser } = useAuthStore();
  const router = useRouter();

  return useMutation({
    mutationFn: (input: LoginInput) => authApi.login(input),
    onSuccess: async (tokens) => {
      setTokens(tokens.access_token, tokens.refresh_token);
      setAccessTokenCookie(tokens.access_token, tokenMaxAgeSeconds(tokens));
      try {
        const user = await authApi.getMe();
        setUser(user);
      } catch {
        // getMe failed but tokens are valid - proceed anyway
      }
      router.push("/projects");
    },
  });
}

export function useRegister() {
  const router = useRouter();

  return useMutation({
    mutationFn: (input: RegisterInput) => authApi.register(input),
    onSuccess: () => {
      router.push("/login");
    },
  });
}

export function useMe() {
  const { accessToken, hasHydrated } = useAuthStore();

  return useQuery({
    queryKey: queryKeys.auth.me,
    queryFn: authApi.getMe,
    enabled: hasHydrated && !!accessToken,
    staleTime: 5 * 60 * 1000,
  });
}

export function useAuthGuard() {
  const {
    accessToken,
    refreshToken,
    hasHydrated,
    setTokens,
    setUser,
    logout,
  } = useAuthStore();
  const router = useRouter();
  const me = useMe();

  useEffect(() => {
    if (!hasHydrated || accessToken) return;

    const cookieToken = getAccessTokenCookie();
    if (cookieToken) {
      setTokens(cookieToken, refreshToken || "");
      return;
    }

    router.replace("/login");
  }, [accessToken, hasHydrated, refreshToken, router, setTokens]);

  useEffect(() => {
    if (me.data) {
      setUser(me.data);
    }
  }, [me.data, setUser]);

  useEffect(() => {
    if (me.isError && accessToken) {
      logout();
      router.replace("/login");
    }
  }, [accessToken, logout, me.isError, router]);

  return hasHydrated && !!accessToken;
}
