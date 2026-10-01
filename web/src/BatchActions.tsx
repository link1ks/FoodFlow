import {useState} from 'react'
import {useQueryClient} from '@tanstack/react-query'
import {Minus,Pencil,Trash2,Leaf,Clock3,TriangleAlert} from 'lucide-react'
import {Quantity,formatQuantity} from './Quantity'
import {BatchNutrition} from './BatchNutrition'
import {BatchCost} from './BatchCost'
import {api,idem} from './api'
import {Button,Field,Notice} from './ui'

export type InventoryBatch={id:string;ingredient_id:string;quantity:string;location:string;bought_on:string;expires_on:string;expires_at:string;expiry_kind:string;source:string;condition:string}

export function freshness(batch:InventoryBatch){
  if(batch.condition==='spoiled')return {label:'已变质',tone:'rose',pct:0,urgent:true}
  if(!batch.expires_at)return {label:'未记录有效期',tone:'slate',pct:0,urgent:false}
  const remaining=new Date(batch.expires_at).getTime()-Date.now()
  if(!Number.isFinite(remaining))return {label:'有效期格式异常',tone:'slate',pct:0,urgent:false}
  const hours=remaining/3_600_000
  const bought=batch.bought_on?new Date(batch.bought_on+'T00:00:00').getTime():NaN
  const total=new Date(batch.expires_at).getTime()-bought
  const pct=Number.isFinite(total)&&total>0?Math.max(0,Math.min(100,Math.round(remaining/total*100))):0
  if(hours<=0)return {label:'已过期',tone:'rose',pct,urgent:true}
  if(hours<=24)return {label:'剩 '+Math.ceil(hours)+' 小时',tone:'rose',pct,urgent:true}
  if(hours<=48)return {label:'剩 '+Math.ceil(hours)+' 小时',tone:'amber',pct,urgent:true}
  if(hours<=120)return {label:'剩 '+Math.ceil(hours/24)+' 天',tone:'amber',pct,urgent:false}
  return {label:'剩 '+Math.ceil(hours/24)+' 天',tone:'emerald',pct,urgent:false}
}

