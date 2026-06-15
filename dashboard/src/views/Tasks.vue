<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import axios from 'axios'
import {
  Plus, Search, Filter, RefreshCw, KanbanSquare,
  ChevronDown, X
} from 'lucide-vue-next'
import { getGatewayApiUrl } from '../lib/gateway.js'
import TaskCard from '../components/TaskCard.vue'
import TaskDetail from '../components/TaskDetail.vue'

const GATEWAY_API = getGatewayApiUrl()

// State
const tasks = ref([])
const agents = ref({})
const stats = ref({})
const isLoading = ref(true)
const selectedTask = ref(null)
const draggedTaskId = ref(null)
const dragOverColumn = ref(null)
const searchQuery = ref('')
const filterAgent = ref('')
const filterPriority = ref('')
const showCreateModal = ref(false)
const pollTimer = ref(null)

// Create form
const newTask = ref({
  title: '', description: '', priority: 'medium', status: 'backlog',
  assigned_agent: '', labels: '', approval: false
})

// Kanban columns
const columns = [
  { key: 'backlog', label: 'Backlog', dot: 'bg-gray-500' },
  { key: 'todo', label: 'Todo', dot: 'bg-blue-500' },
  { key: 'in_progress', label: 'In Progress', dot: 'bg-yellow-500' },
  { key: 'review', label: 'Review', dot: 'bg-purple-500' },
  { key: 'done', label: 'Done', dot: 'bg-emerald-500' },
  { key: 'failed', label: 'Failed', dot: 'bg-red-500' },
]

// Computed: tasks grouped by column
const columnTasks = computed(() => {
  const groups = {}
  columns.forEach(c => { groups[c.key] = [] })

  tasks.value.forEach(t => {
    // Search filter
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase()
      if (!t.title.toLowerCase().includes(q) && !(t.description || '').toLowerCase().includes(q)) return
    }
    // Agent filter
    if (filterAgent.value && t.assigned_agent !== filterAgent.value) return
    // Priority filter
    if (filterPriority.value && t.priority !== filterPriority.value) return

    const col = groups[t.status] !== undefined ? t.status : 'backlog'
    if (groups[col]) groups[col].push(t)
  })

  return groups
})

const agentList = computed(() => {
  const set = new Set()
  tasks.value.forEach(t => { if (t.assigned_agent) set.add(t.assigned_agent) })
  return [...set].sort()
})

// Fetch
async function fetchTasks() {
  try {
    const [tasksRes, agentsRes, statsRes] = await Promise.all([
      axios.get(`${GATEWAY_API}/v1/tasks?limit=200`),
      axios.get(`${GATEWAY_API}/v1/agents`).catch(() => ({ data: { agents: {} } })),
      axios.get(`${GATEWAY_API}/v1/tasks/stats`).catch(() => ({ data: {} })),
    ])
    tasks.value = tasksRes.data.tasks || []
    agents.value = agentsRes.data.agents || {}
    stats.value = statsRes.data || {}
  } catch (e) {
    console.error('Failed to fetch tasks:', e)
  } finally {
    isLoading.value = false
  }
}

// Actions
async function createTask() {
  if (!newTask.value.title.trim()) return

  const payload = {
    title: newTask.value.title,
    description: newTask.value.description,
    priority: newTask.value.priority,
    status: newTask.value.status,
    approval: newTask.value.approval,
  }
  if (newTask.value.assigned_agent) payload.assigned_agent = newTask.value.assigned_agent
  if (newTask.value.labels.trim()) {
    payload.labels = newTask.value.labels.split(',').map(l => l.trim()).filter(Boolean)
  }

  try {
    await axios.post(`${GATEWAY_API}/v1/tasks`, payload)
    showCreateModal.value = false
    newTask.value = { title: '', description: '', priority: 'medium', status: 'backlog', assigned_agent: '', labels: '', approval: false }
    await fetchTasks()
  } catch (e) {
    console.error('Failed to create task:', e)
  }
}

async function moveTask(taskId, newStatus) {
  try {
    await axios.post(`${GATEWAY_API}/v1/tasks/${taskId}/move`, { status: newStatus })
    await fetchTasks()
  } catch (e) {
    console.error('Move failed:', e)
  }
}

async function approveTask(taskId, note) {
  try {
    await axios.post(`${GATEWAY_API}/v1/tasks/${taskId}/approve`, { approved_by: 'dashboard', note })
    selectedTask.value = null
    await fetchTasks()
  } catch (e) {
    console.error('Approve failed:', e)
  }
}

async function rejectTask(taskId, note) {
  try {
    await axios.post(`${GATEWAY_API}/v1/tasks/${taskId}/reject`, { rejected_by: 'dashboard', note })
    selectedTask.value = null
    await fetchTasks()
  } catch (e) {
    console.error('Reject failed:', e)
  }
}

async function deleteTask(taskId) {
  if (!confirm('Delete this task?')) return
  try {
    await axios.delete(`${GATEWAY_API}/v1/tasks/${taskId}`)
    selectedTask.value = null
    await fetchTasks()
  } catch (e) {
    console.error('Delete failed:', e)
  }
}

