import {useRef,useState} from 'react'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {api,idem} from './api'
import {Button,Card,Field,Notice} from './ui'
type Use={batch_id:string;name:string;unit:string;quantity:string;expiry_kind:string}
type Meal={day:string;meal:string;recipe_id:string;title:string;minutes:number;uses:Use[];rollover:Use[];at_risk:Use[]}
type Proposal={id:string;status:string;expires_at:string;request:{servings:number};result:{meals:Meal[];truncated:boolean}}
const label:Record<string,string>={breakfast:'早餐',lunch:'午餐',dinner:'晚餐'}
const shortQuantity=(s:string)=>s.replace(/\.0+$/,'').replace(/(\.\d*?)0+$/,'$1')
function StockLines({items}:{items:Use[]}){return <ul className="mt-1 flex flex-wrap gap-1">{items.map(i=><li key={i.batch_id} title={'批次 '+i.batch_id} className="rounded-md bg-white/80 px-2 py-1 text-xs">{i.name} {shortQuantity(i.quantity)} {i.unit}{i.expiry_kind==='estimate'?' · 有效期估计':''}</li>)}</ul>}
export function MultiMealPlanner({token,root}:{token:string;root:string}){
 const q=useQueryClient(),[open,setOpen]=useState(false),[count,setCount]=useState('5'),[servings,setServings]=useState('2'),[minutes,setMinutes]=useState('45')
 const [day,setDay]=useState(()=>{const d=new Date();d.setDate(d.getDate()+1);return d.toLocaleDateString('en-CA')}),[first,setFirst]=useState('lunch'),[busy,setBusy]=useState(false),[error,setError]=useState(''),[message,setMessage]=useState('')
 const pending=useRef<{identity:string;key:string}|null>(null)
 const proposals=useQuery({queryKey:['multi-meal',root],queryFn:()=>api<Proposal[]>(root+'/multi-meal',token),enabled:open})
 const household=useQuery({queryKey:['household',root],queryFn:()=>api<{role:string}>(root,token),enabled:open})
 const editable=household.data?.role==='owner'||household.data?.role==='editor'
 async function write(path:string,body:unknown){setBusy(true);setError('');setMessage('');const identity=path+JSON.stringify(body);if(pending.current?.identity!==identity)pending.current={identity,key:idem()}
  try{await api(path,token,'POST',body,pending.current.key);pending.current=null;await q.invalidateQueries();if((body as {action?:string}).action==='adopt')setMessage('已采纳到菜单；当天餐次会显示在「今天」。库存将在烹饪完成时扣减。')}
  catch(e){setError(String(e))}finally{setBusy(false)}
 }
 function generate(){const n=Number(count);if(!Number.isInteger(n)||n<1||n>7||!day){setError('请选择日期和 1–7 餐');return}
  const d=new Date(day+'T12:00:00');if(Number.isNaN(d.getTime())){setError('无效日期');return};let meal=first;const slots=[]
  for(let i=0;i<n;i++){slots.push({day:d.toLocaleDateString('en-CA'),meal});if(meal==='lunch')meal='dinner';else{meal='lunch';d.setDate(d.getDate()+1)}}
  void write(root+'/multi-meal',{slots,servings:Number(servings),max_minutes:Number(minutes)})
 }
 return <Card className="mb-5 border-emerald-200 bg-emerald-50/60"><button type="button" className="flex w-full items-center justify-between text-left font-semibold text-emerald-900" aria-expanded={open} onClick={()=>setOpen(!open)}><span>🌿 连续多餐 · 清冰箱</span><span>{open?'收起':'展开规划'}</span></button>
  {open&&<div className="mt-4 space-y-4"><p className="text-sm text-slate-600">用现有库存规划未来几餐，优先利用 48 小时内临期批次。每餐推荐一道菜，剩余批次继续结转；不是完整的营养配餐。</p>
   <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5"><Field label="开始日期" type="date" value={day} onChange={e=>setDay(e.target.value)}/><label className="text-sm font-medium">首餐<select className="mt-1 w-full rounded-xl border border-slate-300 bg-white p-2.5" value={first} onChange={e=>setFirst(e.target.value)}><option value="lunch">午餐</option><option value="dinner">晚餐</option></select></label><Field label="规划餐数" type="number" min="1" max="7" value={count} onChange={e=>setCount(e.target.value)}/><Field label="每餐人数" type="number" min="1" max="20" value={servings} onChange={e=>setServings(e.target.value)}/><Field label="每餐最多分钟" type="number" min="1" max="240" value={minutes} onChange={e=>setMinutes(e.target.value)}/></div>
   <Button disabled={busy||!editable} onClick={generate}>{busy?'处理中…':'生成连续餐次方案'}</Button>
   {household.data?.role==='viewer'&&<Notice>只读成员可以查看规划，采纳与生成需要编辑权限。</Notice>}
   {proposals.isLoading&&<p>读取最近规划…</p>}{proposals.error&&<Notice tone="error">加载失败 <Button size="sm" onClick={()=>proposals.refetch()}>重试</Button></Notice>}
   {error&&<Notice tone="error">{error}</Notice>}{message&&<Notice tone="success">{message}</Notice>}
   {proposals.data?.map((p,index)=><details key={p.id} open={index===0} className="rounded-xl border border-emerald-100 bg-white p-3"><summary className="cursor-pointer font-medium">{p.result.meals[0]?.day} 起 · {p.request.servings} 人 · {{pending:'待采纳',adopted:'已采纳',cancelled:'已取消'}[p.status]||p.status}</summary>
    <div className="mt-3 space-y-3">{p.result.truncated&&<Notice>已达到搜索预算，展示当前找到的可行方案，不保证全局最优。</Notice>}
     {p.result.meals.map((m,i)=><div key={m.day+m.meal} className="rounded-xl border-l-4 border-emerald-400 bg-stone-50 p-3"><div className="flex flex-wrap items-center justify-between gap-2"><b>{m.day} · {label[m.meal]}</b><span className="text-sm">{m.recipe_id?m.title+' · '+m.minutes+' 分钟':'暂无可做菜谱'}</span></div>
      {m.uses.length>0?<div className="mt-2 text-sm text-emerald-800">本餐消耗<StockLines items={m.uses}/></div>:<p className="mt-2 text-sm text-slate-500">当前库存、单位或时间约束下无法安排，不会自动补造食材。</p>}
      {m.at_risk?.length>0&&<div className="mt-2 text-sm text-amber-800">下餐前到期，无法结转<StockLines items={m.at_risk}/></div>}
      {m.rollover.length>0&&<details className="mt-2 text-sm text-slate-500"><summary className="cursor-pointer">{i===p.result.meals.length-1?'规划后剩余':'结转下餐'} · {m.rollover.length} 个批次</summary><StockLines items={m.rollover}/></details>}
     </div>)}
     {p.status==='pending'&&<><p className="text-xs text-slate-500">采纳将确认有菜谱的餐次；空餐次保留为空。方案 30 分钟内有效，库存未预留，采纳与做饭时都会重新校验。</p><div className="flex gap-2"><Button disabled={busy||!editable||!p.result.meals.some(m=>m.recipe_id)} onClick={()=>write(root+'/multi-meal/'+p.id,{action:'adopt'})}>确认并一键采纳</Button><Button variant="ghost" disabled={busy||!editable} onClick={()=>write(root+'/multi-meal/'+p.id,{action:'cancel'})}>取消方案</Button></div></>}
    </div>
   </details>)}
  </div>}
 </Card>
}
