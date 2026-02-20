<script setup>
import { ref, nextTick, watch, computed, onMounted, onUnmounted } from 'vue'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import { Send, X, Loader2, Trash2 } from 'lucide-vue-next'
import { getGatewayApiUrl } from '../lib/gateway.js'

const props = defineProps({
  context: { type: String, default: 'general' },
  contextData: { type: Object, default: () => ({}) }
})

// --- Markdown ---
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
const GATEWAY_API = getGatewayApiUrl()
const isOpen = ref(false)
const messages = ref([])
const newMessage = ref('')
const isLoading = ref(false)
const messagesContainer = ref(null)
const inputRef = ref(null)

// --- Session key per context ---
const sessionKey = computed(() => {
  const suffix = props.contextData?.skillName || props.context
  return `web:assistant:${suffix}`
})

// --- System prompt per context ---
const systemPrompt = computed(() => {
  switch (props.context) {
    case 'skills':
      return `You are Pepebot Assistant 🐸, helping the user manage skills in the Pepebot workspace.
The user is on the Skills page viewing all installed skills.
You can help them:
- Create new skills by generating SKILL.md files and scripts
- Explain what skills do and how to configure them
- Suggest useful skills to add

Skills are stored in ~/.pepebot/workspace/skills/{skill-name}/
Each skill needs at minimum a SKILL.md file with YAML frontmatter (name, description).
When creating a skill, provide the complete SKILL.md content and any necessary scripts.
Be concise and action-oriented.`

    case 'skill-editor':
      return `You are Pepebot Assistant 🐸, helping the user edit skill files.
The user is editing the skill "${props.contextData?.skillName || 'unknown'}".
${props.contextData?.selectedFile ? `Currently viewing file: ${props.contextData.selectedFile}` : ''}
You can help them:
- Edit or improve the current file
- Add new scripts (Python, Bash, JS) to the skill
- Explain how the skill code works
- Debug issues with the skill

Provide complete file contents when creating or editing files.
Be concise and provide working code.`

    case 'workflows':
      return `You are Pepebot Assistant 🐸, helping the user manage workflows.
The user is on the Workflows page. Workflows are JSON files in ~/.pepebot/workspace/workflows/.
You can help them:
- Create new workflow JSON files
- Explain existing workflows
- Add or modify workflow steps

Workflow JSON format:
{
  "name": "Workflow Name",
  "description": "What it does",
  "variables": { "key": "default_value" },
  "steps": [
    { "name": "step_name", "tool": "tool_name", "goal": "description" }
  ]
}
Be concise and provide complete valid JSON.`

    case 'agents':
      return `You are Pepebot Assistant 🐸, helping the user manage agents.
The user is on the Agents page. Agents are configured in ~/.pepebot/workspace/agents/registry.json.
You can help them:
- Create new agent configurations
- Explain agent settings (model, temperature, system prompt)
- Suggest optimal configurations for different use cases
- Enable or disable agents

Be concise and action-oriented.`

    default:
      return 'You are Pepebot Assistant 🐸. Help the user with their request.'
  }
})

// --- Helpers ---
const scrollToBottom = () => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

const toggle = () => {
  isOpen.value = !isOpen.value
  if (isOpen.value) {
    if (messages.value.length === 0) {
      loadHistory()
    }
    nextTick(() => inputRef.value?.focus())
  }
}

const clearChat = () => {
  messages.value = [{
    id: 'intro', role: 'assistant',
    content: getWelcomeMessage()
  }]
}

function getWelcomeMessage() {
  switch (props.context) {
    case 'skills':
      return "🐸 Halo! Mau bikin skill baru atau ada yang perlu dibantu soal skills?"
    case 'skill-editor':
      return `🐸 Halo! Saya bisa bantu edit skill **${props.contextData?.skillName || ''}** — mau tambah script atau edit file?`
    case 'workflows':
      return "🐸 Halo! Mau bikin workflow baru atau perlu bantuan soal workflow yang ada?"
    case 'agents':
      return "🐸 Halo! Mau bikin agent baru atau atur konfigurasi agent yang ada?"
    default:
      return "🐸 Halo! Ada yang bisa saya bantu?"
  }
}

