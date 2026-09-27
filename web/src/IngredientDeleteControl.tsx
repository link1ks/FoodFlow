import {Quantity} from './Quantity'
import {useRef,useState} from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import {Trash2,X} from 'lucide-react'
import {api,idem} from './api'
import {Button,Notice} from './ui'

export function IngredientDeleteControl({id,name,quantity,unit,token,household,onArchived}:{id:string;name:string;quantity:string;unit:string;token:string;household:string;onArchived:()=>void}){
  const [open,setOpen]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState('')
  const key=useRef('')
  async function archive(){setBusy(true);setError('');try{
    if(!key.current)key.current=idem()
    await api('/households/'+household+'/ingredients/'+id,token,'DELETE',undefined,key.current)
    key.current='';setOpen(false);onArchived()
  }catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}}
  return <Dialog.Root open={open} onOpenChange={value=>{setOpen(value);if(!value)setError('')}}><Dialog.Trigger asChild><Button size="sm" variant="ghost" className="text-rose-700 hover:bg-rose-50" aria-label={`删除${name}`}><Trash2 size={15}/> 删除</Button></Dialog.Trigger><Dialog.Portal><Dialog.Overlay className="fixed inset-0 z-40 bg-slate-950/50"/><Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[min(92vw,430px)] -translate-x-1/2 -translate-y-1/2 rounded-2xl bg-white p-5 shadow-2xl"><div className="flex items-start justify-between"><Dialog.Title className="text-lg font-semibold">删除 {name}？</Dialog.Title><Dialog.Close asChild><button type="button" aria-label="关闭删除确认" className="rounded-lg p-1 hover:bg-slate-100"><X size={18}/></button></Dialog.Close></div><Dialog.Description className="mt-2 text-sm text-slate-600">当前剩余 <Quantity value={quantity} unit={unit}/>。确认后会从库存中移除，并对剩余批次记录补偿流水；历史记录仍可审计。以后再次入库同种食材可恢复。</Dialog.Description><div className="mt-5 flex justify-end gap-2"><Dialog.Close asChild><Button variant="outline" disabled={busy}>取消</Button></Dialog.Close><Button variant="danger" disabled={busy} onClick={archive}>{busy?'处理中…':Number(quantity)>0?'删除并清空库存':'确认删除'}</Button></div>{error&&<div className="mt-3"><Notice tone="error">{error}</Notice></div>}</Dialog.Content></Dialog.Portal></Dialog.Root>
}
