import {useRef,useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {ReceiptText} from 'lucide-react'
import {api,idem} from './api'
import {Button,Field,Notice} from './ui'

type Cost={recorded:boolean;total_cost?:string;currency?:string}
export function BatchCost({batch,name,token,household}:{batch:string;name:string;token:string;household:string}){
 const [open,setOpen]=useState(false),[amount,setAmount]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState('')
 const request=useRef<{amount:string;key:string}|undefined>(undefined)
 const path='/households/'+household+'/batches/'+batch+'/cost'
 const cost=useQuery({queryKey:['batch-cost',household,batch],queryFn:()=>api<Cost>(path,token),enabled:open})
 async function save(){
  setError('');setBusy(true)
  if(!request.current||request.current.amount!==amount)request.current={amount,key:idem()}
  try{await api(path,token,'POST',{total_cost:amount},request.current.key);await cost.refetch()}
  catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}
 }
 return <div className="mt-2">
  <button type="button" className="flex items-center gap-1 text-xs text-slate-500 hover:text-emerald-800" aria-expanded={open} aria-label={name+'采购成本'} onClick={()=>setOpen(!open)}><ReceiptText size={13} aria-hidden="true"/>采购成本</button>
  {open&&<div className="mt-2 rounded-xl bg-slate-50 p-3">
   {cost.isLoading?<p>加载中…</p>:cost.error?<Notice tone="error">读取失败 <button onClick={()=>cost.refetch()}>重试</button></Notice>:cost.data?.recorded?<p>本批实付 <strong>¥{cost.data.total_cost}</strong></p>:<>
    <p className="mb-2 text-xs text-slate-500">填写本批全部食材的实付总额。确认后锁定，用于后续出库成本统计；不改写历史消耗。</p>
    <form className="flex items-end gap-2" onSubmit={e=>{e.preventDefault();void save()}}><div className="min-w-0 flex-1"><Field label="实付总额（元）" required type="number" min="0" max="9999999999.99" step="0.01" value={amount} onChange={e=>setAmount(e.target.value)}/></div><Button size="sm" disabled={busy||!amount}>{busy?'保存中…':'确认金额'}</Button></form>
   </>}
   {error&&<div className="mt-2"><Notice tone="error">{error}</Notice></div>}
  </div>}
 </div>
}
