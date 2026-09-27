import {useEffect,useRef,useState} from 'react'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {api,idem} from './api'
import {Button,Field,Notice} from './ui'

type Step={index:number;dish:string;action:string;source:string;resources:number;dependencies:number[];duration:number;start:number;end:number}
type Progress={index:number;started_at:string;completed_at:string|null}
type Session={id:string;status:string;starts_at:string;target_at:string;schedule:{steps:Step[];duration:number;deadline_met:boolean};progress:Progress[]}
const devices=(mask:number)=>[[1,'双手'],[2,'案板'],[4,'主灶'],[8,'副灶']].filter(([bit])=>mask&Number(bit)).map(([,name])=>name).join(' · ')
const clock=(seconds:number)=>`${Math.floor(Math.max(0,seconds)/60)}:${String(Math.max(0,seconds)%60).padStart(2,'0')}`

export function KitchenPipeline({token,root,meal,consumed}:{token:string;root:string;meal:string;consumed:boolean}){
 const q=useQueryClient(),[open,setOpen]=useState(false),[focus,setFocus]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState('')
 const [minutes,setMinutes]=useState('45'),[second,setSecond]=useState(true),[now,setNow]=useState(Date.now())
 const pending=useRef<{identity:string;key:string;body:unknown}|null>(null)
 const path=root+'/meals/'+meal+'/pipeline'
 const data=useQuery({queryKey:['pipeline',path],queryFn:()=>api<{session:Session|null}>(path,token),enabled:open,refetchInterval:open?5000:false})
 const session=data.data?.session
 useEffect(()=>{if(!open)return;const t=setInterval(()=>setNow(Date.now()),1000);return()=>clearInterval(t)},[open])
 useEffect(()=>{if(!focus)return;const onKey=(e:KeyboardEvent)=>{if(e.key==='Escape')setFocus(false)};window.addEventListener('keydown',onKey);return()=>window.removeEventListener('keydown',onKey)},[focus])
 async function write(url:string,body:unknown,identity:string){
  setBusy(true);setError('')
  if(pending.current?.identity!==identity)pending.current={identity,key:idem(),body}
  try{await api(url,token,'POST',pending.current.body,pending.current.key);pending.current=null;await q.invalidateQueries({queryKey:['pipeline',path]})}
  catch(e){setError(String(e))}finally{setBusy(false)}
 }
 function act(index:number,action:string){if(session)void write(root+'/pipelines/'+session.id+'/steps/'+index,{action},session.id+index+action)}
 function create(){const n=Number(minutes);if(!Number.isFinite(n)||n<=0||n>1440){setError('请选择 1–1440 分钟后的出锅时间');return}void write(path,{target_at:new Date(Date.now()+n*60000).toISOString(),second_stove:second},`create:${minutes}:${second}`)}
 const completed=new Set(session?.progress.filter(p=>p.completed_at).map(p=>p.index))
 const active=session?.progress.filter(p=>!p.completed_at)||[]
 const terminal=consumed||session?.status!=='active'
 const orderedSteps=session?[...session.schedule.steps].sort((a,b)=>a.start-b.start||a.index-b.index):[]
 return <div className="mt-3 border-t border-emerald-100 pt-2">
  <button type="button" aria-expanded={open} className="text-sm font-semibold text-emerald-800" onClick={()=>setOpen(!open)}>{open?'收起备餐时间轴':'打开备餐时间轴'} {open?'⌃':'⌄'}</button>
  {open&&<div className={focus?'fixed inset-0 z-50 overflow-y-auto bg-stone-50 p-4 sm:p-10':'mt-3 space-y-3'}>
   {focus&&<div className="mb-6 flex justify-between"><h2 className="text-3xl font-bold">厨房专注模式</h2><Button variant="outline" onClick={()=>setFocus(false)}>退出大屏</Button></div>}
   {data.isLoading&&<p>正在读取备餐安排…</p>}
   {data.error&&<Notice tone="error">读取失败或网络断开，操作前请刷新。<Button size="sm" onClick={()=>data.refetch()}>重试</Button></Notice>}
   {error&&<Notice tone="error">{error}</Notice>}
   {data.isSuccess&&!session&&!consumed&&<div className="space-y-3 rounded-xl bg-white p-3">
    <Field label="计划多久后出锅（分钟）" type="number" min="1" max="1440" value={minutes} onChange={e=>setMinutes(e.target.value)}/>
    <label className="flex items-center gap-2"><input type="checkbox" checked={second} onChange={e=>setSecond(e.target.checked)}/>可使用副灶</label>
    <p className="text-xs text-slate-500">按一人操作、一个案板安排。耗时按示例份量估计，大份量可能更久，请按实际熟度确认完成。</p>
    <Button disabled={busy} onClick={create}>{busy?'安排中…':'生成备餐安排'}</Button>
   </div>}
   {data.isSuccess&&!session&&consumed&&<p>本餐已完成，没有备餐记录。</p>}
   {session&&<>
    <div className="flex flex-wrap items-center justify-between gap-2"><div><b>预计 {Math.ceil(session.schedule.duration/60)} 分钟</b><p className="text-xs text-slate-500">建议 {new Date(session.starts_at).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'})} 开始 · 已完成 {completed.size}/{session.schedule.steps.length} 步</p></div>{!focus&&<Button size="sm" variant="outline" onClick={()=>setFocus(true)}>专注大屏</Button>}</div>
    {!session.schedule.deadline_met&&<Notice>按当前设备安排无法赶上目标时间，已给出当前设备下的可行安排。</Notice>}
    {session.status==='finished'&&<Notice tone="success">工序已完成。请在本餐点击“烹饪完成”确认扣库。</Notice>}
    <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white p-3" aria-label="备餐甘特图">
     <div className="min-w-[380px]"><div className="ml-28 flex justify-between text-xs text-slate-500"><span>开始</span><span>{Math.ceil(session.schedule.duration/60)} 分钟</span></div>
      {orderedSteps.map((s,order)=><div key={s.index} className="my-2 flex items-center gap-2 text-xs"><span className="w-24 shrink-0 truncate" title={s.action}>{order+1}. {s.dish}</span><div className="relative h-5 flex-1 rounded bg-slate-100"><div title={s.action} className={'absolute h-5 min-w-1 rounded '+(completed.has(s.index)?'bg-emerald-700':'bg-emerald-300')} style={{left:`${s.start/session.schedule.duration*100}%`,width:`${s.duration/session.schedule.duration*100}%`}}/></div></div>)}
     </div>
    </div>
    <div className={focus?'mt-6 grid gap-4 lg:grid-cols-2':'space-y-2'}>{orderedSteps.map((s,order)=>{
     const progress=session.progress.find(p=>p.index===s.index),done=!!progress?.completed_at,running=!!progress&&!done
     const ready=s.dependencies.every(d=>completed.has(d))&&!active.some(p=>(session.schedule.steps[p.index].resources&s.resources)!==0)
     const remaining=progress?Math.min(s.duration,Math.ceil((new Date(progress.started_at).getTime()+s.duration*1000-now)/1000)):s.duration
     return <div key={s.index} className={'rounded-xl border p-3 '+(done?'border-slate-100 bg-slate-50 text-slate-500':running?'border-emerald-400 bg-emerald-50':'border-slate-200 bg-white')}>
      <div className="text-xs">{s.dish} · {devices(s.resources)} · 建议第 {Math.floor(s.start/60)} 分钟开始</div><div className={focus?'my-3 text-2xl font-semibold':'my-1 font-medium'}>{order+1}. {s.action}</div>
      <div className="flex items-center justify-between gap-2"><span className={focus?'text-3xl font-bold tabular-nums':'text-sm tabular-nums'}>{done?'已完成':running&&remaining<=0?'预计时间已到':clock(remaining)}</span>
       {!done&&!terminal&&<Button disabled={busy||!!data.error||(!running&&!ready)} size={focus?'lg':'sm'} onClick={()=>act(s.index,running?'complete':'start')}>{running?'确认完成':ready?'开始':'等待前置 / 设备'}</Button>}
      </div>
     </div>
    })}</div>
    <p className="mt-2 text-xs text-slate-500">时间仅供参考，倒计时结束不会自动打卡或扣库存。被动炖煮期间仍须留意火候。</p>
    {session.status==='active'&&<Button variant="ghost" disabled={busy} onClick={()=>act(0,'cancel')}>取消此次备餐安排</Button>}
   </>}
  </div>}
 </div>
}
