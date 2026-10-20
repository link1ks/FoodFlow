# 食材参考营养资料

数据源：[USDA FoodData Central SR Legacy](https://fdc.nal.usda.gov/download-datasets/)，April 2018 CSV 数据集。数据为每 100 克可食部；来源 food.csv 描述与 food_nutrient.csv 原数值保留在 profiles.json。FDC 单品 ID、链接、参考食品生熟状态与适用边界随 API 返回。

当前目录 200 种食材：198 种有参考值，泛指鱼和贝类保留未知。扩充资料含 TFDA 与香港 CFS，见 [来源说明](../../docs/NUTRITION_REFERENCES.md)。未提供的营养素是未知，不是零。

```powershell
python scripts/import-nutrition.py --sources .cache/nutrition-data --out .cache/nutrition-review-01
go test ./internal/nutrition
```

映射由 scripts/nutrition-base-mapping.json 与 scripts/nutrition-reference-mapping.json 显式维护，不用模糊搜索替换品种。输出先进入审核目录，不覆盖正式资料或历史迁移。新增映射须核对原文描述、生熟、部位、加工程度及单位。API 读取嵌入式资料，不联网检索，也不使用模型生成成分表。操作见 [Python 数据工具](../../docs/PYTHON_DATA_TOOLS.md)。

不把个数、毫升自动折算成克，不按全部库存计算一餐摄入，不提供诊疗、减重处方或精确个体需求。模型根据资料给出一般烹饪搭配建议，菜单确认时再使用现有确定性代码计算原料缺口。

## 验证记录（2026-09-26）

- Go 营养映射和 Agent 结构化输出单元测试通过。
- PostgreSQL Testcontainers 建议任务集成测试通过：家庭隔离、幂等请求内容冲突、取消控制、库存不足拒绝、模型请求包含实际所选库存、生成建议不扣库存且不创建菜单。
- 全量 Go 回归仅发现 OpenAPI YAML 描述格式问题；修复后该测试单独重跑通过，`go vet ./...` 通过。
- TypeScript 检查、前端 6 项 Vitest 和 Vite 生产构建通过。
- 本地浏览器验证：展开生菜营养来源，选择已有生菜库存，生成演示建议，显示蒜香生菜候选和显式采用入口。API 与独立 Worker 使用迁移 17。
- 真实模型适配器使用本地 HTTP stub 验证；本机未配置模型凭证，尚未验证外部模型服务。
