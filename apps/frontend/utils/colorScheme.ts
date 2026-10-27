export const PRIMARY_STEPS = [
  '50',
  '100',
  '200',
  '300',
  '400',
  '500',
  '600',
  '700',
  '800',
  '900',
  '950',
] as const

export type ColorScheme = {
  id: string
  label: string
  primary: Record<string, string>
}

export type ColorCatalog = {
  active: string
  schemes: ColorScheme[]
}

export function hexToChannels(hex: string): string | null {
  const raw = hex.trim().replace('#', '')
  const full =
    raw.length === 3
      ? raw
          .split('')
          .map((c) => c + c)
          .join('')
      : raw
  if (!/^[0-9a-fA-F]{6}$/.test(full)) return null
  const n = parseInt(full, 16)
  return `${(n >> 16) & 255} ${(n >> 8) & 255} ${n & 255}`
}

export function primaryChannels(
  primary: Record<string, string>,
): Record<string, string> {
  const out: Record<string, string> = {}
  for (const step of PRIMARY_STEPS) {
    const channels = hexToChannels(primary[step] ?? '')
    if (channels) out[`--p-${step}`] = channels
  }
  return out
}

export function pickScheme(
  catalog: ColorCatalog,
  saved: string | null,
): ColorScheme | undefined {
  return (
    catalog.schemes.find((item) => item.id === saved) ||
    catalog.schemes.find((item) => item.id === catalog.active) ||
    catalog.schemes[0]
  )
}
