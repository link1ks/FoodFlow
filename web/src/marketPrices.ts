export const priceCities=['北京市','上海市','广州市','深圳市','成都市','天津市','重庆市','杭州市','南京市','武汉市','西安市','长沙市','郑州市','济南市','青岛市','苏州市','宁波市','大连市','厦门市','福州市','合肥市','昆明市','贵阳市','南宁市','海口市','沈阳市','长春市','哈尔滨市','石家庄市','太原市','南昌市','兰州市','西宁市','银川市','乌鲁木齐市','呼和浩特市','拉萨市']
export type Benchmark={id:string;ingredient_name:string;category:string;price:string;unit:string;source_agency:string;source_url:string;source_item_name:string;specification:string;price_type:string;recorded_date:string;period_start:string;historical:boolean;weekly_change_percent?:string|null}
export type PriceDashboard={benchmarks:Benchmark[];as_of:string}
export const monitoringLabel=(b:Benchmark)=>b.period_start===b.recorded_date?b.recorded_date:`${b.period_start} 至 ${b.recorded_date}`
export function trendLabel(value:string|null|undefined){
 if(value==null)return '暂无上周同口径数据'
 const n=Number(value)
 if(!Number.isFinite(n))return '暂无上周同口径数据'
 return n===0?'较上周平稳':`${n<0?'↓':'↑'} 较上周${n<0?'下降':'上涨'} ${Math.abs(n).toFixed(2)}%`
}

export function latestPrices(rows:Benchmark[]){
 const result=new Map<string,Benchmark>()
 for(const row of rows){const old=result.get(row.ingredient_name);if(!old||row.recorded_date>old.recorded_date)result.set(row.ingredient_name,row)}
 return result
}

export function priceBadge(row:Benchmark|undefined){
 if(!row)return '今日暂无数据'
 const date=row.period_start===row.recorded_date?row.recorded_date:row.period_start.slice(0,7)+' 月均'
 return `${row.historical?date:'今日'} ¥${row.price}/${row.unit}`
}
