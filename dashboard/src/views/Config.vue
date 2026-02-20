<script setup>
import { ref, onMounted, computed } from 'vue'
import axios from 'axios'
import { Settings, Save, RotateCcw, Activity, Check, AlertTriangle, ChevronDown, ChevronUp, Server, Cpu, Radio, Key, Wrench, Shield, Eye, EyeOff } from 'lucide-vue-next'
import { getGatewayApiUrl } from '../lib/gateway.js'

const GATEWAY_API = getGatewayApiUrl()
const config = ref(null)
const isLoading = ref(true)
const isSaving = ref(false)
const saveMessage = ref('')
const saveError = ref('')
const expandedSections = ref(['agents', 'gateway'])
const showSecrets = ref({})

onMounted(async () => {
    await loadConfig()
})

async function loadConfig() {
    isLoading.value = true
    try {
        const response = await axios.get(`${GATEWAY_API}/config`)
        config.value = response.data
    } catch (e) {
        console.error('Failed to load config:', e)
        saveError.value = 'Failed to load configuration'
    } finally {
        isLoading.value = false
    }
}

async function saveConfig() {
    isSaving.value = true
    saveMessage.value = ''
    saveError.value = ''
    try {
        const response = await axios.put(`${GATEWAY_API}/config`, config.value)
        saveMessage.value = response.data.message || 'Configuration saved!'
        setTimeout(() => saveMessage.value = '', 5000)
    } catch (e) {
        saveError.value = e.response?.data?.error?.message || 'Failed to save configuration'
        setTimeout(() => saveError.value = '', 5000)
    } finally {
        isSaving.value = false
    }
}

async function resetConfig() {
    await loadConfig()
    saveMessage.value = 'Configuration reloaded from disk'
    setTimeout(() => saveMessage.value = '', 3000)
}

function toggleSection(section) {
    const idx = expandedSections.value.indexOf(section)
    if (idx >= 0) expandedSections.value.splice(idx, 1)
    else expandedSections.value.push(section)
}

function toggleSecret(key) {
    showSecrets.value[key] = !showSecrets.value[key]
}

function isSensitive(key) {
    return key.includes('api_key') || key.includes('token') || key.includes('secret')
}

// Channel metadata for display
const channelMeta = {
    whatsapp: { name: 'WhatsApp', color: 'from-green-500/20 to-green-600/20', text: 'text-green-400' },
    telegram: { name: 'Telegram', color: 'from-blue-400/20 to-blue-500/20', text: 'text-blue-400' },
    discord: { name: 'Discord', color: 'from-indigo-400/20 to-purple-500/20', text: 'text-indigo-400' },
    feishu: { name: 'Feishu', color: 'from-blue-500/20 to-cyan-500/20', text: 'text-cyan-400' },
    maixcam: { name: 'MaixCam', color: 'from-orange-500/20 to-red-500/20', text: 'text-orange-400' },
}

const providerMeta = {
    maiarouter: { name: 'MAIA Router' },
    anthropic: { name: 'Anthropic' },
    openai: { name: 'OpenAI' },
    openrouter: { name: 'OpenRouter' },
    groq: { name: 'Groq' },
    zhipu: { name: 'Zhipu AI' },
    vllm: { name: 'vLLM' },
    gemini: { name: 'Google Gemini' },
}
</script>