export function BatchActions({batch,name,unit,token,household,canEdit}:{batch:InventoryBatch;name:string;unit:string;token:string;household:string;canEdit:boolean}){
  const q=useQueryClient()
  const [mode,setMode]=useState<'none'|'waste'|'correct'>('none')
  const [target,setTarget]=useState(formatQuantity(batch.quantity))
  const [busy,setBusy]=useState(false)
  const [error,setError]=useState('')
  const life=freshness(batch)
  async function consume(){
    setError('');setBusy(true)
    try{await api('/households/'+household+'/stock',token,'POST',{ingredient_id:batch.ingredient_id,batch_id:batch.id,quantity:'1',reason:'consume'},idem());await q.invalidateQueries()}
    catch(e){setError(String(e))}finally{setBusy(false)}
  }
  async function waste(note:'spoiled'|'expired'){
    setError('');setBusy(true)
    try{await api('/households/'+household+'/stock',token,'POST',{ingredient_id:batch.ingredient_id,batch_id:batch.id,quantity:batch.quantity,reason:'waste',note},idem());setMode('none');await q.invalidateQueries()}
    catch(e){setError(String(e))}finally{setBusy(false)}
  }
  async function correct(){
    setError('');setBusy(true)
    try{await api('/households/'+household+'/batches/'+batch.id+'/quantity',token,'PATCH',{expected_quantity:batch.quantity,target_quantity:target},idem());setMode('none');await q.invalidateQueries()}
    catch(e){setError(String(e))}finally{setBusy(false)}
  }
  const color=life.tone==='rose'?'text-rose-700 bg-rose-50':life.tone==='amber'?'text-amber-800 bg-amber-50':life.tone==='emerald'?'text-emerald-800 bg-emerald-50':'text-slate-600 bg-slate-50'
  const LifeIcon=life.tone==='rose'?TriangleAlert:life.tone==='amber'?Clock3:Leaf
  const hasTimeline=Number.isFinite(Date.parse(batch.expires_at))&&Number.isFinite(Date.parse(batch.bought_on))&&Date.parse(batch.expires_at)>Date.parse(batch.bought_on+'T00:00:00')
  return <div className="border-t border-slate-100 py-2 text-xs">
    <div className="flex items-center justify-between gap-3"><div className="min-w-0 font-medium text-slate-700">{batch.location||'未指定位置'} · <Quantity value={batch.quantity} unit={unit}/></div>
      {canEdit&&Number(batch.quantity)>0&&<div className="flex shrink-0 gap-1"><button className="rounded-lg p-2 hover:bg-slate-100" title={'快捷消耗 1 '+unit} aria-label={name+'快捷消耗 1 '+unit} disabled={busy||Number(batch.quantity)<1||batch.condition!=='normal'||life.label==='已过期'} onClick={consume}><Minus size={15}/></button><button className="rounded-lg p-2 hover:bg-rose-50" title="报损" aria-label={name+'报损'} onClick={()=>setMode(mode==='waste'?'none':'waste')}><Trash2 size={15}/></button><button className="rounded-lg p-2 hover:bg-slate-100" title="纠正余量" aria-label={name+'纠正余量'} onClick={()=>{setTarget(formatQuantity(batch.quantity));setMode(mode==='correct'?'none':'correct')}}><Pencil size={15}/></button></div>}</div>
    <div className={'mt-1 rounded-xl px-3 py-2.5 '+color}>
      <div className="flex flex-wrap items-center justify-between gap-1"><span className="inline-flex items-center gap-1.5 font-semibold"><LifeIcon size={15} aria-hidden="true"/>{batch.expiry_kind==='estimate'?'预计 · ':''}{life.label}</span>{batch.expires_on&&<span className="text-[11px]">{batch.expires_on} 到期</span>}</div>
      {hasTimeline&&<div role="meter" aria-label={name+'有效期剩余比例'} aria-valuemin={0} aria-valuemax={100} aria-valuenow={life.pct} aria-valuetext={life.label+(batch.expiry_kind==='estimate'?'，估计有效期':'')} title="按购买日期与有效期计算，不代表实际品质检测" className="relative mt-2 h-2.5 overflow-hidden rounded-full bg-black/5"><div className="h-full rounded-full bg-current" style={{width:life.pct+'%'}}/><div aria-hidden="true" className="absolute inset-0" style={{backgroundImage:'repeating-linear-gradient(90deg,transparent 0,transparent calc(10% - 2px),rgba(255,255,255,.65) calc(10% - 2px),rgba(255,255,255,.65) 10%)'}}/></div>}
    </div>
    {mode==='waste'&&<div className="mt-2 flex items-center gap-2 rounded-xl bg-rose-50 p-2"><span className="mr-auto text-rose-800">整批 <Quantity value={batch.quantity} unit={unit}/> 报损原因</span><Button size="sm" variant="outline" disabled={busy} onClick={()=>waste('spoiled')}>变质</Button><Button size="sm" variant="outline" disabled={busy} onClick={()=>waste('expired')}>过期</Button></div>}
    {mode==='correct'&&<div className="mt-2 flex items-end gap-2 rounded-xl bg-slate-50 p-2"><div className="flex-1"><Field label={'实际余量（'+unit+'）'} type="number" min="0" step="0.001" value={target} onChange={e=>setTarget(e.target.value)}/></div><Button size="sm" disabled={busy||target===''||Number(target)<0} onClick={correct}>保存</Button></div>}
    {canEdit&&Number(batch.quantity)>0&&<BatchNutrition batch={batch.id} name={name} unit={unit} token={token} household={household}/>}
    {canEdit&&<BatchCost batch={batch.id} name={name} token={token} household={household}/>}
    {error&&<div className="mt-2"><Notice tone="error">{error}</Notice></div>}
  </div>
}
