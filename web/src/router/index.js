import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    redirect: '/dashboard'
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { title: '登录', public: true }
  },
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/views/DashboardView.vue'),
    meta: { title: '看板', icon: '📊' }
  },
  {
    path: '/augment',
    name: 'augment',
    component: () => import('@/views/AugmentView.vue'),
    meta: { title: 'Augment', icon: '🔑' }
  },
  {
    path: '/openai',
    name: 'openai',
    component: () => import('@/views/OpenAIView.vue'),
    meta: { title: 'OpenAI / Codex', icon: '🤖' }
  },
  {
    path: '/cursor',
    name: 'cursor',
    component: () => import('@/views/CursorView.vue'),
    meta: { title: 'Cursor', icon: '💻' }
  },
  {
    path: '/windsurf',
    name: 'windsurf',
    component: () => import('@/views/WindsurfView.vue'),
    meta: { title: 'Windsurf', icon: '🏄' }
  },
  {
    path: '/antigravity',
    name: 'antigravity',
    component: () => import('@/views/AntigravityView.vue'),
    meta: { title: 'Antigravity', icon: '🚀' }
  },
  {
    path: '/claude',
    name: 'claude',
    component: () => import('@/views/ClaudeView.vue'),
    meta: { title: 'Claude', icon: '🧠' }
  },
  {
    path: '/docs',
    name: 'docs',
    component: () => import('@/views/DocsView.vue'),
    meta: { title: '文档', icon: '📖' }
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/SettingsView.vue'),
    meta: { title: '设置', icon: '⚙️' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  if (to.meta.public) {
    next()
    return
  }

  const token = localStorage.getItem('easyllm_token')
  if (!token) {
    next('/login')
    return
  }
  next()
})

export default router
