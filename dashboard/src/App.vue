<script setup>
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { computed } from 'vue'
import { Home, MessageSquare, Cpu, Zap, GitBranch, Settings, LogOut } from 'lucide-vue-next'
import { setActiveGateway, getActiveGateway } from './lib/gateway.js'

const route = useRoute()
const router = useRouter()

const isSetupPage = computed(() => route.name === 'setup')
const activeGateway = computed(() => getActiveGateway())

function logout() {
  setActiveGateway(null)
  router.push('/setup')
}
</script>

<template>
  <!-- Setup page: full-screen, no sidebar -->
  <div v-if="isSetupPage" class="h-screen bg-[#0a0a0e] text-white font-sans overflow-hidden">
    <RouterView />
  </div>

  <!-- Dashboard: sidebar + content -->
  <div v-else class="flex h-screen bg-[#101014] text-white font-sans overflow-hidden">
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

      <div class="mt-auto flex flex-col gap-4 w-full items-center">
        <RouterLink to="/config" class="p-3 rounded-xl hover:bg-white/10 text-gray-400 hover:text-white transition-all" active-class="bg-white/10 text-white">
            <Settings :size="22" />
        </RouterLink>
        <button
          @click="logout"
          class="p-3 rounded-xl hover:bg-red-500/10 text-gray-500 hover:text-red-400 transition-all"
          title="Disconnect gateway"
        >
          <LogOut :size="20" />
        </button>
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
