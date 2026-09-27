import {useState} from 'react'
import {useQueryClient} from '@tanstack/react-query'
import {api} from './api'

type Member={id:string;name:string;email:string;role:string}

export function MemberRoleControl({member,canEdit,token,household}:{member:Member;canEdit:boolean;token:string;household:string}){
  const query=useQueryClient()
  const [busy,setBusy]=useState(false),[error,setError]=useState('')
  async function change(role:string){setBusy(true);setError('');try{
    await api('/households/'+household+'/members/'+member.id,token,'PATCH',{role})
    await query.invalidateQueries({queryKey:['members','/households/'+household+'/members']})
  }catch(e){setError(e instanceof Error?e.message:String(e))}finally{setBusy(false)}}
  return <div className="text-right">{canEdit&&member.role!=='owner'?<select aria-label={`修改 ${member.name} 的角色`} className="rounded-lg border p-1 text-xs" value={member.role} disabled={busy} onChange={e=>change(e.target.value)}><option value="editor">可编辑</option><option value="viewer">只读</option></select>:<span className="text-slate-500">{member.role==='owner'?'户主':member.role==='editor'?'可编辑':'只读'}</span>}{error&&<p role="alert" className="text-xs text-rose-700">{error}</p>}</div>
}