// Drag & Drop
function onDragStart(task) {
  draggedTaskId.value = task.id
}

function onDragOver(e, colKey) {
  e.preventDefault()
  dragOverColumn.value = colKey
}

function onDragLeave(colKey) {
  if (dragOverColumn.value === colKey) dragOverColumn.value = null
}

async function onDrop(colKey) {
  dragOverColumn.value = null
  if (!draggedTaskId.value) return

  const task = tasks.value.find(t => t.id === draggedTaskId.value)
  if (task && task.status !== colKey) {
    await moveTask(draggedTaskId.value, colKey)
  }
  draggedTaskId.value = null
}

function openTask(task) {
  selectedTask.value = task
}

// WebSocket + fallback polling
const ws = ref(null)
const wsConnected = ref(false)

function connectWebSocket() {
  try {
    const wsUrl = GATEWAY_API.replace(/^http/, 'ws') + '/v1/tasks/stream'
    const socket = new WebSocket(wsUrl)

    socket.onopen = () => {
      wsConnected.value = true
      // Stop polling when WS is connected
      if (pollTimer.value) {
        clearInterval(pollTimer.value)
        pollTimer.value = null
      }
    }

    socket.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.task) {
          // Update or add task in local state
          const idx = tasks.value.findIndex(t => t.id === data.task.id)
          if (idx >= 0) {
            tasks.value[idx] = data.task
          } else {
            tasks.value.push(data.task)
          }
          // Remove deleted tasks
          if (data.type === 'task.deleted') {
            tasks.value = tasks.value.filter(t => t.id !== data.task.id)
          }
          // Update selected task if open
          if (selectedTask.value?.id === data.task.id) {
            selectedTask.value = data.task
          }
        }
      } catch {}
    }

    socket.onclose = () => {
      wsConnected.value = false
      ws.value = null
      // Fall back to polling
      if (!pollTimer.value) {
        pollTimer.value = setInterval(fetchTasks, 5000)
      }
      // Retry WS after 5s
      setTimeout(connectWebSocket, 5000)
    }

    socket.onerror = () => {
      socket.close()
    }

    ws.value = socket
  } catch {
    // WS not available, polling continues
  }
}

onMounted(() => {
  fetchTasks()
  // Start polling immediately, WS will replace it if available
  pollTimer.value = setInterval(fetchTasks, 5000)
  connectWebSocket()
})

