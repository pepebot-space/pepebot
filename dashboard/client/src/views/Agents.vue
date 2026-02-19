<script setup>
import { ref, onMounted, computed } from 'vue'
import axios from 'axios'
import { Cpu, Activity, Server, ToggleLeft, ToggleRight, Thermometer, Hash, ChevronDown, ChevronUp } from 'lucide-vue-next'

const GATEWAY_API = 'http://localhost:18790/v1'
const agents = ref({})
const isLoading = ref(true)
const expandedAgent = ref(null)

onMounted(async () => {
    try {
        const response = await axios.get(`${GATEWAY_API}/agents`)
        if (response.data.agents) {
            agents.value = response.data.agents
        }
    } catch (e) {
        console.error('Failed to fetch agents:', e)
    } finally {
        isLoading.value = false
    }
})

const agentList = computed(() => {
    return Object.entries(agents.value).map(([id, agent]) => ({
        id,
        ...agent
    }))
})

const toggleExpand = (id) => {
    expandedAgent.value = expandedAgent.value === id ? null : id
}
</script>

<template>
  <div class="p-8 h-full max-w-6xl mx-auto overflow-y-auto">
    <header class="mb-8">
      <div class="flex items-center gap-3 mb-2">
        <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-blue-500/20 to-purple-500/20 flex items-center justify-center text-blue-400">
          <Cpu :size="22" />
        </div>
        <div>
          <h1 class="text-2xl font-semibold">Agents</h1>
          <p class="text-sm text-gray-500">Manage your AI agents and configurations</p>
        </div>
      </div>
    </header>

    <!-- Loading -->
    <div v-if="isLoading" class="flex items-center justify-center h-64">
        <div class="animate-spin text-blue-500">
            <Activity :size="32" />
        </div>
    </div>

    <!-- Empty state -->
    <div v-else-if="agentList.length === 0" class="flex flex-col items-center justify-center h-64 text-gray-500">
        <Cpu :size="48" class="mb-4 opacity-30" />
        <p class="text-lg">No agents configured</p>
        <p class="text-sm text-gray-600">Add agents via config or CLI</p>
    </div>

    <!-- Agent Cards -->
    <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-5">
      <div 
        v-for="agent in agentList" 
        :key="agent.id"
        class="bg-[#1e1e24] rounded-2xl border border-white/5 hover:border-white/10 transition-all overflow-hidden"
      >
        <!-- Card Header -->
        <div class="p-5 cursor-pointer" @click="toggleExpand(agent.id)">
          <div class="flex items-start justify-between mb-3">
            <div class="flex items-center gap-3">
              <div class="w-11 h-11 rounded-xl bg-gradient-to-br from-blue-500/20 to-cyan-500/20 flex items-center justify-center text-blue-400">
                <Cpu :size="22" />
              </div>
              <div>
                <h3 class="text-lg font-medium">{{ agent.id }}</h3>
                <p class="text-xs text-gray-500 mt-0.5">{{ agent.description || 'No description' }}</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <div class="px-2.5 py-1 rounded-full text-xs font-medium border"
                :class="agent.enabled
                  ? 'bg-green-500/10 text-green-400 border-green-500/20'
                  : 'bg-red-500/10 text-red-400 border-red-500/20'">
                {{ agent.enabled ? 'Enabled' : 'Disabled' }}
              </div>
              <component :is="expandedAgent === agent.id ? ChevronUp : ChevronDown" :size="16" class="text-gray-500" />
            </div>
          </div>

          <!-- Quick Info -->
          <div class="flex items-center gap-4 text-xs text-gray-400">
            <div class="flex items-center gap-1.5">
              <Server :size="13" />
              <span class="font-mono">{{ agent.model }}</span>
            </div>
            <div v-if="agent.provider" class="flex items-center gap-1.5">
              <span class="text-gray-600">Provider:</span>
              <span>{{ agent.provider }}</span>
            </div>
          </div>
        </div>

        <!-- Expanded Details -->
        <div v-if="expandedAgent === agent.id" class="px-5 pb-5 pt-0 border-t border-white/5">
          <div class="grid grid-cols-2 gap-3 mt-4">
            <div class="bg-white/5 rounded-xl p-3">
              <div class="flex items-center gap-2 text-xs text-gray-500 mb-1">
                <Thermometer :size="13" />
                <span>Temperature</span>
              </div>
              <p class="text-sm font-medium">{{ agent.temperature || 0.7 }}</p>
            </div>
            <div class="bg-white/5 rounded-xl p-3">
              <div class="flex items-center gap-2 text-xs text-gray-500 mb-1">
                <Hash :size="13" />
                <span>Max Tokens</span>
              </div>
              <p class="text-sm font-medium">{{ agent.max_tokens || 'Default' }}</p>
            </div>
            <div v-if="agent.prompt_file" class="bg-white/5 rounded-xl p-3 col-span-2">
              <div class="text-xs text-gray-500 mb-1">Prompt File</div>
              <p class="text-sm font-mono text-gray-300">{{ agent.prompt_file }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
