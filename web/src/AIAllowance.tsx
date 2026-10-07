import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import { Card, Notice } from "./ui";
import type { paths } from "./generated/api";
type Allowance =
  paths["/households/{household}/ai-allowance"]["get"]["responses"][200]["content"]["application/json"];
export function AIAllowance({
  token,
  house,
}: {
  token: string;
  house: string;
}) {
  const q = useQuery({
    queryKey: ["ai-allowance", house, token],
    queryFn: () => api<Allowance>(`/households/${house}/ai-allowance`, token),
    enabled: !!house,
    refetchInterval: 15000,
  });
  const value = q.data;
  return (
    <Card className="mt-4">
      <h2 className="font-semibold">AI 调用额度</h2>
      {value && (
        <div className="mt-3 space-y-2 text-sm">
          <p>
            {value.enabled
              ? "收费调用额度保护已启用"
              : "收费调用未启用；演示规划不预扣额度"}
          </p>
          <p>
            本月家庭预扣 ¥{value.household_reserved} / ¥{value.monthly_limit}
          </p>
          <p>
            本月服务预扣 ¥{value.service_reserved} / ¥{value.monthly_limit}
            ，所有家庭共享此上限
          </p>
          <p>
            每次文字预扣 ¥{value.text_call_allowance}，图片预扣 ¥
            {value.image_call_allowance}
          </p>
          <p>
            今日调用 {value.daily_calls} / {value.daily_limit}{" "}
            次，同时进行或待核查 {value.active_or_uncertain} /{" "}
            {value.concurrent_limit} 个
          </p>
          <Notice>
            这是应用预扣额度，实际费用以服务商账单为准。失败或取消不会自动返还，待核查请求会阻止后续收费调用，需管理员确认。额度只在发送模型请求前扣除。
          </Notice>
        </div>
      )}
      {q.error && <Notice tone="error">{String(q.error)}</Notice>}
    </Card>
  );
}
