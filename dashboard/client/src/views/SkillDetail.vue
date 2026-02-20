<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { Codemirror } from 'vue-codemirror'
import { oneDark } from '@codemirror/theme-one-dark'
import { python } from '@codemirror/lang-python'
import { javascript } from '@codemirror/lang-javascript'
import { markdown } from '@codemirror/lang-markdown'
import { json as jsonLang } from '@codemirror/lang-json'
import { ArrowLeft, File, Folder, FolderOpen, Activity, FileText, Code, Terminal, Save, Check, AlertTriangle } from 'lucide-vue-next'
import AgentChat from '../components/AgentChat.vue'
import { getGatewayApiUrl } from '../lib/gateway.js'

const GATEWAY_API = getGatewayApiUrl()
const route = useRoute()
const router = useRouter()
const skillName = computed(() => route.params.name)
const files = ref([])
const isLoading = ref(true)
const selectedFile = ref(null)
const fileContent = ref('')
const originalContent = ref('')
const isFileLoading = ref(false)
const isSaving = ref(false)
const saveMessage = ref('')
const saveError = ref('')

const isDirty = computed(() => fileContent.value !== originalContent.value)

onMounted(async () => {
    await loadFiles()
})

async function loadFiles() {
    isLoading.value = true
    try {
        const response = await axios.get(`${GATEWAY_API}/skills/${skillName.value}`)
        files.value = response.data.files || []
        // Auto-select SKILL.md if available
        const skillMd = files.value.find(f => f.path === 'SKILL.md')
        if (skillMd) {
            await selectFile(skillMd)
        }
    } catch (e) {
        console.error('Failed to load skill files:', e)
    } finally {
        isLoading.value = false
    }
}

async function selectFile(file) {
    if (file.is_dir) return
    selectedFile.value = file
    isFileLoading.value = true
    try {
        const response = await axios.get(`${GATEWAY_API}/skills/${skillName.value}/${file.path}`, {
            responseType: 'text',
            transformResponse: [data => data] // prevent auto JSON parse
        })
        fileContent.value = response.data
        originalContent.value = response.data
    } catch (e) {
        fileContent.value = `// Error loading file: ${e.message}`
        originalContent.value = fileContent.value
    } finally {
        isFileLoading.value = false
    }
}

async function saveFile() {
    if (!selectedFile.value || !isDirty.value) return
    isSaving.value = true
    saveMessage.value = ''
    saveError.value = ''
    try {
        await axios.post(
            `${GATEWAY_API}/skills/${skillName.value}/${selectedFile.value.path}`,
            fileContent.value,
            { headers: { 'Content-Type': 'text/plain' } }
        )
        originalContent.value = fileContent.value
        saveMessage.value = 'Saved!'
        setTimeout(() => saveMessage.value = '', 3000)
    } catch (e) {
        saveError.value = e.response?.data?.error?.message || 'Save failed'
        setTimeout(() => saveError.value = '', 5000)
    } finally {
        isSaving.value = false
    }
}

// Ctrl+S keyboard shortcut
function handleKeydown(e) {
    if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault()
        saveFile()
    }
}
onMounted(() => window.addEventListener('keydown', handleKeydown))
import { onUnmounted } from 'vue'
onUnmounted(() => window.removeEventListener('keydown', handleKeydown))

// Build a tree structure from flat file list
const fileTree = computed(() => {
    const tree = []
    const dirs = {}

    // Sort: dirs first, then files, alphabetically
    const sorted = [...files.value].sort((a, b) => {
        if (a.is_dir && !b.is_dir) return -1
        if (!a.is_dir && b.is_dir) return 1
        return a.path.localeCompare(b.path)
    })

    for (const file of sorted) {
        const parts = file.path.split('/')
        if (parts.length === 1) {
            // Root level
            tree.push(file)
        } else {
            // Nested file - group under parent dir
            const dirName = parts[0]
            if (!dirs[dirName]) {
                dirs[dirName] = []
            }
            dirs[dirName].push({
                ...file,
                name: parts.slice(1).join('/'),
            })
        }
    }
    return tree
})

// Group files by directory
const groupedFiles = computed(() => {
    const groups = { '': [] }
    for (const file of files.value) {
        const parts = file.path.split('/')
        if (parts.length === 1) {
            groups[''].push(file)
        } else {
            const dir = parts.slice(0, -1).join('/')
            if (!groups[dir]) groups[dir] = []
            groups[dir].push(file)
        }
    }
    return groups
})

// Get CodeMirror extensions based on file type
function getLanguageExtension(filename) {
    const ext = filename?.split('.').pop()?.toLowerCase()
    switch(ext) {
        case 'py': return [python()]
        case 'js': case 'mjs': case 'ts': return [javascript()]
        case 'json': return [jsonLang()]
        case 'md': return [markdown()]
        default: return []
    }
}

const extensions = computed(() => {
    const lang = getLanguageExtension(selectedFile.value?.name)
    return [oneDark, ...lang]
})

// File icon helper
function getFileIcon(file) {
    if (file.is_dir) return Folder
    const ext = file.name.split('.').pop()?.toLowerCase()
    switch(ext) {
        case 'py': case 'js': case 'ts': case 'mjs': return Code
        case 'sh': case 'bash': return Terminal
        case 'md': return FileText
        default: return File
    }
}

function getFileColor(file) {
    if (file.is_dir) return 'text-yellow-400'
    const ext = file.name.split('.').pop()?.toLowerCase()
    switch(ext) {
        case 'py': return 'text-blue-400'
        case 'js': case 'mjs': return 'text-yellow-300'
        case 'json': return 'text-green-400'
        case 'md': return 'text-gray-300'
        case 'sh': case 'bash': return 'text-green-300'
        default: return 'text-gray-400'
    }
}

