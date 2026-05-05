import apiClient from "@/lib/api-client";
import type { ApiResponse } from "@/lib/types/api";
import type { User, TokenPair, LoginInput, RegisterInput } from "@/lib/types/auth";

export const authApi = {
  login: (input: LoginInput) =>
    apiClient.post<ApiResponse<TokenPair>>("/api/auth/login", input).then((r) => r.data.data),

  register: (input: RegisterInput) =>
    apiClient.post<ApiResponse<User>>("/api/auth/register", input).then((r) => r.data.data),

  refresh: (refreshToken: string) =>
    apiClient
      .post<ApiResponse<TokenPair>>("/api/auth/refresh", { refresh_token: refreshToken })
      .then((r) => r.data.data),

  getMe: () =>
    apiClient.get<ApiResponse<User>>("/api/me").then((r) => r.data.data),
};
