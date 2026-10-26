import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import * as IconsVue from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'
import { resolveApiBase } from './api'
import './assets/main.css'

// 桌面环境下先探测回环 API 端口（浏览器环境立即返回），再挂载应用
const boot = async () => {
  await resolveApiBase()
  const app = createApp(App)
  // 全局护栏：任何组件渲染/运行时异常只记日志，不允许把整页拖成白屏
  app.config.errorHandler = (err, _instance, info) => {
    console.error('[app-error]', err, info)
  }
  app.use(router)
  app.use(ElementPlus, { locale: zhCn })
  for (const [name, comp] of Object.entries(IconsVue)) {
    app.component(name, comp)
  }
  app.mount('#app')
}
boot()
