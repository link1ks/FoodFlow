import {useUI} from './store'
import {Trash2} from 'lucide-react'
import {useState} from 'react'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {api,idem} from './api'
import {Button,Card,Field,Notice} from './ui'

type Shop={id:string;name:string;unit:string;needed:string;bought:string;checked:boolean;stocked:boolean;origin:string}
type Catalog={name:string;category:string}

function recommendation(name:string,catalog:Catalog[]){
  const category=catalog.find(x=>x.name===name)?.category||''
  const location=['肉禽','水产'].includes(category)?'冷冻':['蔬菜','水果','蛋奶','豆制品','菌菇'].includes(category)?'冷藏':'常温'
  const days=['肉禽','水产'].includes(category)?14:['蔬菜','水果','豆制品','菌菇'].includes(category)?3:category==='蛋奶'?5:30
  const date=new Date();date.setDate(date.getDate()+days)
  return {location,expires_on:date.toLocaleDateString('en-CA')}
}

export function ShoppingBoard({token,household,canEdit}:{token:string;household:string;canEdit:boolean}){
  const q=useQueryClient(),root='/households/'+household
  const items=useQuery({queryKey:['shopping',root],queryFn:()=>api<Shop[]>(root+'/shopping',token)})
  const catalog=useQuery({queryKey:['ingredient-catalog'],queryFn:()=>api<Catalog[]>('/ingredient-catalog',token)})
  const [review,setReview]=useState<Shop|null>(null),[bought,setBought]=useState(''),[location,setLocation]=useState(''),[expiry,setExpiry]=useState(''),[expiryKind,setExpiryKind]=useState('estimate'),[error,setError]=useState(''),[busy,setBusy]=useState(false)
  const [deleting,setDeleting]=useState<Shop|null>(null)
  async function remove(){if(!deleting)return;setBusy(true);setError('');try{await api(root+'/shopping/'+deleting.id,token,'DELETE');setDeleting(null);await q.invalidateQueries()}catch(e){setError(String(e))}finally{setBusy(false)}}
  function open(item:Shop){const hint=recommendation(item.name,catalog.data||[]);setReview(item);setBought(item.bought==='0.000'?item.needed:item.bought);setLocation(hint.location);setExpiry(hint.expires_on);setExpiryKind('estimate');setError('')}
  async function uncheck(item:Shop){setError('');try{await api(root+'/shopping/'+item.id,token,'PATCH',{checked:false,bought:item.bought});await q.invalidateQueries()}catch(e){setError(String(e))}}
  async function confirm(){if(!review)return;setBusy(true);setError('');try{
    if(!review.checked||review.bought!==bought)await api(root+'/shopping/'+review.id,token,'PATCH',{checked:true,bought})
    await api(root+'/shopping/'+review.id+'/stock',token,'POST',{location,bought_on:new Date().toLocaleDateString('en-CA'),expires_on:expiry,expiry_kind:expiry?expiryKind:'unknown',source:'共享采购确认入库'},idem())
    setReview(null);await q.invalidateQueries()
  }catch(e){setError(String(e));await q.invalidateQueries()}finally{setBusy(false)}}
  return <><div className="mb-5"><h1 className="text-2xl font-bold">共享采购清单</h1><p className="text-sm text-slate-500">勾选后核对实际采购量、存放位置与日期，确认才会入库</p></div>
    {items.isLoading&&<Card>加载采购清单…</Card>}{items.error&&<Notice tone="error">加载失败 <Button size="sm" onClick={()=>items.refetch()}>重试</Button></Notice>}
    {items.data?.length===0&&<Card><h2 className="font-semibold">需要买什么，让菜单帮你列好</h2><p className="mt-2 text-sm text-slate-500">先安排并确认菜单，缺少的食材就会出现在这里，家人也能一起勾选。</p><Button className="mt-4" onClick={()=>useUI.getState().setPage("week")}>去安排菜单</Button></Card>}
    <div className="space-y-3">{items.data?.map(item=><Card key={item.id}><div className="flex items-center justify-between gap-3"><div><div className="font-semibold">{item.name} {item.origin==='system'&&<span className="rounded-full bg-amber-100 px-2 py-1 text-xs text-amber-800">系统建议补货</span>}</div><div className="text-xs text-slate-500">建议 {item.needed} {item.unit}{item.checked?' · 实购 '+item.bought+' '+item.unit:''}</div></div><div className="flex items-center gap-2">{canEdit&&<Button variant="ghost" size="sm" aria-label={'删除采购项'+item.name} onClick={()=>{setDeleting(item);setError('')}}><Trash2 size={16}/></Button>}<span className="text-xs">{item.stocked?'已入库':item.checked?'已买待入库':'待购买'}</span></div></div>
      {canEdit&&!item.stocked&&<div className="mt-3 flex gap-2"><Button size="sm" variant={item.checked?'outline':'default'} onClick={()=>item.checked?uncheck(item):open(item)}>{item.checked?'取消勾选':'勾选已买并入库'}</Button>{item.checked&&<Button size="sm" onClick={()=>open(item)}>核对入库</Button>}</div>}</Card>)}</div>
    {deleting&&<div role="dialog" aria-modal="true" aria-label="删除采购项" className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/35 p-4"><Card className="w-full max-w-sm"><h2 className="font-semibold">删除 {deleting.name}？</h2><p className="my-3 text-sm text-slate-500">从共享清单移除，已入库数量与库存流水会保留。</p>{error&&<Notice tone="error">{error}</Notice>}<div className="mt-3 flex gap-2"><Button variant="danger" disabled={busy} onClick={remove}>{busy?'删除中…':'确认删除'}</Button><Button variant="outline" disabled={busy} onClick={()=>setDeleting(null)}>保留</Button></div></Card></div>}
    {review&&<div className="fixed inset-0 z-40 flex items-end justify-center bg-slate-900/35 p-3 sm:items-center" role="dialog" aria-modal="true" aria-label="采购入库确认"><Card className="w-full max-w-md shadow-xl"><div className="flex items-start justify-between"><div><h2 className="text-lg font-semibold">{review.name} · 入库确认</h2><p className="text-xs text-slate-500">系统预填为估计建议，请按包装标签调整。</p></div><button className="text-xl" aria-label="关闭" onClick={()=>setReview(null)}>×</button></div><div className="mt-4 space-y-3"><Field label={'实际采购量（'+review.unit+'）'} type="number" min="0.001" step="0.001" value={bought} onChange={e=>setBought(e.target.value)}/><label className="block text-sm">存放位置<select className="mt-1 w-full rounded-xl border p-2" value={location} onChange={e=>setLocation(e.target.value)}><option>冷藏</option><option>冷冻</option><option>常温</option></select></label><Field label="有效期（可清空）" type="date" value={expiry} onChange={e=>{setExpiry(e.target.value);setExpiryKind('user')}}/><label className="block text-sm">日期依据<select className="mt-1 w-full rounded-xl border p-2" value={expiryKind} onChange={e=>setExpiryKind(e.target.value)} disabled={!expiry}><option value="estimate">系统估计</option><option value="label">包装标签</option><option value="user">用户确认</option></select></label></div>{error&&<div className="mt-3"><Notice tone="error">{error}</Notice></div>}<div className="mt-4 flex gap-2"><Button disabled={busy||Number(bought)<=0} onClick={confirm}>{busy?'入库中…':'确认并入库'}</Button><Button variant="outline" onClick={()=>setReview(null)}>暂不入库</Button></div></Card></div>}
    {!review&&error&&<div className="mt-3"><Notice tone="error">{error}</Notice></div>}
  </>
}
