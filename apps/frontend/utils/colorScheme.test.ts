import { describe, expect, it } from 'vitest'
import {
  hexToChannels,
  pickScheme,
  primaryChannels,
  type ColorCatalog,
} from './colorScheme'

const catalog: ColorCatalog = {
  active: 'emerald',
  schemes: [
    { id: 'emerald', label: 'Emerald', primary: { '700': '#047857' } },
    { id: 'blue', label: 'Blue', primary: { '700': '#1d4ed8' } },
  ],
}

describe('colorScheme', () => {
  it('turns hex into rgb channels', () => {
    expect(hexToChannels('#047857')).toBe('4 120 87')
    expect(hexToChannels('fff')).toBe('255 255 255')
    expect(hexToChannels('nope')).toBeNull()
  })

  it('maps a primary scale onto css variables', () => {
    expect(primaryChannels({ '700': '#047857', '400': 'nope' })).toEqual({
      '--p-700': '4 120 87',
    })
  })

  it('prefers the saved scheme, then the catalog default', () => {
    expect(pickScheme(catalog, 'blue')?.id).toBe('blue')
    expect(pickScheme(catalog, 'missing')?.id).toBe('emerald')
  })
})