// --- History ---
async function loadHistory() {
  try {
    const res = await fetch(`${GATEWAY_API}/sessions/${sessionKey.value}`)
    if (res.ok) {
      const data = await res.json()
      if (data?.messages?.length > 0) {
        messages.value = data.messages.map((msg, i) => ({
          id: `h-${i}`, role: msg.role, content: msg.content || ''
        }))
        scrollToBottom()
        return
      }
    }
  } catch (e) { /* ignore */ }
  // Default welcome
  messages.value = [{ id: 'intro', role: 'assistant', content: getWelcomeMessage() }]
}

// --- Send Message ---
async function sendMessage() {
  if (!newMessage.value.trim() || isLoading.value) return

  const userText = newMessage.value.trim()
  messages.value.push({ id: Date.now(), role: 'user', content: userText })
  newMessage.value = ''
  isLoading.value = true
  scrollToBottom()

  // assistant placeholder
  const assistantMsg = { id: Date.now() + 1, role: 'assistant', content: '', isStreaming: true }
  messages.value.push(assistantMsg)

  try {
    const payload = {
      model: "maia/gemini-3-pro-preview",
      messages: [
        { role: "system", content: systemPrompt.value },
        { role: "user", content: userText }
      ],
      stream: true
    }

    const response = await fetch(`${GATEWAY_API}/chat/completions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Session-Key': sessionKey.value,
        'X-Agent': 'default'
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
              assistantMsg.content += content
              scrollToBottom()
            }
          } catch (e) { /* skip */ }
        }
      }
    }
  } catch (error) {
    assistantMsg.content += '\n\n*[Error: gagal mendapat respons]*'
  } finally {
    isLoading.value = false
    assistantMsg.isStreaming = false
    scrollToBottom()
  }
}

// --- Keyboard ---
function handleKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    sendMessage()
  }
}
</script>

<template>
  <!-- FAB Button (hidden when panel is open) -->
  <button
    v-if="!isOpen"
    @click="toggle"
    class="fixed bottom-6 right-6 z-50 w-14 h-14 rounded-full shadow-2xl flex items-center justify-center transition-all duration-300 hover:scale-110 bg-gradient-to-br from-green-400 to-emerald-600 hover:from-green-300 hover:to-emerald-500"
    style="box-shadow: 0 4px 20px rgba(16, 185, 129, 0.4)"
  >
    <span class="text-2xl">🐸</span>
  </button>

  <!-- Chat Panel -->
  <Transition name="slide">
    <div
      v-if="isOpen"
      class="fixed top-0 right-0 z-40 h-full w-[380px] bg-[#16161a] border-l border-white/5 flex flex-col shadow-2xl"
    >
      <!-- Header -->
      <div class="h-14 px-4 flex items-center gap-3 border-b border-white/5 bg-[#1e1e24] flex-shrink-0">
        <div class="w-9 h-9 rounded-xl bg-gradient-to-br from-green-400/20 to-emerald-500/20 flex items-center justify-center text-lg">
          🐸
        </div>
        <div class="flex-1 min-w-0">
          <h3 class="text-sm font-semibold text-white truncate">Pepe Assistant</h3>
          <p class="text-[10px] text-gray-500 truncate">{{ context === 'skill-editor' ? contextData?.skillName : context }}</p>
        </div>
        <button @click="clearChat" class="p-1.5 rounded-lg text-gray-500 hover:text-red-400 hover:bg-white/5 transition-colors" title="Clear chat">
          <Trash2 :size="14" />
        </button>
        <button @click="toggle" class="p-1.5 rounded-lg text-gray-500 hover:text-white hover:bg-white/5 transition-colors" title="Close">
          <X :size="16" />
        </button>
      </div>

      <!-- Messages -->
      <div ref="messagesContainer" class="flex-1 overflow-y-auto p-4 space-y-4 scroll-smooth">
        <div
          v-for="msg in messages"
          :key="msg.id"
          class="flex"
          :class="msg.role === 'user' ? 'justify-end' : 'justify-start'"
        >
          <!-- Avatar (assistant only) -->
          <div v-if="msg.role === 'assistant'" class="w-7 h-7 rounded-lg bg-gradient-to-br from-green-500/20 to-emerald-500/20 flex items-center justify-center text-sm flex-shrink-0 mr-2 mt-0.5">
            🐸
          </div>

          <!-- Bubble -->
          <div
            class="max-w-[85%] min-w-0 rounded-2xl px-3.5 py-2.5 text-sm leading-relaxed overflow-hidden"
            :class="msg.role === 'user'
              ? 'bg-blue-500/20 text-blue-50 rounded-br-md'
              : 'bg-white/[0.06] text-gray-200 rounded-bl-md'"
          >
            <div
              v-if="msg.role === 'assistant'"
              v-html="md.render(msg.content || '')"
              class="agent-chat-content prose prose-invert prose-sm break-words overflow-hidden"
            />
            <span v-else class="break-words whitespace-pre-wrap">{{ msg.content }}</span>

            <!-- Streaming cursor -->
            <span v-if="msg.isStreaming" class="inline-block w-1.5 h-4 bg-green-400 ml-0.5 animate-pulse rounded-sm" />
          </div>
        </div>

        <!-- Loading -->
        <div v-if="isLoading && messages[messages.length - 1]?.content === ''" class="flex items-center gap-2 text-gray-500 text-xs pl-9">
          <Loader2 :size="14" class="animate-spin" />
          <span>Thinking...</span>
        </div>
      </div>

      <!-- Input -->
      <div class="p-3 border-t border-white/5 bg-[#1e1e24] flex-shrink-0">
        <div class="flex items-end gap-2">
          <textarea
            ref="inputRef"
            v-model="newMessage"
            @keydown="handleKeydown"
            placeholder="Tanya Pepe..."
            rows="1"
            class="flex-1 bg-white/5 border border-white/10 rounded-xl px-3.5 py-2.5 text-sm text-white placeholder-gray-500 resize-none focus:outline-none focus:border-green-500/50 focus:ring-1 focus:ring-green-500/20 transition-all max-h-28 overflow-y-auto"
            :disabled="isLoading"
          />
          <button
            @click="sendMessage"
            :disabled="!newMessage.trim() || isLoading"
            class="w-10 h-10 rounded-xl flex items-center justify-center transition-all flex-shrink-0"
            :class="newMessage.trim() && !isLoading
              ? 'bg-gradient-to-r from-green-500 to-emerald-600 text-white hover:opacity-90'
              : 'bg-white/5 text-gray-600 cursor-not-allowed'"
          >
            <Loader2 v-if="isLoading" :size="16" class="animate-spin" />
            <Send v-else :size="16" />
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style>
/* Slide transition */
.slide-enter-active,
.slide-leave-active {
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(100%);
}

/* Agent chat markdown styling */
.agent-chat-content { overflow-wrap: break-word; word-break: break-word; }
.agent-chat-content p { margin: 0.25em 0; }
.agent-chat-content p:first-child { margin-top: 0; }
.agent-chat-content p:last-child { margin-bottom: 0; }
.agent-chat-content ul, .agent-chat-content ol { margin: 0.25em 0; padding-left: 1.2em; }
.agent-chat-content li { margin: 0.1em 0; }
.agent-chat-content code {
  background: rgba(255,255,255,0.08);
  padding: 0.15em 0.35em;
  border-radius: 4px;
  font-size: 0.8em;
  word-break: break-all;
}
.agent-chat-content pre.hljs {
  margin: 0.5em 0;
  padding: 0.6em 0.8em;
  border-radius: 8px;
  font-size: 0.75em;
  overflow-x: auto;
  background: rgba(0,0,0,0.3) !important;
  max-width: 100%;
}
.agent-chat-content pre.hljs code {
  background: none;
  padding: 0;
  word-break: normal;
}
.agent-chat-content h1, .agent-chat-content h2, .agent-chat-content h3 {
  font-weight: 600;
  margin: 0.5em 0 0.2em;
}
.agent-chat-content h1 { font-size: 1.1em; }
.agent-chat-content h2 { font-size: 1em; }
.agent-chat-content h3 { font-size: 0.95em; }
.agent-chat-content strong { color: #e2e8f0; }
.agent-chat-content a { color: #60a5fa; text-decoration: underline; word-break: break-all; }
.agent-chat-content blockquote {
  border-left: 3px solid rgba(255,255,255,0.1);
  margin: 0.4em 0;
  padding-left: 0.6em;
  color: #9ca3af;
}
</style>
