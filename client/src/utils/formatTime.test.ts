import { describe, expect, it } from 'vitest'
import { formatMs } from './formatTime'

describe('formatMs', () => {
  it('formats zero', () => {
    expect(formatMs(0)).toBe('0:00')
  })

  it('formats minutes and seconds', () => {
    expect(formatMs(125_000)).toBe('2:05')
  })

  it('pads seconds', () => {
    expect(formatMs(61_000)).toBe('1:01')
  })
})
