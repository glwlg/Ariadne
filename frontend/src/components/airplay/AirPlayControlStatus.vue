<script setup lang="ts">
import { computed } from 'vue'
import AriButton from '../ui/AriButton.vue'
import type { AirPlayStatus } from '../../types/airplay'

const props = defineProps<{ status: AirPlayStatus; busy: boolean }>()
defineEmits<{ open: [] }>()
const label = computed(() => {
  switch (props.status.control.phase) {
    case 'not-installed': return '控制组件未安装'
    case 'stopped': return props.status.running ? '投屏连接后自动启动控制' : '控制未启动'
    case 'starting': return '正在启动蓝牙控制'
    case 'waiting': return '等待 iPhone 鼠标控制连接'
    case 'select-device': return '已连接，请在控制窗口选择手机'
    case 'touchpad': return '触控板已连接'
    case 'direct': return '可直接操作投屏画面'
    case 'closing': return '正在关闭控制'
    case 'exited': return '控制组件异常退出'
    case 'error': return '蓝牙控制不可用'
    default: return '控制状态读取失败'
  }
})
</script>

<template>
  <section class="airplay-control-status" aria-label="手机控制">
    <h3>手机控制</h3>
    <p class="control-state" role="status">{{ label }}</p>
    <dl v-if="status.control.running">
      <div><dt>蓝牙广播</dt><dd>{{ status.control.advertising ? '已开启' : '未开启' }}</dd></div>
      <div><dt>已连接手机</dt><dd>{{ status.control.connectedDevices }} 台</dd></div>
      <div v-if="status.control.targetName"><dt>当前控制</dt><dd>{{ status.control.targetName }}</dd></div>
    </dl>
    <p v-if="status.control.lastError" class="control-error" role="alert">{{ status.control.lastError }}</p>
    <p v-if="status.control.phase === 'touchpad'" class="control-help">如需直接点击或拖动投屏画面，请在 iPhone 忽略电脑后重新配对。</p>
    <AriButton size="sm" :disabled="busy || !status.controlReady || status.control.phase === 'closing'" @click="$emit('open')">
      {{ status.control.running ? '控制设置' : '启动蓝牙控制' }}
    </AriButton>
    <p class="control-help">投屏连接后自动开启控制，记住上次选择的手机。关闭控制设置后仍可操作投屏画面。</p>
    <p v-if="status.control.phase === 'direct'" class="control-help">点击投屏画面后可使用电脑键盘输入，中文请使用手机的中文输入法。Alt+Enter 切换全屏。</p>
    <details>
      <summary>连接帮助</summary>
      <p class="control-help">蓝牙鼠标控制需要开启 iPhone 辅助触控，会显示悬浮按钮。在「辅助触控 → 设备 → 蓝牙设备」中连接电脑。仅有一台手机时自动选择；多台手机首次连接时，请在控制设置中选择。</p>
    </details>
  </section>
</template>

<style scoped>
h3 { margin: 0 0 12px; font-size: 13px; font-weight: 650; }
.control-state { margin: 0 0 12px; font-size: 12px; line-height: 1.7; }
dl { display: grid; gap: 8px; margin: 0 0 12px; font-size: 11px; }
dl div { display: grid; grid-template-columns: 76px minmax(0, 1fr); gap: 8px; }
dt { color: var(--muted-foreground); }
dd { margin: 0; overflow-wrap: anywhere; }
.control-error { color: var(--danger-foreground); font-size: 12px; line-height: 1.7; overflow-wrap: anywhere; }
.control-help { color: var(--muted-foreground); font-size: 12px; line-height: 1.8; margin: 10px 0; }
details { margin-top: 14px; }
summary { font-size: 12px; color: var(--muted); cursor: pointer; }
</style>
