import { createApp } from 'vue'
import App from './App.vue'
import router from './router'

// 引入字体
import 'vfonts/Lato.css'
import 'vfonts/FiraCode.css'
import './assets/dark.css'

const app = createApp(App)

app.use(router)

app.mount('#app')
