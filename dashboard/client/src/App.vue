<script setup>
import { RouterLink, RouterView } from 'vue-router'
import { Home, MessageSquare, Cpu, Zap, GitBranch, Settings } from 'lucide-vue-next'
</script>

<template>
  <div class="flex h-screen bg-[#101014] text-white font-sans overflow-hidden">
    <!-- Sidebar -->
    <aside class="w-16 flex flex-col items-center py-6 border-r border-white/5 bg-[#101014]">
      <div class="mb-8">
        <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-green-400 to-emerald-600 flex items-center justify-center text-black font-bold text-xl">
            P
        </div>
      </div>
      
      <nav class="flex-1 flex flex-col gap-4 w-full items-center">
        <RouterLink to="/" class="p-3 rounded-xl hover:bg-white/10 text-gray-400 hover:text-white transition-all" active-class="bg-white/10 text-white">
          <Home :size="22" />
        </RouterLink>
        <RouterLink to="/chat" class="p-3 rounded-xl hover:bg-white/10 text-gray-400 hover:text-white transition-all" active-class="bg-white/10 text-white">
          <MessageSquare :size="22" />
        </RouterLink>
        <RouterLink to="/agents" class="p-3 rounded-xl hover:bg-white/10 text-gray-400 hover:text-white transition-all" active-class="bg-white/10 text-white">
           <Cpu :size="22" />
        </RouterLink>
        <RouterLink to="/skills" class="p-3 rounded-xl hover:bg-white/10 text-gray-400 hover:text-white transition-all" active-class="bg-white/10 text-white">
           <Zap :size="22" />
        </RouterLink>
        <RouterLink to="/workflows" class="p-3 rounded-xl hover:bg-white/10 text-gray-400 hover:text-white transition-all" active-class="bg-white/10 text-white">
           <GitBranch :size="22" />
        </RouterLink>
      </nav>

      <div class="mt-auto flex flex-col gap-6 w-full items-center">
        <div class="p-3 rounded-xl hover:bg-white/10 text-gray-400 hover:text-white transition-all cursor-pointer">
            <Settings :size="24" />
        </div>
        <div class="w-10 h-10 rounded-full bg-orange-500 overflow-hidden border-2 border-white/20">
            <img src="https://api.dicebear.com/9.x/avataaars/svg?seed=Felix" alt="User" />
        </div>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="flex-1 relative overflow-auto">
      <RouterView />
    </main>
  </div>
</template>

<style>
/* Global scrollbar styling could go here */
</style>
