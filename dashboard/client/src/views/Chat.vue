<script setup>
import { ref, onMounted, nextTick, watch, computed } from 'vue'
import axios from 'axios'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import 'highlight.js/styles/github-dark.css'
import { Send, Paperclip, Loader2, Bot, MessageSquare, Plus, ChevronDown, X, Sparkles } from 'lucide-vue-next'

// --- Markdown Setup ---
const md = new MarkdownIt({
  html: true, linkify: true, typographer: true,
  highlight(str, lang) {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return '<pre class="hljs"><code>' + hljs.highlight(str, { language: lang, ignoreIllegals: true }).value + '</code></pre>'
      } catch (__) {}
    }
    return '<pre class="hljs"><code>' + md.utils.escapeHtml(str) + '</code></pre>'
  }
})

// --- State ---
const messages = ref([])
const newMessage = ref('')
const isLoading = ref(false)
const fileInput = ref(null)
const messagesContainer = ref(null)
const selectedFile = ref(null)
const previewUrl = ref(null)
const textareaRef = ref(null)

// Session & Agent State
const sessions = ref([])
const agents = ref({})
const selectedSessionKey = ref('')
const selectedAgentId = ref('default')
const showSessionMenu = ref(false)

const GATEWAY_API = 'http://localhost:18790/v1'
const BACKEND_API = 'http://localhost:3000/api'

// --- Computed ---
const currentAgent = computed(() => agents.value[selectedAgentId.value] || {})
const sessionLabel = computed(() => {
  if (!selectedSessionKey.value) return 'New Chat'
  const parts = selectedSessionKey.value.split(':')
  if (parts.length >= 2) return parts.slice(1).join(':')
  return selectedSessionKey.value
})

// --- Helpers ---
const scrollToBottom = () => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

const autoResizeTextarea = () => {
  nextTick(() => {
    if (textareaRef.value) {
      textareaRef.value.style.height = 'auto'
      textareaRef.value.style.height = Math.min(textareaRef.value.scrollHeight, 160) + 'px'
    }
  })
}

// --- Fetch Data ---
const fetchAgents = async () => {
  try {
    const res = await axios.get(`${GATEWAY_API}/agents`)
    agents.value = res.data.agents || {}
    if (!agents.value[selectedAgentId.value]) {
      selectedAgentId.value = Object.keys(agents.value)[0] || 'default'
    }
  } catch (e) { console.error("Failed to fetch agents", e) }
}

const fetchSessions = async () => {
  try {
    const res = await axios.get(`${GATEWAY_API}/sessions`)
    sessions.value = (res.data.sessions || []).sort((a, b) => new Date(b.updated || 0) - new Date(a.updated || 0))
    if (!selectedSessionKey.value) {
      const def = sessions.value.find(s => s.key === 'web:default')
      selectedSessionKey.value = def ? 'web:default' : sessions.value[0]?.key || 'web:default'
    }
  } catch (e) { console.error("Failed to fetch sessions", e) }
}

const loadSessionHistory = async (sessionKey) => {
  if (!sessionKey) return
  try {
    const res = await axios.get(`${GATEWAY_API}/sessions/${sessionKey}`)
    const data = res.data
    if (data?.messages?.length > 0) {
      messages.value = data.messages.map((msg, i) => ({
        id: `h-${i}`, role: msg.role, content: msg.content || '',
        timestamp: data.updated ? new Date(data.updated) : new Date()
      }))
    } else {
      messages.value = [welcomeMessage()]
    }
    scrollToBottom()
  } catch (e) {
    messages.value = [welcomeMessage()]
  }
}

function welcomeMessage() {
  return {
    id: 'intro', role: 'assistant',
    content: `Hello! I'm **Pepebot** 🐸

I can help you with:
- 🤖 Android Automation
- 🌐 Web Tasks & Scraping
- 💻 Coding & Development
- 📊 Data Analysis

Select an agent and start chatting!`,
    timestamp: new Date()
  }
}

