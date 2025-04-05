declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

declare module '@vicons/ionicons5' {
  import { Component } from 'vue'
  const Server: Component
  const PrismOutline: Component
  const StatsChart: Component
  const Refresh: Component
  const Sunny: Component
  const Moon: Component
  const Home: Component
  const Settings: Component
  const HelpCircle: Component
  const Globe: Component
  const SwapVertical: Component
  const Trail: Component
  const LinkOutline: Component
  const LockClosed: Component
  export { 
    Server, PrismOutline, StatsChart, Refresh, Sunny, Moon, 
    Home, Settings, HelpCircle, Globe, SwapVertical, 
    Trail, LinkOutline, LockClosed 
  }
} 