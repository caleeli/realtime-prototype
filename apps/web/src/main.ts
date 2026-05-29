import { createApp } from 'vue'
import { createBootstrap, vBTooltip } from 'bootstrap-vue-next'
import App from './App.vue'
import { i18n } from './i18n'

import 'bootstrap/dist/css/bootstrap.min.css'
import 'bootstrap-vue-next/dist/bootstrap-vue-next.css'
import 'bootstrap-icons/font/bootstrap-icons.css'

const app = createApp(App)
app.use(createBootstrap())
app.use(i18n)
app.directive('b-tooltip', vBTooltip)
app.mount('#app')
