<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TooltipProvider } from '@/components/ui/tooltip'
import {
  Sheet,
  SheetContent,
  SheetTrigger,
} from '@/components/ui/sheet'
import { useColorMode } from '@vueuse/core'
import { onMounted, onUnmounted, ref, watch, computed } from 'vue'
import {
  UserPlus,
  History,
  RefreshCw,
  AlertTriangle,
  ChevronDown,
  ChevronUp,
  Settings,
  UploadCloud,
  FolderPlus,
  Sparkles,
  CheckCircle2,
  ExternalLink,
  Copy,
  Check,
} from '@lucide/vue'
import { ConfigManager, type AutoSyncStatus } from '../bindings/app/backend'
import { Events, Browser, Clipboard } from '@wailsio/runtime'
import Button from "./components/ui/button/Button.vue"
import GoogleAccountSelect from './components/GoogleAccountSelect.vue'
import GoogleAuthSetup from "./components/GoogleAuthSetup.vue"
import AutoSyncModal from './components/AutoSyncModal.vue'
import UploadHistoryModal from './components/UploadHistoryModal.vue'
import './index.css'
import SettingsPanel from "./SettingsPanel.vue"
import Upload from './Upload.vue'
import { uploadManager } from './utils/UploadManager'
import Toaster from './components/ui/sonner/Sonner.vue'

import { toast } from "vue-sonner"

useColorMode().value = "dark"

const { state: uploadState } = uploadManager
const copyButtonText = ref('Sao chép JSON');

// Drag state for dual drop zones
const isDraggingFiles = ref(false)

// Album upload flow state
const showAlbumInput = ref(false)
const pendingFiles = ref<string[]>([])
const pendingFileCount = ref(0)

const selectedOption = ref('')
const options = ref<string[]>([])
const accountNeedsTokenBinding = ref<Record<string, boolean>>({})
const albumNameOrKey = ref('')
const tokenBindingEmail = ref('')
const isExtractingTokenBinding = ref(false)
const isAccountSetupOpen = ref(false)
const isSettingsOpen = ref(false)
const removingAccount = ref('')
const isAutoSyncOpen = ref(false)
const isHistoryOpen = ref(false)
const failedQueueCount = ref(0)
const showFailedDetails = ref(false)

async function refreshFailedQueueCount() {
  try {
    const queue = await ConfigManager.GetFailedQueue()
    failedQueueCount.value = queue?.length || 0
  } catch {
    // ignore
  }
}

function handleRetryFiles(files: string[]) {
  if (files && files.length > 0) {
    Events.Emit('startUpload', { files })
  }
}

function retryCurrentFailedFiles() {
  const paths = uploadState.results.failedItems?.map(f => f.path).filter(Boolean) || []
  if (paths.length > 0) {
    Events.Emit('startUpload', { files: paths })
  }
}

function openAutoSyncFromSettings() {
  isSettingsOpen.value = false
  setTimeout(() => {
    isAutoSyncOpen.value = true
  }, 120)
}

