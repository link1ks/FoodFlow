import {useMemo,useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {Search,Landmark} from 'lucide-react'
import {api} from './api'
import {CatalogPicture,type CatalogItem} from './CatalogArt'
import {buildIngredientTrie,searchIngredients} from './ingredientTrie'
import {Button,Card,Field,Notice} from './ui'
import {priceCities,monitoringLabel,trendLabel,latestPrices,priceBadge,type Benchmark,type PriceDashboard} from './marketPrices'

export function LocalPrices({token,household}:{token:string;household:string}){
 const root='/households/'+household+'/prices'
 const [city,setCity]=useState('北京市'),[otherCity,setOtherCity]=useState(''),[customCity,setCustomCity]=useState(false)
 const [selected,setSelected]=useState('鸡蛋'),[search,setSearch]=useState(''),[category,setCategory]=useState('')
 const catalog=useQuery({queryKey:['ingredient-catalog'],queryFn:()=>api<CatalogItem[]>('/ingredient-catalog',token),staleTime:3600000})
 const benchmarks=useQuery({queryKey:['market-benchmarks',household,city],queryFn:()=>api<Benchmark[]>(root+'/benchmarks?'+new URLSearchParams({city}),token),refetchInterval:30000})
 const dashboard=useQuery({queryKey:['market-dashboard',household,city,selected],queryFn:()=>api<PriceDashboard>(root+'/dashboard?'+new URLSearchParams({city,ingredient_name:selected}),token),enabled:!!selected,refetchInterval:30000})
 const trie=useMemo(()=>buildIngredientTrie(catalog.data||[]),[catalog.data])
 const available=useMemo(()=>latestPrices(benchmarks.data||[]),[benchmarks.data])
 const matches=useMemo(()=>{
  const items=searchIngredients(catalog.data||[],trie,search).filter(x=>!category||x.category===category)
  return search?items:items.sort((a,b)=>Number(available.has(b.name))-Number(available.has(a.name)))
 },[catalog.data,trie,search,category,available])
 const categories=[...new Set(catalog.data?.map(x=>x.category)||[])]
 const official=<div className="space-y-3"><h3 className="flex items-center gap-2 font-semibold"><Landmark size={18}/>最近官方监测记录</h3>{dashboard.data?.benchmarks.length===0&&<p className="text-sm text-slate-500">该城市尚未收录此食材的官方监测数据。</p>}{dashboard.data?.benchmarks.slice().sort((a,b)=>b.recorded_date.localeCompare(a.recorded_date)).map(b=><div key={b.id} className="rounded-xl border border-emerald-100 bg-emerald-50/60 p-4"><div className="flex flex-wrap items-baseline justify-between gap-2"><strong className="text-3xl text-emerald-900">¥{b.price}<span className="text-sm font-normal"> / {b.unit}</span></strong><span className="rounded-full bg-white px-2 py-1 text-xs">{b.price_type==='retail_monitor'?'零售监测价':b.price_type==='wholesale_monitor'?'批发监测价':'政府指导价'}</span></div><p className="mt-2 text-sm">{b.source_item_name} · {b.specification}</p><p className="mt-2 text-xs text-slate-600">监测周期：{monitoringLabel(b)}{b.historical?' · 历史快照':''}</p><p className="mt-1 text-xs text-slate-600">发布自：{b.source_agency}</p><div className="mt-3 flex flex-wrap justify-between gap-2 text-xs"><span>{trendLabel(b.weekly_change_percent)}</span><a className="underline" href={b.source_url} target="_blank" rel="noopener noreferrer">查看原始发布 ↗</a></div></div>)}</div>
 return <>
  <header className="mb-5"><h1 className="text-2xl font-bold">查菜价</h1><p className="mt-1 text-sm text-slate-600">先选城市，再选食材，了解官方监测价格与近期变化。</p></header>
  <Card><label className="block text-sm font-medium">当前城市<select className="mt-2 w-full rounded-xl border border-slate-300 bg-white p-3" value={customCity?'other':city} onChange={e=>{if(e.target.value==='other'){setCustomCity(true);setOtherCity('')}else{setCustomCity(false);setCity(e.target.value)}}}>{priceCities.map(x=><option key={x}>{x}</option>)}<option value="other">其他城市…</option></select></label>{customCity&&<form className="mt-2 flex gap-2" onSubmit={e=>{e.preventDefault();if(otherCity.trim()){setCity(otherCity.trim())}}}><Field label="城市全名" required maxLength={32} value={otherCity} onChange={e=>setOtherCity(e.target.value)}/><Button className="self-end" disabled={!otherCity.trim()}>应用</Button></form>}<p className="mt-2 text-xs text-slate-500">正在查看 {city} · 无需定位权限。官方监测基准供参考，实际成交以门店为准。</p></Card>
  <Card className="mt-4"><div className="flex flex-wrap items-center justify-between gap-2"><h2 className="font-semibold">选择食材</h2><label className="text-xs">分类 <select aria-label="食材分类" className="rounded-lg border p-2" value={category} onChange={e=>setCategory(e.target.value)}><option value="">全部分类</option>{categories.map(x=><option key={x}>{x}</option>)}</select></label></div><div className="relative mt-3"><Search size={18} className="absolute left-3 top-3 text-slate-400"/><input aria-label="搜索食材" className="w-full rounded-xl border py-2.5 pl-10 pr-3" placeholder="搜索食材或别名，如西红柿" value={search} onChange={e=>setSearch(e.target.value)}/></div>
   {catalog.isLoading&&<p className="mt-3 text-sm">正在加载食材…</p>}{catalog.error&&<Notice tone="error">食材目录读取失败。<Button variant="ghost" onClick={()=>catalog.refetch()}>重试</Button></Notice>}
   <div className="mt-3 grid max-h-80 grid-cols-3 gap-2 overflow-y-auto sm:grid-cols-4 lg:grid-cols-6">{matches.map(item=><button key={item.id} aria-pressed={item.name===selected} onClick={()=>{setSelected(item.name)}} className={'overflow-hidden rounded-xl border text-left '+(selected===item.name?'border-emerald-600 ring-1 ring-emerald-600':'border-slate-200')}><CatalogPicture item={item} token={token} household={household} className="h-20 w-full object-cover"/><span className="block px-2 pt-1 text-sm font-medium">{item.name}</span><span className="block px-2 pb-2 text-xs text-slate-500">{benchmarks.isLoading?'查询中…':benchmarks.error?'价格读取失败':priceBadge(available.get(item.name))}</span></button>)}</div>{!catalog.isLoading&&matches.length===0&&<p className="mt-3 text-sm">没有匹配食材，请更换名称或分类。</p>}
   <p className="mt-3 text-xs text-slate-600">全部 {catalog.data?.length||0} 种食材 · 今日有价 {new Set((benchmarks.data||[]).filter(b=>!b.historical).map(b=>b.ingredient_name)).size} 种。无当天监测记录时不估价填充。</p>
   {benchmarks.data&&<p className="mt-2 text-xs text-slate-500">该城市已收录 {available.size} 种食材；日期相同为日监测价，起止日期不同为周期均价。不同规格分别展示。</p>}
   {benchmarks.error&&<p className="mt-2 text-xs text-rose-700">城市基准目录读取失败。<button className="underline" onClick={()=>benchmarks.refetch()}>重试</button></p>}
  </Card>
  <Card className="mt-4"><div className="mb-4 flex flex-wrap items-center justify-between gap-3"><div><h2 className="text-xl font-semibold">{selected} · 价格指标</h2><p className="mt-1 text-xs text-slate-500">{city} · 截至 {dashboard.data?.as_of||'加载中'}（北京时间）</p></div></div>
   {dashboard.isLoading&&<p className="py-4 text-sm">正在汇总价格…</p>}{dashboard.error&&<Notice tone="error">价格读取失败：{dashboard.error.message}<Button variant="ghost" onClick={()=>dashboard.refetch()}>重试</Button></Notice>}
   {dashboard.data&&<><div className="mb-4 rounded-xl bg-emerald-50 p-4"><b>今日官方价格</b>{dashboard.data.benchmarks.some(b=>!b.historical)?<p className="mt-1 text-sm">已收录当天官方监测数据，详见下方。</p>:<p className="mt-1 text-sm text-slate-600">今日暂无已核验价格。下方仅展示最近发布的记录，不代表今日现价。</p>}</div>{official}</>}
  </Card>

 </>
}
