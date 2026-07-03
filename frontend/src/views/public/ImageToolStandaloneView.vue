<template>
  <main class="image-tool-page">
    <header class="tool-header">
      <div class="brand">
        <span class="brand-mark">AI</span>
        <span>GPT Image</span>
      </div>

      <div class="mode-tabs" role="tablist" aria-label="Image tool mode">
        <button
          type="button"
          class="mode-tab"
          :class="{ active: mode === 'generate' }"
          @click="mode = 'generate'"
        >
          绘图
        </button>
        <button
          type="button"
          class="mode-tab"
          :class="{ active: mode === 'edit' }"
          @click="mode = 'edit'"
        >
          编辑
        </button>
      </div>
    </header>

    <section class="tool-shell">
      <aside class="parameter-panel">
        <div class="panel-title">参数</div>

        <div class="field-group">
          <label class="field-label" for="imageModel">模型名</label>
          <input id="imageModel" v-model.trim="modelName" class="field-input" autocomplete="off" />
        </div>

        <div class="field-group">
          <label class="field-label" for="baseUrl">Base URL</label>
          <input id="baseUrl" v-model.trim="baseUrl" class="field-input" autocomplete="off" />
        </div>

        <div class="field-group">
          <label class="field-label" for="apiKey">API Key</label>
          <div class="secret-row">
            <input
              id="apiKey"
              v-model="apiKey"
              class="field-input"
              :type="showApiKey ? 'text' : 'password'"
              autocomplete="off"
            />
            <button type="button" class="icon-button" :title="showApiKey ? '隐藏' : '显示'" @click="showApiKey = !showApiKey">
              <span v-if="showApiKey">闭</span>
              <span v-else>显</span>
            </button>
          </div>
        </div>

        <div v-if="mode === 'edit'" class="field-group upload-block">
          <label class="field-label">上传图片<span class="required">*</span></label>
          <label class="upload-area">
            <input class="hidden-input" type="file" accept="image/*" multiple @change="handleImageFiles" />
            <span class="upload-icon">↑</span>
            <span>点击上传</span>
          </label>
          <div v-if="inputImages.length" class="upload-grid">
            <button
              v-for="item in inputImages"
              :key="item.id"
              type="button"
              class="upload-thumb"
              :title="item.file.name"
              @click="removeInputImage(item.id)"
            >
              <img :src="item.url" :alt="item.file.name" />
            </button>
          </div>
        </div>

        <div v-if="mode === 'edit'" class="field-group">
          <label class="field-label">遮罩图</label>
          <label class="upload-area compact">
            <input class="hidden-input" type="file" accept="image/*" @change="handleMaskFile" />
            <span>{{ maskImage ? maskImage.file.name : '选择遮罩' }}</span>
          </label>
        </div>

        <div class="field-group">
          <label class="field-label">分辨率</label>
          <select v-model="resolutionTier" class="field-input" :disabled="aspectRatio === 'auto'">
            <option v-for="option in imageResolutionTierOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </div>

        <div class="field-group">
          <label class="field-label">比例</label>
          <select v-model="aspectRatio" class="field-input">
            <option v-for="option in imageAspectRatioOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
          <div class="field-hint">计算结果: {{ displaySize }}</div>
        </div>

        <div class="field-row">
          <div class="field-group">
            <label class="field-label">质量</label>
            <select v-model="quality" class="field-input">
              <option value="auto">自动</option>
              <option value="low">低</option>
              <option value="medium">中等</option>
              <option value="high">高</option>
            </select>
          </div>

          <div class="field-group">
            <label class="field-label">格式</label>
            <select v-model="outputFormat" class="field-input">
              <option value="png">PNG</option>
              <option value="jpeg">JPEG</option>
              <option value="webp">WebP</option>
            </select>
          </div>
        </div>

        <div v-if="outputFormat !== 'png'" class="field-group">
          <label class="field-label">压缩率 {{ outputCompression }}</label>
          <input v-model.number="outputCompression" class="range-input" type="range" min="0" max="100" />
        </div>

        <div class="field-row">
          <div class="field-group">
            <label class="field-label">数量</label>
            <select v-model.number="imageCount" class="field-input">
              <option :value="1">1</option>
              <option :value="2">2</option>
              <option :value="3">3</option>
              <option :value="4">4</option>
            </select>
          </div>

          <div class="field-group">
            <label class="field-label">审核</label>
            <select v-model="moderation" class="field-input">
              <option value="auto">自动</option>
              <option value="low">低</option>
            </select>
          </div>
        </div>

        <div class="advanced-panel">
          <button type="button" class="advanced-header" @click="advancedOpen = !advancedOpen">
            <span>高级参数</span>
            <span>{{ advancedOpen ? '收起' : '展开' }}</span>
          </button>

          <div v-if="advancedOpen" class="advanced-body">
            <label class="switch-line">
              <input v-model="streamEnabled" type="checkbox" />
              <span>流式预览</span>
            </label>

            <div v-if="streamEnabled" class="field-group">
              <label class="field-label">中间图数量</label>
              <select v-model.number="partialImages" class="field-input">
                <option :value="0">0</option>
                <option :value="1">1</option>
                <option :value="2">2</option>
                <option :value="3">3</option>
              </select>
            </div>
          </div>
        </div>
      </aside>

      <section class="canvas-area">
        <div class="canvas-display">
          <div v-if="status === 'idle'" class="canvas-empty">
            <span class="empty-icon">□</span>
            <h1>开始创作</h1>
            <p>设置参数后提交提示词</p>
          </div>

          <div v-else-if="status === 'loading'" class="canvas-loading">
            <span class="spinner"></span>
            <p>{{ streamEnabled ? '正在接收图像数据...' : '正在生成图像...' }}</p>
          </div>

          <div v-else-if="status === 'error'" class="canvas-error">
            <h2>请求失败</h2>
            <p>{{ errorMessage }}</p>
          </div>

          <div v-else-if="currentImage" class="canvas-result">
            <img :src="currentImage.url" :alt="currentImage.prompt" />
            <div class="image-meta">
              <span>{{ currentImage.size }}</span>
              <span>{{ currentImage.format.toUpperCase() }}</span>
              <span>{{ currentImage.kind === 'partial' ? '预览' : '完成' }}</span>
            </div>
            <div class="canvas-actions">
              <button type="button" class="action-button" @click="copyPrompt">复制提示词</button>
              <button type="button" class="action-button primary" @click="downloadCurrentImage">下载</button>
            </div>
          </div>
        </div>

        <form class="prompt-bar" @submit.prevent="submitRequest">
          <div class="prompt-header">
            <span>提示词</span>
            <span>{{ prompt.length }} / 4000</span>
          </div>
          <div class="prompt-input-row">
            <textarea
              v-model="prompt"
              class="prompt-input"
              maxlength="4000"
              placeholder="描述你想要生成或编辑的图像..."
            ></textarea>
            <button type="submit" class="submit-button" :disabled="isSubmitting">
              {{ isSubmitting ? '处理中' : mode === 'generate' ? '生成' : '编辑' }}
            </button>
          </div>
        </form>
      </section>

      <aside class="history-rail">
        <div class="history-title">图片历史</div>
        <button
          v-for="item in historyItems"
          :key="item.id"
          type="button"
          class="history-item"
          :class="{ active: currentImage?.id === item.id }"
          :title="item.prompt"
          @click="currentImage = item"
        >
          <img :src="item.url" :alt="item.prompt" />
        </button>
      </aside>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  imageAspectRatioOptions,
  imageResolutionTierOptions,
  resolveImageSize,
  type ImageAspectRatio,
  type ImageResolutionTier,
  type ImageToolMode,
} from '@/constants/imageTool'

