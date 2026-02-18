<script setup>
import { ref, onMounted, nextTick } from 'vue'
import axios from 'axios'
import { Send, Image as ImageIcon, Paperclip, Loader2 } from 'lucide-vue-next'

const messages = ref([
  { id: 1, role: 'assistant', content: 'Hello! I am Pepebot. How can I help you today?', timestamp: new Date() }
])
const newMessage = ref('')
const isLoading = ref(false)
const fileInput = ref(null)
const messagesContainer = ref(null)
const selectedFile = ref(null)
const previewUrl = ref(null)

const scrollToBottom = () => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

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

  try {
    let uploadedImageUrl = null;
    
    // Upload image if selected
    if (userMsg.image && fileInput.value && fileInput.value.files[0]) {
        const formData = new FormData();
        formData.append('image', fileInput.value.files[0]);
        // TODO: Replace with actual backend port if consistent
        const uploadRes = await axios.post('http://localhost:3000/api/upload', formData);
        uploadedImageUrl = uploadRes.data.url;
    }

    // Send to backend
    const response = await axios.post('http://localhost:3000/api/chat', {
        message: userText,
        media: uploadedImageUrl ? [uploadedImageUrl] : []
    });

    const reply = {
      id: Date.now() + 1,
      role: 'assistant',
      content: response.data.response || response.data.content || "I received your message.", // Adjust based on actual API response
      timestamp: new Date()
    }
    messages.value.push(reply)
  } catch (error) {
    console.error(error)
    messages.value.push({
      id: Date.now() + 1,
      role: 'assistant',
      content: "Sorry, I encountered an error communicating with the server.",
      isError: true,
      timestamp: new Date()
    })
  } finally {
    isLoading.value = false
    scrollToBottom()
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
</script>

<template>
  <div class="flex flex-col h-full max-w-5xl mx-auto p-4 md:p-6 lg:p-10 relative">
    <!-- Chat History -->
    <div ref="messagesContainer" class="flex-1 overflow-y-auto space-y-6 mb-24 pb-4 px-2 scroll-smooth">
      <div 
        v-for="msg in messages" 
        :key="msg.id" 
        :class="['flex gap-4 max-w-3xl', msg.role === 'user' ? 'ml-auto flex-row-reverse' : '']"
      >
        <!-- Avatar -->
        <div 
            class="w-10 h-10 rounded-full flex-shrink-0 flex items-center justify-center text-sm font-bold shadow-lg"
            :class="msg.role === 'assistant' ? 'bg-gradient-to-br from-green-400 to-emerald-600 text-black' : 'bg-orange-500 text-white'"
        >
          {{ msg.role === 'assistant' ? 'P' : 'U' }}
        </div>

        <!-- Bubble -->
        <div 
            class="rounded-2xl p-4 shadow-sm"
            :class="[
                msg.role === 'user' ? 'bg-[#2a2a30] text-gray-100 rounded-tr-sm' : 'bg-[#1e1e24] text-gray-300 rounded-tl-sm',
                msg.isError ? 'border border-red-500/20 bg-red-500/5' : ''
            ]"
        >
          <!-- Image Preview in Chat -->
          <img v-if="msg.image" :src="msg.image" class="max-w-xs rounded-lg mb-3 border border-white/10" alt="Uploaded image" />
          
          <p class="whitespace-pre-wrap leading-relaxed">{{ msg.content }}</p>
        </div>
      </div>

      <!-- Loading Indicator -->
      <div v-if="isLoading" class="flex gap-4">
        <div class="w-10 h-10 rounded-full bg-gradient-to-br from-green-400 to-emerald-600 flex items-center justify-center text-black font-bold text-sm animate-pulse">
            P
        </div>
        <div class="bg-[#1e1e24] p-4 rounded-2xl rounded-tl-sm flex items-center">
            <Loader2 class="animate-spin text-gray-400" :size="20" />
        </div>
      </div>
    </div>

    <!-- Input Area (Fixed Bottom) -->
    <div class="absolute bottom-6 left-0 right-0 px-4 md:px-10 max-w-5xl mx-auto">
        <div class="bg-[#1e1e24] rounded-2xl p-2 flex items-end gap-2 shadow-2xl border border-white/5 ring-1 ring-white/5 focus-within:ring-white/10 transition-all">
            <!-- File Button -->
            <button @click="triggerFileInput" class="p-3 text-gray-500 hover:text-gray-300 hover:bg-white/5 rounded-xl transition-colors mb-[2px]">
                <Paperclip :size="20" />
            </button>
            <input type="file" ref="fileInput" @change="handleFileUpload" class="hidden" accept="image/*" />

            <!-- Text Input -->
            <div class="flex-1 py-3">
                 <!-- Image Preview Pill -->
                <div v-if="selectedFile" class="flex items-center gap-2 mb-2 bg-[#101014] w-fit px-3 py-1 rounded-full border border-white/10">
                     <span class="text-xs text-gray-400 truncate max-w-[150px]">{{ selectedFile.name }}</span>
                     <button @click="selectedFile = null; previewUrl = null" class="text-gray-500 hover:text-white ml-1">×</button>
                </div>
                <textarea 
                    v-model="newMessage" 
                    @keydown.enter.prevent="sendMessage"
                    placeholder="Message Pepebot..." 
                    class="w-full bg-transparent border-none focus:ring-0 text-gray-100 placeholder-gray-500 resize-none h-6 py-0 max-h-32 mb-1"
                    rows="1"
                    style="min-height: 24px;"
                ></textarea>
            </div>

            <!-- Send Button -->
            <button 
                @click="sendMessage"
                :disabled="!newMessage.trim() && !selectedFile"
                class="p-3 bg-white text-black rounded-xl hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors mb-[2px]"
            >
                <ArrowRight :size="20" v-if="!isLoading" />
                <Loader2 :size="20" class="animate-spin" v-else />
            </button>
        </div>
    </div>
  </div>
</template>
