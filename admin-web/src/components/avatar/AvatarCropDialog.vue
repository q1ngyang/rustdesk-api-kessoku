<template>
  <el-dialog v-model="visible" :title="T('AvatarCrop')" width="min(420px, calc(100vw - 28px))" destroy-on-close @closed="cleanup">
    <div class="avatar-cropper">
      <div class="avatar-cropper__viewport" @pointerdown="startDrag" @pointermove="drag" @pointerup="stopDrag" @pointercancel="stopDrag">
        <img v-if="src" :src="src" alt="" draggable="false" :style="imageStyle"/>
        <span class="avatar-cropper__mask" aria-hidden="true"></span>
      </div>
      <div class="avatar-cropper__zoom"><span>{{ T('Zoom') }}</span><el-slider v-model="zoom" :min="1" :max="3" :step="0.05" @input="clampOffset"/></div>
    </div>
    <template #footer><el-button @click="visible = false">{{ T('Cancel') }}</el-button><el-button type="primary" :loading="rendering" @click="confirmCrop">{{ T('ApplyCrop') }}</el-button></template>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { T } from '@/utils/i18n'

const emit = defineEmits(['cropped'])
const size = 280
const visible = ref(false)
const rendering = ref(false)
const src = ref('')
const zoom = ref(1)
const natural = reactive({ width: 1, height: 1 })
const offset = reactive({ x: 0, y: 0 })
const pointer = reactive({ active: false, id: 0, x: 0, y: 0 })
let sourceImage = null

const baseScale = computed(() => Math.max(size / natural.width, size / natural.height))
const displayWidth = computed(() => natural.width * baseScale.value * zoom.value)
const displayHeight = computed(() => natural.height * baseScale.value * zoom.value)
const imageStyle = computed(() => ({
  width: `${displayWidth.value}px`, height: `${displayHeight.value}px`,
  transform: `translate(calc(-50% + ${offset.x}px), calc(-50% + ${offset.y}px))`,
}))

const clampOffset = () => {
  const maxX = Math.max(0, (displayWidth.value - size) / 2)
  const maxY = Math.max(0, (displayHeight.value - size) / 2)
  offset.x = Math.max(-maxX, Math.min(maxX, offset.x))
  offset.y = Math.max(-maxY, Math.min(maxY, offset.y))
}
const startDrag = event => {
  pointer.active = true; pointer.id = event.pointerId; pointer.x = event.clientX; pointer.y = event.clientY
  event.currentTarget.setPointerCapture(event.pointerId)
}
const drag = event => {
  if (!pointer.active || pointer.id !== event.pointerId) return
  offset.x += event.clientX - pointer.x; offset.y += event.clientY - pointer.y
  pointer.x = event.clientX; pointer.y = event.clientY; clampOffset()
}
const stopDrag = event => {
  if (pointer.id === event.pointerId) pointer.active = false
}
const cleanup = () => {
  if (src.value) URL.revokeObjectURL(src.value)
  src.value = ''; sourceImage = null; pointer.active = false
}
const open = file => {
  cleanup(); zoom.value = 1; offset.x = 0; offset.y = 0
  src.value = URL.createObjectURL(file)
  const image = new Image()
  image.onload = () => {
    sourceImage = image; natural.width = image.naturalWidth; natural.height = image.naturalHeight
    visible.value = true
  }
  image.onerror = () => { cleanup(); ElMessage.error(T('ParamsError')) }
  image.src = src.value
}
const confirmCrop = async () => {
  if (!sourceImage) return
  rendering.value = true
  try {
    const canvas = document.createElement('canvas')
    canvas.width = 512; canvas.height = 512
    const context = canvas.getContext('2d', { alpha: false })
    context.fillStyle = '#ffffff'; context.fillRect(0, 0, 512, 512)
    const ratio = 512 / size
    const drawWidth = displayWidth.value * ratio
    const drawHeight = displayHeight.value * ratio
    context.drawImage(sourceImage, (size / 2 - displayWidth.value / 2 + offset.x) * ratio, (size / 2 - displayHeight.value / 2 + offset.y) * ratio, drawWidth, drawHeight)
    const blob = await new Promise(resolve => canvas.toBlob(resolve, 'image/jpeg', 0.9))
    if (!blob) throw new Error('avatar render failed')
    emit('cropped', new File([blob], 'avatar.jpg', { type: 'image/jpeg' }))
    visible.value = false
  } finally { rendering.value = false }
}

defineExpose({ open })
</script>

<style scoped lang="scss">
.avatar-cropper { display: grid; justify-items: center; gap: 20px; }
.avatar-cropper__viewport { position: relative; width: 280px; max-width: 100%; aspect-ratio: 1; overflow: hidden; border-radius: 18px; background: #11151d; cursor: grab; touch-action: none; user-select: none; }
.avatar-cropper__viewport:active { cursor: grabbing; }
.avatar-cropper__viewport img { position: absolute; top: 50%; left: 50%; max-width: none; pointer-events: none; }
.avatar-cropper__mask { position: absolute; inset: 0; border: 2px solid rgba(255,255,255,.9); border-radius: 50%; box-shadow: 0 0 0 80px rgba(10,14,22,.55); pointer-events: none; }
.avatar-cropper__zoom { display: grid; width: 100%; align-items: center; gap: 14px; grid-template-columns: auto minmax(0, 1fr); color: var(--text-secondary); font-size: 12px; }
</style>
