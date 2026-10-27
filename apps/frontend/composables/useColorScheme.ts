import {
  pickScheme,
  primaryChannels,
  type ColorCatalog,
  type ColorScheme,
} from '~/utils/colorScheme'

const STORAGE_KEY = 'dbackup-color-scheme'

export function useColorScheme() {
  const scheme = useState('color-scheme', () => 'emerald')
  const schemes = useState<ColorScheme[]>('color-schemes', () => [])

  function apply(id: string, list = schemes.value) {
    const picked = list.find((item) => item.id === id)
    if (!picked || !import.meta.client) return
    scheme.value = picked.id
    const root = document.documentElement
    for (const [name, value] of Object.entries(
      primaryChannels(picked.primary),
    )) {
      root.style.setProperty(name, value)
    }
    localStorage.setItem(STORAGE_KEY, picked.id)
  }

  async function init() {
    if (!import.meta.client) return
    let catalog: ColorCatalog
    try {
      const response = await fetch('/color-schemes.json')
      if (!response.ok) return
      catalog = await response.json()
    } catch {
      return
    }
    if (!Array.isArray(catalog.schemes)) return
    schemes.value = catalog.schemes
    const picked = pickScheme(catalog, localStorage.getItem(STORAGE_KEY))
    if (picked) apply(picked.id, catalog.schemes)
  }

  return { scheme, schemes, init, apply }
}