type ImageQuality = 'auto' | 'low' | 'medium' | 'high'
type ImageOutputFormat = 'png' | 'jpeg' | 'webp'
type ModerationMode = 'auto' | 'low'
type RequestStatus = 'idle' | 'loading' | 'success' | 'error'

interface UploadItem {
  id: string
  file: File
  url: string
}

interface ResultItem {
  id: string
  url: string
  b64?: string
  prompt: string
  size: string
  format: ImageOutputFormat
  createdAt: number
  kind: 'partial' | 'final'
}

const mode = ref<ImageToolMode>('generate')
const modelName = ref('gpt-image-2')
const baseUrl = ref('https://api.openai.com')
const apiKey = ref('')
const showApiKey = ref(false)
const prompt = ref('')
const aspectRatio = ref<ImageAspectRatio>('auto')
const resolutionTier = ref<ImageResolutionTier>('standard')
const quality = ref<ImageQuality>('auto')
const outputFormat = ref<ImageOutputFormat>('png')
const outputCompression = ref(100)
const imageCount = ref(1)
const moderation = ref<ModerationMode>('auto')
const streamEnabled = ref(false)
const partialImages = ref(0)
const advancedOpen = ref(false)
const status = ref<RequestStatus>('idle')
const errorMessage = ref('')
const inputImages = ref<UploadItem[]>([])
const maskImage = ref<UploadItem | null>(null)
const currentImage = ref<ResultItem | null>(null)
const historyItems = ref<ResultItem[]>([])
const abortController = ref<AbortController | null>(null)

