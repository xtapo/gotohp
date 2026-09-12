<script setup lang="ts">
import { nextTick, ref, onMounted, watch } from 'vue'
import { ConfigManager } from '../bindings/app/backend'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip'
import {
    NumberField,
    NumberFieldContent,
    NumberFieldDecrement,
    NumberFieldIncrement,
    NumberFieldInput,
} from '@/components/ui/number-field'
import {
    Info,
    Filter,
    HardDrive,
    Trash2,
    Archive,
    ShieldAlert,
    AlertTriangle,
    Image,
    Video,
    Sparkles,
    Check,
} from '@lucide/vue'

const emit = defineEmits<{
    'open-auto-sync': []
}>()

function handleOpenAutoSync() {
    emit('open-auto-sync')
}

interface Settings {
    proxy: string
    useQuota: boolean
    saver: boolean
    recursive: boolean
    forceUpload: boolean
    pairLivePhotos: boolean
    skipIncompleteLivePhotos: boolean
    updateExistingPhotosToLive: boolean
    deleteFromHost: boolean
    disableUnsupportedFilesFilter: boolean
    setDateFromFilename: boolean
    uploadThreads: number
    maxUploadSpeedMBps: number
    startWithWindows: boolean

    // Smart File Filters
    filterIncludePhotos: boolean
    filterIncludeVideos: boolean
    filterIncludeRaw: boolean
    filterIncludeHeic: boolean
    filterIncludeGif: boolean
    minFileSizeKB: number
    maxVideoSizeMB: number

    // Post-Upload / Free Up Space Actions
    postUploadAction: string
    backupFolder: string
}

type BooleanSetting = Exclude<
    keyof Settings,
    'proxy' | 'uploadThreads' | 'maxUploadSpeedMBps' | 'minFileSizeKB' | 'maxVideoSizeMB' | 'postUploadAction' | 'backupFolder'
>

const settings = ref<Settings>({
    proxy: '',
    useQuota: false,
    saver: false,
    recursive: false,
    forceUpload: false,
    pairLivePhotos: false,
    skipIncompleteLivePhotos: true,
    updateExistingPhotosToLive: false,
    deleteFromHost: false,
    disableUnsupportedFilesFilter: false,
    setDateFromFilename: false,
    uploadThreads: 0,
    maxUploadSpeedMBps: 0,
    startWithWindows: false,

    filterIncludePhotos: true,
    filterIncludeVideos: true,
    filterIncludeRaw: true,
    filterIncludeHeic: true,
    filterIncludeGif: true,
    minFileSizeKB: 0,
    maxVideoSizeMB: 0,
    postUploadAction: 'none',
    backupFolder: '',
})

const isHydrating = ref(true)
const showSafetyDialog = ref(false)
const pendingAction = ref('')

const minSizeFilterEnabled = ref(false)
const maxVideoFilterEnabled = ref(false)

const toggleSetting = (setting: BooleanSetting, enabled = true) => {
    if (!enabled) return
    settings.value[setting] = !settings.value[setting]
}

function setFilterMode(mode: 'both' | 'photos' | 'videos') {
    if (mode === 'both') {
        settings.value.filterIncludePhotos = true
        settings.value.filterIncludeVideos = true
    } else if (mode === 'photos') {
        settings.value.filterIncludePhotos = true
        settings.value.filterIncludeVideos = false
    } else if (mode === 'videos') {
        settings.value.filterIncludePhotos = false
        settings.value.filterIncludeVideos = true
    }
}

function onPostUploadActionSelect(action: string) {
    if (action === 'recycle' || action === 'delete') {
        pendingAction.value = action
        showSafetyDialog.value = true
    } else {
        settings.value.postUploadAction = action
    }
}

function confirmSafetyAction() {
    settings.value.postUploadAction = pendingAction.value
    showSafetyDialog.value = false
}

function cancelSafetyAction() {
    showSafetyDialog.value = false
    pendingAction.value = ''
}

