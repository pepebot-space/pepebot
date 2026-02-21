<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { GitBranch, Activity, Play, ChevronDown, ChevronUp, Variable, Layers } from 'lucide-vue-next'
import AgentChat from '../components/AgentChat.vue'
import { getGatewayApiUrl } from '../lib/gateway.js'

const GATEWAY_API = getGatewayApiUrl()
const workflows = ref([])
const isLoading = ref(true)
const expandedWorkflow = ref(null)
const workflowDetails = ref({})

onMounted(async () => {
    try {
        const response = await axios.get(`${GATEWAY_API}/workflows`)
        workflows.value = response.data.workflows || []
    } catch (e) {
        console.error('Failed to fetch workflows:', e)
    } finally {
        isLoading.value = false
    }
})

const toggleExpand = async (wf) => {
    const key = wf.file || wf.name
    if (expandedWorkflow.value === key) {
        expandedWorkflow.value = null
        return
    }
    expandedWorkflow.value = key

    // Fetch full details if not cached
    if (!workflowDetails.value[key]) {
        try {
            const response = await axios.get(`${GATEWAY_API}/workflows/${key}`)
            workflowDetails.value[key] = response.data
        } catch (e) {
            console.error('Failed to fetch workflow details:', e)
        }
    }
}
</script>

<template>
  <div class="p-8 h-full max-w-6xl mx-auto overflow-y-auto">
    <header class="mb-8">
      <div class="flex items-center gap-3 mb-2">
        <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-green-500/20 to-teal-500/20 flex items-center justify-center text-green-400">
          <GitBranch :size="22" />
        </div>
        <div>
          <h1 class="text-2xl font-semibold">Workflows</h1>
          <p class="text-sm text-gray-500">Automated task sequences for your agents</p>
        </div>
      </div>
    </header>

    <!-- Loading -->
    <div v-if="isLoading" class="flex items-center justify-center h-64">
        <div class="animate-spin text-green-500">
            <Activity :size="32" />
        </div>
    </div>

    <!-- Empty state -->
    <div v-else-if="workflows.length === 0" class="flex flex-col items-center justify-center h-64 text-gray-500">
        <GitBranch :size="48" class="mb-4 opacity-30" />
        <p class="text-lg">No workflows configured</p>
        <p class="text-sm text-gray-600">Ask Pepebot to create workflows for you, or add JSON files to <code class="bg-white/10 px-1.5 py-0.5 rounded text-xs">workspace/workflows/</code></p>
    </div>

    <!-- Workflow List -->
    <div v-else class="flex flex-col gap-4">
      <div 
        v-for="wf in workflows" 
        :key="wf.name"
        class="bg-[#1e1e24] rounded-2xl border border-white/5 hover:border-white/10 transition-all overflow-hidden"
      >
        <!-- Card Header -->
        <div class="p-5 cursor-pointer" @click="toggleExpand(wf)">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-emerald-500/20 to-green-500/20 flex items-center justify-center text-emerald-400">
                <Play :size="18" />
              </div>
              <div>
                <h3 class="text-base font-medium">{{ wf.name }}</h3>
                <p class="text-xs text-gray-500 mt-0.5">{{ wf.description || 'No description' }}</p>
              </div>
            </div>
            <div class="flex items-center gap-3">
              <div class="flex items-center gap-1.5 text-xs text-gray-400">
                <Layers :size="13" />
                <span>{{ wf.step_count }} steps</span>
              </div>
              <div v-if="wf.variables && Object.keys(wf.variables).length > 0" class="flex items-center gap-1.5 text-xs text-gray-400">
                <Variable :size="13" />
                <span>{{ Object.keys(wf.variables).length }} vars</span>
              </div>
              <component :is="expandedWorkflow === (wf.file || wf.name) ? ChevronUp : ChevronDown" :size="16" class="text-gray-500" />
            </div>
          </div>
        </div>

        <!-- Expanded: Steps & Variables -->
        <div v-if="expandedWorkflow === (wf.file || wf.name) && workflowDetails[wf.file || wf.name]" class="px-5 pb-5 border-t border-white/5">
          <!-- Variables -->
          <div v-if="workflowDetails[wf.file || wf.name].variables && Object.keys(workflowDetails[wf.file || wf.name].variables).length > 0" class="mt-4 mb-4">
            <h4 class="text-xs text-gray-500 font-medium mb-2 uppercase tracking-wider">Variables</h4>
            <div class="flex flex-wrap gap-2">
              <div v-for="(val, key) in workflowDetails[wf.file || wf.name].variables" :key="key" 
                class="bg-white/5 px-2.5 py-1 rounded-lg text-xs">
                <span class="text-purple-400 font-mono">{{ key }}</span>
                <span class="text-gray-600 mx-1">=</span>
                <span class="text-gray-300">{{ val }}</span>
              </div>
            </div>
          </div>

          <!-- Steps -->
          <div>
            <h4 class="text-xs text-gray-500 font-medium mb-3 uppercase tracking-wider">Steps</h4>
            <div class="space-y-2">
              <div v-for="(step, index) in workflowDetails[wf.file || wf.name].steps" :key="index"
                class="flex items-start gap-3 bg-white/[0.03] rounded-xl p-3">
                <div class="w-6 h-6 rounded-full bg-white/10 flex items-center justify-center text-xs text-gray-400 flex-shrink-0 mt-0.5">
                  {{ index + 1 }}
                </div>
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-gray-200">{{ step.name || `Step ${index + 1}` }}</p>
                  <div v-if="step.tool" class="mt-1">
                    <span class="text-xs font-mono bg-blue-500/10 text-blue-400 px-1.5 py-0.5 rounded">{{ step.tool }}</span>
                  </div>
                  <div v-if="step.skill" class="mt-1">
                    <span class="text-xs font-mono bg-purple-500/10 text-purple-400 px-1.5 py-0.5 rounded">skill: {{ step.skill }}</span>
                  </div>
                  <div v-if="step.agent" class="mt-1">
                    <span class="text-xs font-mono bg-amber-500/10 text-amber-400 px-1.5 py-0.5 rounded">agent: {{ step.agent }}</span>
                  </div>
                  <p v-if="step.goal" class="text-xs text-gray-500 mt-1">{{ step.goal }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Loading details -->
        <div v-else-if="expandedWorkflow === (wf.file || wf.name) && !workflowDetails[wf.file || wf.name]" class="px-5 pb-5 border-t border-white/5">
          <div class="flex items-center justify-center py-6">
            <Activity :size="20" class="animate-spin text-green-500" />
          </div>
        </div>
      </div>
    </div>

    <AgentChat context="workflows" />
  </div>
</template>
