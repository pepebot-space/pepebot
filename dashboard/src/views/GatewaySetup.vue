<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  getGateways, addGateway, updateGateway, removeGateway,
  setActiveGateway, getActiveGatewayId
} from '../lib/gateway.js'
import { Server, Plus, Trash2, Edit3, Check, X, Plug, ExternalLink, ArrowRight } from 'lucide-vue-next'

const router = useRouter()
const gateways = ref([])
const showAddForm = ref(false)
const newUrl = ref('')
const newName = ref('')
const editingId = ref(null)
const editName = ref('')
const editUrl = ref('')
const testingId = ref(null)
const testResult = ref({}) // { [id]: 'ok' | 'fail' }

function refresh() {
  gateways.value = getGateways()
}

onMounted(() => {
  refresh()
  // If no gateways, show add form
  if (gateways.value.length === 0) {
    showAddForm.value = true
  }
})

function handleAdd() {
  if (!newUrl.value.trim()) return
  const gw = addGateway({ name: newName.value, url: newUrl.value })
  newUrl.value = ''
  newName.value = ''
  showAddForm.value = false
  refresh()
  // Auto-select if first gateway
  if (gateways.value.length === 1) {
    connectGateway(gw.id)
  }
}

function startEdit(gw) {
  editingId.value = gw.id
  editName.value = gw.name
  editUrl.value = gw.url
}

function saveEdit(id) {
  updateGateway(id, { name: editName.value.trim(), url: editUrl.value.trim() })
  editingId.value = null
  refresh()
}

function cancelEdit() {
  editingId.value = null
}

function handleRemove(id) {
  removeGateway(id)
  refresh()
}

async function testConnection(gw) {
  testingId.value = gw.id
  testResult.value[gw.id] = null
  try {
    const base = gw.url.replace(/\/+$/, '')
    const url = base.endsWith('/v1') ? `${base}/agents` : `${base}/v1/agents`
    const res = await fetch(url, { signal: AbortSignal.timeout(5000) })
    testResult.value[gw.id] = res.ok ? 'ok' : 'fail'
  } catch {
    testResult.value[gw.id] = 'fail'
  } finally {
    testingId.value = null
    setTimeout(() => { testResult.value[gw.id] = null }, 4000)
  }
}

function connectGateway(id) {
  setActiveGateway(id)
  router.push('/')
}
</script>

