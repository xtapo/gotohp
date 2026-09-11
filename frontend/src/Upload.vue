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
  <div class="flex flex-col h-full w-full px-4 pt-6 pb-4">
    <!-- Header with file count -->
    <div class="text-center mb-3">
      <div class="flex items-center justify-center gap-2">
        <p class="text-2xl font-bold tabular-nums">
          {{ state.uploadedFiles }}<span class="text-muted-foreground font-normal">/</span><span class="text-muted-foreground">{{ state.totalFiles }}</span>
        </p>
        <span
          v-if="state.isPaused"
          class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-amber-500/20 text-amber-500 border border-amber-500/30 animate-pulse"
        >
          Paused
        </span>
      </div>
      <p class="text-xs text-muted-foreground mt-0.5">
        items processed
      </p>
    </div>

    <!-- Stats row -->
    <div class="flex gap-3 justify-center mb-3 text-xs text-muted-foreground">
      <div class="flex items-center gap-1.5">
        <Zap
          :size="12"
          :class="state.isPaused ? 'text-amber-500' : 'text-primary'"
        />
        <span
          class="tabular-nums font-medium"
          :class="state.isPaused ? 'text-amber-500' : 'text-foreground'"
        >
          {{ speedDisplay }}
        </span>
      </div>
      <span class="text-border">|</span>
      <div class="flex items-center gap-1.5">
        <Clock :size="12" />
        <span class="tabular-nums text-foreground">{{ elapsedDisplay }}</span>
      </div>
    </div>

    <!-- Main progress bar -->
    <div class="mb-3">
      <Progress
        :model-value="progressPercent"
        class="h-2.5"
      />
      <div class="flex justify-between mt-1.5 text-xs text-muted-foreground">
        <span v-if="bytesDisplay">{{ bytesDisplay }}</span>
        <span v-else>&nbsp;</span>
        <span class="font-medium">{{ progressPercent }}%</span>
      </div>
    </div>

    <!-- Album creation progress -->
    <div
      v-if="state.albumStatus && (state.isCreatingAlbum || state.albumStatus.IsComplete)"
      class="mb-3 p-3 rounded-lg border bg-muted/30"
    >
      <div class="flex items-center gap-2 mb-2">
        <FolderPlus
          :size="14"
          class="text-primary"
        />
        <span class="text-sm font-medium">
          {{ state.isCreatingAlbum ? 'Adding to album...' : 'Added to album' }}
        </span>
      </div>
      <p class="text-xs text-muted-foreground mb-1.5">
        {{ state.albumStatus.AlbumName }}
      </p>
      <Progress
        :model-value="albumProgressPercent"
        class="h-1.5"
      />
      <p class="text-xs text-muted-foreground mt-1">
        {{ state.albumStatus.ItemsAdded }} / {{ state.albumStatus.TotalItems }} items
      </p>
    </div>

    <div
      v-if="state.warnings.length"
      class="mb-3 space-y-1.5"
    >
      <div
        v-for="(warning, index) in state.warnings"
        :key="`${warning.Code}-${index}`"
        class="flex gap-2 rounded-md bg-amber-500/10 px-2.5 py-2 text-amber-700 dark:text-amber-300"
      >
        <AlertTriangle
          :size="14"
          class="mt-0.5 shrink-0"
        />
        <div class="min-w-0">
          <p class="text-xs leading-snug">
            {{ warning.Message }}
          </p>
          <p class="truncate text-[10px] opacity-75">
            {{ warningFiles(warning.Paths) }}
          </p>
        </div>
      </div>
    </div>

    <!-- Thread list - scrollable -->
    <div class="flex-1 min-h-0 mb-3">
      <p class="text-xs text-muted-foreground mb-1.5">
        Active threads ({{ threadsList.length }})
      </p>
      <ScrollArea class="h-[calc(100%-20px)]">
        <div class="space-y-1.5 pr-3">
          <ThreadProgress
            v-for="thread in threadsList"
            :key="thread.WorkerID"
            :thread="thread"
          />
        </div>
      </ScrollArea>
    </div>

    <!-- Bandwidth limit controller -->
    <div class="mb-3 p-2.5 rounded-lg border bg-muted/20 text-xs select-none">
      <div class="flex items-center justify-between mb-1.5">
        <div class="flex items-center gap-1.5 font-medium text-foreground">
          <Gauge
            :size="13"
            class="text-primary"
          />
          <span>Bandwidth Limit</span>
        </div>
        <span class="font-semibold tabular-nums text-primary">
          {{ currentLimitLabel }}
        </span>
      </div>
      <div class="flex items-center gap-2 mb-1.5">
        <input
          type="range"
          min="0"
          max="50"
          step="1"
          v-model.number="selectedLimitMBps"
          class="w-full h-1.5 bg-muted rounded-lg appearance-none cursor-pointer accent-primary"
          @input="onLimitSliderChange"
        >
      </div>
      <div class="flex justify-between gap-1 text-[10px] text-muted-foreground">
        <button
          v-for="preset in [0, 2, 5, 10, 20]"
          :key="preset"
          type="button"
          class="px-1.5 py-0.5 rounded border text-[10px] transition-colors cursor-pointer"
          :class="selectedLimitMBps === preset ? 'bg-primary text-primary-foreground border-primary font-medium' : 'bg-background hover:bg-muted'"
          @click="applyPresetLimit(preset)"
        >
          {{ preset === 0 ? 'Unlimited' : `${preset} MB/s` }}
        </button>
      </div>
    </div>

    <!-- Action buttons: Pause/Resume + Cancel -->
    <div class="flex gap-2">
      <Button
        :variant="state.isPaused ? 'default' : 'outline'"
        size="sm"
        class="flex-1 cursor-pointer select-none transition-colors"
        :class="state.isPaused ? 'bg-amber-600 hover:bg-amber-700 text-white' : ''"
        @click="togglePause"
      >
        <component
          :is="state.isPaused ? Play : Pause"
          :size="14"
          class="mr-1.5"
        />
        {{ state.isPaused ? 'Resume' : 'Pause' }}
      </Button>

      <Button
        variant="destructive"
        size="sm"
        class="flex-1 cursor-pointer select-none"
        @click="() => uploadManager.cancelUpload()"
      >
        <X
          :size="14"
          class="mr-1.5"
        />
        Cancel Upload
      </Button>
    </div>
  </div>
</template>
