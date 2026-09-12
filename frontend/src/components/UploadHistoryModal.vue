<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, computed } from 'vue'
import { toast } from 'vue-sonner'
import {
  History,
  AlertTriangle,
  RefreshCw,
  Trash2,
  CheckCircle2,
  Copy,
  Check,
  ChevronDown,
  ChevronUp,
  FolderPlus,
  Clock,
  FileWarning,
  ExternalLink,
} from '@lucide/vue'
import {
  ConfigManager,
  type UploadSessionSummary,
  type UploadItemRecord,
  type FailedItemSummary,
} from '../../bindings/app/backend'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Events, Clipboard, Browser } from '@wailsio/runtime'

const isOpen = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{
  (e: 'retry-files', files: string[]): void
}>()

// Active tab: 'failed' or 'history'
const activeTab = ref<'failed' | 'history'>('failed')

// Failed Queue state
const failedQueue = ref<FailedItemSummary[]>([])
const isLoadingQueue = ref(false)
const isRetryingAll = ref(false)

// History state
const historySessions = ref<UploadSessionSummary[]>([])
const isLoadingHistory = ref(false)
const expandedSessionId = ref<number | null>(null)
const sessionDetailsCache = ref<Record<number, UploadItemRecord[]>>({})
const loadingSessionId = ref<number | null>(null)

// Copy feedback tracking
const copiedPath = ref<string | null>(null)
const copiedMediaKey = ref<string | null>(null)

async function openPhotoLink(mediaKey: string) {
  if (!mediaKey) return
  await Browser.OpenURL(`https://photos.google.com/photo/${mediaKey}`)
}

async function copyPhotoLink(mediaKey: string) {
  if (!mediaKey) return
  await Clipboard.SetText(`https://photos.google.com/photo/${mediaKey}`)
  copiedMediaKey.value = mediaKey
  toast.success('Đã sao chép liên kết ảnh!')
  setTimeout(() => {
    if (copiedMediaKey.value === mediaKey) {
      copiedMediaKey.value = null
    }
  }, 1500)
}

async function openAlbumLink(albumNameOrKey: string) {
  if (!albumNameOrKey) return
  const url = albumNameOrKey.startsWith('AF1Qip')
    ? `https://photos.google.com/album/${albumNameOrKey}`
    : 'https://photos.google.com/albums'
  await Browser.OpenURL(url)
}

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + ' ' + sizes[i]
}