const createNewSession = async () => {
  const newKey = `web:${selectedAgentId.value}:${Date.now()}`
  try {
    await axios.post(`${GATEWAY_API}/sessions/${newKey}/new`)
    selectedSessionKey.value = newKey
    messages.value = [welcomeMessage()]
    await fetchSessions()
    showSessionMenu.value = false
  } catch (e) { console.error("Failed to create session", e) }
}

// --- Messaging ---
const sendMessage = async () => {
  if ((!newMessage.value.trim() && !selectedFile.value) || isLoading.value) return

  const userMsg = {
    id: Date.now(), role: 'user',
    content: newMessage.value, image: previewUrl.value,
    timestamp: new Date()
  }

  messages.value.push(userMsg)
  const userText = newMessage.value
  newMessage.value = ''
  selectedFile.value = null
  previewUrl.value = null
  isLoading.value = true
  autoResizeTextarea()
  scrollToBottom()

  const assistantMsg = ref({
    id: Date.now() + 1, role: 'assistant',
    content: '', isStreaming: true, timestamp: new Date()
  })
  messages.value.push(assistantMsg.value)

  try {
    let uploadedImageUrl = null
    if (userMsg.image && fileInput.value?.files[0]) {
      const formData = new FormData()
      formData.append('image', fileInput.value.files[0])
      const uploadRes = await axios.post(`${BACKEND_API}/upload`, formData)
      uploadedImageUrl = uploadRes.data.url
    }

    const payload = {
      model: currentAgent.value.model || "maia/gemini-3-pro-preview",
      messages: [{ role: "user", content: userText }],
      stream: true
    }

    if (uploadedImageUrl) {
      payload.messages[0].content = [
        { type: "text", text: userText },
        { type: "image_url", image_url: { url: uploadedImageUrl } }
      ]
    }

    const response = await fetch(`${GATEWAY_API}/chat/completions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Session-Key': selectedSessionKey.value,
        'X-Agent': selectedAgentId.value
      },
      body: JSON.stringify(payload)
    })

    if (!response.ok) throw new Error(response.statusText)

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop()

      for (const line of lines) {
        const t = line.trim()
        if (!t || t.startsWith(':')) continue
        if (t.startsWith('data:')) {
          const d = t.slice(5).trim()
          if (d === '[DONE]') continue
          try {
            const data = JSON.parse(d)
            const content = data.choices?.[0]?.delta?.content || ''
            if (content) {
              assistantMsg.value.content += content
              scrollToBottom()
            }
          } catch (e) { /* skip */ }
        }
      }
    }
  } catch (error) {
    console.error(error)
    assistantMsg.value.content += "\n\n*[Error: Failed to get response]*"
    assistantMsg.value.isError = true
  } finally {
    isLoading.value = false
    assistantMsg.value.isStreaming = false
    scrollToBottom()
    fetchSessions()
  }
}

const handleFileUpload = (e) => {
  const file = e.target.files[0]
  if (file) {
    selectedFile.value = file
    previewUrl.value = URL.createObjectURL(file)
  }
}

const triggerFileInput = () => fileInput.value.click()

function handleKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    sendMessage()
  }
}

function formatTime(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const now = new Date()
  const diff = now - d
  if (diff < 60000) return 'Now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h`
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' })
}

// --- Lifecycle ---
onMounted(async () => {
  await fetchAgents()
  await fetchSessions()
  if (selectedSessionKey.value) {
    await loadSessionHistory(selectedSessionKey.value)
  } else {
    messages.value = [welcomeMessage()]
  }
})

watch(selectedSessionKey, async (n, o) => { if (n && n !== o) await loadSessionHistory(n) })
watch(newMessage, autoResizeTextarea)
</script>

<template>
  <div class="flex flex-col h-full bg-[#0c0c0f]">

    <!-- Top Bar -->
    <div class="h-14 border-b border-white/5 flex items-center justify-between px-5 bg-[#0c0c0f]/90 backdrop-blur-xl z-10 flex-shrink-0">

      <!-- Agent Selector -->
      <div class="flex items-center gap-2">
        <div class="flex items-center gap-2.5 px-3 py-1.5 rounded-xl bg-white/[0.04] border border-white/[0.06] hover:bg-white/[0.07] transition-all cursor-pointer">
          <div class="w-6 h-6 rounded-lg bg-gradient-to-br from-green-500/30 to-emerald-600/30 flex items-center justify-center">
            <Bot :size="13" class="text-green-400" />
          </div>
          <select v-model="selectedAgentId" class="bg-transparent border-none outline-none text-sm font-medium text-gray-200 cursor-pointer appearance-none pr-3">
            <option v-for="(agent, id) in agents" :key="id" :value="id" class="bg-[#1a1a20]">
              {{ agent.name || id }}
            </option>
          </select>
        </div>
      </div>

      <!-- Session Selector -->
      <div class="relative">
        <button
          @click="showSessionMenu = !showSessionMenu"
          class="flex items-center gap-2 px-3 py-1.5 rounded-xl text-sm text-gray-400 hover:text-gray-200 hover:bg-white/[0.04] transition-all"
        >
          <MessageSquare :size="14" />
          <span class="max-w-[160px] truncate text-xs">{{ sessionLabel }}</span>
          <ChevronDown :size="12" class="text-gray-600" />
        </button>

        <!-- Dropdown -->
        <Transition name="dropdown">
          <div
            v-if="showSessionMenu"
            class="absolute right-0 top-full mt-2 w-72 bg-[#18181b] border border-white/[0.08] rounded-2xl shadow-2xl overflow-hidden z-50"
          >
            <!-- New Session Button -->
            <div class="p-2 border-b border-white/5">
              <button
                @click="createNewSession"
                class="w-full flex items-center gap-2 px-3 py-2.5 rounded-xl bg-green-500/10 text-green-400 hover:bg-green-500/20 text-sm font-medium transition-all"
              >
                <Plus :size="15" />
                <span>New Chat</span>
              </button>
            </div>

            <!-- Sessions List -->
            <div class="max-h-64 overflow-y-auto py-1">
              <button
                v-for="session in sessions"
                :key="session.key"
                @click="selectedSessionKey = session.key; showSessionMenu = false"
                class="w-full flex items-center gap-3 px-4 py-2.5 text-left hover:bg-white/[0.04] transition-colors"
                :class="session.key === selectedSessionKey ? 'bg-white/[0.04]' : ''"
              >
                <div class="w-7 h-7 rounded-lg flex items-center justify-center flex-shrink-0"
                  :class="session.key === selectedSessionKey ? 'bg-green-500/15 text-green-400' : 'bg-white/5 text-gray-500'"
                >
                  <MessageSquare :size="13" />
                </div>
                <div class="flex-1 min-w-0">
                  <p class="text-sm text-gray-300 truncate" :class="session.key === selectedSessionKey ? 'text-white font-medium' : ''">
                    {{ session.key }}
                  </p>
                  <p class="text-[10px] text-gray-600">{{ session.message_count || 0 }} msgs · {{ formatTime(session.updated) }}</p>
                </div>
              </button>
              <div v-if="sessions.length === 0" class="px-4 py-6 text-xs text-center text-gray-600">
                No sessions yet
              </div>
            </div>
          </div>
        </Transition>
      </div>
    </div>

    <!-- Click-away for dropdown -->
    <div v-if="showSessionMenu" @click="showSessionMenu = false" class="fixed inset-0 z-40" />

    <!-- Messages Area -->
    <div class="flex-1 overflow-hidden relative">
      <div ref="messagesContainer" class="h-full overflow-y-auto px-4 pb-40 pt-6 scroll-smooth">
        <div class="max-w-3xl mx-auto space-y-6">

          <div
            v-for="msg in messages"
            :key="msg.id"
            class="flex gap-3"
            :class="msg.role === 'user' ? 'justify-end' : 'justify-start'"
          >
            <!-- Bot Avatar -->
            <div v-if="msg.role === 'assistant'" class="w-8 h-8 rounded-xl bg-gradient-to-br from-green-500 to-emerald-600 flex items-center justify-center flex-shrink-0 mt-1 shadow-lg shadow-green-500/10">
              <span class="text-sm">🐸</span>
            </div>

            <!-- Message Content -->
            <div class="min-w-0" :class="msg.role === 'user' ? 'max-w-[75%]' : 'max-w-[85%]'">
              <!-- Label -->
              <div v-if="msg.role === 'assistant'" class="text-[10px] text-gray-600 mb-1 ml-1 font-medium">Pepebot</div>

              <div
                class="rounded-2xl text-sm leading-relaxed overflow-hidden"
                :class="[
                  msg.role === 'user'
                    ? 'bg-gradient-to-br from-blue-600/30 to-indigo-600/20 border border-blue-500/10 text-gray-100 px-4 py-3 rounded-br-md'
                    : 'text-gray-200',
                  msg.isError ? 'border border-red-500/20 bg-red-500/5 px-4 py-3' : ''
                ]"
              >
                <!-- Image Preview -->
                <img v-if="msg.image" :src="msg.image" class="max-w-xs rounded-xl mb-3 border border-white/10" alt="Upload" />

                <!-- Markdown (assistant) -->
                <div v-if="msg.role === 'assistant'" class="chat-markdown" v-html="md.render(msg.content || '')" />

                <!-- Plain text (user) -->
                <span v-else class="whitespace-pre-wrap break-words">{{ msg.content }}</span>

                <!-- Streaming cursor -->
                <span v-if="msg.isStreaming" class="inline-block w-1.5 h-4 bg-green-400 ml-0.5 animate-pulse rounded-sm align-middle" />
              </div>
            </div>

            <!-- User Avatar -->
            <div v-if="msg.role === 'user'" class="w-8 h-8 rounded-xl bg-gradient-to-br from-blue-500/20 to-indigo-500/20 border border-white/5 flex items-center justify-center flex-shrink-0 mt-1">
              <span class="text-[11px] font-bold text-blue-300">U</span>
            </div>
          </div>

          <!-- Thinking indicator -->
          <div v-if="isLoading && messages[messages.length - 1]?.content === ''" class="flex items-center gap-3 pl-11">
            <div class="flex gap-1">
              <span class="w-2 h-2 bg-green-400/60 rounded-full animate-bounce" style="animation-delay: 0ms" />
              <span class="w-2 h-2 bg-green-400/60 rounded-full animate-bounce" style="animation-delay: 150ms" />
              <span class="w-2 h-2 bg-green-400/60 rounded-full animate-bounce" style="animation-delay: 300ms" />
            </div>
            <span class="text-xs text-gray-600">Thinking...</span>
          </div>

          <div class="h-8" />
        </div>
      </div>
    </div>

    <!-- Input Area -->
    <div class="flex-shrink-0 px-4 pb-5 pt-2">
      <div class="max-w-3xl mx-auto">
        <!-- File Preview -->
        <div v-if="selectedFile" class="mb-2 flex items-center gap-2 bg-[#1a1a20] border border-white/5 rounded-xl px-3 py-2 w-fit">
          <img v-if="previewUrl" :src="previewUrl" class="w-10 h-10 rounded-lg object-cover" alt="preview" />
          <span class="text-xs text-gray-400 truncate max-w-[150px]">{{ selectedFile.name }}</span>
          <button @click="selectedFile = null; previewUrl = null" class="text-gray-500 hover:text-white transition-colors p-0.5">
            <X :size="14" />
          </button>
        </div>

        <!-- Input Box -->
        <div class="bg-[#1a1a20] border border-white/[0.06] rounded-2xl flex items-end gap-1 transition-all duration-300 focus-within:border-green-500/25 focus-within:shadow-[0_0_20px_rgba(34,197,94,0.06)]">
          <!-- Attach -->
          <button @click="triggerFileInput" class="p-3 text-gray-500 hover:text-gray-300 transition-colors flex-shrink-0 self-end">
            <Paperclip :size="18" />
          </button>
          <input type="file" ref="fileInput" @change="handleFileUpload" class="hidden" accept="image/*" />

          <!-- Textarea -->
          <textarea
            ref="textareaRef"
            v-model="newMessage"
            @keydown="handleKeydown"
            placeholder="Message Pepebot..."
            rows="1"
            class="flex-1 bg-transparent border-none outline-none text-gray-100 placeholder-gray-600 resize-none py-3 text-[15px] leading-relaxed min-h-[24px] max-h-[160px] focus:ring-0"
            :disabled="isLoading"
          />

          <!-- Send -->
          <button
            @click="sendMessage"
            :disabled="(!newMessage.trim() && !selectedFile) || isLoading"
            class="m-2 w-9 h-9 rounded-xl flex items-center justify-center transition-all flex-shrink-0"
            :class="newMessage.trim() || selectedFile
              ? 'bg-gradient-to-r from-green-500 to-emerald-600 text-white shadow-lg shadow-green-500/20 hover:shadow-green-500/30 hover:scale-105 active:scale-95'
              : 'bg-white/5 text-gray-600 cursor-not-allowed'"
          >
            <Loader2 v-if="isLoading" :size="16" class="animate-spin" />
            <Send v-else :size="15" />
          </button>
        </div>

        <p class="text-center text-[10px] text-gray-700 mt-2">
          Pepebot can make mistakes. Verify important information.
        </p>
      </div>
    </div>
  </div>
</template>

<style>
/* Dropdown transition */
.dropdown-enter-active { transition: all 0.15s ease-out; }
.dropdown-leave-active { transition: all 0.1s ease-in; }
.dropdown-enter-from { opacity: 0; transform: translateY(-4px) scale(0.98); }
.dropdown-leave-to { opacity: 0; transform: translateY(-4px) scale(0.98); }

/* Chat Markdown */
.chat-markdown {
  font-size: 0.925rem;
  line-height: 1.7;
  color: #d4d4d8;
  overflow-wrap: break-word;
  word-break: break-word;
}
.chat-markdown h1, .chat-markdown h2, .chat-markdown h3 {
  margin-top: 1.2em;
  margin-bottom: 0.4em;
  font-weight: 600;
  color: #f4f4f5;
}
.chat-markdown h1 { font-size: 1.25em; }
.chat-markdown h2 { font-size: 1.1em; }
.chat-markdown h3 { font-size: 1em; }
.chat-markdown p {
  margin-bottom: 0.75em;
}
.chat-markdown p:last-child { margin-bottom: 0; }
.chat-markdown ul, .chat-markdown ol {
  padding-left: 1.5em;
  margin-bottom: 0.75em;
}
.chat-markdown li {
  margin-bottom: 0.2em;
  list-style: disc;
}
.chat-markdown ol li { list-style: decimal; }
.chat-markdown pre {
  background: rgba(0,0,0,0.4);
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 12px;
  padding: 1em 1.2em;
  overflow-x: auto;
  margin: 0.75em 0;
  font-size: 0.85em;
  max-width: 100%;
}
.chat-markdown code {
  background: rgba(255,255,255,0.08);
  padding: 0.15em 0.4em;
  border-radius: 6px;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.88em;
  word-break: break-all;
}
.chat-markdown pre code {
  background: transparent;
  padding: 0;
  word-break: normal;
}
.chat-markdown strong {
  color: #f4f4f5;
  font-weight: 600;
}
.chat-markdown a {
  color: #60a5fa;
  text-decoration: underline;
  text-decoration-color: rgba(96,165,250,0.3);
}
.chat-markdown a:hover { text-decoration-color: rgba(96,165,250,0.7); }
.chat-markdown blockquote {
  border-left: 3px solid rgba(255,255,255,0.1);
  margin: 0.75em 0;
  padding-left: 1em;
  color: #a1a1aa;
}
.chat-markdown hr {
  border: none;
  border-top: 1px solid rgba(255,255,255,0.06);
  margin: 1em 0;
}
</style>
