# 食材参考营养资料

数据源：[USDA FoodData Central SR Legacy](https://fdc.nal.usda.gov/download-datasets/)，April 2018 CSV 数据集。数据为每 100 克可食部；来源 food.csv 描述与 food_nutrient.csv 原数值保留在 profiles.json。FDC 单品 ID、链接、参考食品生熟状态与适用边界随 API 返回。

当前目录 91 种食材：79 种有明确参考食品，12 种标记 unavailable 并提供核对说明。粗粒度名称（肉、油、奶酪等）仅代表指定参考品种，不能视为用户实物检测结果。未核验对应品种或产品时不借用相近食品编造精确值。未提供营养素是未知，不是零。

重建步骤：从官方页面下载 SR Legacy CSV ZIP，在工作区解压，然后运行：

```powershell
node scripts/import-nutrition.cjs .cache/usda-sr/FoodData_Central_sr_legacy_food_csv_2018-04
go test ./internal/nutrition
```

映射和中文说明由 scripts/import-nutrition.cjs 显式维护；不要用模糊搜索自动替换品种。新增映射须核对原文描述、生熟、部位、加工程度及单位。API 内存读取嵌入式版本化资料，不联网检索，也不使用模型生成成分表。

不把个数、毫升自动折算成克，不按全部库存计算一餐摄入，不提供诊疗、减重处方或精确个体需求。模型根据资料给出一般烹饪搭配建议，菜单确认时再使用现有确定性代码计算原料缺口。

## 验证记录（2026-09-26）

- Go 营养映射和 Agent 结构化输出单元测试通过。
- PostgreSQL Testcontainers 建议任务集成测试通过：家庭隔离、幂等请求内容冲突、取消控制、库存不足拒绝、模型请求包含实际所选库存、生成建议不扣库存且不创建菜单。
- 全量 Go 回归仅发现 OpenAPI YAML 描述格式问题；修复后该测试单独重跑通过，`go vet ./...` 通过。
- TypeScript 检查、前端 6 项 Vitest 和 Vite 生产构建通过。
- 本地浏览器验证：展开生菜营养来源，选择已有生菜库存，生成演示建议，显示蒜香生菜候选和显式采用入口。API 与独立 Worker 使用迁移 17。
- 真实模型适配器使用本地 HTTP stub 验证；本机未配置模型凭证，尚未验证外部模型服务。
