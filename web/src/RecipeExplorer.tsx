import {useEffect,useState} from 'react'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {api} from './api'
import {useUI} from './store'
import {Button,Card,Field,Notice} from './ui'
import {NutritionAdvice} from './NutritionAdvice'

type SelectedItem={id:string;name:string;quantity:string;unit:string}
type Requirement={name:string;unit:string;needed:string;selected_available:string;shortage:string|null;unit_needs_confirmation:boolean}
type Option={recipe_id:string;title:string;minutes:number;tags:string[];status:string;matched_ingredients:number;missing_ingredients:number;requirements:Requirement[]}
type FridgePlan={recipes:{id:string;title:string}[];minutes:number;conflicts:{recipe_a:string;recipe_b:string;ingredient:string}[];batch_uses:{recipe_id:string;batch_id:string;ingredient:string;quantity:string}[];warnings:string[];truncated:boolean;inventory_recheck_required:boolean}
const labels:Record<string,string>={ready:'所选食材足够',one_missing:'还缺 1 种',two_missing:'还缺 2 种',more_missing:'还缺多种',unit_confirmation:'单位待确认'}

export function RecipeExplorer({token,household,items,selected,defaultServings,canEdit,onReadyCount}:{token:string;household:string;items:SelectedItem[];selected:string[];defaultServings:number;canEdit:boolean;onReadyCount?:(count:number)=>void}){
  const queryClient=useQueryClient()
  const [servings,setServings]=useState(String(defaultServings||2)),[maxMinutes,setMaxMinutes]=useState('60')
  const [day,setDay]=useState(new Date().toLocaleDateString('en-CA')),[meal,setMeal]=useState('dinner')
  const [busy,setBusy]=useState(''),[error,setError]=useState('')
  const [fridge,setFridge]=useState<FridgePlan|null>(null),[fridgeBusy,setFridgeBusy]=useState(false)
  useEffect(()=>setServings(String(defaultServings||2)),[defaultServings])
  const ids=[...selected].sort()
  const valid=ids.length>0&&Number(servings)>=1&&Number(servings)<=20&&Number(maxMinutes)>=1&&Number(maxMinutes)<=240
  const options=useQuery({queryKey:['recipe-options',household,ids,servings,maxMinutes],enabled:valid,queryFn:()=>api<Option[]>('/households/'+household+'/recipe-options',token,'POST',{selected_ingredient_ids:ids,servings:Number(servings),max_minutes:Number(maxMinutes)})})
  useEffect(()=>onReadyCount?.(valid?options.data?.filter(option=>option.status==='ready').length||0:0),[valid,options.data,onReadyCount])
  useEffect(()=>setFridge(null),[household,ids.join('|'),servings,maxMinutes])
  async function plan(recipeID:string){setBusy(recipeID);setError('');try{
    await api('/households/'+household+'/plans',token,'POST',{meals:[{day,meal,servings:Number(servings),recipe_id:recipeID}]})
    await queryClient.invalidateQueries()
    useUI.getState().setPage('week')
  }catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy('')}}
  async function planCombination(){if(!fridge?.recipes.length)return;setBusy('combination');setError('');try{
    await api('/households/'+household+'/plans',token,'POST',{meals:[{day,meal,servings:Number(servings),recipe_id:fridge.recipes[0].id,additional_recipe_ids:fridge.recipes.slice(1).map(r=>r.id)}]})
    await queryClient.invalidateQueries()
    useUI.getState().setPage('week')
  }catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy('')}}
  async function clearFridge(){setFridgeBusy(true);setError('');try{setFridge(await api<FridgePlan>('/households/'+household+'/clear-fridge',token,'POST',{selected_ingredient_ids:ids,servings:Number(servings),max_minutes:Number(maxMinutes)}))}catch(e){setError(String(e))}finally{setFridgeBusy(false)}}
  return <Card className="mt-5"><h2 className="text-lg font-semibold">用现有食材找菜谱</h2><p className="mt-1 text-sm text-slate-500">根据已选食材，推荐可做的菜。</p>
    <div className="mt-3 flex flex-wrap gap-2">{items.filter(item=>selected.includes(item.id)).map(item=><span key={item.id} className="rounded-full bg-emerald-100 px-3 py-1 text-xs text-emerald-900">{item.name} · {item.quantity} {item.unit}</span>)}{selected.length===0&&<span className="text-sm text-slate-500">还没有选中食材</span>}</div>
    <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4"><Field label="用餐日期" type="date" value={day} onChange={e=>setDay(e.target.value)}/><label className="text-sm">餐次<select className="mt-1 w-full rounded-xl border p-2.5" value={meal} onChange={e=>setMeal(e.target.value)}><option value="breakfast">早餐</option><option value="lunch">午餐</option><option value="dinner">晚餐</option></select></label><Field label="用餐人数" type="number" min="1" max="20" value={servings} onChange={e=>setServings(e.target.value)}/><Field label="最长烹饪时间（分钟）" type="number" min="1" max="240" value={maxMinutes} onChange={e=>setMaxMinutes(e.target.value)}/></div>
    <NutritionAdvice token={token} household={household} selected={selected} servings={Number(servings)} maxMinutes={Number(maxMinutes)} canEdit={canEdit} onChoose={plan}/>
    {valid&&<div className="mt-3"><Button variant="outline" disabled={fridgeBusy} onClick={clearFridge}>{fridgeBusy?'计算中…':'清冰箱 · 推荐多菜组合'}</Button></div>}
    {fridge&&<div className="mt-3 rounded-xl border border-emerald-200 bg-emerald-50 p-3 text-sm"><h3 className="font-semibold">无原料争抢的组合 · 共 {fridge.minutes} 分钟</h3>{fridge.recipes.length?<p className="mt-1">{fridge.recipes.map(r=>r.title).join(' ＋ ')}</p>:<p className="mt-1">当前有效库存无法在时间限制内组成可做菜谱。</p>}{fridge.batch_uses.length>0&&<div className="mt-2 text-xs text-slate-700">{fridge.batch_uses.map((x,i)=><div key={i}>{x.ingredient} · 计划使用 {x.quantity}（批次 {x.batch_id.slice(0,8)}）</div>)}</div>}{fridge.conflicts.length>0&&<p className="mt-2 text-xs text-amber-800">已避开 {fridge.conflicts.length} 组原料争抢，例如 {fridge.conflicts[0].recipe_a} / {fridge.conflicts[0].recipe_b} 共用 {fridge.conflicts[0].ingredient}。</p>}{fridge.warnings.map((w,i)=><p key={i} className="mt-1 text-xs text-amber-800">{w}</p>)}{fridge.truncated&&<p className="mt-1 text-xs text-amber-800">搜索达到预算，展示当前已验证无冲突的组合。</p>}{canEdit&&fridge.recipes.length>0&&<Button size="sm" className="mt-3" disabled={!!busy||!day} onClick={planCombination}>{busy==='combination'?'创建中…':'将组合存为菜单草案'}</Button>}<p className="mt-2 text-xs text-slate-500">此处仅为建议；确认菜单和做饭消耗时会重新读取库存。</p></div>}
    {!valid&&selected.length>0&&<div className="mt-3"><Notice tone="error">人数应为 1–20，烹饪时间应为 1–240 分钟。</Notice></div>}
    {options.isLoading&&<p className="mt-4 text-sm text-slate-500">正在计算菜谱依赖…</p>}{options.error&&<div className="mt-4"><Notice tone="error">{String(options.error)}</Notice><Button variant="outline" size="sm" className="mt-2" onClick={()=>options.refetch()}>重试</Button></div>}
    {valid&&options.data?.length===0&&<p className="mt-4 text-sm text-slate-500">当前所选食材没有匹配菜谱。试试其他食材或放宽烹饪时间。</p>}
    <div className="mt-4 grid gap-3 lg:grid-cols-2">{options.data?.map(option=><div key={option.recipe_id} className="rounded-xl border border-slate-200 bg-slate-50 p-3"><div className="flex items-start justify-between gap-2"><div><h3 className="font-semibold">{option.title}</h3><p className="text-xs text-slate-500">{option.minutes} 分钟 · 匹配 {option.matched_ingredients} 种已选食材</p></div><span className={'rounded-full px-2 py-1 text-xs '+(option.status==='ready'?'bg-emerald-100 text-emerald-800':option.status==='unit_confirmation'?'bg-amber-100 text-amber-800':'bg-sky-100 text-sky-800')}>{labels[option.status]||option.status}</span></div>
      <div className="mt-3 space-y-1">{option.requirements.map((need,index)=><div key={index} className="flex justify-between gap-2 text-xs"><span>{need.name} · 需 {need.needed} {need.unit}</span><span className={need.unit_needs_confirmation?'text-amber-700':need.shortage&&Number(need.shortage)>0?'text-slate-600':'text-emerald-700'}>{need.unit_needs_confirmation?'单位待确认':need.shortage&&Number(need.shortage)>0?'所选还缺 '+need.shortage+' '+need.unit:'已满足'}</span></div>)}</div>
      {canEdit&&<Button size="sm" className="mt-3" disabled={!!busy||!day} onClick={()=>plan(option.recipe_id)}>{busy===option.recipe_id?'创建中…':'创建菜单草案'}</Button>}</div>)}</div>
    {error&&<div className="mt-3"><Notice tone="error">{error}</Notice></div>}
    <p className="mt-3 text-xs text-slate-500">生成方案不扣库存，烹饪完成后再确认消耗。</p>
  </Card>
}
