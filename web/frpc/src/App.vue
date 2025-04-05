<template>
  <n-config-provider :theme="theme" :locale="zhCN" :date-locale="dateZhCN">
    <n-layout class="layout">
      <n-layout-header class="header">
        <div class="header-content">
          <div class="logo">
            <h2>StellarFrp 客户端</h2>
          </div>
          <div class="theme-switch">
            <n-button text @click="toggleDark">
              <template #icon>
                <n-icon size="18">
                  <SunnyIcon v-if="isDark" />
                  <MoonIcon v-else />
                </n-icon>
              </template>
            </n-button>
          </div>
        </div>
      </n-layout-header>
      <n-layout has-sider position="absolute" style="top: 60px">
        <n-layout-sider
          bordered
          show-trigger
          collapse-mode="width"
          :collapsed-width="64"
          :width="240"
          :native-scrollbar="false"
        >
          <n-menu
            v-model:value="activeKey"
            :collapsed-width="64"
            :collapsed-icon-size="22"
            :options="menuOptions"
          />
        </n-layout-sider>
        <n-layout-content content-style="padding: 24px;">
          <router-view />
        </n-layout-content>
      </n-layout>
      <n-layout-footer position="absolute" class="footer">
        <div class="footer-content">
          StellarFrp © 2024
        </div>
      </n-layout-footer>
    </n-layout>
  </n-config-provider>
</template>

<script setup lang="ts">
import { h, ref, computed } from 'vue'
import { NIcon, darkTheme, useOsTheme, NConfigProvider, NLayout, NLayoutHeader, NLayoutContent, NLayoutFooter, NLayoutSider, NMenu, NButton, zhCN, dateZhCN } from 'naive-ui'
import { Sunny as SunnyIcon, Moon as MoonIcon, Home, Settings, HelpCircle } from '@vicons/ionicons5'
import { RouterLink, useRoute } from 'vue-router'
import type { MenuOption } from 'naive-ui'

// 主题切换
const osThemeRef = useOsTheme()
const isDark = ref(osThemeRef.value === 'dark')
const theme = computed(() => (isDark.value ? darkTheme : null))

function toggleDark() {
  isDark.value = !isDark.value
}

// 路由和菜单
const route = useRoute()
const activeKey = ref(route.path)

function renderIcon(icon: any) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

const menuOptions: MenuOption[] = [
  {
    label: () =>
      h(
        RouterLink,
        {
          to: '/'
        },
        { default: () => '概览' }
      ),
    key: '/',
    icon: renderIcon(Home)
  },
  {
    label: () =>
      h(
        RouterLink,
        {
          to: '/configure'
        },
        { default: () => '配置' }
      ),
    key: '/configure',
    icon: renderIcon(Settings)
  },
  {
    label: () =>
      h(
        RouterLink,
        {
          to: '/help'
        },
        { default: () => '帮助' }
      ),
    key: '/help',
    icon: renderIcon(HelpCircle)
  }
]
</script>

<style>
html, body {
  margin: 0;
  padding: 0;
  height: 100%;
}

#app {
  height: 100%;
}

.layout {
  height: 100%;
}

.header {
  position: relative;
  height: 60px;
  padding: 0 24px;
  background-color: var(--n-color, #2080f0);
  color: white;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  z-index: 10;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 100%;
}

.logo h2 {
  margin: 0;
  font-size: 20px;
}

.theme-switch {
  color: white;
}

.theme-switch button {
  color: white;
}

.footer {
  height: 50px;
  padding: 0 24px;
  color: #666;
  border-top: 1px solid #e8e8e8;
  background-color: #f7f7f7;
}

.footer-content {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
}
</style>
