import {useRef,useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {api,idem} from './api'
import {Button,Field,Notice} from './ui'

export type AdviceResult={mode:string;summary:string;tips:string[];recipes:{id:string;title:string;minutes:number;ingredients:string[]}[];selected_inventory:{id:string;name:string;quantity:string;unit:string}[];servings:number}
type AdviceJob={id:string;status:string;progress:number;error:string;result?:AdviceResult}
export function AdviceView({result,onChoose}:{result:AdviceResult;onChoose?:(id:string)=>void|Promise<void>}){
 const [choosing,setChoosing]=useState(false)
 async function choose(id:string){if(choosing||!onChoose)return;setChoosing(true);try{await onChoose(id)}finally{setChoosing(false)}}
 return <div className="mt-3 space-y-3 text-sm"><Notice>{result.mode==='demo'?'演示模式：未调用真实模型。':'模型生成建议：请核对食材与家庭忌口。'}</Notice><p className="text-xs text-slate-500">依据提交时所选：{result.selected_inventory.map(x=>x.name).join('、')} · {result.servings} 人。库存会变化，采用后重新核对。</p><p className="font-medium">{result.summary}</p><ul className="list-disc space-y-1 pl-5">{result.tips.map((tip,i)=><li key={i}>{tip}</li>)}</ul>{result.recipes.map(r=><div key={r.id} className="rounded-xl border bg-white p-3"><b>{r.title} · {r.minutes} 分钟</b><p className="mt-1 text-xs text-slate-600">菜谱食材：{r.ingredients.join('、')}（可能需补充采购）</p>{onChoose&&<Button size="sm" variant="outline" className="mt-2" disabled={choosing} onClick={()=>choose(r.id)}>采用并创建菜单草案</Button>}</div>)}</div>
}
export function NutritionAdvice({token,household,selected,servings,maxMinutes,canEdit,onChoose}:{token:string;household:string;selected:string[];servings:number;maxMinutes:number;canEdit:boolean;onChoose:(id:string)=>void}){
 const [goal,setGoal]=useState(''),[job,setJob]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState('')
 const pending=useRef({body:'',key:''})
 const root='/households/'+household+'/jobs'
 const detail=useQuery({queryKey:['nutrition-advice',household,job],enabled:!!job,queryFn:()=>api<AdviceJob>(root+'/'+job,token),refetchInterval:q=>['queued','running'].includes(q.state.data?.status||'queued')?1500:false})
 async function generate(){if(busy)return;setBusy(true);setError('');const body={selected_ingredient_ids:[...selected].sort(),servings,max_minutes:maxMinutes,goal};const serialized=JSON.stringify(body);if(pending.current.body!==serialized)pending.current={body:serialized,key:idem()};try{const v=await api<{id:string}>(root+'/advice',token,'POST',body,pending.current.key);setJob(v.id);pending.current={body:'',key:''}}catch(e){setError(String(e))}finally{setBusy(false)}}
 async function action(name:'cancel'|'retry'){setBusy(true);setError('');try{await api(root+'/'+job+'/'+name,token,'POST',{});await detail.refetch()}catch(e){setError(String(e))}finally{setBusy(false)}}
 const active=['queued','running'].includes(detail.data?.status||'')|| (!!job&&!detail.data&&!detail.error)
 return <section className="mt-5 rounded-2xl border border-emerald-200 bg-emerald-50/40 p-4"><h3 className="font-semibold">AI 食材搭配与营养建议</h3><p className="mt-1 text-xs text-slate-600">结合已选食材与家庭偏好，提供搭配建议。</p><div className="mt-3"><Field label="本次希望怎么搭配（可选）" placeholder="例如：清淡一点，优先使用蔬菜" maxLength={500} value={goal} onChange={e=>setGoal(e.target.value)}/></div><Button className="mt-3" disabled={!canEdit||!selected.length||servings<1||servings>20||maxMinutes<1||maxMinutes>240||busy||active} onClick={generate}>{busy?'提交中…':job?'生成新建议':'用所选食材生成建议'}</Button>{!canEdit&&<p className="mt-2 text-xs">只读成员可查看营养资料，生成建议需要编辑权限。</p>}
 {detail.isLoading&&job&&<p className="mt-3 text-sm">正在读取任务…</p>}{active&&detail.data&&<div className="mt-3 text-sm">{detail.data.status==='queued'?'建议任务排队中，请确保 Worker 正在运行。':'模型正在生成建议…'}<Button size="sm" variant="ghost" disabled={busy} onClick={()=>action('cancel')}>取消任务</Button></div>}
 {detail.error&&<Notice tone="error">任务读取失败。<Button variant="ghost" onClick={()=>detail.refetch()}>重试读取</Button></Notice>}
 {detail.data?.status==='failed'&&<Notice tone="error">{detail.data.error}<Button variant="outline" size="sm" disabled={busy} onClick={()=>action('retry')}>重试生成（可能再次调用模型）</Button></Notice>}
 {detail.data?.status==='cancelled'&&<p className="mt-3 text-sm">建议任务已取消。</p>}
 {detail.data?.status==='succeeded'&&detail.data.result&&<AdviceView result={detail.data.result} onChoose={canEdit?onChoose:undefined}/>}{error&&<Notice tone="error">{error}</Notice>}
 </section>
}
