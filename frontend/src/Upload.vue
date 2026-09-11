<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import Button from "./components/ui/button/Button.vue"
import { Progress } from "./components/ui/progress"
import { ScrollArea } from "./components/ui/scroll-area"
import ThreadProgress from "./components/ThreadProgress.vue"
import { uploadManager } from './utils/UploadManager'
import { ConfigManager } from '../bindings/app/backend'
import { X, Clock, Zap, FolderPlus, AlertTriangle, Pause, Play, Gauge } from '@lucide/vue'

const { state } = uploadManager

// Elapsed time ticker & pause tracking
const elapsedSeconds = ref(0)
const pausedDuration = ref(0)
let pauseStartTime = 0
let elapsedInterval: ReturnType<typeof setInterval> | null = null

// Bandwidth limit state
const selectedLimitMBps = ref(0)

const currentLimitLabel = computed(() => {
  if (selectedLimitMBps.value <= 0) return 'Unlimited'
  return `${selectedLimitMBps.value} MB/s`
})

function onLimitSliderChange() {
  uploadManager.setBandwidthLimit(selectedLimitMBps.value)
}

function applyPresetLimit(preset: number) {
  selectedLimitMBps.value = preset
  uploadManager.setBandwidthLimit(preset)
}

function togglePause() {
  if (state.isPaused) {
    uploadManager.resumeUpload()
  } else {
    uploadManager.pauseUpload()
  }
}

watch(() => state.isPaused, (isPaused) => {
  if (isPaused) {
    pauseStartTime = Date.now()
  } else if (pauseStartTime > 0) {
    pausedDuration.value += (Date.now() - pauseStartTime)
    pauseStartTime = 0
  }
})

onMounted(async () => {
  try {
    const cfg = await ConfigManager.GetSettings()
    if (cfg && cfg.maxUploadSpeedMBps !== undefined) {
      selectedLimitMBps.value = cfg.maxUploadSpeedMBps
      uploadManager.setBandwidthLimit(cfg.maxUploadSpeedMBps)
    }
  } catch {
    // ignore
  }

  elapsedInterval = setInterval(() => {
    if (state.startTime > 0 && !state.isPaused) {
      const activeElapsed = Math.floor((Date.now() - state.startTime - pausedDuration.value) / 1000)
      elapsedSeconds.value = Math.max(0, activeElapsed)
    }
  }, 1000)
})

onUnmounted(() => {
  if (elapsedInterval) {
    clearInterval(elapsedInterval)
  }
})

const threadsList = computed(() => {
  return Array.from(state.threads.values())
    .filter(thread => thread.Status !== 'idle')
    .sort((a, b) => a.WorkerID - b.WorkerID)
})

const progressPercent = computed(() => {
  if (state.totalFiles === 0) return 0
  return Math.round((state.uploadedFiles / state.totalFiles) * 100)
})

// Format bytes to human readable
function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + ' ' + sizes[i]
}

// Format speed
const speedDisplay = computed(() => {
  if (state.isPaused) return 'Paused'
  if (state.uploadSpeed <= 0) return '--'
  return formatBytes(state.uploadSpeed) + '/s'
})

// Format elapsed time
const elapsedDisplay = computed(() => {
  const seconds = elapsedSeconds.value
  if (seconds < 60) return `${seconds}s`
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (mins < 60) return `${mins}m ${secs}s`
  const hours = Math.floor(mins / 60)
  const remainingMins = mins % 60
  return `${hours}h ${remainingMins}m`
})

// Bytes progress display
const bytesDisplay = computed(() => {
  if (state.totalBytes === 0) return ''
  return `${formatBytes(state.uploadedBytes)} / ${formatBytes(state.totalBytes)}`
})

// Album progress display
const albumProgressPercent = computed(() => {
  if (!state.albumStatus || state.albumStatus.TotalItems === 0) return 0
  return Math.round((state.albumStatus.ItemsAdded / state.albumStatus.TotalItems) * 100)
})

