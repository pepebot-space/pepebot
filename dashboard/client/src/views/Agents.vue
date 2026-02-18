<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { Cpu, Activity, Server } from 'lucide-vue-next'

const agents = ref([])
const isLoading = ref(true)

onMounted(async () => {
    try {
        // Fetch agents from backend (which proxies to gateway)
        // Since /status returns agent info, we can use that.
        // Or if there's a specific agents endpoint. For now, use /api/agents mock/proxy
        const response = await axios.get('http://localhost:3000/api/agents')
        if (response.data.agent) {
             // Adapt single agent response to list if needed, or if it returns multiple
             agents.value = [
                 { 
                    name: response.data.agent || 'Default Agent', 
                    model: response.data.model || 'Unknown', 
                    status: 'Active', 
                    type: 'General Assistant' 
                 }
             ]
             if (response.data.agents && Array.isArray(response.data.agents)) {
                 agents.value = response.data.agents;
             }
        } else if (response.data.agents) {
            agents.value = response.data.agents;
        }
    } catch (e) {
        console.error(e)
        // Fallback mock
        agents.value = [
            { name: 'Pepebot (Main)', model: 'gemini-2.0-flash', status: 'Active', type: 'Orchestrator' },
            { name: 'Coder', model: 'claude-3-5-sonnet', status: 'Idle', type: 'Coding Specialist' },
            { name: 'Researcher', model: 'gpt-4o', status: 'Idle', type: 'Web Search' }
        ]
    } finally {
        isLoading.value = false
    }
})
</script>

<template>
  <div class="p-10 h-full max-w-5xl mx-auto">
    <header class="mb-10">
      <h1 class="text-3xl font-semibold mb-2">My Agents</h1>
      <p class="text-gray-400">Manage and monitor your AI workforce.</p>
    </header>

    <div v-if="isLoading" class="flex items-center justify-center h-64">
        <div class="animate-spin text-green-500">
            <Activity :size="32" />
        </div>
    </div>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <div 
        v-for="(agent, index) in agents" 
        :key="index"
        class="bg-[#1e1e24] p-6 rounded-3xl border border-white/5 hover:border-white/10 transition-all flex flex-col gap-4 group"
      >
        <div class="flex items-start justify-between">
            <div class="w-12 h-12 rounded-2xl bg-gradient-to-br from-blue-500/20 to-purple-500/20 flex items-center justify-center text-blue-400">
                <Cpu :size="24" />
            </div>
            <div class="px-3 py-1 rounded-full text-xs font-medium border" 
                :class="agent.status === 'Active' ? 'bg-green-500/10 text-green-400 border-green-500/20' : 'bg-gray-700/30 text-gray-400 border-white/5'">
                {{ agent.status }}
            </div>
        </div>

        <div>
            <h3 class="text-xl font-medium mb-1">{{ agent.name }}</h3>
            <p class="text-sm text-gray-500">{{ agent.type }}</p>
        </div>

        <div class="mt-auto pt-4 border-t border-white/5 flex items-center gap-2 text-xs text-gray-400">
            <Server :size="14" />
            <span>{{ agent.model }}</span>
        </div>
      </div>

       <!-- Add New Agent Card -->
       <div class="border border-dashed border-white/10 rounded-3xl p-6 flex flex-col items-center justify-center text-gray-500 gap-3 hover:bg-white/5 hover:border-white/20 transition-all cursor-pointer h-full min-h-[200px]">
            <div class="w-12 h-12 rounded-full bg-white/5 flex items-center justify-center">
                <span class="text-2xl">+</span>
            </div>
            <span class="font-medium">Deploy New Agent</span>
       </div>
    </div>
  </div>
</template>
