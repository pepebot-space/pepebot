<script setup>
import { ref, computed, watch } from 'vue'
import {
  X, Clock, User, Cpu, Tag, ShieldCheck, CheckCircle, XCircle,
  ArrowRight, Trash2, RotateCcw, AlertTriangle, FileText
} from 'lucide-vue-next'

const props = defineProps({
  task: { type: Object, default: null },
  agents: { type: Object, default: () => ({}) }
})

const emit = defineEmits(['close', 'move', 'approve', 'reject', 'delete', 'update'])

const approvalNote = ref('')
const rejectNote = ref('')
const showApproval = ref(false)
const showReject = ref(false)

watch(() => props.task, () => {
  approvalNote.value = ''
  rejectNote.value = ''
  showApproval.value = false
  showReject.value = false
})

const statusConfig = {
  backlog: { color: 'text-gray-400', bg: 'bg-gray-500/15', label: 'Backlog' },
  todo: { color: 'text-blue-400', bg: 'bg-blue-500/15', label: 'Todo' },
  in_progress: { color: 'text-yellow-400', bg: 'bg-yellow-500/15', label: 'In Progress' },
  review: { color: 'text-purple-400', bg: 'bg-purple-500/15', label: 'Review' },
  approved: { color: 'text-green-400', bg: 'bg-green-500/15', label: 'Approved' },
  rejected: { color: 'text-red-400', bg: 'bg-red-500/15', label: 'Rejected' },
  done: { color: 'text-emerald-400', bg: 'bg-emerald-500/15', label: 'Done' },
  failed: { color: 'text-red-400', bg: 'bg-red-500/15', label: 'Failed' },
}

const priorityConfig = {
  critical: { color: 'text-red-400', label: 'Critical' },
  high: { color: 'text-orange-400', label: 'High' },
  medium: { color: 'text-blue-400', label: 'Medium' },
  low: { color: 'text-gray-400', label: 'Low' },
}

const st = computed(() => statusConfig[props.task?.status] || statusConfig.backlog)
const pr = computed(() => priorityConfig[props.task?.priority] || priorityConfig.medium)

const canApprove = computed(() => props.task?.status === 'review' && props.task?.approval)
const canReject = computed(() => props.task?.status === 'review')

