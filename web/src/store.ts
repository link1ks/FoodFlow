import {create} from 'zustand'
type State={token:string;household:string;page:string;setToken:(v:string)=>void;setHousehold:(v:string)=>void;setPage:(v:string)=>void}
export const useUI=create<State>(set=>({token:localStorage.getItem('ff_token')||'',household:localStorage.getItem('ff_household')||'',page:'today',setToken:v=>{localStorage.setItem('ff_token',v);set({token:v})},setHousehold:v=>{localStorage.setItem('ff_household',v);set({household:v})},setPage:v=>set({page:v})}))
