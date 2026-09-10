<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, computed } from 'vue'
import { toast } from 'vue-sonner'
import {
  Folder,
  FolderPlus,
  Trash2,
  Play,
  RefreshCw,
  Loader2,
  Image,
  Download,
} from '@lucide/vue'
import { ConfigManager, type AutoSyncStatus, type PresetFolders } from '../../bindings/app/backend'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Events } from '@wailsio/runtime'

const isOpen = defineModel<boolean>('open', { default: false })

const autoSyncEnabled = ref(false)
const syncOnStartup = ref(false)
const syncFolders = ref<string[]>([])
const presetFolders = ref<PresetFolders>({ pictures: '', downloads: '' })

const status = ref<AutoSyncStatus>({
  enabled: false,
  isSyncing: false,
  folderCount: 0,
  watchedFolders: [],
  queueCount: 0,
  syncedCount: 0,
  lastSyncTime: 0,
  currentFile: '',
  statusMessage: 'Idle',
})

const isScanningNow = ref(false)
const isSelectingFolder = ref(false)

const lastSyncFormatted = computed(() => {
  if (!status.value.lastSyncTime) return 'Chưa đồng bộ'
  const date = new Date(status.value.lastSyncTime * 1000)
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
})

async function refreshData() {
  try {
    const config = await ConfigManager.GetSettings()
    autoSyncEnabled.value = config.autoSyncEnabled || false
    syncOnStartup.value = config.syncOnStartup || false
    syncFolders.value = config.syncFolders || []

    const currentStatus = await ConfigManager.GetAutoSyncStatus()
    if (currentStatus) {
      status.value = currentStatus
    }

    const presets = await ConfigManager.GetPresetFolders()
    if (presets) {
      presetFolders.value = presets
    }
  } catch (err) {
    console.error('Failed to load auto sync settings:', err)
  }
}

watch(isOpen, async (open) => {
  if (open) {
    await refreshData()
  }
})

async function toggleAutoSync(val: boolean) {
  autoSyncEnabled.value = val
  try {
    await ConfigManager.SetAutoSyncEnabled(val)
    await refreshData()
    if (val) {
      toast.success('Đã bật chế độ tự động đồng bộ')
    } else {
      toast.info('Đã tắt tự động đồng bộ')
    }
  } catch {
    toast.error('Lỗi khi bật/tắt tự động đồng bộ')
  }
}

async function toggleSyncOnStartup(val: boolean) {
  syncOnStartup.value = val
  try {
    await ConfigManager.SetSyncOnStartup(val)
  } catch (err) {
    console.error('Failed to update sync on startup:', err)
  }
}

async function openFolderPicker() {
  if (isSelectingFolder.value) return
  isSelectingFolder.value = true
  try {
    const selected = await ConfigManager.OpenDirectoryDialog()
    if (selected && selected.trim()) {
      await addFolder(selected.trim())
    }
  } catch (err) {
    console.error('Folder picker error:', err)
  } finally {
    isSelectingFolder.value = false
  }
}

async function addFolder(folderPath: string) {
  if (!folderPath) return
  try {
    await ConfigManager.AddSyncFolder(folderPath)
    await refreshData()
    toast.success(`Đã thêm thư mục: ${folderPath.split(/[\\/]/).pop() || folderPath}`)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error(msg || 'Không thể thêm thư mục')
  }
}

async function removeFolder(folderPath: string) {
  try {
    await ConfigManager.RemoveSyncFolder(folderPath)
    await refreshData()
    toast.info(`Đã gỡ thư mục: ${folderPath.split(/[\\/]/).pop() || folderPath}`)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error(msg || 'Lỗi khi gỡ thư mục')
  }
}

async function triggerSyncNow() {
  if (isScanningNow.value) return
  isScanningNow.value = true
  try {
    await ConfigManager.TriggerSyncNow()
    toast.success('Bắt đầu quét và đồng bộ tệp mới...')
    await refreshData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error(msg || 'Lỗi khi kích hoạt đồng bộ')
  } finally {
    setTimeout(() => {
      isScanningNow.value = false
    }, 1500)
  }
}

