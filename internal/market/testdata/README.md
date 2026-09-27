# 官方价格解析样本

仅保留原文表格事实、表头及必要合并单元格，移除了样式和站点导航。

- national.html：https://www.chinaprice.cn/jsdzqk/61480.jhtml ，报告 2026.9-2，2026-09-22 发布，附表 1—6（7 张表），实际监测周期 2026-08-01 至 2026-08-31。福州多数品种为空，不填补或复制其他城市值。
- suzhou.html：https://fg.suzhou.gov.cn/szfgw/scdt/202609/36d54f601db745c9b72de698a9e7deac.shtml ，2026-09-26 苏州市部分农贸市场零售均价。
- fuzhou.html：https://fgw.fuzhou.gov.cn/fgwzwgk/fzgggz/jgysf/msspjgxq/zfsp/202609/t20260926_5376215.htm ，2026-09-26 福州市主副食品集超均价。

这些是固定历史测试样本，生产同步不读取测试文件。生产每次从公开索引发现实际发布的报告 URL，并保留报告自身日期。规格、单位和城市行结构不符合预期时拒绝该源写入。未匹配目录的细分鱼种等项目跳过。
