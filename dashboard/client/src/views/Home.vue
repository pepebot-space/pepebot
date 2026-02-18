<script setup>
import { ArrowRight, Cloud, Box, Zap } from 'lucide-vue-next'

const greeting = "Hello Charles!" // TODO: Make dynamic
const subtext = "How can I assist you today?"

const cards = [
  { 
    id: 1, 
    title: 'Cloud Storage', 
    icon: Cloud,
    bg: 'bg-[#1e1e24]', 
    text: 'text-gray-400'
  },
  { 
    id: 2, 
    title: 'Integrations', 
    icon: Box,
    bg: 'bg-[#1e1e24]',
    text: 'text-gray-400'
  },
  { 
    id: 3, 
    title: 'Quick Start', 
    icon: Zap,
    bg: 'bg-gradient-to-br from-purple-400 via-pink-400 to-yellow-200',
    text: 'text-black font-semibold',
    isPrimary: true
  }
]
</script>

<template>
  <div class="p-10 h-full flex flex-col max-w-5xl mx-auto">
    <!-- Header -->
    <header class="mb-12 mt-10">
      <h2 class="text-gray-400 text-lg font-medium mb-2">{{ greeting }}</h2>
      <h1 class="text-5xl font-semibold tracking-tight">{{ subtext }}</h1>
    </header>

    <!-- Cards Grid -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-12">
      <div 
        v-for="card in cards" 
        :key="card.id" 
        :class="[
          'relative p-6 rounded-3xl h-64 flex flex-col justify-between transition-transform hover:scale-[1.02] cursor-pointer group',
          card.bg
        ]"
      >
        <div 
          class="w-12 h-12 rounded-2xl flex items-center justify-center"
          :class="card.isPrimary ? 'bg-black/10' : 'bg-white/5'"
        >
          <component :is="card.icon" :size="24" :class="card.text" />
        </div>

        <div class="flex items-center justify-between">
          <span :class="['font-medium text-lg', card.text]">{{ card.title }}</span>
          <ArrowRight v-if="!card.isPrimary" :size="20" class="text-gray-600 group-hover:text-gray-300 transition-colors" />
        </div>
      </div>
    </div>

    <!-- Message Input Area (Footer Mockup) -->
    <div class="mt-auto">
       <!-- Suggestion Pill -->
       <div class="flex w-full justify-center lg:justify-start mb-4">
        <div class="bg-gradient-to-r from-purple-500/20 to-green-500/20 border border-white/10 rounded-full px-4 py-2 flex items-center gap-3 backdrop-blur-md">
            <div class="w-6 h-6 rounded-full bg-purple-500/20 flex items-center justify-center text-purple-300 text-xs">
                <Zap :size="14" />
            </div>
            <span class="text-sm text-gray-300 truncate max-w-md">Trident, what has been the weather forecast for August 23rd in the past 10 years?</span>
            <button class="bg-[#1e1e24] hover:bg-[#2a2a30] text-gray-300 text-xs px-3 py-1 rounded-full ml-2 transition-colors">Ask</button>
        </div>
       </div>
       
       <!-- Input Box Mock (Link to Chat) -->
       <router-link to="/chat" class="w-full h-16 bg-[#1e1e24] rounded-2xl flex items-center px-6 hover:bg-[#25252b] transition-colors cursor-text group">
         <span class="text-gray-500 text-lg">Message Trident...</span>
         <div class="ml-auto flex gap-3">
            <div class="w-10 h-10 rounded-xl bg-white/5 flex items-center justify-center text-gray-400 group-hover:bg-white/10 transition-colors">
                <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>
            </div>
            <button class="w-10 h-10 rounded-xl bg-white text-black flex items-center justify-center hover:bg-gray-200 transition-colors">
                <ArrowRight :size="20" />
            </button>
         </div>
       </router-link>
    </div>
  </div>
</template>
