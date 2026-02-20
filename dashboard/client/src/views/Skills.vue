<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { Zap, Activity, Package, CheckCircle, XCircle, ExternalLink } from 'lucide-vue-next'
import AgentChat from '../components/AgentChat.vue'

const router = useRouter()

const GATEWAY_API = 'http://localhost:18790/v1'
const skills = ref([])
const isLoading = ref(true)
const filter = ref('all') // all, workspace, builtin

onMounted(async () => {
    try {
        const response = await axios.get(`${GATEWAY_API}/skills`)
        skills.value = response.data.skills || []
    } catch (e) {
        console.error('Failed to fetch skills:', e)
    } finally {
        isLoading.value = false
    }
})

const filteredSkills = computed(() => {
    if (filter.value === 'all') return skills.value
    return skills.value.filter(s => s.source === filter.value)
})

const skillCounts = computed(() => ({
    all: skills.value.length,
    workspace: skills.value.filter(s => s.source === 'workspace').length,
    builtin: skills.value.filter(s => s.source === 'builtin').length,
}))
</script>

<template>
  <div class="p-8 h-full max-w-6xl mx-auto overflow-y-auto">
    <header class="mb-8">
      <div class="flex items-center gap-3 mb-4">
        <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-yellow-500/20 to-orange-500/20 flex items-center justify-center text-yellow-400">
          <Zap :size="22" />
        </div>
        <div>
          <h1 class="text-2xl font-semibold">Skills</h1>
          <p class="text-sm text-gray-500">Installed skill extensions for your agents</p>
        </div>
      </div>

      <!-- Filter Tabs -->
      <div class="flex gap-2">
        <button 
          v-for="tab in ['all', 'workspace', 'builtin']" 
          :key="tab"
          @click="filter = tab"
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
          :class="filter === tab 
            ? 'bg-white/10 text-white' 
            : 'text-gray-500 hover:text-gray-300 hover:bg-white/5'"
        >
          {{ tab.charAt(0).toUpperCase() + tab.slice(1) }}
          <span class="ml-1 text-[10px] opacity-60">({{ skillCounts[tab] }})</span>
        </button>
      </div>
    </header>

    <!-- Loading -->
    <div v-if="isLoading" class="flex items-center justify-center h-64">
        <div class="animate-spin text-yellow-500">
            <Activity :size="32" />
        </div>
    </div>

    <!-- Empty state -->
    <div v-else-if="filteredSkills.length === 0" class="flex flex-col items-center justify-center h-64 text-gray-500">
        <Zap :size="48" class="mb-4 opacity-30" />
        <p class="text-lg">No skills found</p>
        <p class="text-sm text-gray-600">Install skills via <code class="bg-white/10 px-1.5 py-0.5 rounded text-xs">pepebot skills install</code></p>
    </div>

    <!-- Skills Grid -->
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <div 
        v-for="skill in filteredSkills" 
        :key="skill.name"
        @click="router.push(`/skills/${skill.name}`)"
        class="bg-[#1e1e24] p-5 rounded-2xl border border-white/5 hover:border-white/10 transition-all flex flex-col gap-3 group cursor-pointer"
      >
        <!-- Header -->
        <div class="flex items-start justify-between">
          <div class="w-10 h-10 rounded-xl flex items-center justify-center"
            :class="skill.source === 'workspace' 
              ? 'bg-gradient-to-br from-purple-500/20 to-pink-500/20 text-purple-400' 
              : 'bg-gradient-to-br from-blue-500/20 to-cyan-500/20 text-blue-400'">
            <Package :size="20" />
          </div>
          <div class="flex items-center gap-1.5">
            <component 
              :is="skill.available ? CheckCircle : XCircle" 
              :size="14" 
              :class="skill.available ? 'text-green-400' : 'text-red-400'" 
            />
            <span class="text-xs" :class="skill.available ? 'text-green-400' : 'text-red-400'">
              {{ skill.available ? 'Ready' : 'Unavailable' }}
            </span>
          </div>
        </div>

        <!-- Info -->
        <div>
          <h3 class="text-base font-medium mb-1">{{ skill.name }}</h3>
          <p class="text-xs text-gray-500 leading-relaxed">{{ skill.description || 'No description available' }}</p>
        </div>

        <!-- Footer -->
        <div class="mt-auto pt-3 border-t border-white/5 flex items-center justify-between text-xs text-gray-500">
          <span class="px-2 py-0.5 rounded-md bg-white/5 font-medium">{{ skill.source }}</span>
          <span v-if="skill.missing" class="text-red-400/70 text-[11px]">{{ skill.missing }}</span>
        </div>
      </div>
    </div>

    <AgentChat context="skills" />
  </div>
</template>
