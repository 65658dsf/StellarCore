<template>
  <div id="app">
    <header class="grid-content header-color">
      <div class="header-content">
        <div class="brand">
          <a href="#">StellarCore</a>
        </div>
        <div class="dark-switch">
          <el-switch
            v-model="darkmodeSwitch"
            inline-prompt
            active-text= "暗色"
            inactive-text="亮色"
            @change="toggleDark"
            style="
              --el-switch-on-color: #444452;
              --el-switch-off-color: #589ef8;
            "
          />
        </div>
      </div>
    </header>
    <section>
      <el-row>
        <el-col id="side-nav" :xs="24" :md="4">
          <el-menu
            default-active="/"
            mode="vertical"
            theme="light"
            router="false"
            @select="handleSelect"
          >
            <el-menu-item index="/">首页</el-menu-item>
            <el-sub-menu index="/proxies">
              <template #title>
                <span>隧道列表</span>
              </template>
              <el-menu-item index="/proxies/tcp">TCP</el-menu-item>
              <el-menu-item index="/proxies/udp">UDP</el-menu-item>
              <el-menu-item index="/proxies/http">HTTP</el-menu-item>
              <el-menu-item index="/proxies/https">HTTPS</el-menu-item>
            </el-sub-menu>
            <el-menu-item index="https://docs.stellarfrp.top">帮助</el-menu-item>
            <el-menu-item index="https://www.stellarfrp.top">官网</el-menu-item>
          </el-menu>
        </el-col>

        <el-col :xs="24" :md="20">
          <div id="content">
            <router-view></router-view>
          </div>
        </el-col>
      </el-row>
    </section>
    <footer></footer>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useDark, useToggle } from '@vueuse/core'

const isDark = useDark()
const darkmodeSwitch = ref(isDark)
const toggleDark = useToggle(isDark)

const handleSelect = (key: string) => {
  if (key == 'https://docs.stellarfrp.top') {
    window.open('https://docs.stellarfrp.top')
  }
  if (key == 'https://www.stellarfrp.top') {
    window.open('https://www.stellarfrp.top')
  }
}
</script>

<style>
body {
  margin: 0px;
  font-family: -apple-system, BlinkMacSystemFont, Helvetica Neue, sans-serif;
}

header {
  width: 100%;
  height: 60px;
}

.header-color {
  background: #58b7ff;
}

html.dark .header-color {
  background: #395c74;
}

.header-content {
  display: flex;
  align-items: center;
}

#content {
  margin-top: 20px;
  padding-right: 40px;
}

.brand {
  display: flex;
  justify-content: flex-start;
}

.brand a {
  color: #fff;
  background-color: transparent;
  margin-left: 20px;
  line-height: 25px;
  font-size: 25px;
  padding: 15px 15px;
  height: 30px;
  text-decoration: none;
}

.dark-switch {
  display: flex;
  justify-content: flex-end;
  flex-grow: 1;
  padding-right: 40px;
}
</style>
