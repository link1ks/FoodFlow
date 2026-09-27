import {describe,expect,it} from 'vitest'
import {buildIngredientTrie,searchIngredients} from './ingredientTrie'
import type {CatalogItem} from './CatalogArt'

const items:CatalogItem[]=[
  {id:'1',name:'番茄',category:'蔬菜',default_unit:'g',aliases:['西红柿']},
  {id:'2',name:'茄子',category:'蔬菜',default_unit:'g',aliases:[]},
  {id:'3',name:'西兰花',category:'蔬菜',default_unit:'g',aliases:[]}
]

describe('ingredient trie',()=>{
  it('finds single-character name and alias prefixes',()=>{
    const trie=buildIngredientTrie(items)
    expect(searchIngredients(items,trie,'番').map(x=>x.name)).toEqual(['番茄'])
    expect(searchIngredients(items,trie,'西红').map(x=>x.name)).toEqual(['番茄'])
  })
  it('keeps substring matches after prefix matches',()=>{
    const trie=buildIngredientTrie(items)
    expect(searchIngredients(items,trie,'茄').map(x=>x.name)).toEqual(['茄子','番茄'])
    expect(searchIngredients(items,trie,'不存在')).toEqual([])
  })
})
