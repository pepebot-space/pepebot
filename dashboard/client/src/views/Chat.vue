<script setup>
import { ref, onMounted, nextTick, watch } from 'vue'
import axios from 'axios'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import 'highlight.js/styles/github-dark.css' // Import a dark theme for code blocks
import { Send, Image as ImageIcon, Paperclip, Loader2, Bot, MessageSquare, Plus, Trash2, ArrowRight } from 'lucide-vue-next'

// --- Markdown Setup ---
const md = new MarkdownIt({
  html: true,
  linkify: true,
  typographer: true,
  highlight: function (str, lang) {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return '<pre class="hljs"><code>' +
               hljs.highlight(str, { language: lang, ignoreIllegals: true }).value +
               '</code></pre>';
      } catch (__) {}
    }
    return '<pre class="hljs"><code>' + md.utils.escapeHtml(str) + '</code></pre>';
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

// Session & Agent State
const sessions = ref([])
const agents = ref({}) // Object from registry
const selectedSessionKey = ref('')
const selectedAgentId = ref('default')
const showSessionMenu = ref(false)

// --- API Base URLs --
const GATEWAY_API = 'http://localhost:18790/v1'
const BACKEND_API = 'http://localhost:3000/api'

// --- Helpers ---
const scrollToBottom = () => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

// --- Fetch Data ---
const fetchAgents = async () => {
    try {
        const res = await axios.get(`${GATEWAY_API}/agents`)
        agents.value = res.data.agents || {}
        // Default to first available if not set
        if (!agents.value[selectedAgentId.value]) {
            selectedAgentId.value = Object.keys(agents.value)[0] || 'default'
        }
    } catch (e) {
        console.error("Failed to fetch agents", e)
    }
}

const fetchSessions = async () => {
    try {
        const res = await axios.get(`${GATEWAY_API}/sessions`)
        sessions.value = res.data.sessions || []
        
        // If no session selected, prioritize web:default
        if (!selectedSessionKey.value) {
             const defaultSession = sessions.value.find(s => s.key === 'web:default')
             if (defaultSession) {
                 selectedSessionKey.value = 'web:default'
             } else if (sessions.value.length > 0) {
                 selectedSessionKey.value = sessions.value[0].key
             } else {
                 selectedSessionKey.value = 'web:default'
             }
        }
    } catch (e) {
        console.error("Failed to fetch sessions", e)
    }
}

const loadSessionHistory = async (sessionKey) => {
    if (!sessionKey) return
    try {
        const res = await axios.get(`${GATEWAY_API}/sessions/${sessionKey}`)
        const data = res.data
        if (data && data.messages && data.messages.length > 0) {
            messages.value = data.messages.map((msg, i) => ({
                id: `history-${i}`,
                role: msg.role,
                content: msg.content || '',
                timestamp: data.updated ? new Date(data.updated) : new Date()
            }))
        } else {
            // No history, show welcome
            messages.value = [{
                id: 'intro',
                role: 'assistant',
                content: 'Hello! I am **Pepebot** 🐸. \n\nI can help you with:\n- Android Automation\n- Web Tasks\n- Coding & more.\n\nSelect an agent and start chatting!',
                timestamp: new Date()
            }]
        }
        scrollToBottom()
    } catch (e) {
        console.error("Failed to load session history", e)
        // On error (e.g. session doesn't exist yet), show welcome
        messages.value = [{
            id: 'intro',
            role: 'assistant',
            content: 'Hello! I am **Pepebot** 🐸. \n\nI can help you with:\n- Android Automation\n- Web Tasks\n- Coding & more.\n\nSelect an agent and start chatting!',
            timestamp: new Date()
        }]
    }
}

const createNewSession = async () => {
    const newKey = `web:${selectedAgentId.value}:${Date.now()}`
    try {
        await axios.post(`${GATEWAY_API}/sessions/${newKey}/new`)
        selectedSessionKey.value = newKey
        messages.value = [] // Clear local chat
        await fetchSessions() // Refresh list
        showSessionMenu.value = false
    } catch (e) {
        console.error("Failed to create session", e)
    }
}

// --- Messaging ---
const sendMessage = async () => {
  if ((!newMessage.value.trim() && !selectedFile.value) || isLoading.value) return

  const userMsg = {
    id: Date.now(),
    role: 'user',
    content: newMessage.value,
    image: previewUrl.value,
    timestamp: new Date()
  }
  
  messages.value.push(userMsg)
  const userText = newMessage.value
  newMessage.value = ''
  selectedFile.value = null
  previewUrl.value = null
  isLoading.value = true
  scrollToBottom()

  // Create placeholder for assistant response
  const assistantMsgId = Date.now() + 1
  const assistantMsg = ref({
      id: assistantMsgId,
      role: 'assistant',
      content: '', // Start empty
      isStreaming: true,
      timestamp: new Date()
  })
  messages.value.push(assistantMsg.value)

  try {
    let uploadedImageUrl = null;
    
    // Upload image if selected (Still uses Backend API)
    if (userMsg.image && fileInput.value && fileInput.value.files[0]) {
        const formData = new FormData();
        formData.append('image', fileInput.value.files[0]);
        const uploadRes = await axios.post(`${BACKEND_API}/upload`, formData);
        uploadedImageUrl = uploadRes.data.url;
    }

    // Prepare payload for Gateway
    const payload = {
        model: agents.value[selectedAgentId.value]?.model || "maia/gemini-3-pro-preview",
        messages: [
            { role: "user", content: userText }
        ],
        stream: true
    }

    // Add image if present (OpenAI Vision format)
    if (uploadedImageUrl) {
        payload.messages[0].content = [
            { type: "text", text: userText },
            { type: "image_url", image_url: { url: uploadedImageUrl } }
        ]
    }

    // Use fetch for Streaming directly to Gateway
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
        
        // Keep the last line in the buffer as it might be incomplete
        buffer = lines.pop()

        for (const line of lines) {
            const trimmedLine = line.trim()
            if (!trimmedLine || trimmedLine === ':' || trimmedLine.startsWith(':')) continue // Ignore comments/empty

            if (trimmedLine.startsWith('data:')) {
                const dataStr = trimmedLine.slice(5).trim()
                if (dataStr === '[DONE]') continue
                
                try {
                    const data = JSON.parse(dataStr)
                    const content = data.choices?.[0]?.delta?.content || ''
                    if (content) {
                        assistantMsg.value.content += content
                        scrollToBottom()
                    }
                } catch (e) {
                    console.warn("Failed to parse SSE JSON:", dataStr)
                }
            }
        }
    }
  } catch (error) {
    console.error(error)
    assistantMsg.value.content += "\n\n*[Error: Failed to get response from server]*"
    assistantMsg.value.isError = true
  } finally {
    isLoading.value = false
    assistantMsg.value.isStreaming = false
    scrollToBottom()
    fetchSessions() // Update session list (message count)
  }
}