<template>
  <div class="p-8 h-full max-w-5xl mx-auto overflow-y-auto pb-24">
    <header class="mb-8">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-gray-500/20 to-zinc-500/20 flex items-center justify-center text-gray-400">
            <Settings :size="22" />
          </div>
          <div>
            <h1 class="text-2xl font-semibold">Configuration</h1>
            <p class="text-sm text-gray-500">Edit <code class="bg-white/10 px-1.5 py-0.5 rounded text-xs">~/.pepebot/config.json</code></p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button @click="resetConfig" class="px-3 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-gray-400 hover:text-white transition-all flex items-center gap-2 text-sm">
            <RotateCcw :size="15" />
            Reload
          </button>
          <button @click="saveConfig" :disabled="isSaving"
            class="px-4 py-2 rounded-xl bg-gradient-to-r from-green-500 to-emerald-600 hover:from-green-400 hover:to-emerald-500 text-black font-medium text-sm transition-all flex items-center gap-2 disabled:opacity-50">
            <Save :size="15" />
            {{ isSaving ? 'Saving...' : 'Save' }}
          </button>
        </div>
      </div>

      <!-- Status Messages -->
      <div v-if="saveMessage" class="mt-4 px-4 py-3 rounded-xl bg-green-500/10 border border-green-500/20 text-green-400 text-sm flex items-center gap-2">
        <Check :size="16" /> {{ saveMessage }}
      </div>
      <div v-if="saveError" class="mt-4 px-4 py-3 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-sm flex items-center gap-2">
        <AlertTriangle :size="16" /> {{ saveError }}
      </div>
    </header>

    <!-- Loading -->
    <div v-if="isLoading" class="flex items-center justify-center h-64">
      <Activity :size="32" class="animate-spin text-gray-500" />
    </div>

    <div v-else-if="config" class="flex flex-col gap-4">

      <!-- ============ AGENTS SECTION ============ -->
      <div class="bg-[#1e1e24] rounded-2xl border border-white/5 overflow-hidden">
        <div class="p-5 flex items-center justify-between cursor-pointer" @click="toggleSection('agents')">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-lg bg-gradient-to-br from-blue-500/20 to-purple-500/20 flex items-center justify-center text-blue-400">
              <Cpu :size="18" />
            </div>
            <div>
              <h2 class="text-base font-medium">Agent Defaults</h2>
              <p class="text-xs text-gray-500">Default model, temperature, and token limits</p>
            </div>
          </div>
          <component :is="expandedSections.includes('agents') ? ChevronUp : ChevronDown" :size="16" class="text-gray-500" />
        </div>
        <div v-if="expandedSections.includes('agents') && config.agents" class="px-5 pb-5 border-t border-white/5">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
            <div>
              <label class="block text-xs text-gray-500 mb-1.5">Model</label>
              <input v-model="config.agents.defaults.model" type="text"
                class="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500/50 font-mono" />
            </div>
            <div>
              <label class="block text-xs text-gray-500 mb-1.5">Workspace</label>
              <input v-model="config.agents.defaults.workspace" type="text"
                class="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500/50 font-mono" />
            </div>
            <div>
              <label class="block text-xs text-gray-500 mb-1.5">Temperature</label>
              <input v-model.number="config.agents.defaults.temperature" type="number" step="0.1" min="0" max="2"
                class="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500/50" />
            </div>
            <div>
              <label class="block text-xs text-gray-500 mb-1.5">Max Tokens</label>
              <input v-model.number="config.agents.defaults.max_tokens" type="number" min="256"
                class="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500/50" />
            </div>
            <div>
              <label class="block text-xs text-gray-500 mb-1.5">Max Tool Iterations</label>
              <input v-model.number="config.agents.defaults.max_tool_iterations" type="number" min="1"
                class="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500/50" />
            </div>
          </div>
        </div>
      </div>

      <!-- ============ GATEWAY SECTION ============ -->
      <div class="bg-[#1e1e24] rounded-2xl border border-white/5 overflow-hidden">
        <div class="p-5 flex items-center justify-between cursor-pointer" @click="toggleSection('gateway')">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-lg bg-gradient-to-br from-cyan-500/20 to-blue-500/20 flex items-center justify-center text-cyan-400">
              <Server :size="18" />
            </div>
            <div>
              <h2 class="text-base font-medium">Gateway</h2>
              <p class="text-xs text-gray-500">HTTP API server settings</p>
            </div>
          </div>
          <component :is="expandedSections.includes('gateway') ? ChevronUp : ChevronDown" :size="16" class="text-gray-500" />
        </div>
        <div v-if="expandedSections.includes('gateway') && config.gateway" class="px-5 pb-5 border-t border-white/5">
          <div class="grid grid-cols-2 gap-4 mt-4">
            <div>
              <label class="block text-xs text-gray-500 mb-1.5">Host</label>
              <input v-model="config.gateway.host" type="text"
                class="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500/50 font-mono" />
            </div>
            <div>
              <label class="block text-xs text-gray-500 mb-1.5">Port</label>
              <input v-model.number="config.gateway.port" type="number"
                class="w-full bg-white/5 border border-white/10 rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500/50" />
            </div>
          </div>
        </div>
      </div>

      <!-- ============ PROVIDERS SECTION ============ -->
      <div class="bg-[#1e1e24] rounded-2xl border border-white/5 overflow-hidden">
        <div class="p-5 flex items-center justify-between cursor-pointer" @click="toggleSection('providers')">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-lg bg-gradient-to-br from-purple-500/20 to-pink-500/20 flex items-center justify-center text-purple-400">
              <Key :size="18" />
            </div>
            <div>
              <h2 class="text-base font-medium">Providers</h2>
              <p class="text-xs text-gray-500">LLM provider API keys and endpoints</p>
            </div>
          </div>
          <component :is="expandedSections.includes('providers') ? ChevronUp : ChevronDown" :size="16" class="text-gray-500" />
        </div>
        <div v-if="expandedSections.includes('providers') && config.providers" class="px-5 pb-5 border-t border-white/5">
          <div class="space-y-4 mt-4">
            <div v-for="(provider, key) in config.providers" :key="key"
              class="bg-white/[0.03] rounded-xl p-4">
              <h3 class="text-sm font-medium mb-3 text-gray-300">{{ providerMeta[key]?.name || key }}</h3>
              <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div v-for="(val, field) in provider" :key="field">
                  <label class="block text-xs text-gray-500 mb-1">{{ field }}</label>
                  <div class="relative">
                    <input 
                      v-model="config.providers[key][field]"
                      :type="isSensitive(field) && !showSecrets[key+field] ? 'password' : 'text'"
                      class="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-purple-500/50 font-mono pr-8"
                      :placeholder="isSensitive(field) ? '••••••••' : ''"
                    />
                    <button v-if="isSensitive(field)" @click="toggleSecret(key+field)" 
                      class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300">
                      <component :is="showSecrets[key+field] ? EyeOff : Eye" :size="14" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ============ CHANNELS SECTION ============ -->
      <div class="bg-[#1e1e24] rounded-2xl border border-white/5 overflow-hidden">
        <div class="p-5 flex items-center justify-between cursor-pointer" @click="toggleSection('channels')">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-lg bg-gradient-to-br from-green-500/20 to-teal-500/20 flex items-center justify-center text-green-400">
              <Radio :size="18" />
            </div>
            <div>
              <h2 class="text-base font-medium">Channels</h2>
              <p class="text-xs text-gray-500">Messaging platform integrations</p>
            </div>
          </div>
          <component :is="expandedSections.includes('channels') ? ChevronUp : ChevronDown" :size="16" class="text-gray-500" />
        </div>
        <div v-if="expandedSections.includes('channels') && config.channels" class="px-5 pb-5 border-t border-white/5">
          <div class="space-y-4 mt-4">
            <div v-for="(channel, key) in config.channels" :key="key"
              class="bg-white/[0.03] rounded-xl p-4">
              <div class="flex items-center justify-between mb-3">
                <h3 class="text-sm font-medium" :class="channelMeta[key]?.text || 'text-gray-300'">
                  {{ channelMeta[key]?.name || key }}
                </h3>
                <label class="flex items-center gap-2 cursor-pointer">
                  <span class="text-xs text-gray-500">{{ channel.enabled ? 'Enabled' : 'Disabled' }}</span>
                  <div class="relative">
                    <input type="checkbox" v-model="config.channels[key].enabled" class="sr-only peer" />
                    <div class="w-9 h-5 bg-white/10 rounded-full peer peer-checked:bg-green-500/30 transition-colors"></div>
                    <div class="absolute left-0.5 top-0.5 w-4 h-4 bg-gray-400 rounded-full peer-checked:translate-x-4 peer-checked:bg-green-400 transition-all"></div>
                  </div>
                </label>
              </div>
              <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                <template v-for="(val, field) in channel" :key="field">
                  <div v-if="field !== 'enabled' && field !== 'allow_from'">
                    <label class="block text-xs text-gray-500 mb-1">{{ field }}</label>
                    <div class="relative">
                      <input 
                        v-model="config.channels[key][field]"
                        :type="isSensitive(field) && !showSecrets[key+field] ? 'password' : (typeof val === 'number' ? 'number' : 'text')"
                        class="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-green-500/50 font-mono pr-8"
                      />
                      <button v-if="isSensitive(field)" @click="toggleSecret(key+field)"
                        class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300">
                        <component :is="showSecrets[key+field] ? EyeOff : Eye" :size="14" />
                      </button>
                    </div>
                  </div>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ============ TOOLS SECTION ============ -->
      <div class="bg-[#1e1e24] rounded-2xl border border-white/5 overflow-hidden">
        <div class="p-5 flex items-center justify-between cursor-pointer" @click="toggleSection('tools')">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-lg bg-gradient-to-br from-yellow-500/20 to-orange-500/20 flex items-center justify-center text-yellow-400">
              <Wrench :size="18" />
            </div>
            <div>
              <h2 class="text-base font-medium">Tools</h2>
              <p class="text-xs text-gray-500">Web search and tool configuration</p>
            </div>
          </div>
          <component :is="expandedSections.includes('tools') ? ChevronUp : ChevronDown" :size="16" class="text-gray-500" />
        </div>
        <div v-if="expandedSections.includes('tools') && config.tools" class="px-5 pb-5 border-t border-white/5">
          <div class="mt-4">
            <h3 class="text-sm font-medium text-gray-300 mb-3">Web Search</h3>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div>
                <label class="block text-xs text-gray-500 mb-1">API Key</label>
                <div class="relative">
                  <input 
                    v-model="config.tools.web.search.api_key"
                    :type="!showSecrets['web_search_key'] ? 'password' : 'text'"
                    class="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-yellow-500/50 font-mono pr-8"
                    placeholder="••••••••"
                  />
                  <button @click="toggleSecret('web_search_key')"
                    class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300">
                    <component :is="showSecrets['web_search_key'] ? EyeOff : Eye" :size="14" />
                  </button>
                </div>
              </div>
              <div>
                <label class="block text-xs text-gray-500 mb-1">Max Results</label>
                <input v-model.number="config.tools.web.search.max_results" type="number" min="1" max="20"
                  class="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-yellow-500/50" />
              </div>
            </div>
          </div>
        </div>
      </div>

    </div>
  </div>
</template>