const derivedSize = computed(() => resolveImageSize(mode.value, aspectRatio.value, resolutionTier.value))
const displaySize = computed(() => derivedSize.value === 'auto' ? 'auto' : derivedSize.value.replace('x', '×'))
const isSubmitting = computed(() => status.value === 'loading')

watch(mode, () => {
  status.value = currentImage.value ? 'success' : 'idle'
  errorMessage.value = ''
})

watch(aspectRatio, (value) => {
  if (value === 'auto') {
    resolutionTier.value = 'standard'
  }
})

function nextId(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

function revokeUpload(item: UploadItem | null): void {
  if (item?.url) URL.revokeObjectURL(item.url)
}

function handleImageFiles(event: Event): void {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? []).slice(0, 16)
  inputImages.value.forEach(revokeUpload)
  inputImages.value = files.map((file) => ({
    id: nextId('upload'),
    file,
    url: URL.createObjectURL(file),
  }))
  target.value = ''
}

function removeInputImage(id: string): void {
  const item = inputImages.value.find((entry) => entry.id === id)
  revokeUpload(item ?? null)
  inputImages.value = inputImages.value.filter((entry) => entry.id !== id)
}

function handleMaskFile(event: Event): void {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  revokeUpload(maskImage.value)
  maskImage.value = file ? { id: nextId('mask'), file, url: URL.createObjectURL(file) } : null
  target.value = ''
}

function normalizeBaseUrl(value: string): string {
  return value.trim().replace(/\/+$/, '')
}

function buildImageUrl(kind: ImageToolMode): string {
  const base = normalizeBaseUrl(baseUrl.value)
  const path = kind === 'generate' ? '/images/generations' : '/images/edits'
  if (base.endsWith('/v1')) return `${base}${path}`
  return `${base}/v1${path}`
}

function mimeFromFormat(format: ImageOutputFormat): string {
  return format === 'jpeg' ? 'image/jpeg' : `image/${format}`
}

function createResultFromBase64(b64: string, kind: ResultItem['kind']): ResultItem {
  return {
    id: nextId(kind),
    url: `data:${mimeFromFormat(outputFormat.value)};base64,${b64}`,
    b64,
    prompt: prompt.value.trim(),
    size: derivedSize.value,
    format: outputFormat.value,
    createdAt: Date.now(),
    kind,
  }
}

function addFinalResult(item: ResultItem): void {
  currentImage.value = item
  historyItems.value = [item, ...historyItems.value].slice(0, 24)
  status.value = 'success'
}

