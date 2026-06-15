<script setup>
import { computed } from 'vue'
import { GripVertical, MoreHorizontal, Clock, ShieldCheck } from 'lucide-vue-next'

const props = defineProps({
  task: { type: Object, required: true }
})

const emit = defineEmits(['open', 'dragstart'])

const priorityConfig = {
  critical: { bg: 'bg-red-500/15', text: 'text-red-400', label: 'critical' },
  high: { bg: 'bg-orange-500/15', text: 'text-orange-400', label: 'high' },
  medium: { bg: 'bg-blue-500/15', text: 'text-blue-400', label: 'medium' },
  low: { bg: 'bg-gray-500/15', text: 'text-gray-400', label: 'low' },
}

const prio = computed(() => priorityConfig[props.task.priority] || priorityConfig.medium)

function timeAgo(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const now = new Date()
  const diff = now - d
  if (diff < 60000) return 'now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h`
  return `${Math.floor(diff / 86400000)}d`
}

function onDragStart(e) {
  e.dataTransfer.effectAllowed = 'move'
  e.dataTransfer.setData('text/plain', props.task.id)
  emit('dragstart', props.task)
}
</script>

<template>
  <div
    draggable="true"
    @dragstart="onDragStart"
    @click="$emit('open', task)"
    class="bg-[#1a1a20] border border-white/5 rounded-xl p-3.5 hover:border-white/15 cursor-grab active:cursor-grabbing transition-all group relative"
  >
    <!-- Top: priority + approval badge + menu -->
    <div class="flex items-center gap-1.5 mb-2">
      <span :class="[prio.bg, prio.text]" class="text-[10px] px-2 py-0.5 rounded-full font-medium uppercase tracking-wider">
        {{ prio.label }}
      </span>
      <ShieldCheck v-if="task.approval" :size="12" class="text-amber-400" title="Requires approval" />
      <div class="flex-1" />
      <button
        @click.stop="$emit('open', task)"
        class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-white/10 transition-all"
      >
        <MoreHorizontal :size="14" class="text-gray-500" />
      </button>
    </div>

    <!-- Title -->
    <h4 class="text-sm font-medium text-gray-200 mb-2 line-clamp-2 leading-snug">{{ task.title }}</h4>

    <!-- Bottom: agent + time -->
    <div class="flex items-center justify-between">
      <span
        v-if="task.assigned_agent"
        class="text-[11px] px-2 py-0.5 rounded-md bg-purple-500/15 text-purple-300 truncate max-w-[120px]"
      >
        {{ task.assigned_agent }}
      </span>
      <span v-else class="text-[11px] text-gray-600 italic">unassigned</span>

      <span class="text-[11px] text-gray-600 flex items-center gap-1">
        <Clock :size="10" />
        {{ timeAgo(task.updated_at) }}
      </span>
    </div>

    <!-- Labels -->
    <div v-if="task.labels?.length" class="flex gap-1 mt-2 flex-wrap">
      <span
        v-for="label in task.labels.slice(0, 3)"
        :key="label"
        class="text-[10px] px-1.5 py-0.5 rounded bg-white/5 text-gray-500"
      >
        {{ label }}
      </span>
      <span v-if="task.labels.length > 3" class="text-[10px] text-gray-600">
        +{{ task.labels.length - 3 }}
      </span>
    </div>

    <!-- Drag handle hint -->
    <div class="absolute top-1/2 -left-0.5 -translate-y-1/2 opacity-0 group-hover:opacity-30 transition-opacity">
      <GripVertical :size="12" class="text-gray-500" />
    </div>
  </div>
</template>
