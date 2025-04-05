declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

declare module '@vicons/ionicons5' {
  import { Component } from 'vue'
  const Refresh: Component
  const Upload: Component
  const Sunny: Component
  const Moon: Component
  const Home: Component
  const Settings: Component
  const HelpCircle: Component
  export { Refresh, Upload, Sunny, Moon, Home, Settings, HelpCircle }
} 