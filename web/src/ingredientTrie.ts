import type {CatalogItem} from './CatalogArt'

type Node={children:Map<string,Node>;ids:Set<string>}
const node=():Node=>({children:new Map(),ids:new Set()})
const normalize=(value:string)=>value.trim().toLocaleLowerCase()

export class IngredientTrie {
  private root=node()
  insert(value:string,id:string){
    let current=this.root
    for(const char of [...normalize(value)]){
      let child=current.children.get(char)
      if(!child){child=node();current.children.set(char,child)}
      child.ids.add(id)
      current=child
    }
  }
  prefixIDs(value:string){
    let current=this.root
    for(const char of [...normalize(value)]){
      const child=current.children.get(char)
      if(!child)return new Set<string>()
      current=child
    }
    return new Set(current.ids)
  }
}

export function buildIngredientTrie(items:readonly CatalogItem[]){
  const trie=new IngredientTrie()
  for(const item of items){trie.insert(item.name,item.id);for(const alias of item.aliases)trie.insert(alias,item.id)}
  return trie
}

export function searchIngredients(items:readonly CatalogItem[],trie:IngredientTrie,query:string){
  const term=normalize(query)
  if(!term)return [...items].sort((a,b)=>a.category.localeCompare(b.category,'zh-CN')||a.name.localeCompare(b.name,'zh-CN'))
  const prefix=trie.prefixIDs(term)
  return items.filter(item=>prefix.has(item.id)||normalize(item.name).includes(term)||item.aliases.some(alias=>normalize(alias).includes(term)))
    .sort((a,b)=>Number(prefix.has(b.id))-Number(prefix.has(a.id))||a.category.localeCompare(b.category,'zh-CN')||a.name.localeCompare(b.name,'zh-CN'))
}