const autoSyncStatus = ref<AutoSyncStatus>({
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

watch(selectedOption, async (newValue) => {
  if (newValue) {
    try {
      await ConfigManager.SetSelected(newValue)
      await updateTokenBindingPrompt(newValue)
      console.log('Successfully updated selected value:', newValue)
    } catch (error) {
      console.error('Failed to update selected value:', error)
      toast.error('Failed to update selected account.')
    }
  } else {
    tokenBindingEmail.value = ''
  }
})

async function updateTokenBindingPrompt(email: string) {
  tokenBindingEmail.value = accountNeedsTokenBinding.value[email] ? email : ''
}

async function refreshCredentials() {
  try {
    const state = await ConfigManager.GetAccounts()
    const nextNeedsTokenBinding: Record<string, boolean> = {}
    const nextOptions = state.accounts.map(account => {
      nextNeedsTokenBinding[account.email] = account.needsTokenBinding
      return account.email
    })

    accountNeedsTokenBinding.value = nextNeedsTokenBinding
    options.value = nextOptions
    selectedOption.value = state.selected || ''
    if (state.selected) {
      await updateTokenBindingPrompt(state.selected)
    } else {
      tokenBindingEmail.value = ''
    }
  } catch (error) {
    console.error('Failed to refresh Google accounts:', error)
    toast.error('Could not refresh Google accounts', {
      description: error instanceof Error ? error.message : String(error),
    })
  }
}

async function addTokenBindingAliasFromADB() {
  if (!tokenBindingEmail.value) return

  isExtractingTokenBinding.value = true
  try {
    await ConfigManager.AddTokenBindingAliasFromADB(tokenBindingEmail.value)
    await refreshCredentials()
    tokenBindingEmail.value = ''
    toast.success('Token binding key added.')
  } catch (error) {
    console.error('Failed to add token binding key:', error)
    toast.error('Failed to add token binding key', {
      description: error instanceof Error ? error.message : String(error),
    })
  } finally {
    isExtractingTokenBinding.value = false
  }
}

async function removeCredentials(email: string) {
  removingAccount.value = email
  try {
    await ConfigManager.RemoveCredentials(email)

    const removedSelectedAccount = selectedOption.value === email
    await refreshCredentials()
    if (removedSelectedAccount && options.value.length > 0 && !selectedOption.value) {
      selectedOption.value = options.value[0]
    }
    toast.success('Credentials removed.')
    return true
  } catch (error) {
    console.error('Failed to remove credentials:', error)
    toast.error('Failed to remove credentials.')
    return false
  } finally {
    removingAccount.value = ''
  }
}

function openAccountSetup() {
  isAccountSetupOpen.value = true
}

onMounted(async () => {
  await refreshCredentials()
  try {
    const autoStatus = await ConfigManager.GetAutoSyncStatus()
    if (autoStatus) {
      autoSyncStatus.value = autoStatus
    }
  } catch {
    // ignore
  }

  Events.On('autosync:status', (event: { data: AutoSyncStatus }) => {
    if (event?.data) {
      autoSyncStatus.value = event.data
    }
  })

  refreshFailedQueueCount()
  Events.On('uploadHistoryUpdated', () => {
    refreshFailedQueueCount()
  })
})

const handleCopyClick = () => {
  uploadManager.copyResultsAsJson();
  copyButtonText.value = 'Đã sao chép!';
  setTimeout(() => copyButtonText.value = 'Sao chép JSON', 1200);
};

const isCopiedLink = ref(false)

const hasCreatedAlbum = computed(() => {
  return Boolean(uploadState.albumStatus?.AlbumKeys && uploadState.albumStatus.AlbumKeys.length > 0)
})

const copyLinkButtonText = computed(() => {
  if (isCopiedLink.value) return 'Đã sao chép!'
  if (hasCreatedAlbum.value) return 'Sao chép link Album'
  if (uploadState.results.success.length === 1) return 'Sao chép link'
  return `Sao chép ${uploadState.results.success.length} link`
})

const openInGooglePhotos = async () => {
  let targetUrl = 'https://photos.google.com/'
  const authUserParam = selectedOption.value ? `?authuser=${encodeURIComponent(selectedOption.value)}` : ''

  if (hasCreatedAlbum.value && uploadState.albumStatus?.AlbumKeys?.[0]) {
    targetUrl = uploadManager.getAlbumUrl(uploadState.albumStatus.AlbumKeys[0]) + authUserParam
  } else if (uploadState.results.success.length > 0) {
    const latestPhoto = uploadState.results.success[uploadState.results.success.length - 1]
    if (latestPhoto?.mediaKey) {
      targetUrl = uploadManager.getPhotoUrl(latestPhoto.mediaKey) + authUserParam
    }
  } else if (selectedOption.value) {
    targetUrl = `https://photos.google.com/${authUserParam}`
  }

  await Browser.OpenURL(targetUrl)
}

const handleCopyShareLink = async () => {
  if (hasCreatedAlbum.value && uploadState.albumStatus?.AlbumKeys?.[0]) {
    await uploadManager.copyAlbumLink(uploadState.albumStatus.AlbumKeys[0])
    toast.success('Đã sao chép liên kết album!', {
      description: 'Dán vào trình duyệt hoặc gửi cho người khác.'
    })
  } else if (uploadState.results.success.length === 1) {
    const mediaKey = uploadState.results.success[0].mediaKey
    if (mediaKey) {
      await Clipboard.SetText(uploadManager.getPhotoUrl(mediaKey))
      toast.success('Đã sao chép liên kết ảnh!')
    }
  } else if (uploadState.results.success.length > 1) {
    const mediaKeys = uploadState.results.success.map(s => s.mediaKey).filter(Boolean)
    await uploadManager.copyPhotoLinks(mediaKeys)
    toast.success(`Đã sao chép ${mediaKeys.length} liên kết ảnh!`)
  }
  isCopiedLink.value = true
  setTimeout(() => {
    isCopiedLink.value = false
  }, 1500)
}

// Global drag event handlers to detect file dragging
let dragLeaveTimeout: ReturnType<typeof setTimeout> | null = null

const onDragEnter = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
  
  isDraggingFiles.value = true
}

const onDragOver = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  e.preventDefault()
  
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
}

