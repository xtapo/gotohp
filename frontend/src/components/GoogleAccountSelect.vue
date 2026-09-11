<script setup lang="ts">
import { ref, watch } from 'vue'
import { Check, Plus, X } from '@lucide/vue'
import {
  SelectItem as SelectItemPrimitive,
  SelectItemIndicator,
  SelectItemText,
} from 'reka-ui'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface Props {
  modelValue?: string
  options?: string[]
  removingAccount?: string
}

interface Emits {
  (event: 'update:modelValue', value: string): void
  (event: 'item-removed', value: string): void
  (event: 'add'): void
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  options: () => [],
  removingAccount: '',
})

const emit = defineEmits<Emits>()
const isOpen = ref(false)
const selectedValue = ref(props.modelValue)

watch(() => props.modelValue, (newValue) => {
  if (newValue !== selectedValue.value) {
    selectedValue.value = newValue || ''
  }
})

watch(selectedValue, (newValue) => {
  emit('update:modelValue', newValue)
})

</script>

<template>
  <Select
    v-model="selectedValue"
    v-model:open="isOpen"
  >
    <SelectTrigger
      class="h-8.5 px-3 rounded-full bg-zinc-900/90 hover:bg-zinc-800/90 border border-white/10 hover:border-white/20 shadow-sm backdrop-blur-md max-w-[240px] text-xs font-medium text-zinc-200 transition-all select-none gap-2 cursor-pointer focus-visible:ring-emerald-500/40"
      style="--wails-draggable: none"
    >
      <div class="flex items-center gap-2 min-w-0 flex-1">
        <!-- Google Photos mini 4-color pinwheel indicator -->
        <span class="flex size-4 shrink-0 items-center justify-center rounded-full bg-gradient-to-tr from-blue-500 via-emerald-500 to-amber-500 p-[1px]">
          <span class="size-full rounded-full bg-zinc-900 flex items-center justify-center">
            <span class="size-1.5 rounded-full bg-emerald-400" />
          </span>
        </span>
        <SelectValue
          placeholder="Chọn tài khoản"
          class="truncate text-xs text-zinc-200"
        />
      </div>
    </SelectTrigger>
    <SelectContent
      align="start"
      class="min-w-[220px] max-w-[280px] bg-zinc-900/95 backdrop-blur-xl border border-white/10 shadow-2xl rounded-xl p-1 text-xs"
      style="--wails-draggable: none"
    >
      <SelectGroup>
        <div
          v-for="option in options"
          :key="option"
          class="relative flex items-center group/item"
        >
          <SelectItemPrimitive
            :value="option"
            class="focus:bg-white/10 focus:text-white relative flex min-w-0 w-full cursor-pointer items-center rounded-lg py-2 pr-8 pl-7 text-xs text-zinc-300 outline-none select-none transition-colors"
          >
            <span class="absolute left-2 flex size-3.5 items-center justify-center text-emerald-400">
              <SelectItemIndicator>
                <Check class="size-3.5" />
              </SelectItemIndicator>
            </span>
            <SelectItemText class="min-w-0 flex-1">
              <span class="block truncate font-medium">{{ option }}</span>
            </SelectItemText>
          </SelectItemPrimitive>
          <button
            type="button"
            class="absolute right-1.5 top-1/2 z-10 flex size-5 -translate-y-1/2 items-center justify-center rounded-md text-zinc-500 hover:bg-red-500/20 hover:text-red-400 transition-colors disabled:pointer-events-none disabled:opacity-50"
            :aria-label="`Xóa ${option}`"
            :title="`Xóa ${option}`"
            :disabled="removingAccount === option"
            @pointerdown.stop.prevent
            @click.stop.prevent="emit('item-removed', option)"
          >
            <X class="size-3" />
          </button>
        </div>
      </SelectGroup>
      <SelectSeparator
        v-if="options.length"
        class="bg-white/10 my-1"
      />
      <button
        type="button"
        class="relative flex w-full cursor-pointer items-center gap-2 rounded-lg py-2 px-2.5 text-xs text-zinc-400 hover:text-white hover:bg-white/10 outline-none select-none transition-colors"
        @pointerdown.stop.prevent
        @click.stop.prevent="isOpen = false; emit('add')"
      >
        <Plus class="size-3.5 text-emerald-400" />
        <span>Thêm tài khoản Google...</span>
      </button>
    </SelectContent>
  </Select>
</template>