function setPreviewResult(item: ResultItem): void {
  currentImage.value = item
  status.value = 'success'
}

function validateRequest(): string | null {
  if (!modelName.value.trim()) return '请输入模型名'
  if (!baseUrl.value.trim()) return '请输入 Base URL'
  if (!apiKey.value.trim()) return '请输入 API Key'
  if (!prompt.value.trim()) return '请输入提示词'
  if (mode.value === 'edit' && inputImages.value.length === 0) return '编辑模式需要至少上传一张图片'
  if (!derivedSize.value) return '当前分辨率和比例组合不可用'
  return null
}

function buildJSONPayload(): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    model: modelName.value.trim(),
    prompt: prompt.value.trim(),
    size: derivedSize.value,
    quality: quality.value,
    output_format: outputFormat.value,
    n: imageCount.value,
    moderation: moderation.value,
  }

  if (outputFormat.value !== 'png') {
    payload.output_compression = outputCompression.value
  }

  if (streamEnabled.value) {
    payload.stream = true
    payload.partial_images = partialImages.value
  }

  return payload
}

function buildFormDataPayload(): FormData {
  const form = new FormData()
  const payload = buildJSONPayload()

  Object.entries(payload).forEach(([key, value]) => {
    form.append(key, String(value))
  })

  inputImages.value.forEach((item) => {
    form.append('image[]', item.file, item.file.name)
  })

  if (maskImage.value) {
    form.append('mask', maskImage.value.file, maskImage.value.file.name)
  }

  return form
}

function buildHeaders(isJSON: boolean): HeadersInit {
  const headers: Record<string, string> = {
    Authorization: `Bearer ${apiKey.value.trim()}`,
    Accept: streamEnabled.value ? 'text/event-stream, application/json' : 'application/json',
  }

  if (isJSON) {
    headers['Content-Type'] = 'application/json'
  }

  return headers
}

async function submitRequest(): Promise<void> {
  const validationError = validateRequest()
  if (validationError) {
    status.value = 'error'
    errorMessage.value = validationError
    return
  }

  abortController.value?.abort()
  abortController.value = new AbortController()
  status.value = 'loading'
  errorMessage.value = ''

  try {
    const isGenerate = mode.value === 'generate'
    const response = await fetch(buildImageUrl(mode.value), {
      method: 'POST',
      headers: buildHeaders(isGenerate),
      body: isGenerate ? JSON.stringify(buildJSONPayload()) : buildFormDataPayload(),
      signal: abortController.value.signal,
    })

    if (!response.ok) {
      await throwResponseError(response)
    }

    const contentType = response.headers.get('content-type') || ''
    if (streamEnabled.value || contentType.includes('text/event-stream')) {
      await handleStreamResponse(response)
      return
    }

    const data = await response.json()
    handleJSONResponse(data)
  } catch (error) {
    if ((error as DOMException).name === 'AbortError') return
    status.value = 'error'
    errorMessage.value = error instanceof Error ? error.message : '请求失败'
  } finally {
    abortController.value = null
  }
}

async function throwResponseError(response: Response): Promise<never> {
  let message = `${response.status} ${response.statusText}`
  try {
    const data = await response.json()
    message = data?.error?.message || data?.message || message
  } catch {
    const text = await response.text().catch(() => '')
    if (text) message = text.slice(0, 500)
  }
  throw new Error(message)
}

function handleJSONResponse(data: any): void {
  const items = Array.isArray(data?.data) ? data.data : []
  const results = items
    .map((item: any) => {
      if (item?.b64_json) return createResultFromBase64(item.b64_json, 'final')
      if (item?.url) {
        return {
          id: nextId('url'),
          url: item.url,
          prompt: prompt.value.trim(),
          size: data?.size || derivedSize.value,
          format: outputFormat.value,
          createdAt: Date.now(),
          kind: 'final',
        } as ResultItem
      }
      return null
    })
    .filter(Boolean) as ResultItem[]

  if (!results.length) {
    throw new Error('响应中没有可显示的图片')
  }

  results.reverse().forEach(addFinalResult)
  currentImage.value = results[0]
}