function formatDate(timestamp: number): string {
  if (!timestamp) return '--'
  const date = new Date(timestamp * 1000)
  return date.toLocaleString([], {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

async function fetchFailedQueue() {
  isLoadingQueue.value = true
  try {
    const queue = await ConfigManager.GetFailedQueue()
    failedQueue.value = queue || []
  } catch (err) {
    console.error('Failed to fetch failed queue:', err)
  } finally {
    isLoadingQueue.value = false
  }
}

async function fetchHistory() {
  isLoadingHistory.value = true
  try {
    const sessions = await ConfigManager.GetUploadHistory(50, 0)
    historySessions.value = sessions || []
  } catch (err) {
    console.error('Failed to fetch upload history:', err)
  } finally {
    isLoadingHistory.value = false
  }
}

async function refreshAll() {
  await Promise.all([fetchFailedQueue(), fetchHistory()])
}

watch(isOpen, (open) => {
  if (open) {
    refreshAll()
  }
})

// 1-Click Retry All Failed
async function retryAllFailed() {
  isRetryingAll.value = true
  try {
    const filesToRetry = await ConfigManager.GetAllFailedFilesForRetry()
    if (!filesToRetry || filesToRetry.length === 0) {
      toast.info('Không có file nào sẵn sàng để thử lại.')
      return
    }
    isOpen.value = false
    emit('retry-files', filesToRetry)
    toast.success(`Bắt đầu thử lại ${filesToRetry.length} file lỗi!`)
  } catch (err) {
    console.error('Failed to retry all failed files:', err)
    toast.error('Lỗi khi chuẩn bị thử lại file lỗi.')
  } finally {
    isRetryingAll.value = false
  }
}

// Retry single file
function retrySingleFile(filePath: string) {
  if (!filePath) return
  isOpen.value = false
  emit('retry-files', [filePath])
  toast.success('Bắt đầu thử lại 1 file lỗi!')
}

// Retry all failed in a specific session
async function retrySessionFailed(sessionId: number) {
  try {
    const filesToRetry = await ConfigManager.GetFailedFilesForRetry(sessionId)
    if (!filesToRetry || filesToRetry.length === 0) {
      toast.info('Không tìm thấy file lỗi còn tồn tại trên máy trong phiên này.')
      return
    }
    isOpen.value = false
    emit('retry-files', filesToRetry)
    toast.success(`Bắt đầu thử lại ${filesToRetry.length} file lỗi trong phiên!`)
  } catch (err) {
    console.error('Failed to retry session failed files:', err)
    toast.error('Lỗi khi thử lại file lỗi trong phiên.')
  }
}

// Dismiss single failed item from queue
async function dismissItem(id: number) {
  try {
    await ConfigManager.DismissFailedItem(id)
    failedQueue.value = failedQueue.value.filter(item => item.id !== id)
    toast.success('Đã xóa file khỏi hàng đợi lỗi')
  } catch {
    toast.error('Không thể xóa file khỏi hàng đợi')
  }
}

// Clear all failed queue items
async function clearAllFailedQueue() {
  try {
    await ConfigManager.ClearFailedQueue()
    failedQueue.value = []
    toast.success('Đã xóa toàn bộ hàng đợi file lỗi')
  } catch {
    toast.error('Không thể xóa hàng đợi')
  }
}

// Clear entire upload history
async function clearAllHistory() {
  if (!confirm('Bạn có chắc chắn muốn xóa toàn bộ lịch sử các phiên upload?')) return
  try {
    await ConfigManager.ClearUploadHistory()
    historySessions.value = []
    sessionDetailsCache.value = {}
    failedQueue.value = []
    toast.success('Đã xóa toàn bộ lịch sử upload')
  } catch {
    toast.error('Không thể xóa lịch sử upload')
  }
}

// Expand / collapse session items
async function toggleSessionExpand(sessionId: number) {
  if (expandedSessionId.value === sessionId) {
    expandedSessionId.value = null
    return
  }
  expandedSessionId.value = sessionId

  if (!sessionDetailsCache.value[sessionId]) {
    loadingSessionId.value = sessionId
    try {
      const details = await ConfigManager.GetSessionDetails(sessionId)
      if (details?.items) {
        sessionDetailsCache.value[sessionId] = details.items
      }
    } catch (err) {
      console.error('Failed to load session details:', err)
    } finally {
      loadingSessionId.value = null
    }
  }
}

async function copyToClipboard(text: string) {
  try {
    await Clipboard.SetText(text)
    copiedPath.value = text
    setTimeout(() => {
      if (copiedPath.value === text) {
        copiedPath.value = null
      }
    }, 1500)
  } catch {
    // fallback
  }
}

function getSourceLabel(source: string): string {
  switch (source) {
    case 'autosync':
      return 'Tự Động'
    case 'retry':
      return 'Thử Lại'
    default:
      return 'Thủ Công'
  }
}

onMounted(() => {
  Events.On('uploadHistoryUpdated', () => {
    refreshAll()
  })
})

onUnmounted(() => {
  // cleanup
})

defineExpose({
  refresh: refreshAll,
  failedCount: computed(() => failedQueue.value.length),
})
</script>

<template>
  <Sheet v-model:open="isOpen">
    <SheetContent
      side="bottom"
      class="max-h-[88vh] overflow-hidden flex flex-col px-6 py-5"
      style="--wails-draggable: none"
    >
      <SheetHeader class="mb-3 shrink-0">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <History class="size-4" />
            </div>
            <SheetTitle class="text-lg font-semibold">
              Lịch Sử & Quản Lý File Lỗi
            </SheetTitle>
          </div>

          <!-- Tab switchers -->
          <div class="flex items-center bg-muted/60 p-1 rounded-lg border text-xs">
            <button
              type="button"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-md font-medium transition-all cursor-pointer select-none"
              :class="activeTab === 'failed' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="activeTab = 'failed'"
            >
              <AlertTriangle
                class="size-3.5"
                :class="failedQueue.length > 0 ? 'text-destructive' : 'text-muted-foreground'"
              />
              <span>Hàng Đợi Lỗi</span>
              <span
                v-if="failedQueue.length > 0"
                class="inline-flex items-center justify-center px-1.5 py-0.2 text-[10px] font-bold rounded-full bg-destructive text-destructive-foreground"
              >
                {{ failedQueue.length }}
              </span>
            </button>

            <button
              type="button"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-md font-medium transition-all cursor-pointer select-none"
              :class="activeTab === 'history' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="activeTab = 'history'"
            >
              <Clock class="size-3.5" />
              <span>Lịch Sử Phiên</span>
              <span
                v-if="historySessions.length > 0"
                class="text-[10px] text-muted-foreground"
              >
                ({{ historySessions.length }})
              </span>
            </button>
          </div>
        </div>
      </SheetHeader>

      <!-- TAB 1: Failed Queue -->
      <div
        v-if="activeTab === 'failed'"
        class="flex flex-col flex-1 min-h-0 overflow-hidden"
      >
        <!-- Action bar for Failed Queue -->
        <div class="flex items-center justify-between pb-3 border-b border-border/40 shrink-0">
          <div class="flex items-center gap-2">
            <span class="text-xs text-muted-foreground">
              {{ failedQueue.length === 0 ? 'Không có file lỗi nào trong hàng đợi' : `Có ${failedQueue.length} file chưa tải lên thành công:` }}
            </span>
          </div>

          <div class="flex items-center gap-2">
            <Button
              v-if="failedQueue.length > 0"
              variant="outline"
              size="sm"
              class="cursor-pointer text-xs h-8 text-muted-foreground hover:text-destructive hover:border-destructive/30"
              @click="clearAllFailedQueue"
            >
              <Trash2 class="size-3.5 mr-1" />
              Xóa danh sách lỗi
            </Button>

            <!-- 1-CLICK RETRY BUTTON -->
            <Button
              size="sm"
              class="cursor-pointer text-xs h-8 bg-amber-600 hover:bg-amber-700 text-white font-medium shadow-sm transition-colors"
              :disabled="failedQueue.length === 0 || isRetryingAll"
              @click="retryAllFailed"
            >
              <RefreshCw
                class="size-3.5 mr-1.5"
                :class="{ 'animate-spin': isRetryingAll }"
              />
              Thử lại tất cả file lỗi ({{ failedQueue.length }})
            </Button>
          </div>
        </div>

        <!-- Failed Items List -->
        <div class="flex-1 overflow-y-auto pr-1 pt-3 space-y-2.5">
          <!-- Empty State -->
          <div
            v-if="failedQueue.length === 0"
            class="flex flex-col items-center justify-center py-16 text-center text-muted-foreground"
          >
            <div class="flex size-12 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-500 mb-3">
              <CheckCircle2 class="size-6" />
            </div>
            <p class="text-sm font-semibold text-foreground">
              Tuyệt vời! Không có file lỗi nào
            </p>
            <p class="text-xs text-muted-foreground mt-1 max-w-xs">
              Mọi ảnh và video đều đã được đồng bộ lên Google Photos thành công.
            </p>
          </div>

          <!-- Failed Queue Cards -->
          <div
            v-for="item in failedQueue"
            :key="item.id"
            class="rounded-xl border border-destructive/20 bg-card/60 p-3 shadow-xs hover:border-destructive/40 transition-colors"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="font-medium text-sm truncate text-foreground">
                    {{ item.fileName }}
                  </span>
                  <span
                    v-if="item.fileSize > 0"
                    class="text-[11px] text-muted-foreground shrink-0"
                  >
                    ({{ formatBytes(item.fileSize) }})
                  </span>
                </div>

                <!-- Path row with copy -->
                <div class="flex items-center gap-1.5 mt-0.5">
                  <span class="text-xs text-muted-foreground/80 truncate font-mono select-all">
                    {{ item.filePath }}
                  </span>
                  <button
                    type="button"
                    class="text-muted-foreground hover:text-foreground shrink-0 cursor-pointer p-0.5"
                    title="Sao chép đường dẫn"
                    @click="copyToClipboard(item.filePath)"
                  >
                    <component
                      :is="copiedPath === item.filePath ? Check : Copy"
                      v-if="copiedPath === item.filePath"
                      class="size-3 text-emerald-500"
                    />
                    <Copy
                      v-else
                      class="size-3"
                    />
                  </button>
                </div>

                <!-- Specific Error Reason Box -->
                <div class="mt-2 flex items-start gap-1.5 rounded-lg bg-destructive/10 border border-destructive/20 p-2 text-xs text-destructive dark:text-red-400">
                  <FileWarning class="size-3.5 mt-0.5 shrink-0" />
                  <span class="leading-tight select-text break-words">
                    {{ item.errorMessage || 'Lỗi tải lên không xác định' }}
                  </span>
                </div>

                <div class="flex items-center gap-2 mt-2 text-[11px] text-muted-foreground">
                  <Clock class="size-3" />
                  <span>{{ formatDate(item.createdAt) }}</span>
                </div>
              </div>

              <!-- Action buttons for item -->
              <div class="flex flex-col gap-1.5 shrink-0">
                <Button
                  size="sm"
                  variant="outline"
                  class="cursor-pointer text-xs h-7 px-2 text-amber-500 hover:text-amber-600 hover:bg-amber-500/10 border-amber-500/30"
                  @click="retrySingleFile(item.filePath)"
                >
                  <RefreshCw class="size-3 mr-1" />
                  Thử lại
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  class="cursor-pointer text-xs h-7 px-2 text-muted-foreground hover:text-foreground"
                  @click="dismissItem(item.id)"
                >
                  Bỏ qua
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- TAB 2: Upload History -->
      <div
        v-else-if="activeTab === 'history'"
        class="flex flex-col flex-1 min-h-0 overflow-hidden"
      >
        <!-- Header row -->
        <div class="flex items-center justify-between pb-3 border-b border-border/40 shrink-0">
          <span class="text-xs text-muted-foreground">
            Hiển thị tối đa 50 phiên tải lên gần nhất
          </span>
          <Button
            v-if="historySessions.length > 0"
            variant="outline"
            size="sm"
            class="cursor-pointer text-xs h-8 text-muted-foreground hover:text-destructive hover:border-destructive/30"
            @click="clearAllHistory"
          >
            <Trash2 class="size-3.5 mr-1" />
            Xóa toàn bộ lịch sử
          </Button>
        </div>

        <!-- Sessions List -->
        <div class="flex-1 overflow-y-auto pr-1 pt-3 space-y-2.5">
          <!-- Empty History -->
          <div
            v-if="historySessions.length === 0"
            class="flex flex-col items-center justify-center py-16 text-center text-muted-foreground"
          >
            <Clock class="size-8 text-muted-foreground/50 mb-2" />
            <p class="text-sm font-semibold text-foreground">
              Chưa có lịch sử upload
            </p>
            <p class="text-xs text-muted-foreground mt-1">
              Các phiên upload tiếp theo sẽ được lưu lại tự động tại đây.
            </p>
          </div>

          <!-- Session Card -->
          <div
            v-for="session in historySessions"
            :key="session.id"
            class="rounded-xl border bg-card/60 p-3.5 shadow-xs transition-colors hover:border-border/80"
          >
            <div
              class="flex items-center justify-between cursor-pointer select-none"
              @click="toggleSessionExpand(session.id)"
            >
              <div class="flex flex-col gap-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-xs font-semibold text-foreground">
                    {{ formatDate(session.startedAt) }}
                  </span>
                  <!-- Source badge -->
                  <span class="px-1.5 py-0.2 rounded text-[10px] font-medium bg-muted text-muted-foreground border">
                    {{ getSourceLabel(session.source) }}
                  </span>
                  <!-- Status badge -->
                  <span
                    v-if="session.status === 'completed' && session.failedCount === 0"
                    class="px-1.5 py-0.2 rounded text-[10px] font-medium bg-emerald-500/15 text-emerald-500 border border-emerald-500/30"
                  >
                    Hoàn tất
                  </span>
                  <span
                    v-else-if="session.failedCount > 0"
                    class="px-1.5 py-0.2 rounded text-[10px] font-medium bg-destructive/15 text-destructive border border-destructive/30"
                  >
                    Có {{ session.failedCount }} lỗi
                  </span>
                  <span
                    v-else-if="session.status === 'cancelled'"
                    class="px-1.5 py-0.2 rounded text-[10px] font-medium bg-amber-500/15 text-amber-500 border border-amber-500/30"
                  >
                    Đã hủy
                  </span>
                </div>

                <!-- Counts & details -->
                <div class="flex items-center gap-3 text-xs text-muted-foreground">
                  <span class="text-emerald-500 font-medium">
                    ✓ {{ session.successCount }} thành công
                  </span>
                  <span
                    v-if="session.failedCount > 0"
                    class="text-destructive font-medium"
                  >
                    ✗ {{ session.failedCount }} lỗi
                  </span>
                  <span
                    v-if="session.skippedCount > 0"
                    class="text-amber-500 font-medium"
                  >
                    ⊘ {{ session.skippedCount }} bỏ qua
                  </span>
                  <span
                    v-if="session.albumName"
                    class="flex items-center gap-1 text-primary truncate hover:underline cursor-pointer"
                    title="Mở album trên Google Photos"
                    @click.stop="openAlbumLink(session.albumName)"
                  >
                    <FolderPlus class="size-3" />
                    {{ session.albumName }}
                    <ExternalLink class="size-2.5 opacity-70 ml-0.5" />
                  </span>
                </div>
              </div>

              <!-- Expand toggle -->
              <div class="flex items-center gap-1.5 shrink-0">
                <Button
                  v-if="session.failedCount > 0"
                  size="sm"
                  variant="outline"
                  class="cursor-pointer text-xs h-7 px-2 text-amber-500 hover:text-amber-600 hover:bg-amber-500/10 border-amber-500/30"
                  @click.stop="retrySessionFailed(session.id)"
                >
                  <RefreshCw class="size-3 mr-1" />
                  Thử lại file lỗi
                </Button>
                <component
                  :is="expandedSessionId === session.id ? ChevronUp : ChevronDown"
                  class="size-4 text-muted-foreground ml-1"
                />
              </div>
            </div>

            <!-- Expanded items view -->
            <div
              v-if="expandedSessionId === session.id"
              class="mt-3 pt-3 border-t border-border/40"
            >
              <div
                v-if="loadingSessionId === session.id"
                class="flex items-center justify-center py-4 text-xs text-muted-foreground gap-2"
              >
                <RefreshCw class="size-3 animate-spin" />
                Đang tải chi tiết phiên...
              </div>

              <div
                v-else-if="sessionDetailsCache[session.id]?.length"
                class="space-y-1.5 max-h-56 overflow-y-auto pr-1"
              >
                <div
                  v-for="subItem in sessionDetailsCache[session.id]"
                  :key="subItem.id"
                  class="flex items-start justify-between gap-2 p-2 rounded-lg bg-muted/20 border border-border/30 text-xs"
                >
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-2">
                      <span class="font-medium truncate text-foreground">
                        {{ subItem.fileName }}
                      </span>
                      <span
                        v-if="subItem.status === 'success'"
                        class="text-[10px] text-emerald-500 font-medium"
                      >
                        ✓ Thành công
                      </span>
                      <span
                        v-else-if="subItem.status === 'skipped'"
                        class="text-[10px] text-amber-500 font-medium"
                      >
                        ⊘ Đã bỏ qua
                      </span>
                      <span
                        v-else-if="subItem.status === 'failed'"
                        class="text-[10px] text-destructive font-medium"
                      >
                        ✗ Thất bại
                      </span>
                    </div>

                    <p class="text-[11px] text-muted-foreground truncate font-mono mt-0.5">
                      {{ subItem.filePath }}
                    </p>

                    <!-- Error or skip message -->
                    <p
                      v-if="subItem.errorMessage"
                      class="text-[11px] text-destructive mt-1 bg-destructive/10 p-1.5 rounded"
                    >
                      {{ subItem.errorMessage }}
                    </p>
                    <p
                      v-else-if="subItem.skipReason"
                      class="text-[11px] text-amber-500 mt-1 bg-amber-500/10 p-1.5 rounded"
                    >
                      {{ subItem.skipReason }}
                    </p>
                  </div>

                  <div
                    v-if="subItem.status === 'failed'"
                    class="shrink-0"
                  >
                    <Button
                      size="sm"
                      variant="ghost"
                      class="h-6 px-1.5 text-[11px] text-amber-500 hover:text-amber-600"
                      @click="retrySingleFile(subItem.filePath)"
                    >
                      Thử lại
                    </Button>
                  </div>

                  <div
                    v-else-if="subItem.status === 'success' && subItem.mediaKey"
                    class="flex items-center gap-0.5 shrink-0"
                  >
                    <Button
                      size="sm"
                      variant="ghost"
                      class="h-6 w-6 p-0 text-muted-foreground hover:text-primary cursor-pointer"
                      title="Mở ảnh trên Google Photos"
                      @click.stop="openPhotoLink(subItem.mediaKey)"
                    >
                      <ExternalLink class="size-3" />
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      class="h-6 w-6 p-0 text-muted-foreground hover:text-foreground cursor-pointer"
                      title="Sao chép link ảnh"
                      @click.stop="copyPhotoLink(subItem.mediaKey)"
                    >
                      <component
                        :is="copiedMediaKey === subItem.mediaKey ? Check : Copy"
                        class="size-3"
                        :class="copiedMediaKey === subItem.mediaKey ? 'text-emerald-500' : ''"
                      />
                    </Button>
                  </div>
                </div>
              </div>

              <div
                v-else
                class="text-xs text-muted-foreground py-2 text-center"
              >
                Không có chi tiết file nào được ghi nhận.
              </div>
            </div>
          </div>
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>
