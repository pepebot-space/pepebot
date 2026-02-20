<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import {
  MessageSquare, Cpu, Zap, GitBranch, Activity,
  ArrowRight, TrendingUp, Clock, Bot, Sparkles,
  ChevronRight, Send
} from 'lucide-vue-next'

const router = useRouter()
const GATEWAY_API = 'http://localhost:18790/v1'

// --- Live Data ---
const stats = ref({ agents: 0, skills: 0, workflows: 0, sessions: 0 })
const recentSessions = ref([])
const agents = ref({})
const isLoading = ref(true)

// --- Time-based greeting ---
const greeting = computed(() => {
  const hour = new Date().getHours()
  if (hour < 6) return '🌙 Selamat malam'
  if (hour < 12) return '🌅 Selamat pagi'
  if (hour < 17) return '☀️ Selamat siang'
  if (hour < 21) return '🌆 Selamat sore'
  return '🌙 Selamat malam'
})

const currentTime = ref('')
const updateTime = () => {
  const now = new Date()
  currentTime.value = now.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
}
setInterval(updateTime, 1000)
updateTime()

const currentDate = computed(() => {
  return new Date().toLocaleDateString('id-ID', {
    weekday: 'long', year: 'numeric', month: 'long', day: 'numeric'
  })
})

// --- Quick Actions ---
const quickActions = [
  { label: 'Chat with Agent', icon: MessageSquare, route: '/chat', gradient: 'from-blue-500 to-indigo-600' },
  { label: 'Manage Skills', icon: Zap, route: '/skills', gradient: 'from-amber-500 to-orange-600' },
  { label: 'Run Workflow', icon: GitBranch, route: '/workflows', gradient: 'from-green-500 to-emerald-600' },
  { label: 'Configure Agent', icon: Cpu, route: '/agents', gradient: 'from-purple-500 to-pink-600' },
]

// --- Fetch all data ---
onMounted(async () => {
  try {
    const [agentsRes, skillsRes, workflowsRes, sessionsRes] = await Promise.all([
      axios.get(`${GATEWAY_API}/agents`),
      axios.get(`${GATEWAY_API}/skills`),
      axios.get(`${GATEWAY_API}/workflows`),
      axios.get(`${GATEWAY_API}/sessions`),
    ])

    agents.value = agentsRes.data.agents || {}
    const allSessions = sessionsRes.data.sessions || []
    const allSkills = skillsRes.data.skills || []
    const allWorkflows = workflowsRes.data.workflows || []

    stats.value = {
      agents: Object.keys(agents.value).length,
      skills: allSkills.length,
      workflows: allWorkflows.length,
      sessions: allSessions.length
    }

    // Get recent sessions (latest 5)
    recentSessions.value = allSessions
      .sort((a, b) => new Date(b.updated || 0) - new Date(a.updated || 0))
      .slice(0, 5)
  } catch (e) {
    console.error('Failed to fetch dashboard data:', e)
  } finally {
    isLoading.value = false
  }
})

// --- Animated counters ---
const animatedStats = ref({ agents: 0, skills: 0, workflows: 0, sessions: 0 })

function animateCounter(key, target) {
  const duration = 1200
  const start = Date.now()
  const tick = () => {
    const elapsed = Date.now() - start
    const progress = Math.min(elapsed / duration, 1)
    const eased = 1 - Math.pow(1 - progress, 3) // ease out cubic
    animatedStats.value[key] = Math.round(target * eased)
    if (progress < 1) requestAnimationFrame(tick)
  }
  tick()
}

// Watch for data load, then animate
import { watch } from 'vue'
watch(() => stats.value, (newStats) => {
  Object.entries(newStats).forEach(([key, val]) => animateCounter(key, val))
}, { deep: true })

function formatSessionTime(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const now = new Date()
  const diff = now - d
  if (diff < 60000) return 'Just now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' })
}

// --- Quick message input ---
const quickMessage = ref('')
function goToChat() {
  router.push('/chat')
}
</script>