async function handleStreamResponse(response: Response): Promise<void> {
  const reader = response.body?.getReader()
  if (!reader) {
    throw new Error('浏览器未返回可读取的流')
  }

  const decoder = new TextDecoder()
  let buffer = ''
  const streamState: { finalAdded: boolean; lastPreview: ResultItem | null } = {
    finalAdded: false,
    lastPreview: null,
  }

  const consumePayload = (payload: any) => {
    if (payload?.error) {
      throw new Error(payload.error.message || payload.error.code || '流式响应错误')
    }

    const type = String(payload?.type || '')
    const b64 = payload?.b64_json || payload?.partial_image_b64 || payload?.result

    if (b64 && typeof b64 === 'string') {
      const kind: ResultItem['kind'] = type.includes('completed') || type.includes('done') ? 'final' : 'partial'
      const item = createResultFromBase64(b64, kind)
      streamState.lastPreview = item
      if (kind === 'final') {
        addFinalResult(item)
        streamState.finalAdded = true
      } else {
        setPreviewResult(item)
      }
    }

    if (Array.isArray(payload?.data)) {
      payload.data.forEach((entry: any) => {
        if (entry?.b64_json) {
          const item = createResultFromBase64(entry.b64_json, 'final')
          addFinalResult(item)
          streamState.finalAdded = true
        }
      })
    }
  }

  const consumeBlock = (block: string) => {
    const dataLines = block
      .split(/\r?\n/)
      .filter((line) => line.startsWith('data:'))
      .map((line) => line.replace(/^data:\s?/, ''))

    if (!dataLines.length) return
    const raw = dataLines.join('\n').trim()
    if (!raw || raw === '[DONE]') return
    consumePayload(JSON.parse(raw))
  }

  while (true) {
    const { done, value } = await reader.read()
    buffer += decoder.decode(value, { stream: !done })
    const blocks = buffer.split(/\n\n|\r\n\r\n/)
    buffer = blocks.pop() ?? ''
    blocks.forEach(consumeBlock)
    if (done) break
  }

  if (buffer.trim()) consumeBlock(buffer)
  if (!streamState.finalAdded && streamState.lastPreview !== null) {
    const finalPreview: ResultItem = {
      id: streamState.lastPreview.id,
      url: streamState.lastPreview.url,
      b64: streamState.lastPreview.b64,
      prompt: streamState.lastPreview.prompt,
      size: streamState.lastPreview.size,
      format: streamState.lastPreview.format,
      createdAt: streamState.lastPreview.createdAt,
      kind: 'final',
    }
    addFinalResult(finalPreview)
  }
  if (!currentImage.value) throw new Error('流式响应中没有可显示的图片')
}

async function copyPrompt(): Promise<void> {
  await navigator.clipboard.writeText(currentImage.value?.prompt || prompt.value)
}

function downloadCurrentImage(): void {
  if (!currentImage.value) return
  const link = document.createElement('a')
  link.href = currentImage.value.url
  link.download = `gpt-image-${currentImage.value.size}-${currentImage.value.id}.${currentImage.value.format}`
  document.body.appendChild(link)
  link.click()
  link.remove()
}

onBeforeUnmount(() => {
  abortController.value?.abort()
  inputImages.value.forEach(revokeUpload)
  revokeUpload(maskImage.value)
})
</script>

