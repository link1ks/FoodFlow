import { useData } from "./shared";
import { Button, Notice } from "../ui";

export type Capabilities = {
  vision_enabled: boolean;
  model_enabled: boolean;
  insights_enabled: boolean;
};
export function ModelNotice() {
  const capabilities = useData<Capabilities>("capabilities", "/capabilities");
  if (capabilities.isLoading)
    return <p className="mb-3 text-sm text-slate-500">正在读取生成能力…</p>;
  if (capabilities.error)
    return (
      <Notice tone="error">
        无法读取生成能力。
        <Button size="sm" onClick={() => capabilities.refetch()}>
          重试
        </Button>
      </Notice>
    );
  return capabilities.data?.model_enabled ? null : (
    <div className="mb-3">
      <Notice>
        演示模式：使用示例方案，不调用真实模型。确认前请核对菜单。
      </Notice>
    </div>
  );
}
