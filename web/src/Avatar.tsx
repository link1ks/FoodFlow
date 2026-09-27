import {UserRound} from 'lucide-react'

export function Avatar({name,large=false}:{name?:string;large?:boolean}){
 const initial=Array.from(name?.trim()||'')[0]
 return <span role="img" aria-label={name?name+'的头像':'默认头像'} className={'inline-flex shrink-0 items-center justify-center rounded-full border border-emerald-200 bg-gradient-to-br from-emerald-100 to-teal-200 font-semibold text-emerald-900 '+(large?'h-16 w-16 text-2xl':'h-10 w-10 text-base')}>{initial||<UserRound size={large?28:20}/>}</span>
}
