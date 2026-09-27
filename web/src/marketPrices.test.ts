import {expect,it} from 'vitest'
import {trendLabel,monitoringLabel,latestPrices,priceBadge,type Benchmark} from './marketPrices'
it('does not report missing historical comparisons as stable',()=>{
 expect(trendLabel(null)).toBe('暂无上周同口径数据')
 expect(trendLabel('0.00')).toBe('较上周平稳')
 expect(trendLabel('-5.00')).toContain('下降 5.00%')
})
it('uses latest date rather than source order for catalog badges',()=>{
 const rows=[{ingredient_name:'鸡蛋',recorded_date:'2026-09-25',price:'6.50'},{ingredient_name:'鸡蛋',recorded_date:'2026-06-30',price:'6.14'}] as Benchmark[]
 expect(latestPrices(rows).get('鸡蛋')?.price).toBe('6.50')
})
it('retains the monitoring period instead of labelling a month as a day',()=>{
 expect(monitoringLabel({period_start:'2026-06-01',recorded_date:'2026-06-30'} as Benchmark)).toBe('2026-06-01 至 2026-06-30')
})
it('labels monthly averages explicitly on ingredient cards',()=>{
 expect(priceBadge({historical:true,period_start:'2026-08-01',recorded_date:'2026-08-31',price:'5.00',unit:'500g'} as Benchmark)).toBe('2026-08 月均 ¥5.00/500g')
})