onUnmounted(() => {
  if (pollTimer.value) clearInterval(pollTimer.value)
  if (ws.value) ws.value.close()
})
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden">
    <!-- Header -->
    <div class="px-6 py-5 border-b border-white/5 flex-shrink-0">
      <div class="flex items-center justify-between mb-4">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-cyan-500 to-blue-600 flex items-center justify-center">
            <KanbanSquare :size="20" class="text-white" />
          </div>
          <div>
            <h1 class="text-xl font-bold text-white">Tasks</h1>
            <p class="text-xs text-gray-500">{{ stats.total || 0 }} total tasks</p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button @click="fetchTasks" class="p-2 rounded-lg hover:bg-white/10 text-gray-400 transition-colors" title="Refresh">
            <RefreshCw :size="16" :class="{ 'animate-spin': isLoading }" />
          </button>
          <button
            @click="showCreateModal = true"
            class="flex items-center gap-2 px-4 py-2 rounded-xl bg-gradient-to-r from-cyan-500 to-blue-600 text-white text-sm font-medium hover:opacity-90 transition-opacity"
          >
            <Plus :size="16" /> New Task
          </button>
        </div>
      </div>

      <!-- Filters -->
      <div class="flex items-center gap-3">
        <div class="relative flex-1 max-w-xs">
          <Search :size="14" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
          <input
            v-model="searchQuery"
            placeholder="Search tasks..."
            class="w-full bg-[#1a1a20] border border-white/5 rounded-lg pl-9 pr-3 py-2 text-sm text-gray-300 placeholder-gray-600 focus:outline-none focus:border-white/15"
          />
        </div>
        <select
          v-model="filterAgent"
          class="bg-[#1a1a20] border border-white/5 rounded-lg px-3 py-2 text-sm text-gray-300 focus:outline-none focus:border-white/15 appearance-none cursor-pointer"
        >
          <option value="">All Agents</option>
          <option v-for="a in agentList" :key="a" :value="a">{{ a }}</option>
        </select>
        <select
          v-model="filterPriority"
          class="bg-[#1a1a20] border border-white/5 rounded-lg px-3 py-2 text-sm text-gray-300 focus:outline-none focus:border-white/15 appearance-none cursor-pointer"
        >
          <option value="">All Priorities</option>
          <option value="critical">Critical</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </select>
      </div>
    </div>

    <!-- Kanban Board -->
    <div class="flex-1 overflow-x-auto overflow-y-hidden">
      <div class="flex gap-4 p-6 h-full min-w-max">
        <div
          v-for="col in columns"
          :key="col.key"
          class="flex flex-col w-[280px] flex-shrink-0"
        >
          <!-- Column header -->
          <div class="flex items-center gap-2 mb-3 px-1">
            <div :class="col.dot" class="w-2 h-2 rounded-full" />
            <span class="text-sm font-medium text-gray-400">{{ col.label }}</span>
            <span class="text-[11px] text-gray-600 bg-white/5 px-1.5 py-0.5 rounded">
              {{ columnTasks[col.key]?.length || 0 }}
            </span>
          </div>

          <!-- Drop zone -->
          <div
            @dragover.prevent="onDragOver($event, col.key)"
            @dragleave="onDragLeave(col.key)"
            @drop="onDrop(col.key)"
            class="flex-1 space-y-2.5 p-2 rounded-xl transition-all overflow-y-auto"
            :class="dragOverColumn === col.key
              ? 'bg-white/5 ring-2 ring-dashed ring-white/10'
              : 'bg-transparent'"
          >
            <TaskCard
              v-for="t in columnTasks[col.key]"
              :key="t.id"
              :task="t"
              @open="openTask"
              @dragstart="onDragStart"
            />

            <!-- Empty state -->
            <div
              v-if="!columnTasks[col.key]?.length"
              class="flex items-center justify-center py-12 text-gray-700 text-xs"
            >
              No tasks
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Task Modal -->
    <Teleport to="body">
      <div v-if="showCreateModal" class="fixed inset-0 z-50 flex items-center justify-center" @click.self="showCreateModal = false">
        <div class="absolute inset-0 bg-black/50" @click="showCreateModal = false" />
        <div class="relative bg-[#1a1a20] border border-white/10 rounded-2xl w-full max-w-md p-6 shadow-2xl">
          <div class="flex items-center justify-between mb-5">
            <h3 class="text-lg font-semibold text-white">New Task</h3>
            <button @click="showCreateModal = false" class="p-1 rounded-lg hover:bg-white/10 text-gray-400">
              <X :size="18" />
            </button>
          </div>

          <div class="space-y-4">
            <div>
              <label class="text-xs text-gray-500 mb-1 block">Title *</label>
              <input
                v-model="newTask.title"
                placeholder="Task title..."
                class="w-full bg-[#141418] border border-white/10 rounded-lg px-3 py-2.5 text-sm text-gray-200 placeholder-gray-600 focus:outline-none focus:border-white/20"
                @keyup.enter="createTask"
              />
            </div>

            <div>
              <label class="text-xs text-gray-500 mb-1 block">Description</label>
              <textarea
                v-model="newTask.description"
                placeholder="Details..."
                rows="2"
                class="w-full bg-[#141418] border border-white/10 rounded-lg px-3 py-2.5 text-sm text-gray-200 placeholder-gray-600 resize-none focus:outline-none focus:border-white/20"
              />
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="text-xs text-gray-500 mb-1 block">Status</label>
                <select v-model="newTask.status" class="w-full bg-[#141418] border border-white/10 rounded-lg px-3 py-2.5 text-sm text-gray-200 focus:outline-none">
                  <option value="backlog">Backlog</option>
                  <option value="todo">Todo</option>
                </select>
              </div>
              <div>
                <label class="text-xs text-gray-500 mb-1 block">Priority</label>
                <select v-model="newTask.priority" class="w-full bg-[#141418] border border-white/10 rounded-lg px-3 py-2.5 text-sm text-gray-200 focus:outline-none">
                  <option value="low">Low</option>
                  <option value="medium">Medium</option>
                  <option value="high">High</option>
                  <option value="critical">Critical</option>
                </select>
              </div>
            </div>

            <div>
              <label class="text-xs text-gray-500 mb-1 block">Assign Agent</label>
              <select v-model="newTask.assigned_agent" class="w-full bg-[#141418] border border-white/10 rounded-lg px-3 py-2.5 text-sm text-gray-200 focus:outline-none">
                <option value="">Unassigned</option>
                <option v-for="(def, name) in agents" :key="name" :value="name">{{ name }}</option>
              </select>
            </div>

            <div>
              <label class="text-xs text-gray-500 mb-1 block">Labels (comma-separated)</label>
              <input
                v-model="newTask.labels"
                placeholder="code, review, deploy..."
                class="w-full bg-[#141418] border border-white/10 rounded-lg px-3 py-2.5 text-sm text-gray-200 placeholder-gray-600 focus:outline-none focus:border-white/20"
              />
            </div>

            <label class="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" v-model="newTask.approval" class="rounded border-white/20 bg-[#141418] text-cyan-500 focus:ring-0" />
              <span class="text-sm text-gray-400">Requires approval</span>
            </label>

            <button
              @click="createTask"
              :disabled="!newTask.title.trim()"
              class="w-full py-2.5 rounded-xl bg-gradient-to-r from-cyan-500 to-blue-600 text-white text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-30 disabled:cursor-not-allowed"
            >
              Create Task
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Task Detail -->
    <TaskDetail
      :task="selectedTask"
      :agents="agents"
      @close="selectedTask = null"
      @move="moveTask"
      @approve="approveTask"
      @reject="rejectTask"
      @delete="deleteTask"
    />
  </div>
</template>
