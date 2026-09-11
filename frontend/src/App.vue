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
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { UserPlus, History, RefreshCw, AlertTriangle, ChevronDown, ChevronUp } from '@lucide/vue'
import { ConfigManager, type AutoSyncStatus } from '../bindings/app/backend'
import { Events } from '@wailsio/runtime'
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
const copyButtonText = ref('Copy as JSON');

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
  copyButtonText.value = 'Copied!';
  setTimeout(() => copyButtonText.value = 'Copy as JSON', 1000);
};



// Global drag event handlers to detect file dragging
let dragLeaveTimeout: ReturnType<typeof setTimeout> | null = null

const onDragEnter = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  
  // Clear any pending drag leave timeout
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
  
  isDraggingFiles.value = true
}

const onDragOver = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  e.preventDefault()
  
  // Clear any pending drag leave timeout - we're still dragging
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
}

const onDragLeave = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  
  // Use timeout to detect if we've truly left the window
  // dragover will cancel this if we're still in the window
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
  }
  dragLeaveTimeout = setTimeout(() => {
    isDraggingFiles.value = false
    dragLeaveTimeout = null
  }, 50)
}

const onDrop = () => {
  // Clear any pending timeout
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
  
  // Delay resetting isDraggingFiles to allow Wails to process the drop target
  // before Vue re-renders and hides the drop zones
  setTimeout(() => {
    isDraggingFiles.value = false
  }, 100)
}

// Album upload confirmation
const confirmAlbumUpload = async () => {
  // Set album name in backend (not persisted to disk)
  await ConfigManager.SetAlbumName(albumNameOrKey.value)
  await ConfigManager.SetAlbumAutoMode(false)
  // Start upload with pending files
  Events.Emit('startUpload', { files: pendingFiles.value })
  showAlbumInput.value = false
  pendingFiles.value = []
  pendingFileCount.value = 0
  albumNameOrKey.value = '' // Reset for next upload
}

const cancelAlbumUpload = () => {
  showAlbumInput.value = false
  pendingFiles.value = []
  pendingFileCount.value = 0
  albumNameOrKey.value = '' // Reset on cancel too
}