<style scoped>
.image-tool-page {
  min-height: 100vh;
  background:
    linear-gradient(rgba(16, 185, 129, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(6, 182, 212, 0.04) 1px, transparent 1px),
    #ffffff;
  background-size: 48px 48px;
  color: #172033;
  font-family: Nunito, "Segoe UI", system-ui, sans-serif;
}

.tool-header {
  position: sticky;
  top: 0;
  z-index: 20;
  display: grid;
  grid-template-columns: minmax(160px, 1fr) auto minmax(160px, 1fr);
  align-items: center;
  gap: 16px;
  height: 70px;
  padding: 0 24px;
  border-bottom: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(18px);
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
  font-weight: 800;
}

.brand-mark {
  display: inline-flex;
  width: 34px;
  height: 34px;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  background: linear-gradient(135deg, #10b981, #06b6d4);
  color: #fff;
  font-size: 12px;
}

.mode-tabs {
  display: inline-flex;
  gap: 6px;
  padding: 5px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 10px;
  background: #f1f5f9;
}

.mode-tab {
  min-width: 92px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: #475569;
  cursor: pointer;
  font-size: 14px;
  font-weight: 700;
  padding: 9px 18px;
}

.mode-tab.active {
  background: linear-gradient(135deg, #10b981, #06b6d4);
  box-shadow: 0 8px 24px rgba(16, 185, 129, 0.24);
  color: #fff;
}

.tool-shell {
  display: grid;
  grid-template-columns: 292px minmax(0, 1fr) 96px;
  height: calc(100vh - 70px);
  overflow: hidden;
}

.parameter-panel,
.history-rail {
  overflow: auto;
  border-right: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(255, 255, 255, 0.92);
}

.parameter-panel {
  padding: 18px;
}

.panel-title,
.history-title {
  color: #0f172a;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.field-group {
  margin-top: 16px;
}

.field-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.field-label {
  display: block;
  margin-bottom: 7px;
  color: #475569;
  font-size: 12px;
  font-weight: 800;
}

.required {
  color: #e11d48;
}

.field-input {
  width: 100%;
  border: 1px solid rgba(15, 23, 42, 0.12);
  border-radius: 8px;
  background: #fff;
  color: #172033;
  font-size: 13px;
  outline: none;
  padding: 10px 11px;
  transition: border-color 0.18s, box-shadow 0.18s;
}

.field-input:focus {
  border-color: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.14);
}

.field-input:disabled {
  background: #f8fafc;
  color: #94a3b8;
}

.secret-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 40px;
  gap: 8px;
}

.icon-button {
  border: 1px solid rgba(15, 23, 42, 0.12);
  border-radius: 8px;
  background: #f8fafc;
  color: #475569;
  cursor: pointer;
  font-size: 12px;
  font-weight: 800;
}

.field-hint {
  margin-top: 7px;
  color: #64748b;
  font-size: 12px;
}

.range-input {
  width: 100%;
  accent-color: #10b981;
}

.upload-area {
  display: flex;
  min-height: 78px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px dashed rgba(16, 185, 129, 0.42);
  border-radius: 8px;
  background: #f8fffc;
  color: #047857;
  cursor: pointer;
  font-size: 13px;
  font-weight: 800;
}

.upload-area.compact {
  min-height: 42px;
  padding: 0 12px;
}

.upload-icon {
  font-size: 18px;
}

.hidden-input {
  display: none;
}

.upload-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  margin-top: 10px;
}

.upload-thumb {
  aspect-ratio: 1;
  overflow: hidden;
  border: 1px solid rgba(15, 23, 42, 0.1);
  border-radius: 8px;
  background: #f8fafc;
  cursor: pointer;
  padding: 0;
}

.upload-thumb img,
.history-item img,
.canvas-result img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.advanced-panel {
  margin-top: 16px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 8px;
  background: #f8fafc;
}

.advanced-header {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  border: 0;
  background: transparent;
  color: #334155;
  cursor: pointer;
  font-size: 13px;
  font-weight: 800;
  padding: 11px 12px;
}

.advanced-body {
  padding: 0 12px 12px;
}

.switch-line {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #475569;
  font-size: 13px;
  font-weight: 700;
}

.canvas-area {
  display: grid;
  min-width: 0;
  grid-template-rows: minmax(0, 1fr) 148px;
  background: #f8fafc;
}

