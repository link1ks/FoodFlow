import { afterEach, expect, it, vi } from "vitest";
import { api } from "./api";
afterEach(() => vi.unstubAllGlobals());
it("explains an HTML gateway failure instead of blaming JSON parsing", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        new Response("<html>Bad Gateway</html>", { status: 502 }),
      ),
  );
  await expect(
    api("/login", "", "POST", {
      account: "fixture@example.test",
      password: "fake",
    }),
  ).rejects.toThrow("服务暂时无法连接");
});
it("preserves the server credential validation message", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ error: "手机号、邮箱或密码错误" }), {
          status: 401,
        }),
      ),
  );
  await expect(api("/login", "", "POST", {})).rejects.toThrow(
    "手机号、邮箱或密码错误",
  );
});
it("returns a successful JSON response", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: "fixture" }))),
  );
  await expect(api("/me", "fixture-token")).resolves.toEqual({ id: "fixture" });
});
