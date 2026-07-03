export type ImageToolMode = 'generate' | 'edit'
export type ImageAspectRatio = 'auto' | '1:1' | '3:4' | '9:16' | '4:3' | '16:9'
export type ImageResolutionTier = 'standard' | 'high' | 'ultra'

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
  { value: 'high', label: '高清' },
  { value: 'ultra', label: '超清' },
]

export const imageSizeMatrix: SizeMatrix = {
  generate: {
    auto: { standard: 'auto', high: 'auto', ultra: 'auto' },
    '1:1': { standard: '1024x1024', high: '1536x1536', ultra: '2048x2048' },
    '3:4': { standard: '768x1024', high: '1536x2048', ultra: '1920x2560' },
    '9:16': { standard: '720x1280', high: '1440x2560', ultra: '1728x3072' },
    '4:3': { standard: '1024x768', high: '2048x1536', ultra: '3072x2304' },
    '16:9': { standard: '1280x720', high: '2048x1152', ultra: '3840x2160' },
  },
  edit: {
    auto: { standard: 'auto', high: 'auto', ultra: 'auto' },
    '1:1': { standard: '1024x1024', high: '1536x1536', ultra: '2048x2048' },
    '3:4': { standard: '768x1024', high: '1536x2048', ultra: '1920x2560' },
    '9:16': { standard: '720x1280', high: '1440x2560', ultra: '1728x3072' },
    '4:3': { standard: '1024x768', high: '2048x1536', ultra: '3072x2304' },
    '16:9': { standard: '1280x720', high: '2048x1152', ultra: '3840x2160' },
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
