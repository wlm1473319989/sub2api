export type ImageToolMode = 'generate' | 'edit'
export type ImageAspectRatio = 'auto' | '1:1' | '3:4' | '9:16' | '4:3' | '16:9'
export type ImageResolutionTier = 'standard' | 'clear' | 'high' | 'fine' | 'ultra'

type SizeMatrix = Record<ImageToolMode, Record<ImageAspectRatio, Record<ImageResolutionTier, string>>>

export const imageAspectRatioOptions: Array<{ value: ImageAspectRatio; label: string }> = [
  { value: 'auto', label: '自动' },
  { value: '1:1', label: '方形 1:1' },
  { value: '3:4', label: '竖版 3:4' },
  { value: '9:16', label: '故事 9:16' },
  { value: '4:3', label: '横屏 4:3' },
  { value: '16:9', label: '宽屏 16:9' },
]

export const imageResolutionTierOptions: Array<{ value: ImageResolutionTier; label: string }> = [
  { value: 'standard', label: '标准' },
  { value: 'clear', label: '清晰' },
  { value: 'high', label: '高清 2K' },
  { value: 'fine', label: '精细 3K级' },
  { value: 'ultra', label: '超清 最高' },
]

export const imageSizeMatrix: SizeMatrix = {
  generate: {
    auto: { standard: 'auto', clear: 'auto', high: 'auto', fine: 'auto', ultra: 'auto' },
    '1:1': { standard: '1024x1024', clear: '1536x1536', high: '2048x2048', fine: '2560x2560', ultra: '2880x2880' },
    '3:4': { standard: '768x1024', clear: '1152x1536', high: '1536x2048', fine: '2304x3072', ultra: '2448x3264' },
    '9:16': { standard: '576x1024', clear: '864x1536', high: '1152x2048', fine: '1728x3072', ultra: '2160x3840' },
    '4:3': { standard: '1024x768', clear: '1536x1152', high: '2048x1536', fine: '3072x2304', ultra: '3264x2448' },
    '16:9': { standard: '1024x576', clear: '1536x864', high: '2048x1152', fine: '3072x1728', ultra: '3840x2160' },
  },
  edit: {
    auto: { standard: 'auto', clear: 'auto', high: 'auto', fine: 'auto', ultra: 'auto' },
    '1:1': { standard: '1024x1024', clear: '1536x1536', high: '2048x2048', fine: '2560x2560', ultra: '2880x2880' },
    '3:4': { standard: '768x1024', clear: '1152x1536', high: '1536x2048', fine: '2304x3072', ultra: '2448x3264' },
    '9:16': { standard: '576x1024', clear: '864x1536', high: '1152x2048', fine: '1728x3072', ultra: '2160x3840' },
    '4:3': { standard: '1024x768', clear: '1536x1152', high: '2048x1536', fine: '3072x2304', ultra: '3264x2448' },
    '16:9': { standard: '1024x576', clear: '1536x864', high: '2048x1152', fine: '3072x1728', ultra: '3840x2160' },
  },
}

export function resolveImageSize(
  mode: ImageToolMode,
  aspectRatio: ImageAspectRatio,
  resolutionTier: ImageResolutionTier,
): string {
  if (aspectRatio === 'auto') return 'auto'
  return imageSizeMatrix[mode][aspectRatio][resolutionTier]
}