const handleFileUpload = (event) => {
  const file = event.target.files[0]
  if (file) {
    selectedFile.value = file
    previewUrl.value = URL.createObjectURL(file)
  }
}

const triggerFileInput = () => {
  fileInput.value.click()
}

// --- Lifecycle ---
onMounted(async () => {
    await fetchAgents()
    await fetchSessions()
    
    // Load history for the selected session
    if (selectedSessionKey.value) {
        await loadSessionHistory(selectedSessionKey.value)
    } else {
        messages.value.push({
            id: 'intro',
            role: 'assistant',
            content: 'Hello! I am **Pepebot** 🐸. \n\nI can help you with:\n- Android Automation\n- Web Tasks\n- Coding & more.\n\nSelect an agent and start chatting!',
            timestamp: new Date()
        })
    }
})

// Watch for session changes to load history
watch(selectedSessionKey, async (newKey, oldKey) => {
    if (newKey && newKey !== oldKey) {
        await loadSessionHistory(newKey)
    }
})

// Watch for agent changes to update session key default
watch(selectedAgentId, (newId) => {
    if (selectedSessionKey.value.startsWith('web:') && selectedSessionKey.value.split(':').length === 2) {
         selectedSessionKey.value = `web:${newId}`
    }
})

</script>

<template>
  <div class="flex flex-col h-full relative bg-[#09090b]">
    
    <!-- Top Bar: Session & Agent Controls -->
    <div class="h-14 border-b border-white/5 flex items-center justify-between px-4 md:px-6 bg-[#09090b]/80 backdrop-blur-md z-10">
        
        <!-- Left: Agent Selector -->
        <div class="flex items-center gap-3">
            <div class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-white/5 border border-white/5 hover:bg-white/10 transition-colors cursor-pointer group">
                <Bot :size="16" class="text-green-400" />
                <select v-model="selectedAgentId" class="bg-transparent border-none outline-none text-sm font-medium text-gray-200 cursor-pointer appearance-none pr-4">
                    <option v-for="(agent, id) in agents" :key="id" :value="id">
                        {{ agent.name || id }}
                    </option>
                </select>
            </div>
        </div>

        <!-- Right: Session Selector -->
        <div class="relative">
             <button @click="showSessionMenu = !showSessionMenu" class="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium text-gray-300 hover:bg-white/5 transition-colors">
                <MessageSquare :size="16" />
                <span class="max-w-[100px] truncate">{{ selectedSessionKey || 'New Session' }}</span>
                <span class="text-xs text-gray-500">▼</span>
             </button>

             <!-- Dropdown -->
             <div v-if="showSessionMenu" class="absolute right-0 top-full mt-2 w-64 bg-[#18181b] border border-white/10 rounded-xl shadow-2xl py-1 z-50 animate-in fade-in slide-in-from-top-2">
                 <div class="px-2 py-1.5 border-b border-white/5 mb-1">
                     <button @click="createNewSession" class="w-full flex items-center gap-2 px-3 py-2 rounded-lg bg-green-500/10 text-green-400 hover:bg-green-500/20 text-sm font-medium transition-colors">
                         <Plus :size="14" />
                         <span>New Session</span>
                     </button>
                 </div>
                 <div class="max-h-60 overflow-y-auto">
                     <button 
                        v-for="session in sessions" 
                        :key="session.key"
                        @click="selectedSessionKey = session.key; showSessionMenu = false"
                        class="w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-white/5 hover:text-white transition-colors truncate"
                        :class="session.key === selectedSessionKey ? 'bg-white/5 text-white' : ''"
                     >
                        {{ session.key }}
                        <span class="block text-[10px] text-gray-500">{{ new Date(session.updated || Date.now()).toLocaleTimeString() }} • {{ session.message_count }} msgs</span>
                     </button>
                      <div v-if="sessions.length === 0" class="px-4 py-3 text-xs text-center text-gray-500">
                         No active sessions
                     </div>
                 </div>
             </div>
        </div>
    </div>


    <!-- Chat Area -->
    <div class="flex-1 overflow-hidden relative">
        <div ref="messagesContainer" class="h-full overflow-y-auto px-4 md:px-6 pb-32 pt-6 scroll-smooth">
            <div class="max-w-3xl mx-auto space-y-8">
                 <div 
                    v-for="msg in messages" 
                    :key="msg.id" 
                    :class="['flex gap-4 group', msg.role === 'user' ? 'justify-end' : 'justify-start']"
                 >
                    
                    <!-- Assistant Avatar -->
                    <div v-if="msg.role === 'assistant'" class="w-8 h-8 rounded-lg bg-gradient-to-br from-green-500 to-emerald-700 flex items-center justify-center shadow-lg flex-shrink-0 mt-1">
                         <span class="font-bold text-black text-xs">P</span>
                    </div>

                    <!-- Message Bubble -->
                    <div class="max-w-[85%] relative">
                        <!-- User Name / Time -->
                        <div v-if="msg.role === 'assistant'" class="text-[10px] text-gray-500 mb-1 ml-1">Pepebot</div>

                        <div 
                            class="rounded-2xl px-5 py-3 shadow-sm text-sm leading-relaxed"
                            :class="[
                                msg.role === 'user' 
                                    ? 'bg-[#27272a] text-gray-100 rounded-tr-sm' 
                                    : 'bg-transparent text-gray-200 pl-0 pt-0', // Minimalist for bot
                                msg.isError ? 'border border-red-500/20 bg-red-500/5' : ''
                            ]"
                        >
                            <!-- Image Preview -->
                            <img v-if="msg.image" :src="msg.image" class="max-w-xs rounded-lg mb-3 border border-white/10" alt="Uploaded image" />
                            
                            <!-- Markdown Content -->
                            <div v-if="msg.role === 'assistant'" class="markdown-body" v-html="md.render(msg.content)"></div>
                            <div v-else class="whitespace-pre-wrap">{{ msg.content }}</div>

                             <!-- Streaming Cursor -->
                            <span v-if="msg.isStreaming" class="inline-block w-1.5 h-4 ml-1 bg-green-500 animate-pulse align-middle"></span>
                        </div>
                    </div>

                    <!-- User Avatar -->
                    <div v-if="msg.role === 'user'" class="w-8 h-8 rounded-lg bg-[#27272a] border border-white/5 flex items-center justify-center shadow-lg flex-shrink-0 mt-1">
                         <span class="font-bold text-gray-400 text-xs">U</span>
                    </div>

                 </div>

                 <!-- Footer Spacer -->
                 <div class="h-8"></div>
            </div>
        </div>
    </div>

    <!-- Input Area (Floating) -->
    <div class="absolute bottom-6 left-0 right-0 px-4 md:px-6 flex justify-center z-20">
         <div class="w-full max-w-3xl bg-[#18181b]/90 backdrop-blur-xl border border-white/10 rounded-2xl p-2 shadow-2xl ring-1 ring-black/20 flex items-end gap-2 transition-all focus-within:ring-green-500/20 focus-within:border-green-500/30">
            
            <!-- Attach Button -->
            <button @click="triggerFileInput" class="p-3 text-gray-400 hover:text-gray-200 hover:bg-white/5 rounded-xl transition-colors shrink-0">
                <Paperclip :size="20" />
            </button>
            <input type="file" ref="fileInput" @change="handleFileUpload" class="hidden" accept="image/*" />

            <!-- Input Field -->
            <div class="flex-1 py-3 min-w-0">
                <div v-if="selectedFile" class="flex items-center gap-2 mb-2 bg-[#27272a] w-fit px-3 py-1 rounded-full border border-white/5">
                        <span class="text-xs text-gray-300 truncate max-w-[120px]">{{ selectedFile.name }}</span>
                        <button @click="selectedFile = null; previewUrl = null" class="text-gray-500 hover:text-white ml-1">×</button>
                </div>
                <textarea 
                    v-model="newMessage" 
                    @keydown.enter.prevent="sendMessage"
                    placeholder="Message Pepebot..." 
                    class="w-full bg-transparent border-none focus:ring-0 text-gray-100 placeholder-gray-500 resize-none h-6 py-0 max-h-48 text-base"
                    rows="1"
                    style="min-height: 24px;"
                ></textarea>
            </div>

            <!-- Send Button -->
             <button 
                @click="sendMessage"
                :disabled="(!newMessage.trim() && !selectedFile) || isLoading"
                class="p-3 bg-white text-black rounded-xl hover:bg-gray-200 disabled:opacity-30 disabled:cursor-not-allowed transition-all shrink-0 active:scale-95"
            >
                <Loader2 :size="20" class="animate-spin" v-if="isLoading" />
                <ArrowRight :size="20" v-else />
            </button>

         </div>
    </div>

  </div>
</template>

<style>
/* Markdown Styles */
.markdown-body {
    font-size: 0.95rem;
    line-height: 1.6;
    color: #e4e4e7;
}
.markdown-body h1, .markdown-body h2, .markdown-body h3 {
    margin-top: 1.5em;
    margin-bottom: 0.5em;
    font-weight: 600;
    color: #fff;
}
.markdown-body p {
    margin-bottom: 1em;
}
.markdown-body ul, .markdown-body ol {
    padding-left: 1.5em;
    margin-bottom: 1em;
}
.markdown-body li {
    margin-bottom: 0.25em;
    list-style: disc;
}
.markdown-body pre {
    background: #18181b;
    border: 1px solid rgba(255,255,255,0.1);
    border-radius: 0.5rem;
    padding: 1rem;
    overflow-x: auto;
    margin: 1em 0;
}
.markdown-body code {
    background: rgba(255,255,255,0.1);
    padding: 0.2em 0.4em;
    border-radius: 0.3em;
    font-family: monospace;
    font-size: 0.9em;
}
.markdown-body pre code {
    background: transparent;
    padding: 0;
}
.markdown-body strong {
    color: #fff;
    font-weight: 600;
}
</style>
