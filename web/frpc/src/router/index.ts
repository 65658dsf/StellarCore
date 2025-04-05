import { createRouter, createWebHashHistory } from 'vue-router'
import Overview from '../components/Overview.vue'
import ClientConfigure from '../components/ClientConfigure.vue'
import Help from '../components/Help.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      name: '概览',
      component: Overview
    },
    {
      path: '/configure',
      name: '配置',
      component: ClientConfigure
    },
    {
      path: '/help',
      name: '帮助',
      component: Help
    }
  ]
})

export default router
