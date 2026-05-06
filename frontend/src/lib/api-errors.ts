import { isAxiosError } from "axios";
import type { ApiError } from "@/lib/types/api";

export function apiErrorMessage(error: unknown, fallback: string) {
  if (isAxiosError<ApiError>(error)) {
    return error.response?.data?.error?.message || error.message || fallback;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return fallback;
}