const onDragLeave = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
  }
  dragLeaveTimeout = setTimeout(() => {
    isDraggingFiles.value = false
    dragLeaveTimeout = null
  }, 60)
}

const onDrop = () => {
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
  
  setTimeout(() => {
    isDraggingFiles.value = false
  }, 100)
}

// Album upload confirmation
const confirmAlbumUpload = async () => {
  await ConfigManager.SetAlbumName(albumNameOrKey.value)
  await ConfigManager.SetAlbumAutoMode(false)
  Events.Emit('startUpload', { files: pendingFiles.value })
  showAlbumInput.value = false
  pendingFiles.value = []
  pendingFileCount.value = 0
  albumNameOrKey.value = ''
}

const cancelAlbumUpload = () => {
  showAlbumInput.value = false
  pendingFiles.value = []
  pendingFileCount.value = 0
  albumNameOrKey.value = ''
}

const albumErrorHandler = (e: Event) => {
  const event = e as CustomEvent<{ AlbumName: string; Error: string }>
  const { AlbumName, Error } = event.detail
  if (Error.includes('404')) {
    toast.error('Album not found', {
      description: `The album key "${AlbumName}" does not exist or is invalid.`,
    })
  } else {
    toast.error('Failed to create album', {
      description: `Album "${AlbumName}": ${Error}`,
    })
  }
}

const uploadErrorHandler = (e: Event) => {
  const event = e as CustomEvent<{ FileName: string; Message: string }>
  const { FileName, Message } = event.detail
  const errorMessage = Message.replace(/^Error:\s*/, '')
  toast.error(FileName ? `Upload failed: ${FileName}` : 'Upload failed', {
    description: errorMessage,
    duration: 10000,
    important: true,
  })
}

onMounted(() => {
  document.addEventListener('dragenter', onDragEnter)
  document.addEventListener('dragleave', onDragLeave)
  document.addEventListener('dragover', onDragOver)
  document.addEventListener('drop', onDrop)
  window.addEventListener('albumError', albumErrorHandler)
  window.addEventListener('uploadError', uploadErrorHandler)

  Events.On('files-dropped', async (event: { data: { files: string[]; dropZone: string } }) => {
    const { files, dropZone } = event.data

    if (dropZone === 'album') {
      pendingFiles.value = files
      pendingFileCount.value = files.length
      showAlbumInput.value = true
    } else {
      try {
        await ConfigManager.SetAlbumName('')
        await ConfigManager.SetAlbumAutoMode(dropZone === 'auto-album')
        await Events.Emit('startUpload', { files })
      } catch (error) {
        console.error('Failed to start upload:', error)
        toast.error('Failed to start upload', {
          description: error instanceof Error ? error.message : String(error),
        })
      }
    }
  })
})

onUnmounted(() => {
  document.removeEventListener('dragenter', onDragEnter)
  document.removeEventListener('dragleave', onDragLeave)
  document.removeEventListener('dragover', onDragOver)
  document.removeEventListener('drop', onDrop)
  window.removeEventListener('albumError', albumErrorHandler)
  window.removeEventListener('uploadError', uploadErrorHandler)
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
  }
})
</script>

