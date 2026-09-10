<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { Browser } from '@wailsio/runtime'
import { toast } from 'vue-sonner'
import {
  ChevronDown,
  ExternalLink,
  Loader2,
  ClipboardPaste,
  ShieldCheck,
  XCircle,
} from '@lucide/vue'
import { ConfigManager } from '../../bindings/app/backend'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

const embeddedSetupURL = 'https://accounts.google.com/EmbeddedSetup'

const emit = defineEmits<{
  (event: 'account-added'): void
}>()

const isOpen = defineModel<boolean>('open', { default: false })
const oauthToken = ref('')
const rawCredential = ref('')
const isConnecting = ref(false)
const isInAppLoggingIn = ref(false)
const isAddingRawCredential = ref(false)
const showManualOptions = ref(false)
const showRawCredential = ref(false)

watch(isOpen, (open) => {
  if (!open) {
    oauthToken.value = ''
    rawCredential.value = ''
    showManualOptions.value = false
    showRawCredential.value = false
    if (isInAppLoggingIn.value) {
      cancelInAppLogin()
    }
  } else {
    checkClipboardForToken(false)
  }
})

// Auto-check clipboard when window regains focus while dialog is open
function handleWindowFocus() {
  if (isOpen.value && !isConnecting.value && !isInAppLoggingIn.value) {
    checkClipboardForToken(false)
  }
}

onMounted(() => {
  window.addEventListener('focus', handleWindowFocus)
})

onUnmounted(() => {
  window.removeEventListener('focus', handleWindowFocus)
})

async function checkClipboardForToken(showToastOnEmpty = true) {
  try {
    if (!navigator.clipboard?.readText) return
    const text = await navigator.clipboard.readText()
    const trimmed = text.trim()

    if (trimmed.startsWith('oauth2_4/') || trimmed.startsWith('oauth_token=')) {
      const cleanToken = trimmed.replace(/^oauth_token=/, '')
      if (cleanToken !== oauthToken.value) {
        oauthToken.value = cleanToken
        toast.info('Recognized Google oauth_token from clipboard!', {
          description: 'Click "Connect account" to finish.',
        })
      }
    } else if (trimmed.includes('androidId=') && trimmed.includes('Email=')) {
      if (trimmed !== rawCredential.value) {
        rawCredential.value = trimmed
        showManualOptions.value = true
        showRawCredential.value = true
        toast.info('Recognized credential string from clipboard!')
      }
    } else if (showToastOnEmpty) {
      toast.info('No Google token found in clipboard', {
        description: 'Copy the token from your browser first.',
      })
    }
  } catch {
    if (showToastOnEmpty) {
      toast.error('Could not access clipboard')
    }
  }
}

async function startInAppLogin() {
  isInAppLoggingIn.value = true
  try {
    const connectedEmail = await ConfigManager.StartInAppGoogleLogin()
    isOpen.value = false
    emit('account-added')
    toast.success('Google account connected.', {
      description: connectedEmail,
    })
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    if (message.includes('closed') || message.includes('cancelled')) {
      toast.info('Sign-in cancelled')
    } else {
      toast.error('Could not connect Google account', {
        description: message,
      })
    }
  } finally {
    isInAppLoggingIn.value = false
  }
}

async function cancelInAppLogin() {
  try {
    await ConfigManager.CancelInAppGoogleLogin()
  } catch {
    // Ignore error on cancel
  } finally {
    isInAppLoggingIn.value = false
  }
}

async function openEmbeddedSetup() {
  try {
    await Browser.OpenURL(embeddedSetupURL)
  } catch (error) {
    toast.error('Could not open the system browser', {
      description: error instanceof Error ? error.message : String(error),
    })
  }
}

