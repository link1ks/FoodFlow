import * as React from 'react'
import {Slot} from '@radix-ui/react-slot'
import {cva,type VariantProps} from 'class-variance-authority'
import {clsx} from 'clsx'
import {twMerge} from 'tailwind-merge'
export const cn=(...v:(string|undefined|false)[])=>twMerge(clsx(v))
const variants=cva('inline-flex items-center justify-center rounded-xl text-sm font-semibold transition disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-emerald-600',{variants:{variant:{default:'bg-emerald-700 text-white hover:bg-emerald-800',outline:'border border-slate-300 bg-white text-slate-800 hover:bg-slate-50',ghost:'text-slate-700 hover:bg-slate-100',danger:'bg-rose-600 text-white hover:bg-rose-700'},size:{default:'h-10 px-4',sm:'h-8 px-3',lg:'h-12 px-5'}},defaultVariants:{variant:'default',size:'default'}})
export function Button({asChild=false,variant,size,className,...props}:React.ButtonHTMLAttributes<HTMLButtonElement>&VariantProps<typeof variants>&{asChild?:boolean}){const Comp=asChild?Slot:'button';return <Comp className={cn(variants({variant,size}),className)} {...props}/>}
export function Card({children,className}:{children:React.ReactNode;className?:string}){return <section className={cn('rounded-2xl border border-slate-200 bg-white p-4 shadow-sm',className)}>{children}</section>}
export function Field({label,...props}:{label:string}&React.InputHTMLAttributes<HTMLInputElement>){return <label className="block text-sm font-medium text-slate-700">{label}<input className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 outline-none focus:border-emerald-600" {...props}/></label>}
export function Notice({children,tone='info'}:{children:React.ReactNode;tone?:'info'|'error'|'success'}){return <div role={tone==='error'?'alert':undefined} className={cn('rounded-xl p-3 text-sm',tone==='error'?'bg-rose-50 text-rose-800':tone==='success'?'bg-emerald-50 text-emerald-800':'bg-sky-50 text-sky-800')}>{children}</div>}
