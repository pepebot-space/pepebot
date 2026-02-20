import { createRouter, createWebHistory } from 'vue-router'
import Home from '../views/Home.vue'
import Chat from '../views/Chat.vue'

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes: [
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

export default router