function formatTime(dateStr) {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return d.toLocaleString('id-ID', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function doApprove() {
  emit('approve', props.task.id, approvalNote.value)
  showApproval.value = false
}

function doReject() {
  emit('reject', props.task.id, rejectNote.value)
  showReject.value = false
}
</script>

<template>
  <!-- Overlay -->
  <Teleport to="body">
    <div v-if="task" class="fixed inset-0 z-50 flex justify-end" @click.self="$emit('close')">
      <div class="absolute inset-0 bg-black/50" @click="$emit('close')" />

      <!-- Panel -->
      <div class="relative w-full max-w-lg bg-[#141418] border-l border-white/5 overflow-y-auto shadow-2xl">
        <!-- Header -->
        <div class="sticky top-0 bg-[#141418]/95 backdrop-blur border-b border-white/5 px-6 py-4 flex items-center justify-between z-10">
          <div class="flex items-center gap-2">
            <span :class="[st.bg, st.color]" class="text-xs px-2.5 py-1 rounded-full font-medium">
              {{ st.label }}
            </span>
            <span :class="pr.color" class="text-xs">{{ pr.label }}</span>
          </div>
          <button @click="$emit('close')" class="p-2 rounded-lg hover:bg-white/10 text-gray-400">
            <X :size="18" />
          </button>
        </div>

        <div class="px-6 py-5 space-y-6">
          <!-- Title -->
          <h2 class="text-xl font-semibold text-white leading-tight">{{ task.title }}</h2>

          <!-- Description -->
          <div v-if="task.description" class="text-sm text-gray-400 leading-relaxed">
            {{ task.description }}
          </div>

          <!-- Meta grid -->
          <div class="grid grid-cols-2 gap-4 text-sm">
            <div class="space-y-1">
              <div class="text-gray-600 text-xs flex items-center gap-1"><Cpu :size="12" /> Agent</div>
              <div class="text-gray-300">{{ task.assigned_agent || 'Unassigned' }}</div>
            </div>
            <div class="space-y-1">
              <div class="text-gray-600 text-xs flex items-center gap-1"><User :size="12" /> Created by</div>
              <div class="text-gray-300">{{ task.created_by || '-' }}</div>
            </div>
            <div class="space-y-1">
              <div class="text-gray-600 text-xs flex items-center gap-1"><Clock :size="12" /> Created</div>
              <div class="text-gray-300">{{ formatTime(task.created_at) }}</div>
            </div>
            <div class="space-y-1">
              <div class="text-gray-600 text-xs flex items-center gap-1"><Clock :size="12" /> Updated</div>
              <div class="text-gray-300">{{ formatTime(task.updated_at) }}</div>
            </div>
            <div v-if="task.started_at" class="space-y-1">
              <div class="text-gray-600 text-xs">Started</div>
              <div class="text-gray-300">{{ formatTime(task.started_at) }}</div>
            </div>
            <div v-if="task.completed_at" class="space-y-1">
              <div class="text-gray-600 text-xs">Completed</div>
              <div class="text-gray-300">{{ formatTime(task.completed_at) }}</div>
            </div>
          </div>

          <!-- Labels -->
          <div v-if="task.labels?.length">
            <div class="text-gray-600 text-xs mb-2 flex items-center gap-1"><Tag :size="12" /> Labels</div>
            <div class="flex gap-1.5 flex-wrap">
              <span v-for="label in task.labels" :key="label"
                class="text-xs px-2 py-0.5 rounded-md bg-white/5 text-gray-400">
                {{ label }}
              </span>
            </div>
          </div>

          <!-- Approval info -->
          <div v-if="task.approval" class="bg-amber-500/5 border border-amber-500/20 rounded-xl p-4 space-y-2">
            <div class="flex items-center gap-2 text-amber-400 text-sm font-medium">
              <ShieldCheck :size="16" /> Requires Approval
            </div>
            <div v-if="task.approved_by" class="text-xs text-gray-400">
              <span v-if="task.status === 'done'">Approved by {{ task.approved_by }}</span>
              <span v-else-if="task.status === 'rejected'">Rejected by {{ task.approved_by }}</span>
            </div>
            <div v-if="task.approval_note" class="text-xs text-gray-500 italic">
              "{{ task.approval_note }}"
            </div>
          </div>

          <!-- Approval actions -->
          <div v-if="canApprove || canReject" class="space-y-3">
            <div class="text-gray-600 text-xs font-medium uppercase tracking-wider">Approval Actions</div>
            <div class="flex gap-2">
              <button
                v-if="canApprove"
                @click="showApproval = !showApproval; showReject = false"
                class="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-green-500/10 text-green-400 hover:bg-green-500/20 transition-colors text-sm font-medium"
              >
                <CheckCircle :size="16" /> Approve
              </button>
              <button
                v-if="canReject"
                @click="showReject = !showReject; showApproval = false"
                class="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors text-sm font-medium"
              >
                <XCircle :size="16" /> Reject
              </button>
            </div>

            <!-- Approve form -->
            <div v-if="showApproval" class="bg-green-500/5 border border-green-500/20 rounded-xl p-3 space-y-2">
              <textarea
                v-model="approvalNote"
                placeholder="Approval note (optional)..."
                class="w-full bg-transparent border border-white/10 rounded-lg px-3 py-2 text-sm text-gray-300 placeholder-gray-600 resize-none focus:outline-none focus:border-green-500/30"
                rows="2"
              />
              <button @click="doApprove"
                class="w-full px-3 py-2 rounded-lg bg-green-500/20 text-green-400 text-sm font-medium hover:bg-green-500/30 transition-colors">
                Confirm Approve
              </button>
            </div>

            <!-- Reject form -->
            <div v-if="showReject" class="bg-red-500/5 border border-red-500/20 rounded-xl p-3 space-y-2">
              <textarea
                v-model="rejectNote"
                placeholder="Reason for rejection..."
                class="w-full bg-transparent border border-white/10 rounded-lg px-3 py-2 text-sm text-gray-300 placeholder-gray-600 resize-none focus:outline-none focus:border-red-500/30"
                rows="2"
              />
              <button @click="doReject"
                class="w-full px-3 py-2 rounded-lg bg-red-500/20 text-red-400 text-sm font-medium hover:bg-red-500/30 transition-colors">
                Confirm Reject
              </button>
            </div>
          </div>

          <!-- Result -->
          <div v-if="task.result">
            <div class="text-gray-600 text-xs mb-2 flex items-center gap-1"><FileText :size="12" /> Result</div>
            <div class="bg-[#1a1a20] border border-white/5 rounded-xl p-3 text-sm text-gray-300 whitespace-pre-wrap break-words max-h-48 overflow-y-auto">
              {{ task.result }}
            </div>
          </div>

          <!-- Error -->
          <div v-if="task.error">
            <div class="text-red-500 text-xs mb-2 flex items-center gap-1"><AlertTriangle :size="12" /> Error</div>
            <div class="bg-red-500/5 border border-red-500/20 rounded-xl p-3 text-sm text-red-300 whitespace-pre-wrap break-words max-h-48 overflow-y-auto">
              {{ task.error }}
            </div>
          </div>

          <!-- Actions -->
          <div class="border-t border-white/5 pt-4 flex gap-2">
            <button
              v-if="task.status === 'failed' || task.status === 'rejected' || task.status === 'done'"
              @click="$emit('move', task.id, 'todo')"
              class="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-blue-500/10 text-blue-400 hover:bg-blue-500/20 transition-colors text-xs font-medium"
            >
              <RotateCcw :size="14" /> Reopen
            </button>
            <div class="flex-1" />
            <button
              @click="$emit('delete', task.id)"
              class="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors text-xs font-medium"
            >
              <Trash2 :size="14" /> Delete
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