.canvas-display {
  position: relative;
  display: flex;
  min-height: 0;
  align-items: center;
  justify-content: center;
  overflow: auto;
  padding: 28px;
}

.canvas-empty,
.canvas-loading,
.canvas-error,
.canvas-result {
  text-align: center;
}

.empty-icon {
  display: inline-flex;
  width: 72px;
  height: 72px;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(16, 185, 129, 0.22);
  border-radius: 8px;
  color: #10b981;
  font-size: 52px;
}

.canvas-empty h1,
.canvas-error h2 {
  margin: 18px 0 8px;
  color: #0f172a;
  font-size: 22px;
}

.canvas-empty p,
.canvas-loading p,
.canvas-error p {
  margin: 0;
  color: #64748b;
  font-size: 13px;
}

.spinner {
  display: inline-block;
  width: 38px;
  height: 38px;
  border: 3px solid rgba(16, 185, 129, 0.18);
  border-top-color: #10b981;
  border-radius: 999px;
  animation: spin 0.8s linear infinite;
}

.canvas-result {
  width: min(100%, 980px);
}

.canvas-result img {
  max-width: 100%;
  max-height: calc(100vh - 290px);
  width: auto;
  height: auto;
  border: 1px solid rgba(15, 23, 42, 0.1);
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 20px 60px rgba(15, 23, 42, 0.12);
}

.image-meta,
.canvas-actions {
  display: flex;
  justify-content: center;
  gap: 8px;
  margin-top: 12px;
}

.image-meta span {
  border-radius: 999px;
  background: #fff;
  color: #475569;
  font-size: 12px;
  font-weight: 800;
  padding: 5px 10px;
}

.action-button,
.submit-button {
  border: 0;
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 800;
  padding: 10px 14px;
}

.action-button {
  border: 1px solid rgba(15, 23, 42, 0.1);
  background: #fff;
  color: #334155;
}

.action-button.primary,
.submit-button {
  background: linear-gradient(135deg, #10b981, #06b6d4);
  color: #fff;
}

.submit-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.prompt-bar {
  border-top: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(255, 255, 255, 0.95);
  padding: 14px 18px 16px;
}

.prompt-header {
  display: flex;
  justify-content: space-between;
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
  margin-bottom: 8px;
}

.prompt-input-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 100px;
  gap: 12px;
}

.prompt-input {
  min-height: 82px;
  resize: none;
  border: 1px solid rgba(15, 23, 42, 0.12);
  border-radius: 8px;
  color: #172033;
  font-size: 14px;
  outline: none;
  padding: 12px 14px;
}

.prompt-input:focus {
  border-color: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.14);
}

.history-rail {
  border-right: 0;
  border-left: 1px solid rgba(15, 23, 42, 0.08);
  padding: 16px 12px;
}

.history-title {
  margin-bottom: 12px;
  text-align: center;
}

.history-item {
  position: relative;
  width: 68px;
  height: 68px;
  overflow: hidden;
  border: 2px solid transparent;
  border-radius: 8px;
  background: #f1f5f9;
  cursor: pointer;
  margin: 0 auto 10px;
  padding: 0;
}

.history-item.active {
  border-color: #10b981;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 1024px) {
  .tool-shell {
    grid-template-columns: 260px minmax(0, 1fr);
  }

  .history-rail {
    display: none;
  }
}

@media (max-width: 760px) {
  .tool-header {
    position: static;
    grid-template-columns: 1fr;
    height: auto;
    justify-items: stretch;
    padding: 14px;
  }

  .brand {
    justify-content: center;
  }

  .mode-tabs {
    justify-content: center;
  }

  .tool-shell {
    grid-template-columns: 1fr;
    height: auto;
    overflow: visible;
  }

  .parameter-panel {
    border-right: 0;
    border-bottom: 1px solid rgba(15, 23, 42, 0.08);
  }

  .canvas-area {
    min-height: 640px;
  }

  .prompt-input-row {
    grid-template-columns: 1fr;
  }
}
</style>
