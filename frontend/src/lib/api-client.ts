import axios, { AxiosError } from "axios";
import { API_BASE_URL } from "@/lib/config";
import { useAuthStore } from "@/lib/stores/auth-store";
import { setAccessTokenCookie } from "@/lib/auth-cookies";
import type { ApiResponse, ApiError } from "@/lib/types/api";
import type { TokenPair } from "@/lib/types/auth";

const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: { "Content-Type": "application/json" },
  timeout: 15_000,
  timeoutErrorMessage: "API request timed out",
});

// Request interceptor: attach Bearer token
apiClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor: handle 401, attempt refresh
let refreshPromise: Promise<TokenPair> | null = null;

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError<ApiError>) => {
    const originalRequest = error.config as AxiosError["config"] & {
      _retry?: boolean;
    };
    if (!originalRequest) return Promise.reject(error);

    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;
      const refreshToken = useAuthStore.getState().refreshToken;

      if (!refreshToken) {
        useAuthStore.getState().logout();
        if (typeof window !== "undefined") {
          window.location.href = "/login";
        }
        return Promise.reject(error);
      }

      try {
        // Deduplicate concurrent refresh attempts
        if (!refreshPromise) {
          refreshPromise = authRefresh(refreshToken).finally(() => {
            refreshPromise = null;
          });
        }
        const tokens = await refreshPromise;
        useAuthStore.getState().setTokens(
          tokens.access_token,
          tokens.refresh_token
        );
        setAccessTokenCookie(tokens.access_token, tokenMaxAgeSeconds(tokens));
        originalRequest.headers = originalRequest.headers ?? {};
        originalRequest.headers.Authorization = `Bearer ${tokens.access_token}`;
        return apiClient(originalRequest);
      } catch {
        useAuthStore.getState().logout();
        if (typeof window !== "undefined") {
          window.location.href = "/login";
        }
        return Promise.reject(error);
      }
    }

    return Promise.reject(error);
  }
);

async function authRefresh(
  refreshToken: string
): Promise<TokenPair> {
  const res = await axios.post<ApiResponse<TokenPair>>(
    `${API_BASE_URL}/api/auth/refresh`,
    { refresh_token: refreshToken }
  );
  return res.data.data;
}

function tokenMaxAgeSeconds(tokens: TokenPair) {
  if (tokens.expires_in) return tokens.expires_in;
  if (tokens.expires_at) return Math.max(0, tokens.expires_at - Date.now() / 1000);
  return 86400;
}

export default apiClient;
