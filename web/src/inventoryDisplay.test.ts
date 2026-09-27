import {afterEach,expect,it,vi} from 'vitest'
import {formatQuantity} from './Quantity'
import {freshness,type InventoryBatch} from './BatchActions'

afterEach(()=>vi.useRealTimers())
it('preserves precision while removing insignificant zeros',()=>{
  expect(formatQuantity('11.000')).toBe('11')
  expect(formatQuantity('1.250')).toBe('1.25')
  expect(formatQuantity('0.001')).toBe('0.001')
  expect(formatQuantity('9007199254740993.100')).toBe('9007199254740993.1')
})
it('distinguishes urgent, expired, unknown and spoiled batches',()=>{
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-09-26T00:00:00Z'))
  const batch={bought_on:'2026-09-25',expires_at:'2026-09-26T08:00:00Z',condition:'normal'} as InventoryBatch
  expect(freshness(batch)).toMatchObject({label:'剩 8 小时',tone:'rose',urgent:true})
  expect(freshness({...batch,expires_at:'2026-09-26T00:00:00Z'})).toMatchObject({label:'已过期',pct:0})
  expect(freshness({...batch,expires_at:''})).toMatchObject({label:'未记录有效期',urgent:false})
  expect(freshness({...batch,condition:'spoiled'})).toMatchObject({label:'已变质',pct:0,urgent:true})
})
