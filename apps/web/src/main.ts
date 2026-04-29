import { createApp } from 'vue'
import { createBootstrap } from 'bootstrap-vue-next'
import App from './App.vue'

import 'bootstrap/dist/css/bootstrap.min.css'
import 'bootstrap-vue-next/dist/bootstrap-vue-next.css'
import 'bootstrap-icons/font/bootstrap-icons.css'

const app = createApp(App)
app.use(createBootstrap())
app.mount('#app')