onMounted(async () => {
    try {
        const config = await ConfigManager.GetSettings()
        settings.value = {
            proxy: config.proxy || '',
            useQuota: config.useQuota || false,
            saver: config.saver || false,
            recursive: config.recursive || false,
            forceUpload: config.forceUpload || false,
            pairLivePhotos: config.pairLivePhotos || false,
            skipIncompleteLivePhotos: config.skipIncompleteLivePhotos ?? true,
            updateExistingPhotosToLive: config.updateExistingPhotosToLive || false,
            deleteFromHost: config.deleteFromHost || false,
            disableUnsupportedFilesFilter: config.disableUnsupportedFilesFilter || false,
            setDateFromFilename: config.setDateFromFilename || false,
            uploadThreads: config.uploadThreads || 1,
            maxUploadSpeedMBps: config.maxUploadSpeedMBps || 0,
            startWithWindows: config.startWithWindows || false,

            filterIncludePhotos: config.filterIncludePhotos ?? true,
            filterIncludeVideos: config.filterIncludeVideos ?? true,
            filterIncludeRaw: config.filterIncludeRaw ?? true,
            filterIncludeHeic: config.filterIncludeHeic ?? true,
            filterIncludeGif: config.filterIncludeGif ?? true,
            minFileSizeKB: config.minFileSizeKB || 0,
            maxVideoSizeMB: config.maxVideoSizeMB || 0,
            postUploadAction: config.postUploadAction || (config.deleteFromHost ? 'delete' : 'none'),
            backupFolder: config.backupFolder || '',
        }

        minSizeFilterEnabled.value = (config.minFileSizeKB || 0) > 0
        maxVideoFilterEnabled.value = (config.maxVideoSizeMB || 0) > 0
    } finally {
        await nextTick()
        isHydrating.value = false
    }
})

// Watchers
watch(() => settings.value.proxy, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetProxy(newValue)
})

watch(() => settings.value.maxUploadSpeedMBps, async (newValue) => {
    if (isHydrating.value) return
    const speed = newValue < 0 ? 0 : newValue
    await ConfigManager.SetMaxUploadSpeedMBps(speed)
})

watch(() => settings.value.useQuota, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetUseQuota(newValue)
})

watch(() => settings.value.saver, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetSaver(newValue)
})

watch(() => settings.value.recursive, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetRecursive(newValue)
})

watch(() => settings.value.forceUpload, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetForceUpload(newValue)
})

watch(() => settings.value.pairLivePhotos, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetPairLivePhotos(newValue)
    if (newValue && !settings.value.skipIncompleteLivePhotos) {
        settings.value.skipIncompleteLivePhotos = true
        await ConfigManager.SetSkipIncompleteLivePhotos(true)
    }
})

watch(() => settings.value.skipIncompleteLivePhotos, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetSkipIncompleteLivePhotos(newValue)
})

watch(() => settings.value.updateExistingPhotosToLive, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetUpdateExistingPhotosToLive(newValue)
})

watch(() => settings.value.disableUnsupportedFilesFilter, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetDisableUnsupportedFilesFilter(newValue)
})

watch(() => settings.value.setDateFromFilename, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetSetDateFromFilename(newValue)
})

watch(() => settings.value.uploadThreads, async (newValue) => {
    if (isHydrating.value) return
    if (newValue < 1) {
        settings.value.uploadThreads = 1
    } else {
        await ConfigManager.SetUploadThreads(newValue)
    }
})

watch(() => settings.value.startWithWindows, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetStartWithWindows(newValue)
})

// Filter Watchers
watch(() => settings.value.filterIncludePhotos, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetFilterIncludePhotos(newValue)
})

watch(() => settings.value.filterIncludeVideos, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetFilterIncludeVideos(newValue)
})

watch(() => settings.value.filterIncludeRaw, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetFilterIncludeRaw(newValue)
})

watch(() => settings.value.filterIncludeHeic, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetFilterIncludeHeic(newValue)
})

watch(() => settings.value.filterIncludeGif, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetFilterIncludeGif(newValue)
})

watch(minSizeFilterEnabled, (enabled) => {
    if (enabled && settings.value.minFileSizeKB <= 0) {
        settings.value.minFileSizeKB = 50
    } else if (!enabled) {
        settings.value.minFileSizeKB = 0
    }
})

watch(() => settings.value.minFileSizeKB, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetMinFileSizeKB(newValue)
})