<template>
  <div class="h-full overflow-y-auto">
    <!-- Hero Section with animated gradient bg -->
    <div class="relative overflow-hidden">
      <!-- Animated gradient blobs -->
      <div class="absolute inset-0 overflow-hidden pointer-events-none">
        <div class="absolute -top-32 -right-32 w-96 h-96 bg-green-500/10 rounded-full blur-3xl animate-blob" />
        <div class="absolute top-20 -left-20 w-80 h-80 bg-emerald-500/8 rounded-full blur-3xl animate-blob animation-delay-2000" />
        <div class="absolute bottom-0 right-1/3 w-72 h-72 bg-teal-500/6 rounded-full blur-3xl animate-blob animation-delay-4000" />
      </div>

      <div class="relative p-8 pb-6 max-w-6xl mx-auto">
        <!-- Top bar: date + time -->
        <div class="flex items-center justify-between mb-8">
          <div class="flex items-center gap-2 text-gray-500 text-sm">
            <Clock :size="14" />
            <span>{{ currentDate }}</span>
          </div>
          <div class="text-3xl font-light text-gray-400 tabular-nums tracking-wider">
            {{ currentTime }}
          </div>
        </div>

        <!-- Greeting -->
        <div class="mb-8">
          <h1 class="text-4xl font-bold tracking-tight mb-2">
            <span class="bg-gradient-to-r from-green-400 via-emerald-400 to-teal-400 bg-clip-text text-transparent">
              {{ greeting }}
            </span>
          </h1>
          <p class="text-gray-500 text-lg">Dashboard overview for <span class="text-gray-300">Pepebot</span> 🐸</p>
        </div>

        <!-- Stats Cards -->
        <div class="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          <div
            v-for="(stat, idx) in [
              { key: 'agents', label: 'Agents', icon: Cpu, iconBg: 'rgba(168,85,247,0.15)', iconColor: '#c084fc', gradient: 'from-purple-500/20 to-pink-500/10' },
              { key: 'skills', label: 'Skills', icon: Zap, iconBg: 'rgba(245,158,11,0.15)', iconColor: '#fbbf24', gradient: 'from-amber-500/20 to-orange-500/10' },
              { key: 'workflows', label: 'Workflows', icon: GitBranch, iconBg: 'rgba(34,197,94,0.15)', iconColor: '#4ade80', gradient: 'from-green-500/20 to-emerald-500/10' },
              { key: 'sessions', label: 'Sessions', icon: MessageSquare, iconBg: 'rgba(59,130,246,0.15)', iconColor: '#60a5fa', gradient: 'from-blue-500/20 to-indigo-500/10' },
            ]"
            :key="stat.key"
            class="relative group bg-[#1a1a20] border border-white/5 rounded-2xl p-5 hover:border-white/10 transition-all duration-300 cursor-pointer overflow-hidden"
            :style="{ animationDelay: `${idx * 100}ms` }"
            @click="router.push(stat.key === 'sessions' ? '/chat' : `/${stat.key === 'agents' ? 'agents' : stat.key}`)"
          >
            <!-- Hover gradient -->
            <div :class="`absolute inset-0 bg-gradient-to-br ${stat.gradient} opacity-0 group-hover:opacity-100 transition-opacity duration-500`" />
            
            <div class="relative">
              <div class="flex items-center justify-between mb-3">
                <div class="w-10 h-10 rounded-xl flex items-center justify-center" :style="{ background: stat.iconBg, color: stat.iconColor }">
                  <component :is="stat.icon" :size="20" />
                </div>
                <ChevronRight :size="16" class="text-gray-600 group-hover:text-gray-400 group-hover:translate-x-0.5 transition-all" />
              </div>
              <div class="text-3xl font-bold text-white mb-0.5 tabular-nums">
                {{ animatedStats[stat.key] }}
              </div>
              <div class="text-xs text-gray-500 uppercase tracking-wider font-medium">
                {{ stat.label }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Content Grid -->
    <div class="px-8 pb-8 max-w-6xl mx-auto">
      <div class="grid grid-cols-1 lg:grid-cols-5 gap-6">

        <!-- Quick Actions (left, 3 cols) -->
        <div class="lg:col-span-3">
          <h3 class="text-sm text-gray-500 uppercase tracking-wider font-medium mb-4 flex items-center gap-2">
            <Sparkles :size="14" />
            Quick Actions
          </h3>
          <div class="grid grid-cols-2 gap-3">
            <router-link
              v-for="action in quickActions"
              :key="action.label"
              :to="action.route"
              class="group relative bg-[#1a1a20] border border-white/5 rounded-2xl p-5 hover:border-white/10 transition-all duration-300 overflow-hidden"
            >
              <!-- Background gradient on hover -->
              <div :class="`absolute inset-0 bg-gradient-to-br ${action.gradient} opacity-0 group-hover:opacity-10 transition-opacity duration-500`" />
              
              <div class="relative flex items-start gap-4">
                <div :class="`w-12 h-12 rounded-xl bg-gradient-to-br ${action.gradient} flex items-center justify-center text-white flex-shrink-0 shadow-lg group-hover:scale-110 transition-transform duration-300`">
                  <component :is="action.icon" :size="22" />
                </div>
                <div class="min-w-0">
                  <h4 class="text-sm font-semibold text-gray-200 group-hover:text-white transition-colors">{{ action.label }}</h4>
                  <ArrowRight :size="14" class="text-gray-600 group-hover:text-gray-400 mt-1 group-hover:translate-x-1 transition-all" />
                </div>
              </div>
            </router-link>
          </div>

          <!-- Chat input shortcut -->
          <router-link 
            to="/chat" 
            class="mt-4 flex items-center gap-3 bg-[#1a1a20] border border-white/5 rounded-2xl px-5 py-4 hover:border-green-500/30 hover:bg-[#1e1e26] transition-all group cursor-text"
          >
            <div class="w-9 h-9 rounded-xl bg-gradient-to-br from-green-500/20 to-emerald-500/20 flex items-center justify-center text-lg flex-shrink-0">
              🐸
            </div>
            <span class="flex-1 text-gray-500 text-sm">Message Pepebot...</span>
            <div class="w-8 h-8 rounded-lg bg-white/5 group-hover:bg-green-500/20 flex items-center justify-center transition-all">
              <Send :size="14" class="text-gray-500 group-hover:text-green-400 transition-colors" />
            </div>
          </router-link>
        </div>

        <!-- Recent Sessions (right, 2 cols) -->
        <div class="lg:col-span-2">
          <h3 class="text-sm text-gray-500 uppercase tracking-wider font-medium mb-4 flex items-center gap-2">
            <Activity :size="14" />
            Recent Sessions
          </h3>
          <div class="bg-[#1a1a20] border border-white/5 rounded-2xl overflow-hidden">
            <div v-if="isLoading" class="flex items-center justify-center py-12">
              <Activity :size="20" class="animate-spin text-gray-500" />
            </div>
            <div v-else-if="recentSessions.length === 0" class="py-12 text-center text-gray-600 text-sm">
              No sessions yet
            </div>
            <div v-else>
              <router-link
                v-for="(session, idx) in recentSessions"
                :key="session.key"
                to="/chat"
                class="flex items-center gap-3 px-4 py-3.5 hover:bg-white/[0.03] transition-colors group"
                :class="idx < recentSessions.length - 1 ? 'border-b border-white/5' : ''"
              >
                <div class="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-500/15 to-indigo-500/15 flex items-center justify-center text-blue-400 flex-shrink-0">
                  <MessageSquare :size="14" />
                </div>
                <div class="flex-1 min-w-0">
                  <p class="text-sm text-gray-300 truncate font-medium">{{ session.key }}</p>
                  <p class="text-[11px] text-gray-600">{{ session.message_count || 0 }} messages</p>
                </div>
                <div class="text-[11px] text-gray-600 flex-shrink-0">
                  {{ formatSessionTime(session.updated) }}
                </div>
              </router-link>
            </div>
          </div>

          <!-- System Status -->
          <div class="mt-4 bg-[#1a1a20] border border-white/5 rounded-2xl p-4">
            <div class="flex items-center gap-2 mb-3">
              <div class="w-2 h-2 rounded-full bg-green-400 animate-pulse" />
              <span class="text-xs text-gray-400 font-medium">System Online</span>
            </div>
            <div class="space-y-2">
              <div class="flex items-center justify-between text-xs">
                <span class="text-gray-500">Gateway API</span>
                <span class="text-green-400 font-mono">:18790</span>
              </div>
              <div class="flex items-center justify-between text-xs">
                <span class="text-gray-500">Dashboard</span>
                <span class="text-green-400 font-mono">:3000</span>
              </div>
              <div class="flex items-center justify-between text-xs">
                <span class="text-gray-500">Active Agents</span>
                <span class="text-gray-300">{{ stats.agents }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style>
@keyframes blob {
  0%, 100% { transform: translate(0, 0) scale(1); }
  33% { transform: translate(30px, -20px) scale(1.1); }
  66% { transform: translate(-20px, 20px) scale(0.9); }
}
.animate-blob { animation: blob 8s ease-in-out infinite; }
.animation-delay-2000 { animation-delay: 2s; }
.animation-delay-4000 { animation-delay: 4s; }
</style>
