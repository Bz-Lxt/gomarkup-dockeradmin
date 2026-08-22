import { createRouter, createWebHistory } from 'vue-router'
import DashboardPage from './pages/DashboardPage.vue'
import ContainersPage from './pages/ContainersPage.vue'
import AlertsPage from './pages/AlertsPage.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', name: 'dashboard', component: DashboardPage, meta: { title: '总览' } },
    { path: '/containers', name: 'containers', component: ContainersPage, meta: { title: '容器' } },
    { path: '/alerts', name: 'alerts', component: AlertsPage, meta: { title: '告警' } },
  ],
})