function warningFiles(paths: string[]): string {
  return paths.map(path => path.split(/[\\/]/).pop()).filter(Boolean).join(', ')
}
</script>

<template>
  <div class="flex flex-col h-full w-full px-4 pt-4 pb-3 select-none ambient-mesh justify-between">
    <!-- Header with file count and progress -->
    <div class="rounded-2xl border border-white/10 bg-zinc-900/70 backdrop-blur-xl p-3.5 mb-2.5 shadow-xl">
      <div class="flex items-center justify-between">
        <div>
          <div class="flex items-center gap-2">
            <p class="text-2xl font-bold tabular-nums text-zinc-100 tracking-tight">
              {{ state.uploadedFiles }}<span class="text-zinc-500 font-normal text-lg">/</span><span class="text-zinc-400 text-lg">{{ state.totalFiles }}</span>
            </p>
            <span
              v-if="state.isPaused"
              class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/20 text-amber-400 border border-amber-500/30 animate-pulse"
            >
              Tạm dừng
            </span>
          </div>
          <p class="text-[11px] text-zinc-500 mt-0.5">
            tệp đang được xử lý
          </p>
        </div>

        <!-- Live speed and elapsed time -->
        <div class="flex flex-col items-end gap-1 text-xs">
          <div class="flex items-center gap-1.5 px-2 py-0.5 rounded-md bg-white/[0.04] border border-white/5">
            <Zap
              :size="11"
              :class="state.isPaused ? 'text-amber-400' : 'text-emerald-400'"
            />
            <span
              class="tabular-nums font-medium text-[11px]"
              :class="state.isPaused ? 'text-amber-400' : 'text-emerald-300'"
            >
              {{ speedDisplay }}
            </span>
          </div>
          <div class="flex items-center gap-1 text-zinc-400 text-[11px]">
            <Clock
              :size="11"
              class="text-zinc-500"
            />
            <span class="tabular-nums">{{ elapsedDisplay }}</span>
          </div>
        </div>
      </div>

      <!-- Main progress bar -->
      <div class="mt-3">
        <Progress
          :model-value="progressPercent"
          class="h-2 bg-zinc-800"
        />
        <div class="flex justify-between mt-1 text-[11px] text-zinc-400">
          <span v-if="bytesDisplay">{{ bytesDisplay }}</span>
          <span v-else>&nbsp;</span>
          <span class="font-semibold text-emerald-400">{{ progressPercent }}%</span>
        </div>
      </div>
    </div>

    <!-- Album creation progress -->
    <div
      v-if="state.albumStatus && (state.isCreatingAlbum || state.albumStatus.IsComplete)"
      class="mb-2.5 p-3 rounded-xl border border-blue-500/20 bg-blue-500/10 backdrop-blur-md"
    >
      <div class="flex items-center gap-2 mb-1.5">
        <FolderPlus
          :size="14"
          class="text-blue-400"
        />
        <span class="text-xs font-semibold text-blue-200">
          {{ state.isCreatingAlbum ? 'Đang thêm vào album...' : 'Đã thêm vào album' }}
        </span>
      </div>
      <p class="text-[11px] text-zinc-400 mb-1.5 truncate">
        {{ state.albumStatus.AlbumName }}
      </p>
      <Progress
        :model-value="albumProgressPercent"
        class="h-1.5 bg-zinc-800"
      />
      <p class="text-[10px] text-zinc-400 mt-1">
        {{ state.albumStatus.ItemsAdded }} / {{ state.albumStatus.TotalItems }} tệp
      </p>
    </div>

    <!-- Warnings notice -->
    <div
      v-if="state.warnings.length"
      class="mb-2.5 space-y-1.5"
    >
      <div
        v-for="(warning, index) in state.warnings"
        :key="`${warning.Code}-${index}`"
        class="flex gap-2 rounded-xl bg-amber-500/10 border border-amber-500/20 px-2.5 py-2 text-amber-300"
      >
        <AlertTriangle
          :size="13"
          class="mt-0.5 shrink-0 text-amber-400"
        />
        <div class="min-w-0">
          <p class="text-[11px] leading-snug">
            {{ warning.Message }}
          </p>
          <p class="truncate text-[10px] opacity-75">
            {{ warningFiles(warning.Paths) }}
          </p>
        </div>
      </div>
    </div>

    <!-- Active threads list -->
    <div class="flex-1 min-h-0 mb-2.5 rounded-2xl border border-white/10 bg-zinc-900/50 backdrop-blur-xl p-3 flex flex-col shadow-md">
      <div class="flex items-center justify-between mb-2">
        <span class="text-xs font-medium text-zinc-300">
          Luồng xử lý hoạt động
        </span>
        <span class="text-[10px] font-mono px-1.5 py-0.2 rounded-full bg-white/5 text-zinc-400">
          {{ threadsList.length }} luồng
        </span>
      </div>
      <ScrollArea class="h-[calc(100%-24px)]">
        <div class="space-y-1.5 pr-2">
          <ThreadProgress
            v-for="thread in threadsList"
            :key="thread.WorkerID"
            :thread="thread"
          />
        </div>
      </ScrollArea>
    </div>

    <!-- Bandwidth limit controller -->
    <div class="mb-2.5 p-2.5 rounded-xl border border-white/10 bg-zinc-900/60 backdrop-blur-md text-xs select-none shadow-md">
      <div class="flex items-center justify-between mb-1.5">
        <div class="flex items-center gap-1.5 font-medium text-zinc-200">
          <Gauge
            :size="13"
            class="text-emerald-400"
          />
          <span>Giới hạn băng thông</span>
        </div>
        <span class="font-semibold tabular-nums text-emerald-400">
          {{ currentLimitLabel }}
        </span>
      </div>
      <div class="flex items-center gap-2 mb-1.5">
        <input
          v-model.number="selectedLimitMBps"
          type="range"
          min="0"
          max="50"
          step="1"
          class="w-full h-1.5 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-emerald-500"
          @input="onLimitSliderChange"
        >
      </div>
      <div class="flex justify-between gap-1 text-[10px] text-zinc-400">
        <button
          v-for="preset in [0, 2, 5, 10, 20]"
          :key="preset"
          type="button"
          class="px-2 py-0.5 rounded-md border text-[10px] transition-colors cursor-pointer"
          :class="selectedLimitMBps === preset ? 'bg-emerald-500 text-zinc-950 border-emerald-500 font-semibold' : 'bg-white/[0.04] border-white/5 hover:bg-white/[0.08] text-zinc-300'"
          @click="applyPresetLimit(preset)"
        >
          {{ preset === 0 ? 'Không giới hạn' : `${preset} MB/s` }}
        </button>
      </div>
    </div>

    <!-- Action buttons: Pause/Resume + Cancel -->
    <div class="flex gap-2">
      <Button
        :variant="state.isPaused ? 'default' : 'outline'"
        size="sm"
        class="flex-1 h-9 rounded-xl cursor-pointer select-none transition-all text-xs font-medium border-white/10"
        :class="state.isPaused ? 'bg-amber-600 hover:bg-amber-500 text-white' : 'bg-zinc-900/80 hover:bg-zinc-800 text-zinc-200'"
        @click="togglePause"
      >
        <component
          :is="state.isPaused ? Play : Pause"
          :size="13"
          class="mr-1.5"
        />
        {{ state.isPaused ? 'Tiếp tục' : 'Tạm dừng' }}
      </Button>

      <Button
        variant="destructive"
        size="sm"
        class="flex-1 h-9 rounded-xl cursor-pointer select-none text-xs font-medium shadow-md shadow-red-500/10"
        @click="() => uploadManager.cancelUpload()"
      >
        <X
          :size="13"
          class="mr-1.5"
        />
        Hủy tải lên
      </Button>
    </div>
  </div>
</template>
