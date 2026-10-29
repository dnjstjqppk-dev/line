<template>
  <component :is="`svg-icon-${name}`" :class="class" />
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  name: { type: String, required: true },
  class: { type: [String, Array], default: '' }
})

const iconMap = {
  home: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/></svg>',
  chat: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2z"/></svg>',
  voom: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M21 3H3c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h18c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zM5 17H3v-2h2v2zm0-4H3V7h2v6zm8 10c-1.1 0-1.99.9-1.99 2S20.9 21 22 21s2-.9 2-2-.9-2-2-2-2 .9-2 2m0-12c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm-6 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z"/></svg>',
  openchat: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/></svg>',
  wallet: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M20 6H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2zm0 4H4V8c0-1.1.9-2 2-2h16c1.1 0 2 .9 2 2v2z"/></svg>',
  profile: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/></svg>',
  search: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M15.5 14h-.79l-.28-.27A6.471 6.471 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/></svg>',
  settings: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M19.43 12.98c.04-.32.07-.64.07-.98s-.03-.66-.07-.98l2.11-1.65c.19-.15.24-.42.12-.64l-2-3.46c-.13-.22-.33-.34-.58-.34H17c-1.1 0-2-.9-2-2V5c0-1.1-.9-2-2-2H7c-1.1 0-2 .9-2 2v2c0 1.1-.9 2-2 2H3c-.25 0-.45.12-.58.34L1 8.73c-.12.22-.07.49.12.64l2.11 1.65c-.04.32-.07.66-.07.98s.03.66.07.98l-2 3.46c-.13.22-.12.49.12.64l2.11 1.65c-.04.32-.07.64-.07.98s.03.66.07.98l2 3.46c.13.22.33.34.58.34h1c1.1 0 2 .9 2 2v2c0 1.1.9 2 2 2h2c1.1 0 2-.9 2 2v-2c0-1.1.9-2 2-2h2c.25 0 .45-.12.58-.34l2-1.65c.19-.15.24-.42.12-.64l-2-3.46c-.12-.23-.17-.5-.12-.75zM12 15c1.66 0 3-1.34 3-3S13.66 9 12 9 9 10.34 9 12s1.34 3 3 3z"/></svg>',
  send: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>',
  plus: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6z"/></svg>',
  check: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M9 16.17l4.24-4.23.9.91L14 17.5l-5 5-.9-.91z"/></svg>',
  close: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>',
  more: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 16c-2.21 0-4 1.79-4 4h8c0-2.21-1.79-4-4-4zm0-8c-2.21 0-4 1.79-4 4h8c0-2.21-1.79-4-4-4zm0 4c-2.21 0-4 1.79-4 4h8c0-2.21-1.79-4-4-4z"/></svg>',
  heart: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg>',
  clock: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8zm0-18c-4.41 0-8 3.59-8 8s3.59 8 8 8 8-3.59 8-8-3.59-8-8-8zm0 15.5c-3.31 0-6-2.69-6-6s2.69-6 6-6 6 2.69 6 6-2.69 6-6 6z"/></svg>',
  pin: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M21 20V10c0-2.2-1.8-4-4-4H9c-2.21 0-4 1.79-4 4v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2z"/></svg>',
  read: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M21 6h-2v9L6 13.31 4.31 14.69l2.31 2.32 5.1-5.1V18h2V6z"/></svg>',
  unread: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M11.49 16.49L16.5 21.5h-4.29c-.46 0-.69-.39-.5-.83L12 14.5l-1.21-3.83c.19-.44.04-.83-.5-.83H3.5c-.89 0-1.71.45-2.24 1.21-.44.62-.27 1.45.17 2.05l1.47 1.96A3.493 3.493 0 0 0 4.5 21c1.1 0 2.09-.56 2.81-1.44L6 17.01v4.28c0 .66.37 1.24.9 1.57l4.67-.94A2.2 2.2 0 0 0 12 21c-.41 0-.79-.11-1.15-.29-.14-.07-.29-.07-.43 0-.36.18-.74.29-1.15.29-.68 0-1.3-.22-1.82-.6L10 17.42v-4.9l-1.94-2.59c-.36-.48-.49-1.1-.32-1.66.19-.62 1.03-.88 1.46-.32l1.46 1.94c.33.44.85.71 1.4.71.52 0 1-.17 1.39-.44zM19 16c0-1.1-.9-2-2-2s-2 .9-2 2v2h4v-2z"/></svg>',
  warning: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2V7h2v7z"/></svg>',
  info: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M11 17h2v-6h-2v6zm1-15C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm0-14c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6-2.69-6-6-6z"/></svg>',
  error: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg>',
  success: '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15-5-5 1.41-1.41L10 14.17l7.59-7.59 1.41 1.41L12 16.34z"/></svg>',
}

const getIconComponent = (name) => {
  const svgContent = iconMap[name] || iconMap.chat
  return {
    template: `<svg class="line-icon" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg" innerHTML="${svgContent.replace(/"/g, '&quot;')}"></svg>`
  }
}

const icon = computed(() => getIconComponent(props.name))
</script>

<style scoped>
.line-icon {
  width: 100%;
  height: 100%;
  display: block;
}
</style>