"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { authApi } from "@/lib/api/auth";
import { useAuthStore } from "@/lib/stores/auth-store";
import { queryKeys } from "@/lib/hooks/query-keys";
import type { LoginInput, RegisterInput } from "@/lib/types/auth";

export function useLogin() {
  const { setTokens, setUser } = useAuthStore();
  const router = useRouter();

  return useMutation({
    mutationFn: (input: LoginInput) => authApi.login(input),
    onSuccess: async (tokens) => {
      setTokens(tokens.access_token, tokens.refresh_token);
      document.cookie = `access_token=${tokens.access_token}; path=/; max-age=${tokens.expires_in}`;
      const user = await authApi.getMe();
      setUser(user);
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
  const { accessToken } = useAuthStore();

  return useQuery({
    queryKey: queryKeys.auth.me,
    queryFn: authApi.getMe,
    enabled: !!accessToken,
    staleTime: 5 * 60 * 1000,
  });
}

export function useAuthGuard() {
  const { accessToken, logout } = useAuthStore();

  if (!accessToken) {
    if (typeof window !== "undefined") {
      window.location.href = "/login";
    }
    return false;
  }
  return true;
}
