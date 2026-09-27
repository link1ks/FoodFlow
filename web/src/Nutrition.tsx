export type NutritionProfile={status:'reference'|'unavailable';basis:string;reference_food?:string;source?:string;source_url?:string;note:string;nutrients:Record<string,string>}
const fields=[['energy_kcal','能量','kcal'],['protein_g','蛋白质','g'],['fat_g','脂肪','g'],['carbs_g','碳水化合物','g'],['fiber_g','膳食纤维','g'],['sodium_mg','钠','mg']]
export function NutritionDetails({profile}:{profile?:NutritionProfile}){
 return <details className="mt-3 rounded-xl border border-emerald-100 bg-emerald-50/50 p-3 text-sm"><summary className="cursor-pointer font-medium text-emerald-900">营养价值 · {profile?.status==='reference'?'每100克参考值':'暂无已核验数值'}</summary>
 {profile?.status==='reference'&&<div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">{fields.map(([key,label,unit])=><div key={key} className="rounded-lg bg-white p-2"><span className="text-xs text-slate-500">{label}</span><b className="block">{profile.nutrients[key]==null?'未提供':`${Number(profile.nutrients[key])} ${unit}`}</b></div>)}</div>}
 <p className="mt-2 text-xs text-slate-600">{profile?.note||'未匹配到参考资料。请核对具体食材或包装营养标签。'}</p>
 {profile?.status==='reference'&&<><p className="mt-2 text-xs text-slate-500">{profile.basis}；不是当前库存总量或实际批次检测值。生熟、品种及品牌会影响数值。</p><p className="mt-1 break-words text-xs text-slate-500">参考食品：{profile.reference_food}</p><a className="mt-2 block text-xs text-emerald-800 underline" href={profile.source_url} target="_blank" rel="noopener noreferrer">{profile.source} ↗</a></>}
 </details>
}