<template>
  <main
    class="w-screen h-screen flex flex-col items-center bg-[#090a0f] text-foreground ambient-mesh relative overflow-hidden select-none"
    style="--wails-draggable: drag"
  >
    <!-- FULLSCREEN DRAG OVERLAY (Triggered when dragging files over window) -->
    <div
      v-if="!uploadState.isUploading && isDraggingFiles && options.length > 0"
      class="fixed inset-0 z-50 p-4 flex flex-col gap-3 bg-[#090a0f]/95 backdrop-blur-2xl animate-in fade-in duration-200"
      style="--wails-draggable: none"
    >
      <!-- Zone 1: Regular Upload -->
      <div
        data-file-drop-target
        data-drop-zone="regular"
        class="flex-1 flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-white/20 bg-white/[0.02] hover:bg-emerald-500/10 hover:border-emerald-500/60 transition-all duration-200 drop-zone group cursor-pointer p-4 text-center"
      >
        <div class="drop-zone-icon size-12 rounded-2xl bg-zinc-800/80 border border-white/10 flex items-center justify-center text-zinc-300 group-hover:text-emerald-400 group-hover:scale-110 transition-all mb-2 shadow-lg">
          <UploadCloud class="size-6 stroke-[1.75]" />
        </div>
        <h2 class="drop-zone-title text-base font-semibold text-zinc-200 select-none tracking-tight">
          Upload Trực Tiếp
        </h2>
        <p class="drop-zone-desc text-xs text-zinc-400 mt-1 select-none max-w-[260px]">
          Tải ảnh và video vào Google Photos mà không tạo album
        </p>
      </div>

      <!-- Zone 2: Upload to Album -->
      <div
        data-file-drop-target
        data-drop-zone="album"
        class="flex-1 flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-white/20 bg-white/[0.02] hover:bg-blue-500/10 hover:border-blue-500/60 transition-all duration-200 drop-zone group cursor-pointer p-4 text-center"
      >
        <div class="drop-zone-icon size-12 rounded-2xl bg-zinc-800/80 border border-white/10 flex items-center justify-center text-zinc-300 group-hover:text-blue-400 group-hover:scale-110 transition-all mb-2 shadow-lg">
          <FolderPlus class="size-6 stroke-[1.75]" />
        </div>
        <h2 class="drop-zone-title text-base font-semibold text-zinc-200 select-none tracking-tight">
          Tải Vào Album
        </h2>
        <p class="drop-zone-desc text-xs text-zinc-400 mt-1 select-none max-w-[260px]">
          Chỉ định hoặc tạo album mới để nhóm các tệp tải lên
        </p>
      </div>

      <!-- Zone 3: Auto Album -->
      <div
        data-file-drop-target
        data-drop-zone="auto-album"
        class="flex-1 flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-white/20 bg-white/[0.02] hover:bg-amber-500/10 hover:border-amber-500/60 transition-all duration-200 drop-zone group cursor-pointer p-4 text-center"
      >
        <div class="drop-zone-icon size-12 rounded-2xl bg-zinc-800/80 border border-white/10 flex items-center justify-center text-zinc-300 group-hover:text-amber-400 group-hover:scale-110 transition-all mb-2 shadow-lg">
          <Sparkles class="size-6 stroke-[1.75]" />
        </div>
        <h2 class="drop-zone-title text-base font-semibold text-zinc-200 select-none tracking-tight">
          Tự Động Tạo Album
        </h2>
        <p class="drop-zone-desc text-xs text-zinc-400 mt-1 select-none max-w-[260px]">
          Tự tạo album Google Photos tự động theo tên thư mục
        </p>
      </div>
    </div>

    <!-- MAIN INTERFACE CONTAINER (When not uploading) -->
    <div
      v-else-if="!uploadState.isUploading"
      class="w-full h-full flex flex-col justify-between p-4 max-w-[420px] mx-auto select-none"
      data-file-drop-target
    >
      <!-- STATE 1: NO GOOGLE ACCOUNT CONNECTED -->
      <template v-if="options.length === 0">
        <div class="flex items-center justify-between w-full pt-1 px-1">
          <div class="flex items-center gap-2">
            <span class="size-2 rounded-full bg-zinc-600" />
            <span class="text-xs font-medium text-zinc-400">Chưa kết nối</span>
          </div>
          <Sheet v-model:open="isSettingsOpen">
            <SheetTrigger as-child>
              <button
                type="button"
                class="size-8.5 rounded-full flex items-center justify-center bg-zinc-900/80 hover:bg-zinc-800 border border-white/10 text-zinc-400 hover:text-white transition-all shadow-sm cursor-pointer"
                title="Cài đặt"
                style="--wails-draggable: none"
              >
                <Settings class="size-4" />
              </button>
            </SheetTrigger>
            <SheetContent
              side="bottom"
              class="max-h-[85vh] overflow-y-auto"
              style="--wails-draggable: none"
            >
              <TooltipProvider disable-hoverable-content>
                <SettingsPanel @open-auto-sync="openAutoSyncFromSettings" />
              </TooltipProvider>
            </SheetContent>
          </Sheet>
        </div>

        <div class="flex-1 my-4 w-full rounded-2xl border border-white/10 bg-zinc-900/50 backdrop-blur-xl p-6 flex flex-col items-center justify-center gap-4 text-center shadow-xl">
          <div class="size-14 rounded-2xl bg-gradient-to-tr from-blue-500/20 via-emerald-500/20 to-amber-500/20 border border-white/10 flex items-center justify-center text-zinc-200 shadow-inner">
            <UserPlus class="size-7 text-emerald-400" />
          </div>
          <div>
            <h2 class="text-base font-semibold text-zinc-100 tracking-tight">
              Kết Nối Google Photos
            </h2>
            <p class="text-xs text-zinc-400 mt-1 max-w-[240px] leading-relaxed">
              Thêm tài khoản Google để bắt đầu sao lưu ảnh và video chất lượng cao không giới hạn.
            </p>
          </div>
          <Button
            class="mt-2 h-9 px-5 rounded-full bg-emerald-500 hover:bg-emerald-600 text-zinc-950 font-semibold text-xs shadow-lg shadow-emerald-500/20 cursor-pointer"
            style="--wails-draggable: none"
            @click="openAccountSetup"
          >
            Thêm tài khoản Google
          </Button>
        </div>

        <div class="w-full text-center text-[11px] text-zinc-600 pb-1">
          gotohp • Google Photos Desktop Uploader
        </div>
      </template>

      <!-- STATE 2: READY WITH ACCOUNT -->
      <template v-else>
        <!-- ALBUM INPUT VIEW (When files dropped on album zone) -->
        <template v-if="showAlbumInput">
          <div class="flex items-center justify-between w-full pt-1 px-1">
            <div class="flex items-center gap-2">
              <span class="size-2 rounded-full bg-blue-400 animate-pulse" />
              <span class="text-xs font-medium text-zinc-300">Tạo Album</span>
            </div>
          </div>

          <div
            class="flex-1 my-3 w-full rounded-2xl border border-white/10 bg-zinc-900/80 backdrop-blur-xl p-6 flex flex-col items-center justify-center gap-4 text-center shadow-2xl"
            style="--wails-draggable: none"
          >
            <div class="size-13 rounded-2xl bg-blue-500/10 border border-blue-500/20 text-blue-400 flex items-center justify-center shadow-inner">
              <FolderPlus class="size-6.5" />
            </div>
            <div>
              <h2 class="text-base font-semibold text-zinc-100 tracking-tight">
                Tải Vào Album
              </h2>
              <p class="text-xs text-zinc-400 mt-1">
                {{ pendingFileCount }} tệp đã sẵn sàng tải lên
              </p>
            </div>
            
            <div class="flex flex-col gap-1.5 w-full max-w-xs text-left">
              <Label
                for="album-input"
                class="text-zinc-400 text-xs font-medium"
              >Tên album hoặc Key album</Label>
              <Input
                id="album-input"
                v-model="albumNameOrKey"
                placeholder="Nhập tên album hoặc key AF1Qip..."
                class="h-9 text-xs bg-zinc-950/60 border-white/15 focus-visible:ring-blue-500/40"
                autofocus
              />
            </div>

            <div class="flex gap-2.5 w-full max-w-xs pt-1">
              <Button
                variant="outline"
                class="flex-1 h-9 rounded-xl border-white/10 text-xs cursor-pointer select-none"
                @click="cancelAlbumUpload"
              >
                Hủy
              </Button>
              <Button
                class="flex-1 h-9 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-medium text-xs cursor-pointer select-none shadow-lg shadow-blue-600/20"
                :disabled="!albumNameOrKey.trim()"
                @click="confirmAlbumUpload"
              >
                Tải lên
              </Button>
            </div>
          </div>
        </template>

        <!-- NORMAL DASHBOARD VIEW -->
        <template v-else>
          <!-- TOP HEADER ROW -->
          <header class="flex items-center justify-between gap-2 w-full pt-1 px-1">
            <!-- Account Selector Pill -->
            <GoogleAccountSelect
              v-model="selectedOption"
              :options="options"
              :removing-account="removingAccount"
              @item-removed="removeCredentials"
              @add="openAccountSetup"
            />

            <!-- Settings Sheet Trigger -->
            <Sheet v-model:open="isSettingsOpen">
              <SheetTrigger as-child>
                <button
                  type="button"
                  class="size-8.5 rounded-full flex items-center justify-center bg-zinc-900/80 hover:bg-zinc-800 border border-white/10 hover:border-white/20 text-zinc-400 hover:text-white transition-all shadow-sm cursor-pointer"
                  title="Cài đặt"
                  style="--wails-draggable: none"
                >
                  <Settings class="size-4" />
                </button>
              </SheetTrigger>
              <SheetContent
                side="bottom"
                class="max-h-[85vh] overflow-y-auto"
                style="--wails-draggable: none"
              >
                <TooltipProvider disable-hoverable-content>
                  <SettingsPanel @open-auto-sync="openAutoSyncFromSettings" />
                </TooltipProvider>
              </SheetContent>
            </Sheet>
          </header>

          <!-- TOKEN BINDING PROMPT (If needed for rooted device) -->
          <div
            v-if="tokenBindingEmail"
            class="w-full my-2 border border-amber-500/20 bg-amber-500/10 rounded-xl p-3 flex flex-col gap-2.5"
            style="--wails-draggable: none"
          >
            <p class="text-xs text-amber-200/90 leading-relaxed">
              Tài khoản này cần token binding key từ thiết bị Android đã root.
            </p>
            <Button
              size="sm"
              class="cursor-pointer select-none text-xs h-7.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg"
              :disabled="isExtractingTokenBinding"
              @click="addTokenBindingAliasFromADB"
            >
              {{ isExtractingTokenBinding ? 'Đang đọc qua ADB...' : 'Đọc từ ADB' }}
            </Button>
          </div>

          <!-- RECENT UPLOAD RESULTS SUMMARY CARD (When finished previous uploads) -->
          <div
            v-if="uploadState.uploadedFiles > 0 || uploadState.results.fail.length > 0"
            class="my-2.5 w-full rounded-2xl border border-white/10 bg-zinc-900/70 backdrop-blur-xl p-3.5 flex flex-col gap-2.5 shadow-xl"
            style="--wails-draggable: none"
          >
            <div class="flex items-center justify-between w-full">
              <div class="flex items-center gap-1.5">
                <CheckCircle2 class="size-4 text-emerald-400" />
                <h3 class="text-xs font-semibold text-zinc-200">
                  Kết Quả Tải Lên Gần Đây
                </h3>
              </div>
              <span
                v-if="uploadState.results.fail.length > 0"
                class="text-[11px] text-red-400 font-medium bg-red-500/10 px-2 py-0.5 rounded-full border border-red-500/20"
              >
                {{ uploadState.results.fail.length }} lỗi
              </span>
            </div>

            <!-- Stats grid -->
            <div class="grid grid-cols-3 gap-1.5 w-full text-xs">
              <div class="flex flex-col items-center justify-center p-1.5 rounded-lg bg-white/[0.03] border border-white/5">
                <span class="text-[10px] text-zinc-500">Thành công</span>
                <span class="font-semibold text-emerald-400">{{ uploadState.results.success.length }}</span>
              </div>
              <div class="flex flex-col items-center justify-center p-1.5 rounded-lg bg-white/[0.03] border border-white/5">
                <span class="text-[10px] text-zinc-500">Thất bại</span>
                <span
                  class="font-semibold"
                  :class="uploadState.results.fail.length > 0 ? 'text-red-400' : 'text-zinc-400'"
                >{{ uploadState.results.fail.length }}</span>
              </div>
              <div class="flex flex-col items-center justify-center p-1.5 rounded-lg bg-white/[0.03] border border-white/5">
                <span class="text-[10px] text-zinc-500">Bỏ qua</span>
                <span class="font-semibold text-amber-400">{{ uploadState.results.skipped.length }}</span>
              </div>
            </div>

            <!-- Retry failed button if failures exist -->
            <div
              v-if="uploadState.results.fail.length > 0"
              class="w-full flex flex-col gap-2 pt-1 border-t border-white/5"
            >
              <Button
                size="sm"
                class="w-full cursor-pointer select-none bg-amber-600 hover:bg-amber-500 text-white font-medium text-xs h-7.5 rounded-lg transition-colors shadow-sm"
                @click="retryCurrentFailedFiles"
              >
                <RefreshCw class="size-3 mr-1.5" />
                Thử lại các tệp lỗi ({{ uploadState.results.fail.length }})
              </Button>

              <button
                type="button"
                class="flex items-center justify-between w-full text-[11px] text-zinc-400 hover:text-zinc-200 cursor-pointer select-none"
                @click="showFailedDetails = !showFailedDetails"
              >
                <span class="flex items-center gap-1 text-red-400 font-medium">
                  <AlertTriangle class="size-3" />
                  Xem chi tiết file lỗi
                </span>
                <component
                  :is="showFailedDetails ? ChevronUp : ChevronDown"
                  class="size-3"
                />
              </button>

              <div
                v-if="showFailedDetails && uploadState.results.failedItems?.length"
                class="max-h-28 overflow-y-auto space-y-1 pr-1 w-full text-left"
              >
                <div
                  v-for="(item, idx) in uploadState.results.failedItems"
                  :key="idx"
                  class="p-2 rounded-lg bg-red-500/10 border border-red-500/20 text-[11px] text-red-300"
                >
                  <p class="font-medium truncate text-zinc-200">
                    {{ item.fileName }}
                  </p>
                  <p class="text-[10px] text-red-400 mt-0.5 truncate">
                    {{ item.error }}
                  </p>
                </div>
              </div>
            </div>

            <!-- Quick Actions: Open in Google Photos & Copy Link -->
            <div
              v-if="uploadState.results.success.length > 0"
              class="w-full flex flex-col gap-2 pt-2 border-t border-white/5"
            >
              <div class="flex items-center gap-2 w-full">
                <!-- Primary Action: Open in Browser -->
                <Button
                  size="sm"
                  class="flex-1 cursor-pointer select-none bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white font-medium text-xs h-8 rounded-lg transition-all shadow-sm flex items-center justify-center gap-1.5"
                  @click="openInGooglePhotos"
                >
                  <ExternalLink class="size-3.5" />
                  <span>{{ hasCreatedAlbum ? 'Mở Album trên Web' : 'Mở trên Google Photos' }}</span>
                </Button>

                <!-- Copy Link Button -->
                <Button
                  variant="outline"
                  size="sm"
                  class="cursor-pointer select-none text-xs h-8 px-3 rounded-lg border-white/10 hover:bg-white/5 flex items-center gap-1.5 shrink-0"
                  @click="handleCopyShareLink"
                >
                  <component
                    :is="isCopiedLink ? Check : Copy"
                    class="size-3.5"
                    :class="isCopiedLink ? 'text-emerald-400' : 'text-zinc-300'"
                  />
                  <span>{{ copyLinkButtonText }}</span>
                </Button>
              </div>

              <!-- Sharing Tip -->
              <div class="flex items-start gap-1.5 text-[10.5px] text-zinc-400 bg-white/[0.02] p-2 rounded-lg border border-white/5 leading-relaxed">
                <Sparkles class="size-3.5 text-amber-400 shrink-0 mt-0.5" />
                <span>
                  {{ hasCreatedAlbum
                    ? 'Mở album trên trình duyệt rồi bấm biểu tượng Chia sẻ ➔ Tạo liên kết để lấy link công khai.'
                    : 'Mở ảnh trên trình duyệt rồi bấm biểu tượng Chia sẻ ➔ Tạo liên kết để lấy link công khai.'
                  }}
                </span>
              </div>
            </div>

            <!-- Action buttons -->
            <div class="flex items-center gap-2 w-full pt-0.5">
              <Button
                variant="outline"
                size="sm"
                class="flex-1 cursor-pointer select-none text-[11px] h-7 rounded-lg border-white/10"
                @click="handleCopyClick"
              >
                {{ copyButtonText }}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                class="cursor-pointer select-none text-[11px] h-7 text-zinc-400 hover:text-zinc-200"
                @click="isHistoryOpen = true"
              >
                Lịch sử
              </Button>
            </div>
          </div>

          <!-- HERO DROP TARGET CARD (The Centerpiece of gotohp) -->
          <div
            data-file-drop-target
            data-drop-zone="regular"
            class="group relative flex-1 my-3 w-full rounded-2xl border border-dashed border-white/15 hover:border-emerald-500/50 bg-gradient-to-b from-white/[0.04] to-transparent hover:from-emerald-500/[0.03] backdrop-blur-md p-6 flex flex-col items-center justify-center text-center transition-all duration-300 shadow-xl drop-zone cursor-default"
            style="--wails-draggable: none"
          >
            <!-- Ambient hover halo -->
            <div class="absolute inset-0 rounded-2xl bg-radial from-emerald-500/10 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />

            <!-- Elevated Icon with Glow -->
            <div class="relative mb-3.5">
              <div class="absolute -inset-1 rounded-2xl bg-emerald-500/20 blur-md group-hover:bg-emerald-500/35 transition-all duration-300" />
              <div class="relative size-14 rounded-2xl bg-gradient-to-b from-zinc-800 to-zinc-900 border border-white/15 shadow-xl flex items-center justify-center text-emerald-400 group-hover:scale-105 group-hover:text-emerald-300 transition-all duration-300">
                <UploadCloud class="size-7 stroke-[1.75]" />
              </div>
            </div>

            <!-- Headlines -->
            <h2 class="text-sm font-semibold text-zinc-100 tracking-tight select-none">
              Kéo & Thả ảnh, video vào đây
            </h2>
            <p class="text-xs text-zinc-400 mt-1 select-none max-w-[240px] leading-relaxed">
              Hỗ trợ kéo thả cả thư mục hoặc nhiều tệp cùng lúc
            </p>

            <!-- Supported Format Chips -->
            <div class="mt-4 flex flex-wrap items-center justify-center gap-1.5 text-[10px] text-zinc-400 select-none">
              <span class="px-2 py-0.5 rounded-full bg-white/[0.04] border border-white/[0.06] font-mono">JPG</span>
              <span class="px-2 py-0.5 rounded-full bg-white/[0.04] border border-white/[0.06] font-mono">PNG</span>
              <span class="px-2 py-0.5 rounded-full bg-white/[0.04] border border-white/[0.06] font-mono">MP4</span>
              <span class="px-2 py-0.5 rounded-full bg-white/[0.04] border border-white/[0.06] font-mono">RAW</span>
              <span class="px-2 py-0.5 rounded-full bg-white/[0.04] border border-white/[0.06] font-mono">Live Photo</span>
            </div>

            <!-- Smart Drop Modes Tip -->
            <div class="mt-3.5 flex items-center gap-1.5 text-[11px] text-zinc-400">
              <Sparkles class="size-3 text-amber-400/80" />
              <span>Kéo tệp vào cửa sổ để chọn 3 chế độ tải</span>
            </div>
          </div>

          <!-- FOOTER ACTION DOCK -->
          <footer class="flex items-center gap-2 w-full pt-1 px-1">
            <!-- Auto-Sync Status Button -->
            <button
              type="button"
              class="flex-1 h-9 px-3 rounded-xl bg-zinc-900/80 hover:bg-zinc-800/90 border border-white/10 hover:border-white/20 text-xs font-medium text-zinc-200 transition-all flex items-center justify-between cursor-pointer shadow-sm"
              style="--wails-draggable: none"
              @click="isAutoSyncOpen = true"
            >
              <div class="flex items-center gap-2">
                <span
                  class="size-2 rounded-full"
                  :class="[
                    !autoSyncStatus.enabled ? 'bg-zinc-600' : autoSyncStatus.isSyncing ? 'bg-amber-400 animate-pulse' : 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.6)]'
                  ]"
                />
                <span>Auto-Sync</span>
              </div>
              <span class="text-[11px] text-zinc-500 font-normal">
                {{ !autoSyncStatus.enabled ? 'Tắt' : autoSyncStatus.isSyncing ? 'Đang đồng bộ...' : `${autoSyncStatus.folderCount || 0} thư mục` }}
              </span>
            </button>

            <!-- Upload History Button -->
            <button
              type="button"
              class="h-9 px-3.5 rounded-xl bg-zinc-900/80 hover:bg-zinc-800/90 border border-white/10 hover:border-white/20 text-xs font-medium text-zinc-200 transition-all flex items-center gap-1.5 relative cursor-pointer shadow-sm"
              style="--wails-draggable: none"
              @click="isHistoryOpen = true"
            >
              <History class="size-3.5 text-zinc-400" />
              <span>Lịch sử</span>
              <span
                v-if="failedQueueCount > 0"
                class="inline-flex items-center justify-center px-1.5 py-0.2 text-[10px] font-bold rounded-full bg-red-500 text-white shadow-sm"
              >
                {{ failedQueueCount }}
              </span>
            </button>
          </footer>
        </template>
      </template>
    </div>

    <!-- STATE 3: UPLOAD IN PROGRESS -->
    <div
      v-if="uploadState.isUploading"
      class="w-full h-full"
    >
      <Upload />
    </div>

    <!-- MODALS & OVERLAYS -->
    <GoogleAuthSetup
      v-model:open="isAccountSetupOpen"
      @account-added="refreshCredentials"
    />
    <AutoSyncModal
      v-model:open="isAutoSyncOpen"
    />
    <UploadHistoryModal
      v-model:open="isHistoryOpen"
      @retry-files="handleRetryFiles"
    />
    <Toaster
      position="bottom-center"
      rich-colors
      expand
      :visible-toasts="4"
    />
  </main>
</template>
