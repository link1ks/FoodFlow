import {useEffect,useRef,useState} from 'react'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {Camera,ImagePlus} from 'lucide-react'
import {api} from './api'
import {Button,Card,Field,Notice} from './ui'

export function ImageUpload({token,household,onQueued}:{token:string;household:string;onQueued:()=>void}){
  const capabilities=useQuery({queryKey:['capabilities'],queryFn:()=>api<{vision_enabled:boolean}>('/capabilities',token)})
  const [file,setFile]=useState<File|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState('')
  const picker=useRef<HTMLInputElement>(null)
  const [preview,setPreview]=useState('')
  useEffect(()=>{if(!file){setPreview('');return}const url=URL.createObjectURL(file);setPreview(url);return()=>URL.revokeObjectURL(url)},[file])
  async function upload(){if(!file)return;setBusy(true);setError('');try{
    if(file.size>5*1024*1024)throw Error('图片不能超过 5 MiB')
    const form=new FormData();form.append('image',file)
    const response=await fetch('/api/households/'+household+'/jobs/image',{method:'POST',headers:{Authorization:'Bearer '+token},body:form})
    const result=await response.json().catch(()=>({error:'响应无法解析'}))
    if(!response.ok)throw Error(result.error||'上传失败')
    setFile(null);onQueued()
  }catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}}
  return <Card><h2 className="mb-2 font-semibold">拍照识别食材</h2><p className="mb-3 text-sm text-slate-500">识别后核对食材，填写数量与有效期。</p>
    {capabilities.data&&!capabilities.data.vision_enabled?<Notice>当前未配置视觉模型，图片识别不可用。可使用下方分类食材目录入库。</Notice>:<div className="space-y-3"><input ref={picker} aria-label="选择食材图片" type="file" accept="image/jpeg,image/png,image/webp" onChange={e=>{setFile(e.target.files?.[0]||null);setError('');e.target.value=''}} className="hidden"/><button type="button" disabled={busy||!capabilities.data?.vision_enabled} onClick={()=>picker.current?.click()} className="flex w-full items-center gap-4 rounded-2xl border-2 border-dashed border-emerald-300 bg-emerald-50 p-5 text-left transition hover:border-emerald-600 hover:bg-emerald-100 focus-visible:outline-2 focus-visible:outline-emerald-700 disabled:opacity-50"><span className="rounded-xl bg-white p-3 text-emerald-700"><Camera size={28}/></span><span className="min-w-0"><span className="flex items-center gap-2 font-semibold text-emerald-900"><ImagePlus size={18}/>{file?'更换食材照片':'拍照 / 选择食材照片'}</span><span className="mt-1 block truncate text-xs text-slate-600">{file?file.name:'支持 JPG、PNG、WebP，最大 5 MiB'}</span></span></button>{preview&&<img src={preview} alt="待识别食材预览" className="max-h-48 rounded-xl object-contain"/>}<Button disabled={!file||busy||capabilities.isLoading} onClick={upload}>{busy?'正在上传…':'提交识别任务'}</Button></div>}
    {capabilities.error&&<Notice tone="error">无法读取识别配置，请刷新重试。</Notice>}{error&&<div className="mt-2"><Notice tone="error">{error}</Notice></div>}
  </Card>
}

export function ImageReview({token,household,job,name,category}:{token:string;household:string;job:string;name:string;category:string}){
  const query=useQueryClient()
  const [preview,setPreview]=useState('')
  const [itemName,setItemName]=useState(name),[itemCategory,setItemCategory]=useState(category)
  const [unit,setUnit]=useState('g'),[quantity,setQuantity]=useState(''),[location,setLocation]=useState(''),[boughtOn,setBoughtOn]=useState(''),[expiresOn,setExpiresOn]=useState('')
  const [busy,setBusy]=useState(false),[error,setError]=useState('')
  const last=useRef<{signature:string;key:string}|null>(null)
  useEffect(()=>{let url='';const controller=new AbortController();fetch('/api/households/'+household+'/jobs/'+job+'/image',{headers:{Authorization:'Bearer '+token},signal:controller.signal}).then(r=>{if(!r.ok)throw Error('image unavailable');return r.blob()}).then(blob=>{if(!controller.signal.aborted){url=URL.createObjectURL(blob);setPreview(url)}}).catch(()=>{});return()=>{controller.abort();if(url)URL.revokeObjectURL(url)}},[token,household,job])
  async function confirm(){setBusy(true);setError('');try{
    const body={name:itemName,category:itemCategory,unit,quantity,location,bought_on:boughtOn,expires_on:expiresOn}
    const signature=JSON.stringify(body)
    if(last.current?.signature!==signature)last.current={signature,key:crypto.randomUUID()}
    await api('/households/'+household+'/jobs/'+job+'/image/confirm',token,'POST',body,last.current.key)
    query.invalidateQueries()
  }catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}}
  return <div className="mt-3 space-y-3"><Notice>模型建议：{name}（{category||'未分类'}）。请核对后明确确认；识别结果不会自动入库。</Notice>{preview&&<img src={preview} alt="待核对食材原图" className="max-h-52 rounded-xl object-contain"/>}
    <div className="grid gap-2 sm:grid-cols-2"><Field label="食材名称" value={itemName} onChange={e=>setItemName(e.target.value)}/><Field label="分类" value={itemCategory} onChange={e=>setItemCategory(e.target.value)}/><label className="text-sm">单位<select className="mt-1 w-full rounded-xl border p-2.5" value={unit} onChange={e=>setUnit(e.target.value)}>{['g','kg','ml','l','个','只','包','袋','盒','份'].map(v=><option key={v}>{v}</option>)}</select></label><Field label="实际数量" value={quantity} onChange={e=>setQuantity(e.target.value)}/><Field label="存放位置" value={location} onChange={e=>setLocation(e.target.value)}/><Field label="购买日期（可留空）" type="date" value={boughtOn} onChange={e=>setBoughtOn(e.target.value)}/><Field label="有效期（可留空，用户确认）" type="date" value={expiresOn} onChange={e=>setExpiresOn(e.target.value)}/></div>
    <Button disabled={busy||!itemName.trim()||!quantity} onClick={confirm}>{busy?'入库中…':'确认识别内容并入库'}</Button>{error&&<Notice tone="error">{error}</Notice>}
  </div>
}