// Handle album error event
const albumErrorHandler = (e: Event) => {
  const event = e as CustomEvent<{ AlbumName: string; Error: string }>
  const { AlbumName, Error } = event.detail
  // Check if it's a 404 error (album key not found)
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

  // Listen for files-dropped event from backend
  Events.On('files-dropped', async (event: { data: { files: string[]; dropZone: string } }) => {
    const { files, dropZone } = event.data

    if (dropZone === 'album') {
      // Show album input screen
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
    class="w-screen h-screen flex flex-col items-center"
    style="--wails-draggable: drag"
  >
    <!-- Drop zones shown when dragging files -->
    <div
      v-if="!uploadState.isUploading && isDraggingFiles && options.length > 0"
      class="w-screen h-screen flex flex-col gap-3 p-6"
      style="--wails-draggable: none"
    >
      <div
        data-file-drop-target
        data-drop-zone="regular"
        class="flex-1 flex flex-col items-center justify-center border-2 border-dashed border-muted-foreground/50 rounded-xl transition-all duration-200 drop-zone"
      >
        <h2 class="text-xl font-semibold select-none text-muted-foreground">
          Upload Only
        </h2>
        <p class="text-sm text-muted-foreground/70 mt-2 select-none text-center px-4">
          Upload files without adding to any album
        </p>
      </div>
      <div
        data-file-drop-target
        data-drop-zone="album"
        class="flex-1 flex flex-col items-center justify-center border-2 border-dashed border-muted-foreground/50 rounded-xl transition-all duration-200 drop-zone"
      >
        <h2 class="text-xl font-semibold select-none text-muted-foreground">
          Upload to Album
        </h2>
        <p class="text-sm text-muted-foreground/70 mt-2 select-none text-center px-4">
          Upload and add to a specific album (you'll enter the name)
        </p>
      </div>
      <div
        data-file-drop-target
        data-drop-zone="auto-album"
        class="flex-1 flex flex-col items-center justify-center border-2 border-dashed border-muted-foreground/50 rounded-xl transition-all duration-200 drop-zone"
      >
        <h2 class="text-xl font-semibold select-none text-muted-foreground">
          Auto Album
        </h2>
        <p class="text-sm text-muted-foreground/70 mt-2 select-none text-center px-4">
          Upload and create albums automatically based on folder names
        </p>
      </div>
    </div>

    <!-- Normal UI (not dragging) -->
    <div
      v-else-if="!uploadState.isUploading"
      class="w-screen h-screen flex flex-col items-center gap-4 max-w-md px-6 pt-30"
      data-file-drop-target
    >
      <template v-if="options.length === 0">
        <div class="flex max-w-xs flex-col items-center gap-4 text-center">
          <div class="flex size-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
            <UserPlus class="size-5" />
          </div>
          <div class="flex flex-col gap-1">
            <h1 class="text-xl font-semibold select-none">
              Connect Google Photos
            </h1>
            <p class="text-sm text-muted-foreground select-none">
              Add an account before uploading photos and videos.
            </p>
          </div>
          <Button
            class="cursor-pointer select-none"
            @click="openAccountSetup"
          >
            Add Google account
          </Button>
        </div>
      </template>

      <template v-else>
        <!-- Show album input screen when files dropped on album zone -->
        <template v-if="showAlbumInput">
          <div
            class="flex flex-col items-center justify-center gap-6 p-8"
            style="--wails-draggable: none"
          >
            <h1 class="text-xl font-semibold select-none">
              Upload to Album
            </h1>
            <p class="text-muted-foreground select-none">
              {{ pendingFileCount }} file(s) ready to upload
            </p>
            
            <div class="flex flex-col gap-2 w-full max-w-xs">
              <Label
                for="album-input"
                class="text-muted-foreground text-sm"
              >Album name or key</Label>
              <Input
                id="album-input"
                v-model="albumNameOrKey"
                placeholder="Album name or AF1Qip... key"
                autofocus
              />
            </div>

            <div class="flex gap-4">
              <Button
                variant="outline"
                class="cursor-pointer select-none"
                @click="cancelAlbumUpload"
              >
                Cancel
              </Button>
              <Button
                class="cursor-pointer select-none"
                :disabled="!albumNameOrKey.trim()"
                @click="confirmAlbumUpload"
              >
                Upload
              </Button>
            </div>
          </div>
        </template>

        <!-- Normal UI when not dragging -->
        <template v-else>
          <h1 class="text-xl font-semibold select-none">
            Drop files to upload
          </h1>
          <GoogleAccountSelect
            v-model="selectedOption"
            :options="options"
            :removing-account="removingAccount"
            @item-removed="removeCredentials"
            @add="openAccountSetup"
          />
          <div
            v-if="tokenBindingEmail"
            class="w-full max-w-xs border rounded-lg p-3 flex flex-col gap-3"
            style="--wails-draggable: none"
          >
            <p class="text-sm text-muted-foreground">
              This credential needs a token binding key from the rooted Android device it was captured from.
            </p>
            <Button
              class="cursor-pointer select-none"
              :disabled="isExtractingTokenBinding"
              @click="addTokenBindingAliasFromADB"
            >
              {{ isExtractingTokenBinding ? 'Reading ADB...' : 'Read from ADB' }}
            </Button>
          </div>

          <div class="flex gap-2.5">
            <Button
              variant="outline"
              class="cursor-pointer select-none gap-2"
              @click="isAutoSyncOpen = true"
            >
              <span
                class="size-2 rounded-full"
                :class="[
                  !autoSyncStatus.enabled ? 'bg-zinc-500' : autoSyncStatus.isSyncing ? 'bg-amber-500 animate-pulse' : 'bg-emerald-500'
                ]"
              />
              <span>Auto-Sync</span>
            </Button>

            <Button
              variant="outline"
              class="cursor-pointer select-none gap-1.5 relative"
              @click="isHistoryOpen = true"
            >
              <History class="size-3.5 text-muted-foreground" />
              <span>Lịch sử</span>
              <span
                v-if="failedQueueCount > 0"
                class="inline-flex items-center justify-center px-1.5 py-0.2 text-[10px] font-bold rounded-full bg-destructive text-destructive-foreground"
              >
                {{ failedQueueCount }}
              </span>
            </Button>

            <Sheet v-model:open="isSettingsOpen">
              <SheetTrigger as-child>
                <Button
                  variant="outline"
                  class="cursor-pointer select-none"
                >
                  Settings
                </Button>
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

          <div
            v-if="uploadState.uploadedFiles > 0 || uploadState.results.fail.length > 0"
            class="flex flex-col items-center gap-2.5 border rounded-xl p-4 mt-4 w-full max-w-sm bg-card/60 backdrop-blur-sm"
          >
            <div class="flex items-center justify-between w-full">
              <h2 class="text-sm font-semibold select-none">
                Kết Quả Upload
              </h2>
              <span
                v-if="uploadState.results.fail.length > 0"
                class="text-xs text-destructive font-medium bg-destructive/10 px-2 py-0.5 rounded-full border border-destructive/20"
              >
                {{ uploadState.results.fail.length }} lỗi
              </span>
            </div>

            <!-- Stats grid -->
            <div class="grid grid-cols-2 gap-2 w-full text-xs">
              <div class="flex items-center justify-between p-2 rounded-lg bg-muted/40 border">
                <span class="text-muted-foreground">Thành công:</span>
                <span class="font-semibold text-emerald-500">{{ uploadState.results.success.length }}</span>
              </div>
              <div class="flex items-center justify-between p-2 rounded-lg bg-muted/40 border">
                <span class="text-muted-foreground">Thất bại:</span>
                <span class="font-semibold text-destructive">{{ uploadState.results.fail.length }}</span>
              </div>
              <div class="flex items-center justify-between p-2 rounded-lg bg-muted/40 border">
                <span class="text-muted-foreground">Bỏ qua:</span>
                <span class="font-semibold text-amber-500">{{ uploadState.results.skipped.length }}</span>
              </div>
              <div class="flex items-center justify-between p-2 rounded-lg bg-muted/40 border">
                <span class="text-muted-foreground">Cảnh báo:</span>
                <span class="font-semibold text-foreground">{{ uploadState.results.warnings.length }}</span>
              </div>
            </div>

            <!-- 1-CLICK RETRY FAILED BUTTON WHEN FAILURES OCCUR -->
            <div
              v-if="uploadState.results.fail.length > 0"
              class="w-full flex flex-col gap-2 pt-1 border-t border-border/40"
            >
              <Button
                size="sm"
                class="w-full cursor-pointer select-none bg-amber-600 hover:bg-amber-700 text-white font-medium text-xs h-8 shadow-sm transition-colors"
                @click="retryCurrentFailedFiles"
              >
                <RefreshCw class="size-3.5 mr-1.5" />
                Thử lại tất cả file lỗi ({{ uploadState.results.fail.length }})
              </Button>

              <!-- Expand/collapse failed items details -->
              <button
                type="button"
                class="flex items-center justify-between w-full text-xs text-muted-foreground hover:text-foreground pt-1 cursor-pointer select-none"
                @click="showFailedDetails = !showFailedDetails"
              >
                <span class="flex items-center gap-1 text-destructive font-medium">
                  <AlertTriangle class="size-3" />
                  Chi tiết các file bị lỗi
                </span>
                <component
                  :is="showFailedDetails ? ChevronUp : ChevronDown"
                  class="size-3.5"
                />
              </button>

              <!-- Failed items list -->
              <div
                v-if="showFailedDetails && uploadState.results.failedItems?.length"
                class="max-h-40 overflow-y-auto space-y-1.5 pr-1 w-full"
              >
                <div
                  v-for="(item, idx) in uploadState.results.failedItems"
                  :key="idx"
                  class="p-2 rounded-lg bg-destructive/10 border border-destructive/20 text-xs text-destructive text-left"
                >
                  <p class="font-medium truncate text-foreground">
                    {{ item.fileName }}
                  </p>
                  <p class="text-[11px] text-muted-foreground truncate font-mono mt-0.5">
                    {{ item.path }}
                  </p>
                  <p class="text-[11px] font-normal mt-1 leading-tight text-destructive dark:text-red-400">
                    Lý do: {{ item.error }}
                  </p>
                </div>
              </div>
            </div>

            <!-- Footer action buttons -->
            <div class="flex items-center gap-2 w-full pt-1">
              <Button
                variant="outline"
                size="sm"
                class="flex-1 cursor-pointer select-none text-xs h-8"
                @click="handleCopyClick"
              >
                {{ copyButtonText }}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                class="cursor-pointer select-none text-xs h-8 text-muted-foreground hover:text-foreground"
                @click="isHistoryOpen = true"
              >
                Xem lịch sử
              </Button>
            </div>
          </div>
        </template>
      </template>
    </div>
    <div
      v-if="uploadState.isUploading"
      class="w-full h-full"
    >
      <Upload />
    </div>
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