<template>
  <div class="min-h-screen bg-[#0a0a0e] text-white flex flex-col items-center justify-center p-6">

    <!-- Background blobs -->
    <div class="fixed inset-0 overflow-hidden pointer-events-none">
      <div class="absolute top-1/4 left-1/4 w-96 h-96 bg-green-500/5 rounded-full blur-3xl animate-blob" />
      <div class="absolute bottom-1/4 right-1/4 w-80 h-80 bg-emerald-500/5 rounded-full blur-3xl animate-blob animation-delay-2000" />
    </div>

    <div class="relative z-10 w-full max-w-2xl">
      <!-- Logo & Title -->
      <div class="text-center mb-10">
        <div class="w-20 h-20 mx-auto mb-4 rounded-2xl bg-gradient-to-br from-green-400 to-emerald-600 flex items-center justify-center text-4xl shadow-2xl shadow-green-500/20">
          🐸
        </div>
        <h1 class="text-3xl font-bold tracking-tight mb-2">Pepebot Dashboard</h1>
        <p class="text-gray-500 text-sm">Connect to a gateway server to get started</p>
      </div>

      <!-- Gateway List -->
      <div class="space-y-3 mb-6">
        <div
          v-for="gw in gateways"
          :key="gw.id"
          class="bg-[#14141a] border border-white/[0.06] rounded-2xl overflow-hidden transition-all hover:border-white/10"
        >
          <!-- Normal view -->
          <div v-if="editingId !== gw.id" class="p-4 flex items-center gap-4">
            <div class="w-11 h-11 rounded-xl bg-gradient-to-br from-green-500/15 to-emerald-500/15 flex items-center justify-center flex-shrink-0">
              <Server :size="20" class="text-green-400" />
            </div>
            <div class="flex-1 min-w-0">
              <h3 class="text-sm font-semibold text-gray-200 truncate">{{ gw.name }}</h3>
              <p class="text-xs text-gray-600 truncate font-mono">{{ gw.url }}</p>
            </div>

            <!-- Test result indicator -->
            <div v-if="testResult[gw.id] === 'ok'" class="text-xs text-green-400 flex items-center gap-1">
              <div class="w-2 h-2 rounded-full bg-green-400" /> Online
            </div>
            <div v-else-if="testResult[gw.id] === 'fail'" class="text-xs text-red-400 flex items-center gap-1">
              <div class="w-2 h-2 rounded-full bg-red-400" /> Offline
            </div>

            <!-- Actions -->
            <div class="flex items-center gap-1">
              <button
                @click="testConnection(gw)"
                :disabled="testingId === gw.id"
                class="p-2 rounded-lg text-gray-500 hover:text-gray-300 hover:bg-white/5 transition-all"
                title="Test connection"
              >
                <Plug :size="15" :class="testingId === gw.id ? 'animate-pulse' : ''" />
              </button>
              <button @click="startEdit(gw)" class="p-2 rounded-lg text-gray-500 hover:text-gray-300 hover:bg-white/5 transition-all" title="Edit">
                <Edit3 :size="15" />
              </button>
              <button @click="handleRemove(gw.id)" class="p-2 rounded-lg text-gray-500 hover:text-red-400 hover:bg-red-500/5 transition-all" title="Remove">
                <Trash2 :size="15" />
              </button>
              <button
                @click="connectGateway(gw.id)"
                class="ml-1 px-4 py-2 rounded-xl bg-gradient-to-r from-green-500 to-emerald-600 text-white text-sm font-medium hover:shadow-lg hover:shadow-green-500/20 transition-all hover:scale-[1.02] active:scale-95"
              >
                Connect
              </button>
            </div>
          </div>

          <!-- Edit view -->
          <div v-else class="p-4 space-y-3">
            <div class="flex gap-3">
              <input
                v-model="editName"
                placeholder="Gateway name"
                class="flex-1 bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm text-white placeholder-gray-600 outline-none focus:border-green-500/30"
              />
              <input
                v-model="editUrl"
                placeholder="http://host:port"
                class="flex-[2] bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm text-white placeholder-gray-600 font-mono outline-none focus:border-green-500/30"
              />
            </div>
            <div class="flex justify-end gap-2">
              <button @click="cancelEdit" class="px-3 py-1.5 rounded-lg text-sm text-gray-400 hover:text-white hover:bg-white/5 transition-all">
                Cancel
              </button>
              <button @click="saveEdit(gw.id)" class="px-4 py-1.5 rounded-lg text-sm bg-green-500/20 text-green-400 hover:bg-green-500/30 transition-all font-medium">
                Save
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Add Form -->
      <Transition name="fade">
        <div v-if="showAddForm" class="bg-[#14141a] border border-white/[0.06] rounded-2xl p-5 mb-4">
          <h3 class="text-sm font-medium text-gray-300 mb-3">Add Gateway Server</h3>
          <div class="flex gap-3 mb-3">
            <input
              v-model="newName"
              placeholder="Name (optional)"
              class="flex-1 bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm text-white placeholder-gray-600 outline-none focus:border-green-500/30 transition-colors"
            />
            <input
              v-model="newUrl"
              placeholder="http://localhost:18790"
              class="flex-[2] bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm text-white placeholder-gray-600 font-mono outline-none focus:border-green-500/30 transition-colors"
              @keydown.enter="handleAdd"
            />
          </div>
          <div class="flex justify-between items-center">
            <p class="text-[11px] text-gray-600">
              Enter the URL of your Pepebot gateway server
            </p>
            <div class="flex gap-2">
              <button @click="showAddForm = false" class="px-3 py-2 rounded-xl text-sm text-gray-400 hover:text-white hover:bg-white/5 transition-all">
                Cancel
              </button>
              <button
                @click="handleAdd"
                :disabled="!newUrl.trim()"
                class="px-5 py-2 rounded-xl text-sm font-medium transition-all"
                :class="newUrl.trim()
                  ? 'bg-gradient-to-r from-green-500 to-emerald-600 text-white hover:shadow-lg hover:shadow-green-500/20'
                  : 'bg-white/5 text-gray-600 cursor-not-allowed'"
              >
                Add Server
              </button>
            </div>
          </div>
        </div>
      </Transition>

      <!-- Add Button -->
      <button
        v-if="!showAddForm"
        @click="showAddForm = true"
        class="w-full py-4 rounded-2xl border-2 border-dashed border-white/[0.06] text-gray-500 hover:text-gray-300 hover:border-white/10 hover:bg-white/[0.02] transition-all flex items-center justify-center gap-2 text-sm"
      >
        <Plus :size="18" />
        Add Gateway Server
      </button>

      <!-- Footer -->
      <div class="text-center mt-8">
        <p class="text-[11px] text-gray-700">
          Pepebot Dashboard v1.0 · Gateway configs stored locally in your browser
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
@keyframes blob {
  0%, 100% { transform: translate(0, 0) scale(1); }
  33% { transform: translate(30px, -20px) scale(1.1); }
  66% { transform: translate(-20px, 20px) scale(0.9); }
}
.animate-blob { animation: blob 8s ease-in-out infinite; }
.animation-delay-2000 { animation-delay: 2s; }

.fade-enter-active { transition: all 0.2s ease-out; }
.fade-leave-active { transition: all 0.15s ease-in; }
.fade-enter-from { opacity: 0; transform: translateY(-8px); }
.fade-leave-to { opacity: 0; transform: translateY(-8px); }
</style>