function handleStatusEvent(eventData: { data?: AutoSyncStatus }) {
  if (eventData?.data) {
    status.value = eventData.data
  }
}

function handleFileUploaded(eventData: { data?: { path: string; fileName: string; size: number } }) {
  if (eventData?.data) {
    const file = eventData.data
    toast.success(`Đã sao lưu: ${file.fileName}`, {
      description: 'Tải lên Google Photos thành công trong nền',
      duration: 3500,
    })
    status.value.syncedCount++
    status.value.lastSyncTime = Math.floor(Date.now() / 1000)
  }
}

onMounted(() => {
  refreshData()
  Events.On('autosync:status', handleStatusEvent)
  Events.On('autosync:file-uploaded', handleFileUploaded)
})

onUnmounted(() => {
  // Clean up listeners if needed
})
</script>

<template>
  <Sheet v-model:open="isOpen">
    <SheetContent
      side="bottom"
      class="max-h-[85vh] overflow-y-auto px-6 py-5"
    >
      <SheetHeader class="mb-4">
        <div class="flex items-center gap-2">
          <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <RefreshCw
              class="size-4"
              :class="{ 'animate-spin': status.isSyncing }"
            />
          </div>
          <SheetTitle class="text-lg font-semibold">
            Tự Động Đồng Bộ (Auto-Sync)
          </SheetTitle>
        </div>
        <p class="text-xs text-muted-foreground">
          Tự động phát hiện ảnh/video mới và âm thầm tải lên Google Photos trong nền.
        </p>
      </SheetHeader>

      <div class="flex flex-col gap-4">
        <!-- Master Status Card -->
        <div class="rounded-xl border bg-card/60 p-4 shadow-sm backdrop-blur-sm">
          <div class="flex items-center justify-between">
            <div class="flex flex-col gap-0.5">
              <Label
                class="text-sm font-semibold cursor-pointer"
                for="auto-sync-master"
              >
                Bật tự động đồng bộ
              </Label>
              <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span
                  class="size-2 rounded-full"
                  :class="[
                    !autoSyncEnabled ? 'bg-zinc-500' : status.isSyncing ? 'bg-amber-500 animate-pulse' : 'bg-emerald-500'
                  ]"
                />
                <span>
                  {{ !autoSyncEnabled ? 'Đang tắt' : status.isSyncing ? `Đang đồng bộ: ${status.currentFile}` : `Đang giám sát ${syncFolders.length} thư mục` }}
                </span>
              </div>
            </div>
            <Switch
              id="auto-sync-master"
              :model-value="autoSyncEnabled"
              @update:model-value="toggleAutoSync"
            />
          </div>

          <!-- Stats & Quick Action -->
          <div class="mt-3.5 pt-3 border-t flex items-center justify-between text-xs text-muted-foreground">
            <div class="flex items-center gap-4">
              <div>
                Đã đồng bộ: <span class="font-medium text-foreground">{{ status.syncedCount }}</span>
              </div>
              <div>
                Hàng đợi: <span class="font-medium text-foreground">{{ status.queueCount }}</span>
              </div>
              <div>
                Lần cuối: <span class="font-medium text-foreground">{{ lastSyncFormatted }}</span>
              </div>
            </div>
            <Button
              size="sm"
              variant="outline"
              class="h-7 px-2.5 text-xs gap-1.5 cursor-pointer"
              :disabled="!autoSyncEnabled || isScanningNow"
              @click="triggerSyncNow"
            >
              <Loader2
                v-if="isScanningNow"
                class="size-3 animate-spin"
              />
              <Play
                v-else
                class="size-3 text-primary fill-primary"
              />
              <span>Đồng bộ ngay</span>
            </Button>
          </div>
        </div>

        <!-- Monitored Folders Section -->
        <div class="flex flex-col gap-2">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-1.5">
              <Label class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Thư mục giám sát
              </Label>
              <span class="rounded-full bg-muted px-1.5 py-0.2 text-[10px] font-medium text-muted-foreground">
                {{ syncFolders.length }}
              </span>
            </div>
            <Button
              size="sm"
              variant="secondary"
              class="h-7 text-xs gap-1.5 cursor-pointer"
              :disabled="isSelectingFolder"
              @click="openFolderPicker"
            >
              <FolderPlus class="size-3.5 text-primary" />
              <span>+ Chọn thư mục</span>
            </Button>
          </div>

          <!-- Preset Quick Add Chips -->
          <div class="flex items-center gap-2">
            <span class="text-[11px] text-muted-foreground">Thêm nhanh:</span>
            <button
              v-if="presetFolders.pictures"
              type="button"
              class="inline-flex items-center gap-1 rounded-md border bg-muted/40 px-2 py-0.5 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground cursor-pointer transition-colors"
              :disabled="syncFolders.includes(presetFolders.pictures)"
              @click="addFolder(presetFolders.pictures)"
            >
              <Image class="size-3 text-sky-500" />
              <span>+ Pictures</span>
            </button>
            <button
              v-if="presetFolders.downloads"
              type="button"
              class="inline-flex items-center gap-1 rounded-md border bg-muted/40 px-2 py-0.5 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground cursor-pointer transition-colors"
              :disabled="syncFolders.includes(presetFolders.downloads)"
              @click="addFolder(presetFolders.downloads)"
            >
              <Download class="size-3 text-emerald-500" />
              <span>+ Downloads</span>
            </button>
          </div>

          <!-- Folders List -->
          <div class="rounded-xl border bg-card/40 overflow-hidden">
            <div
              v-if="syncFolders.length === 0"
              class="py-7 px-4 text-center"
            >
              <Folder class="size-8 mx-auto text-muted-foreground/40 mb-1.5" />
              <p class="text-xs text-muted-foreground">
                Chưa có thư mục nào được giám sát.
              </p>
              <p class="text-[11px] text-muted-foreground/70 mt-0.5">
                Nhấn "+ Chọn thư mục" hoặc các nút thêm nhanh ở trên để bắt đầu.
              </p>
            </div>

            <div
              v-else
              class="divide-y max-h-40 overflow-y-auto"
            >
              <div
                v-for="folder in syncFolders"
                :key="folder"
                class="flex items-center justify-between px-3 py-2 text-xs hover:bg-muted/30 transition-colors group"
              >
                <div class="flex items-center gap-2 min-w-0 pr-2">
                  <Folder class="size-3.5 shrink-0 text-amber-500" />
                  <span
                    class="font-mono text-[11px] truncate text-foreground"
                    :title="folder"
                  >
                    {{ folder }}
                  </span>
                </div>
                <Button
                  size="icon"
                  variant="ghost"
                  class="size-6 text-muted-foreground hover:text-destructive shrink-0 cursor-pointer"
                  title="Gỡ thư mục"
                  @click="removeFolder(folder)"
                >
                  <Trash2 class="size-3" />
                </Button>
              </div>
            </div>
          </div>
        </div>

        <!-- Sync Options -->
        <div class="rounded-xl border p-3 flex items-center justify-between text-xs">
          <div class="flex flex-col gap-0.5">
            <Label
              for="sync-on-startup"
              class="cursor-pointer font-medium"
            >
              Quét đồng bộ khi mở ứng dụng
            </Label>
            <p class="text-[11px] text-muted-foreground">
              Tự động kiểm tra và tải lên các tệp mới khi gotohp khởi chạy.
            </p>
          </div>
          <Switch
            id="sync-on-startup"
            :model-value="syncOnStartup"
            @update:model-value="toggleSyncOnStartup"
          />
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>
