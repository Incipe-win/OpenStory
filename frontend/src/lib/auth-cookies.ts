const ACCESS_TOKEN_COOKIE = "access_token";

export function getAccessTokenCookie(): string | null {
  if (typeof document === "undefined") return null;

  const value = document.cookie
    .split("; ")
    .find((row) => row.startsWith(`${ACCESS_TOKEN_COOKIE}=`))
    ?.split("=")[1];

  return value ? decodeURIComponent(value) : null;
}

export function setAccessTokenCookie(token: string, maxAgeSeconds: number) {
  if (typeof document === "undefined") return;

  const maxAge = Math.max(0, Math.floor(maxAgeSeconds));
  document.cookie = `${ACCESS_TOKEN_COOKIE}=${encodeURIComponent(token)}; path=/; max-age=${maxAge}; SameSite=Lax`;
}

export function clearAccessTokenCookie() {
  if (typeof document === "undefined") return;

  document.cookie = `${ACCESS_TOKEN_COOKIE}=; path=/; max-age=0; SameSite=Lax`;
}