async function connectAccount() {
  const normalizedToken = oauthToken.value.trim()
  if (!normalizedToken) return

  isConnecting.value = true
  try {
    const connectedEmail = await ConfigManager.AddGoogleAccount(normalizedToken)
    oauthToken.value = ''
    isOpen.value = false
    emit('account-added')
    toast.success('Google account connected.', {
      description: connectedEmail,
    })
  } catch (error) {
    toast.error('Could not connect Google account', {
      description: error instanceof Error ? error.message : String(error),
    })
  } finally {
    oauthToken.value = ''
    isConnecting.value = false
  }
}

async function addRawCredential() {
  const credential = rawCredential.value.trim()
  if (!credential) return

  isAddingRawCredential.value = true
  try {
    await ConfigManager.AddCredentials(credential)
    rawCredential.value = ''
    isOpen.value = false
    emit('account-added')
    toast.success('Credentials added.')
  } catch (error) {
    toast.error('Could not add credentials', {
      description: error instanceof Error ? error.message : String(error),
    })
  } finally {
    isAddingRawCredential.value = false
  }
}
</script>

<template>
  <Sheet v-model:open="isOpen">
    <SheetContent
      side="bottom"
      class="max-h-[92vh] overflow-y-auto"
      style="--wails-draggable: none"
    >
      <div class="mx-auto flex w-full max-w-sm flex-col px-5 pt-1 pb-6">
        <SheetHeader class="gap-0 px-0 pb-4 text-center">
          <SheetTitle>Connect Google Photos</SheetTitle>
        </SheetHeader>

        <!-- Primary Flow: Automatic In-App Sign-in -->
        <div class="flex flex-col gap-3">
          <div
            v-if="isInAppLoggingIn"
            class="flex flex-col items-center gap-3 rounded-xl border border-primary/20 bg-primary/5 p-4 text-center"
          >
            <Loader2 class="size-8 animate-spin text-primary" />
            <div class="flex flex-col gap-1">
              <p class="text-sm font-medium">
                Waiting for Google sign-in...
              </p>
              <p class="text-xs text-muted-foreground leading-relaxed">
                Sign in on the pop-up window and click <span class="font-medium text-foreground">I agree</span>.<br>
                The app will automatically capture your session and close the window.
              </p>
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              class="mt-1 cursor-pointer select-none text-xs"
              @click="cancelInAppLogin"
            >
              <XCircle class="size-3.5 mr-1" />
              Cancel sign-in
            </Button>
          </div>

          <div
            v-else
            class="flex flex-col gap-2"
          >
            <Button
              type="button"
              class="relative flex h-11 w-full cursor-pointer items-center justify-center gap-2.5 rounded-lg bg-foreground font-medium text-background shadow transition-transform hover:opacity-90 active:scale-[0.99] select-none"
              :disabled="isConnecting"
              @click="startInAppLogin"
            >
              <!-- Google Colorful SVG Icon -->
              <svg
                class="size-4 shrink-0"
                viewBox="0 0 24 24"
              >
                <path
                  fill="#4285F4"
                  d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.8-2.4 3.67v3.05h3.88c2.27-2.09 3.66-5.17 3.66-9.16z"
                />
                <path
                  fill="#34A853"
                  d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.24v3.15C3.26 21.36 7.33 24 12 24z"
                />
                <path
                  fill="#FBBC05"
                  d="M5.28 14.27A7.18 7.18 0 0 1 4.9 12c0-.79.14-1.57.38-2.27V6.58H1.24A11.97 11.97 0 0 0 0 12c0 1.92.45 3.74 1.24 5.42l4.04-3.15z"
                />
                <path
                  fill="#EA4335"
                  d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.33 0 3.26 2.64 1.24 6.58l4.04 3.15c.95-2.83 3.6-4.98 6.72-4.98z"
                />
              </svg>
              <span>Sign in with Google (Automatic)</span>
            </Button>
            <p class="flex items-center justify-center gap-1.5 text-center text-[11px] text-muted-foreground select-none">
              <ShieldCheck class="size-3.5 text-emerald-500" />
              <span>Token is captured automatically without DevTools</span>
            </p>
          </div>
        </div>

        <!-- Quick Clipboard Paste Assistant -->
        <div class="mt-4 flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2 text-xs">
          <span class="text-muted-foreground select-none">Copied token elsewhere?</span>
          <button
            type="button"
            class="flex items-center gap-1 font-medium text-foreground hover:underline select-none cursor-pointer"
            @click="() => checkClipboardForToken(true)"
          >
            <ClipboardPaste class="size-3.5" />
            <span>Paste from clipboard</span>
          </button>
        </div>

        <!-- Manual & Advanced Fallback Accordion -->
        <div class="mt-4 border-t pt-3">
          <button
            type="button"
            class="flex w-full items-center justify-between text-xs text-muted-foreground select-none hover:text-foreground"
            @click="showManualOptions = !showManualOptions"
          >
            <span>Manual token / Advanced sign-in</span>
            <ChevronDown
              class="size-3.5 transition-transform"
              :class="showManualOptions && 'rotate-180'"
            />
          </button>

          <div
            v-if="showManualOptions"
            class="mt-3 flex flex-col gap-4"
          >
            <!-- Step 1: Open in external browser -->
            <div class="flex flex-col gap-2">
              <p class="flex items-center gap-2 text-xs font-medium select-none">
                <span class="flex size-4 shrink-0 items-center justify-center rounded-full bg-muted text-[10px]">1</span>
                Open sign-in in system browser
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                class="cursor-pointer select-none text-xs"
                :disabled="isConnecting"
                @click="openEmbeddedSetup"
              >
                <ExternalLink class="size-3.5 mr-1" />
                Open sign-in page
              </Button>
              <ol class="list-decimal space-y-1 rounded-lg border bg-muted/30 py-2 pr-3 pl-6 text-[11px] leading-snug text-muted-foreground">
                <li>Sign in, then click <span class="text-foreground">I agree</span>.</li>
                <li>Open DevTools (F12) → Storage/Application → Cookies → accounts.google.com.</li>
                <li>Copy the <code class="text-foreground">oauth_token</code> value.</li>
              </ol>
            </div>

            <!-- Step 2: Paste token -->
            <div class="flex flex-col gap-2">
              <Label
                for="google-oauth-token"
                class="flex items-center gap-2 text-xs font-medium"
              >
                <span class="flex size-4 shrink-0 items-center justify-center rounded-full bg-muted text-[10px]">2</span>
                Paste oauth_token
              </Label>
              <Input
                id="google-oauth-token"
                v-model="oauthToken"
                type="password"
                autocomplete="off"
                spellcheck="false"
                placeholder="Paste the cookie value (oauth2_4/...)"
                :disabled="isConnecting"
                @keydown.enter="connectAccount"
              />
              <Button
                type="button"
                size="sm"
                class="cursor-pointer select-none text-xs"
                :disabled="!oauthToken.trim() || isConnecting"
                @click="connectAccount"
              >
                {{ isConnecting ? 'Connecting...' : 'Connect account' }}
              </Button>
            </div>

            <!-- Raw Credential option -->
            <div class="border-t pt-2">
              <button
                type="button"
                class="flex w-full items-center gap-1 text-[11px] text-muted-foreground select-none hover:text-foreground"
                @click="showRawCredential = !showRawCredential"
              >
                <ChevronDown
                  class="size-3 transition-transform"
                  :class="showRawCredential && 'rotate-180'"
                />
                Paste raw credential string (androidId=...)
              </button>
              <div
                v-if="showRawCredential"
                class="mt-2 flex flex-col gap-2"
              >
                <Input
                  id="google-raw-credential"
                  v-model="rawCredential"
                  type="password"
                  autocomplete="off"
                  spellcheck="false"
                  placeholder="androidId=...&Email=..."
                  :disabled="isAddingRawCredential"
                  @keydown.enter="addRawCredential"
                />
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  class="cursor-pointer select-none text-xs"
                  :disabled="!rawCredential.trim() || isAddingRawCredential"
                  @click="addRawCredential"
                >
                  {{ isAddingRawCredential ? 'Adding...' : 'Add credential' }}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>
