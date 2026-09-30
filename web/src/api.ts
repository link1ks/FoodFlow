export async function api<T = unknown>(
  path: string,
  token: string,
  method = "GET",
  body?: unknown,
  key?: string,
): Promise<T> {
  key =
    key ??
    (method !== "GET" && method !== "HEAD" ? crypto.randomUUID() : undefined);
  const res = await fetch("/api" + path, {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: "Bearer " + token } : {}),
      ...(key ? { "Idempotency-Key": key } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  let json: unknown;
  try {
    json = await res.json();
  } catch {
    throw new Error(
      res.status >= 500
        ? "服务暂时无法连接，请稍后重试"
        : "服务返回异常，请刷新页面后重试",
    );
  }
  if (!res.ok) {
    const message =
      json && typeof json === "object" && "error" in json
        ? (json as { error: unknown }).error
        : null;
    throw new Error(typeof message === "string" ? message : "请求失败，请重试");
  }
  return json as T;
}
export const idem = () => crypto.randomUUID();
