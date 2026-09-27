/** Trim insignificant decimal zeros without converting precise values to Number. */
export function formatQuantity(value:string){
 const match=/^(-?\d+)(\.\d+)?$/.exec(value)
 const fraction=match?.[2]?.slice(1).replace(/0+$/,'')||''
 return match?match[1]+(fraction?'.'+fraction:''):value
}
export function Quantity({value,unit}:{value:string;unit?:string}){
 const display=formatQuantity(value)
 return <span className="whitespace-nowrap tabular-nums">{display}{unit&&<> {unit}</>}</span>
}