function formatSize(bytes) {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

const expandedDirs = ref(new Set())
function toggleDir(dir) {
    if (expandedDirs.value.has(dir)) {
        expandedDirs.value.delete(dir)
    } else {
        expandedDirs.value.add(dir)
    }
}
</script>

<template>
  <div class="flex h-full">
    <!-- File Sidebar -->
    <div class="w-64 border-r border-white/5 bg-[#16161a] flex flex-col flex-shrink-0">
      <!-- Header -->
      <div class="p-4 border-b border-white/5">
        <button @click="router.push('/skills')" class="flex items-center gap-2 text-gray-400 hover:text-white transition-colors text-sm mb-3">
          <ArrowLeft :size="14" />
          <span>Back to Skills</span>
        </button>
        <h2 class="text-base font-semibold truncate">{{ skillName }}</h2>
        <p class="text-xs text-gray-500 mt-0.5">{{ files.filter(f => !f.is_dir).length }} files</p>
      </div>

      <!-- Loading -->
      <div v-if="isLoading" class="flex items-center justify-center flex-1">
        <Activity :size="20" class="animate-spin text-gray-500" />
      </div>

      <!-- File Tree -->
      <div v-else class="flex-1 overflow-y-auto py-2">
        <!-- Root files -->
        <template v-for="(dirFiles, dir) in groupedFiles" :key="dir">
          <!-- Directory header -->
          <div v-if="dir !== ''" 
            class="flex items-center gap-2 px-4 py-1.5 text-xs text-yellow-400/80 cursor-pointer hover:bg-white/5"
            @click="toggleDir(dir)">
            <component :is="expandedDirs.has(dir) ? FolderOpen : Folder" :size="14" />
            <span class="font-medium">{{ dir }}</span>
          </div>

          <!-- Files in this group -->
          <template v-if="dir === '' || expandedDirs.has(dir)">
            <div 
              v-for="file in dirFiles" 
              :key="file.path"
              v-show="!file.is_dir"
              @click="selectFile(file)"
              class="flex items-center gap-2 px-4 py-1.5 text-sm cursor-pointer transition-colors group"
              :class="[
                selectedFile?.path === file.path
                  ? 'bg-blue-500/10 text-white border-r-2 border-blue-400'
                  : 'text-gray-400 hover:bg-white/5 hover:text-gray-200',
                dir !== '' ? 'pl-8' : ''
              ]"
            >
              <component :is="getFileIcon(file)" :size="14" :class="getFileColor(file)" />
              <span class="truncate flex-1">{{ file.name }}</span>
              <span class="text-[10px] text-gray-600 group-hover:text-gray-500">{{ formatSize(file.size) }}</span>
            </div>
          </template>
        </template>
      </div>
    </div>

    <!-- Editor Panel -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Editor Tab Bar -->
      <div v-if="selectedFile" class="h-10 bg-[#1e1e24] border-b border-white/5 flex items-center px-4 gap-2 flex-shrink-0">
        <component :is="getFileIcon(selectedFile)" :size="14" :class="getFileColor(selectedFile)" />
        <span class="text-sm text-gray-300">{{ selectedFile.path }}</span>
        <span v-if="isDirty" class="w-2 h-2 rounded-full bg-orange-400" title="Unsaved changes"></span>
        <span class="text-[10px] text-gray-600 ml-1">{{ formatSize(selectedFile.size) }}</span>
        <div class="ml-auto flex items-center gap-2">
          <span v-if="saveMessage" class="text-xs text-green-400 flex items-center gap-1"><Check :size="12" /> {{ saveMessage }}</span>
          <span v-if="saveError" class="text-xs text-red-400 flex items-center gap-1"><AlertTriangle :size="12" /> {{ saveError }}</span>
          <button 
            @click="saveFile" 
            :disabled="!isDirty || isSaving"
            class="px-3 py-1 rounded-lg text-xs font-medium transition-all flex items-center gap-1.5"
            :class="isDirty 
              ? 'bg-green-500/20 text-green-400 hover:bg-green-500/30' 
              : 'bg-white/5 text-gray-600 cursor-not-allowed'"
          >
            <Save :size="12" />
            {{ isSaving ? 'Saving...' : 'Save' }}
          </button>
        </div>
      </div>

      <!-- Code Editor -->
      <div v-if="selectedFile && !isFileLoading" class="flex-1 overflow-auto">
        <Codemirror
          v-model="fileContent"
          :extensions="extensions"
          :style="{ height: '100%', fontSize: '13px' }"
        />
      </div>

      <!-- Loading state -->
      <div v-else-if="isFileLoading" class="flex-1 flex items-center justify-center">
        <Activity :size="24" class="animate-spin text-gray-500" />
      </div>

      <!-- Empty state -->
      <div v-else class="flex-1 flex flex-col items-center justify-center text-gray-500">
        <FileText :size="48" class="mb-4 opacity-20" />
        <p class="text-sm">Select a file to view</p>
      </div>
    </div>

    <AgentChat 
      context="skill-editor" 
      :contextData="{ skillName: skillName, selectedFile: selectedFile?.path }" 
    />
  </div>
</template>

<style>
/* CodeMirror overrides for dark theme integration */
.cm-editor {
    height: 100% !important;
    background: #1e1e24 !important;
}
.cm-editor .cm-gutters {
    background: #1a1a20 !important;
    border-right: 1px solid rgba(255,255,255,0.05) !important;
}
.cm-editor .cm-activeLineGutter,
.cm-editor .cm-activeLine {
    background: rgba(255,255,255,0.03) !important;
}
</style>
