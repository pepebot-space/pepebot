import { createRouter, createWebHistory } from 'vue-router'
import { isGatewayConfigured } from '../lib/gateway.js'
import Home from '../views/Home.vue'
import Chat from '../views/Chat.vue'

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes: [
        {
            path: '/setup',
            name: 'setup',
            component: () => import('../views/GatewaySetup.vue'),
            meta: { noAuth: true }
        },
        {
            path: '/',
            name: 'home',
            component: Home
        },
        {
            path: '/chat',
            name: 'chat',
            component: Chat
        },
        {
            path: '/agents',
            name: 'agents',
            component: () => import('../views/Agents.vue')
        },
        {
            path: '/skills',
            name: 'skills',
            component: () => import('../views/Skills.vue')
        },
        {
            path: '/skills/:name',
            name: 'skill-detail',
            component: () => import('../views/SkillDetail.vue')
        },
        {
            path: '/workflows',
            name: 'workflows',
            component: () => import('../views/Workflows.vue')
        },
        {
            path: '/config',
            name: 'config',
            component: () => import('../views/Config.vue')
        }
    ]
})

// Navigation guard — redirect to setup if no gateway configured
router.beforeEach((to, from, next) => {
    if (to.meta.noAuth) {
        next()
    } else if (!isGatewayConfigured()) {
        next({ name: 'setup' })
    } else {
        next()
    }
})

export default router