watch(maxVideoFilterEnabled, (enabled) => {
    if (enabled && settings.value.maxVideoSizeMB <= 0) {
        settings.value.maxVideoSizeMB = 2048
    } else if (!enabled) {
        settings.value.maxVideoSizeMB = 0
    }
})

watch(() => settings.value.maxVideoSizeMB, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetMaxVideoSizeMB(newValue)
})

// Post Upload Action Watchers
watch(() => settings.value.postUploadAction, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetPostUploadAction(newValue)
    settings.value.deleteFromHost = (newValue === 'delete')
})

watch(() => settings.value.backupFolder, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetBackupFolder(newValue)
})
</script>

<template>
  <div
    class="flex flex-col gap-4 m-4 pb-6"
    style="--wails-draggable: none"
  >
    <!-- SECTION 1: CẤU HÌNH TẢI LÊN (GENERAL UPLOAD PREFERENCES) -->
    <div class="flex flex-col gap-2.5">
      <div class="flex items-center gap-2 pb-1 border-b border-white/10">
        <Sparkles class="size-4 text-emerald-400" />
        <h3 class="text-xs font-semibold uppercase tracking-wider text-zinc-300">
          Cài Đặt Tải Lên
        </h3>
      </div>

      <!-- Threads -->
      <NumberField
        v-model="settings.uploadThreads"
        class="flex items-center justify-between"
      >
        <Label
          for="upload-threads"
          class="size-full"
        >Số Luồng Tải Lên (Threads)</Label>
        <NumberFieldContent>
          <NumberFieldDecrement
            class="cursor-pointer"
            :disabled="settings.uploadThreads <= 1"
          />
          <NumberFieldInput />
          <NumberFieldIncrement class="cursor-pointer" />
        </NumberFieldContent>
      </NumberField>

      <!-- Max Speed -->
      <div class="flex flex-col gap-1.5 p-3 rounded-xl border border-white/10 bg-white/[0.02] select-none">
        <div class="flex items-center justify-between">
          <Label
            for="max-upload-speed"
            class="cursor-pointer"
          >Giới Hạn Tốc Độ Tải</Label>
          <span class="text-xs font-semibold tabular-nums text-primary">
            {{ settings.maxUploadSpeedMBps > 0 ? `${settings.maxUploadSpeedMBps} MB/s` : 'Không giới hạn' }}
          </span>
        </div>
        <div class="flex items-center gap-2">
          <input
            id="max-upload-speed"
            v-model.number="settings.maxUploadSpeedMBps"
            type="range"
            min="0"
            max="50"
            step="1"
            class="w-full h-1.5 bg-muted rounded-lg appearance-none cursor-pointer accent-primary"
          >
        </div>
        <div class="flex justify-between gap-1 text-[10px] text-muted-foreground">
          <button
            v-for="preset in [0, 2, 5, 10, 20]"
            :key="preset"
            type="button"
            class="px-2 py-0.5 rounded border text-[10px] transition-colors cursor-pointer"
            :class="settings.maxUploadSpeedMBps === preset ? 'bg-primary text-primary-foreground border-primary font-medium' : 'bg-background hover:bg-muted'"
            @click="settings.maxUploadSpeedMBps = preset"
          >
            {{ preset === 0 ? 'Tối đa' : `${preset} MB/s` }}
          </button>
        </div>
      </div>

      <!-- Switches -->
      <div class="flex items-center justify-between">
        <Label
          for="use-quota"
          class="size-full cursor-pointer"
        >Tính Vào Dung Lượng Google (Use Quota)</Label>
        <Switch
          id="use-quota"
          v-model="settings.useQuota"
        />
      </div>

      <div class="flex items-center justify-between">
        <Label
          for="saver-mode"
          class="size-full cursor-pointer"
        >Chất Lượng Tiết Kiệm (Storage Saver)</Label>
        <Switch
          id="saver-mode"
          v-model="settings.saver"
        />
      </div>

      <div class="flex items-center justify-between">
        <Label
          for="recursive"
          class="size-full cursor-pointer"
        >Quét Cả Thư Mục Con (Recursive)</Label>
        <Switch
          id="recursive"
          v-model="settings.recursive"
        />
      </div>

      <div class="flex items-center justify-between">
        <div
          class="flex min-w-0 flex-1 cursor-pointer items-center gap-1.5"
          @click.self="toggleSetting('forceUpload')"
        >
          <Label
            for="force-upload"
            class="cursor-pointer"
          >Tải Lên Bắt Buộc (Bỏ Qua Check Trùng)</Label>
          <Tooltip>
            <TooltipTrigger as-child>
              <button
                type="button"
                class="inline-flex size-5 shrink-0 cursor-help items-center justify-center rounded-full text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                aria-label="About Force Upload"
              >
                <Info class="size-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent class="max-w-72">
              Bỏ qua bước kiểm tra hash trên Google Photos và luôn tải lên file mới.
            </TooltipContent>
          </Tooltip>
        </div>
        <Switch
          id="force-upload"
          v-model="settings.forceUpload"
        />
      </div>

      <!-- Live Photos -->
      <div class="flex items-center justify-between">
        <div
          class="flex min-w-0 flex-1 cursor-pointer items-center gap-1.5 pr-4"
          @click.self="toggleSetting('pairLivePhotos')"
        >
          <Label
            for="pair-live-photos"
            class="cursor-pointer"
          >Ghép Cặp Apple Live Photo</Label>
          <Tooltip>
            <TooltipTrigger as-child>
              <button
                type="button"
                class="inline-flex size-5 shrink-0 cursor-help items-center justify-center rounded-full text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              >
                <Info class="size-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent class="max-w-72">
              Tự động nhận diện ảnh và video MOV đi kèm để tạo Live Photo hoàn chỉnh.
            </TooltipContent>
          </Tooltip>
        </div>
        <Switch
          id="pair-live-photos"
          v-model="settings.pairLivePhotos"
        />
      </div>

      <div
        v-if="settings.pairLivePhotos"
        class="flex flex-col gap-2 pl-3 border-l-2 border-primary/30"
      >
        <div class="flex items-center justify-between">
          <Label
            for="skip-incomplete-live"
            class="cursor-pointer text-xs"
          >Bỏ qua Live Photo thiếu cặp (chỉ có ảnh hoặc video)</Label>
          <Switch
            id="skip-incomplete-live"
            v-model="settings.skipIncompleteLivePhotos"
          />
        </div>
        <div class="flex items-center justify-between">
          <Label
            for="update-existing-live"
            class="cursor-pointer text-xs"
          >Nâng cấp ảnh tĩnh đã có thành Live Photo</Label>
          <Switch
            id="update-existing-live"
            v-model="settings.updateExistingPhotosToLive"
          />
        </div>
      </div>

      <div class="flex items-center justify-between">
        <Label
          for="set-date-from-filename"
          class="size-full cursor-pointer"
        >Lấy Ngày Chụp Theo Tên File</Label>
        <Switch
          id="set-date-from-filename"
          v-model="settings.setDateFromFilename"
        />
      </div>

      <div>
        <Input
          v-model="settings.proxy"
          type="text"
          placeholder="Proxy URL (tùy chọn, vd: socks5://127.0.0.1:1080)"
        />
      </div>

      <div class="flex items-center justify-between">
        <div class="flex flex-col">
          <Label
            for="start-with-windows"
            class="cursor-pointer font-medium"
          >Khởi Động Cùng Windows</Label>
          <span class="text-xs text-muted-foreground">Chạy ẩn dưới khay hệ thống khi bật máy</span>
        </div>
        <Switch
          id="start-with-windows"
          v-model="settings.startWithWindows"
        />
      </div>
    </div>

    <!-- SECTION 2: BỘ LỌC ĐỊNH DẠNG & DUNG LƯỢNG (DETAILED UPLOAD FILTERS) -->
    <div class="flex flex-col gap-3 p-3.5 rounded-2xl border border-blue-500/20 bg-blue-500/[0.03]">
      <div class="flex items-center justify-between pb-2 border-b border-white/10">
        <div class="flex items-center gap-2">
          <Filter class="size-4 text-blue-400" />
          <h3 class="text-xs font-semibold uppercase tracking-wider text-blue-300">
            Bộ Lọc Định Dạng & Dung Lượng
          </h3>
        </div>
        <!-- Quick Mode Presets -->
        <div class="flex items-center gap-1">
          <button
            type="button"
            class="px-2 py-0.5 rounded text-[10px] font-medium border cursor-pointer transition-colors"
            :class="settings.filterIncludePhotos && settings.filterIncludeVideos ? 'bg-blue-600 text-white border-blue-500' : 'bg-white/5 text-zinc-400 hover:text-white border-white/10'"
            @click="setFilterMode('both')"
          >
            Cả hai
          </button>
          <button
            type="button"
            class="px-2 py-0.5 rounded text-[10px] font-medium border cursor-pointer transition-colors"
            :class="settings.filterIncludePhotos && !settings.filterIncludeVideos ? 'bg-blue-600 text-white border-blue-500' : 'bg-white/5 text-zinc-400 hover:text-white border-white/10'"
            @click="setFilterMode('photos')"
          >
            Chỉ Ảnh
          </button>
          <button
            type="button"
            class="px-2 py-0.5 rounded text-[10px] font-medium border cursor-pointer transition-colors"
            :class="!settings.filterIncludePhotos && settings.filterIncludeVideos ? 'bg-blue-600 text-white border-blue-500' : 'bg-white/5 text-zinc-400 hover:text-white border-white/10'"
            @click="setFilterMode('videos')"
          >
            Chỉ Video
          </button>
        </div>
      </div>

      <!-- Type Toggles -->
      <div class="grid grid-cols-2 gap-2">
        <div class="flex items-center justify-between p-2 rounded-xl bg-white/[0.02] border border-white/5">
          <div class="flex items-center gap-1.5">
            <Image class="size-3.5 text-emerald-400" />
            <Label
              for="filter-photos"
              class="text-xs cursor-pointer"
            >Tải Ảnh</Label>
          </div>
          <Switch
            id="filter-photos"
            v-model="settings.filterIncludePhotos"
          />
        </div>

        <div class="flex items-center justify-between p-2 rounded-xl bg-white/[0.02] border border-white/5">
          <div class="flex items-center gap-1.5">
            <Video class="size-3.5 text-indigo-400" />
            <Label
              for="filter-videos"
              class="text-xs cursor-pointer"
            >Tải Video</Label>
          </div>
          <Switch
            id="filter-videos"
            v-model="settings.filterIncludeVideos"
          />
        </div>
      </div>

      <!-- Sub-format Toggles (RAW, HEIC, GIF) -->
      <div
        class="flex flex-col gap-2 pt-1 border-t border-white/5 transition-opacity"
        :class="settings.filterIncludePhotos ? 'opacity-100' : 'opacity-40 pointer-events-none'"
      >
        <div class="text-[11px] font-medium text-zinc-400">
          Định dạng ảnh chuyên sâu:
        </div>
        <div class="flex items-center justify-between">
          <div class="flex flex-col">
            <Label
              for="filter-raw"
              class="cursor-pointer text-xs"
            >Máy Ảnh Chuyên Nghiệp (RAW)</Label>
            <span class="text-[10px] text-zinc-500">CR2, CR3, NEF, ARW, DNG, ORF, RW2...</span>
          </div>
          <Switch
            id="filter-raw"
            v-model="settings.filterIncludeRaw"
            :disabled="!settings.filterIncludePhotos"
          />
        </div>

        <div class="flex items-center justify-between">
          <div class="flex flex-col">
            <Label
              for="filter-heic"
              class="cursor-pointer text-xs"
            >Ảnh Apple HEIC / HEIF</Label>
            <span class="text-[10px] text-zinc-500">Định dạng ảnh iPhone, iPad (.heic, .heif)</span>
          </div>
          <Switch
            id="filter-heic"
            v-model="settings.filterIncludeHeic"
            :disabled="!settings.filterIncludePhotos"
          />
        </div>

        <div class="flex items-center justify-between">
          <div class="flex flex-col">
            <Label
              for="filter-gif"
              class="cursor-pointer text-xs"
            >Ảnh Động GIF</Label>
            <span class="text-[10px] text-zinc-500">Cho phép hoặc bỏ qua ảnh động .gif</span>
          </div>
          <Switch
            id="filter-gif"
            v-model="settings.filterIncludeGif"
            :disabled="!settings.filterIncludePhotos"
          />
        </div>
      </div>

      <!-- Size Filters -->
      <div class="flex flex-col gap-3 pt-2 border-t border-white/5">
        <!-- Min Size Filter -->
        <div class="flex flex-col gap-1.5">
          <div class="flex items-center justify-between">
            <div class="flex flex-col">
              <Label
                for="min-size-toggle"
                class="cursor-pointer text-xs font-medium"
              >Bỏ Qua File Quá Nhỏ</Label>
              <span class="text-[10px] text-zinc-500">Lọc bỏ icon rác, thumbnail cache</span>
            </div>
            <Switch
              id="min-size-toggle"
              v-model="minSizeFilterEnabled"
            />
          </div>
          <div
            v-if="minSizeFilterEnabled"
            class="flex items-center gap-2 mt-1"
          >
            <Input
              v-model.number="settings.minFileSizeKB"
              type="number"
              min="1"
              max="10000"
              class="h-8 text-xs w-28"
            />
            <span class="text-xs text-zinc-400">KB</span>
            <div class="flex gap-1 ml-auto">
              <button
                v-for="preset in [20, 50, 100]"
                :key="preset"
                type="button"
                class="px-2 py-0.5 rounded border text-[10px] cursor-pointer"
                :class="settings.minFileSizeKB === preset ? 'bg-primary text-primary-foreground border-primary' : 'bg-white/5 border-white/10 text-zinc-400'"
                @click="settings.minFileSizeKB = preset"
              >
                {{ preset }} KB
              </button>
            </div>
          </div>
        </div>

        <!-- Max Video Size Filter -->
        <div class="flex flex-col gap-1.5">
          <div class="flex items-center justify-between">
            <div class="flex flex-col">
              <Label
                for="max-video-toggle"
                class="cursor-pointer text-xs font-medium"
              >Bỏ Qua Video Quá Lớn</Label>
              <span class="text-[10px] text-zinc-500">Tránh video tốn băng thông hoặc vượt hạn mức</span>
            </div>
            <Switch
              id="max-video-toggle"
              v-model="maxVideoFilterEnabled"
            />
          </div>
          <div
            v-if="maxVideoFilterEnabled"
            class="flex items-center gap-2 mt-1"
          >
            <Input
              v-model.number="settings.maxVideoSizeMB"
              type="number"
              min="50"
              max="50000"
              class="h-8 text-xs w-28"
            />
            <span class="text-xs text-zinc-400">MB ({{ (settings.maxVideoSizeMB / 1024).toFixed(1) }} GB)</span>
            <div class="flex gap-1 ml-auto">
              <button
                v-for="preset in [1024, 2048, 4096]"
                :key="preset"
                type="button"
                class="px-2 py-0.5 rounded border text-[10px] cursor-pointer"
                :class="settings.maxVideoSizeMB === preset ? 'bg-primary text-primary-foreground border-primary' : 'bg-white/5 border-white/10 text-zinc-400'"
                @click="settings.maxVideoSizeMB = preset"
              >
                {{ preset / 1024 }} GB
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- SECTION 3: GIẢI PHÓNG DUNG LƯỢNG MÁY TÍNH (FREE UP SPACE / POST-UPLOAD ACTIONS) -->
    <div class="flex flex-col gap-3 p-3.5 rounded-2xl border border-amber-500/20 bg-amber-500/[0.03]">
      <div class="flex items-center justify-between pb-1.5 border-b border-white/10">
        <div class="flex items-center gap-2">
          <HardDrive class="size-4 text-amber-400" />
          <h3 class="text-xs font-semibold uppercase tracking-wider text-amber-300">
            Giải Phóng Dung Lượng Máy Tính
          </h3>
        </div>
      </div>

      <p class="text-[11px] text-zinc-400 leading-relaxed">
        Tương tự tính năng "Giải phóng bộ nhớ" trên điện thoại: Sau khi Google Photos xác nhận file đã tải lên thành công (khớp mã hash), tự động xử lý file gốc:
      </p>

      <!-- Action Options Grid -->
      <div class="flex flex-col gap-2">
        <!-- Option 1: None -->
        <div
          class="flex items-center justify-between p-2.5 rounded-xl border cursor-pointer transition-all"
          :class="settings.postUploadAction === 'none' ? 'bg-white/[0.08] border-white/20 shadow-sm' : 'bg-white/[0.02] border-white/5 hover:border-white/10'"
          @click="onPostUploadActionSelect('none')"
        >
          <div class="flex items-center gap-2.5">
            <div class="size-7 rounded-lg bg-zinc-800 flex items-center justify-center text-zinc-400">
              <ShieldAlert class="size-4" />
            </div>
            <div class="flex flex-col">
              <span class="text-xs font-medium text-zinc-200">Giữ nguyên file gốc</span>
              <span class="text-[10px] text-zinc-500">Mặc định an toàn nhất, không thao tác gì với máy</span>
            </div>
          </div>
          <Check
            v-if="settings.postUploadAction === 'none'"
            class="size-4 text-emerald-400"
          />
        </div>

        <!-- Option 2: Move to _BackedUp -->
        <div
          class="flex items-center justify-between p-2.5 rounded-xl border cursor-pointer transition-all"
          :class="settings.postUploadAction === 'backup' ? 'bg-blue-500/10 border-blue-500/30 shadow-sm' : 'bg-white/[0.02] border-white/5 hover:border-white/10'"
          @click="onPostUploadActionSelect('backup')"
        >
          <div class="flex items-center gap-2.5">
            <div class="size-7 rounded-lg bg-blue-500/20 text-blue-400 flex items-center justify-center">
              <Archive class="size-4" />
            </div>
            <div class="flex flex-col">
              <span class="text-xs font-medium text-blue-200">Di chuyển sang thư mục lưu trữ (_BackedUp/)</span>
              <span class="text-[10px] text-zinc-500">Tự động gom file đã tải vào thư mục _BackedUp</span>
            </div>
          </div>
          <Check
            v-if="settings.postUploadAction === 'backup'"
            class="size-4 text-blue-400"
          />
        </div>

        <!-- Option 3: Move to Windows Recycle Bin -->
        <div
          class="flex items-center justify-between p-2.5 rounded-xl border cursor-pointer transition-all"
          :class="settings.postUploadAction === 'recycle' ? 'bg-amber-500/10 border-amber-500/30 shadow-sm' : 'bg-white/[0.02] border-white/5 hover:border-white/10'"
          @click="onPostUploadActionSelect('recycle')"
        >
          <div class="flex items-center gap-2.5">
            <div class="size-7 rounded-lg bg-amber-500/20 text-amber-400 flex items-center justify-center">
              <Trash2 class="size-4" />
            </div>
            <div class="flex flex-col">
              <span class="text-xs font-medium text-amber-200">Chuyển vào Thùng rác Windows (Recycle Bin)</span>
              <span class="text-[10px] text-zinc-500">An toàn: có thể hoàn tác khôi phục từ Recycle Bin bất cứ lúc nào</span>
            </div>
          </div>
          <Check
            v-if="settings.postUploadAction === 'recycle'"
            class="size-4 text-amber-400"
          />
        </div>

        <!-- Option 4: Permanent Delete -->
        <div
          class="flex items-center justify-between p-2.5 rounded-xl border cursor-pointer transition-all"
          :class="settings.postUploadAction === 'delete' ? 'bg-red-500/10 border-red-500/30 shadow-sm' : 'bg-white/[0.02] border-white/5 hover:border-white/10'"
          @click="onPostUploadActionSelect('delete')"
        >
          <div class="flex items-center gap-2.5">
            <div class="size-7 rounded-lg bg-red-500/20 text-red-400 flex items-center justify-center">
              <AlertTriangle class="size-4" />
            </div>
            <div class="flex flex-col">
              <span class="text-xs font-medium text-red-300">Xóa vĩnh viễn khỏi máy</span>
              <span class="text-[10px] text-zinc-500">Xóa file trực tiếp, không vào Thùng rác (cẩn thận!)</span>
            </div>
          </div>
          <Check
            v-if="settings.postUploadAction === 'delete'"
            class="size-4 text-red-400"
          />
        </div>
      </div>

      <!-- Custom Backup Folder path (if backup selected) -->
      <div
        v-if="settings.postUploadAction === 'backup'"
        class="flex flex-col gap-1.5 pt-2 border-t border-white/5"
      >
        <Label
          for="backup-folder-path"
          class="text-xs text-zinc-300"
        >Đường dẫn thư mục lưu trữ (Để trống sẽ tạo _BackedUp tại chỗ):</Label>
        <Input
          id="backup-folder-path"
          v-model="settings.backupFolder"
          type="text"
          placeholder="Mặc định: thư_mục_gốc\_BackedUp\"
          class="h-8 text-xs bg-zinc-900 border-white/15"
        />
      </div>
    </div>

    <!-- AUTO SYNC LINK -->
    <div class="flex items-center justify-between pt-2 border-t border-white/10">
      <div class="flex flex-col">
        <Label
          class="cursor-pointer font-medium"
          @click.stop="handleOpenAutoSync"
        >
          Đồng Bộ Tự Động Thư Mục (Auto-Sync)
        </Label>
        <span class="text-xs text-muted-foreground">Theo dõi thư mục và tải lên nền tự động</span>
      </div>
      <Button
        type="button"
        size="sm"
        variant="outline"
        class="h-7 text-xs cursor-pointer select-none"
        @click.stop="handleOpenAutoSync"
      >
        Quản lý
      </Button>
    </div>

    <!-- SAFETY CONFIRMATION DIALOG MODAL -->
    <div
      v-if="showSafetyDialog"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4"
    >
      <div class="w-full max-w-sm rounded-2xl border border-white/15 bg-zinc-900 p-5 shadow-2xl flex flex-col gap-4 text-center animate-in fade-in zoom-in-95 duration-200">
        <div class="size-12 rounded-2xl bg-amber-500/10 border border-amber-500/20 text-amber-400 flex items-center justify-center mx-auto">
          <AlertTriangle class="size-6" />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-zinc-100">
            Xác Nhận An Toàn Dữ Liệu
          </h3>
          <p class="text-xs text-zinc-400 mt-1 leading-relaxed">
            Bạn vừa chọn chế độ:
            <span
              class="font-semibold"
              :class="pendingAction === 'delete' ? 'text-red-400' : 'text-amber-400'"
            >
              {{ pendingAction === 'delete' ? 'Xóa vĩnh viễn' : 'Chuyển vào Thùng rác Windows' }}
            </span>.
          </p>
          <div class="mt-3 p-2.5 rounded-xl bg-white/[0.03] border border-white/10 text-left text-[11px] text-zinc-300 space-y-1.5">
            <div class="flex items-start gap-1.5">
              <Check class="size-3.5 text-emerald-400 shrink-0 mt-0.5" />
              <span>File chỉ được dọn dẹp <b>SAU KHI Google Photos xác nhận</b> đã tải lên thành công và khớp mã hash.</span>
            </div>
            <div
              v-if="pendingAction === 'recycle'"
              class="flex items-start gap-1.5"
            >
              <Check class="size-3.5 text-emerald-400 shrink-0 mt-0.5" />
              <span>File trong Thùng rác Windows có thể bấm chuột phải chọn <b>Restore</b> để lấy lại nguyên trạng.</span>
            </div>
            <div
              v-if="pendingAction === 'delete'"
              class="flex items-start gap-1.5 text-red-300"
            >
              <AlertTriangle class="size-3.5 text-red-400 shrink-0 mt-0.5" />
              <span>Cảnh báo: Xóa vĩnh viễn sẽ không thể phục hồi từ Thùng rác!</span>
            </div>
          </div>
        </div>

        <div class="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            class="flex-1 h-8 text-xs cursor-pointer"
            @click="cancelSafetyAction"
          >
            Hủy bỏ
          </Button>
          <Button
            size="sm"
            class="flex-1 h-8 text-xs cursor-pointer font-medium"
            :class="pendingAction === 'delete' ? 'bg-red-600 hover:bg-red-500 text-white' : 'bg-amber-600 hover:bg-amber-500 text-white'"
            @click="confirmSafetyAction"
          >
            Tôi đã hiểu & Xác nhận
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
