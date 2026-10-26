<script setup lang="ts">
import { Airplay, Copy, Monitor, Play, RefreshCw, ShieldCheck, Smartphone, Square } from '@lucide/vue'
import { computed } from 'vue'
import AriButton from '../ui/AriButton.vue'
import AirPlayLogs from './AirPlayLogs.vue'
import AirPlayControlStatus from './AirPlayControlStatus.vue'
import { useAirPlay } from '../../composables/useAirPlay'
import type { AirPlayCommand } from '../../types/airplay'

const { status, error, pending, refreshing, feedback, stateLabel, refresh, run, copy, copyingDiagnostics, copyDiagnostics } = useAirPlay()
const mirroring = computed(() => status.value?.running && status.value.phase === 'mirroring')
const busy = computed(() => !!pending.value)
const heading = computed(() => {
  if (error.value) return '暂时无法读取接收状态'
  if (!status.value) return '正在读取接收状态…'
  if (!status.value.ready) return '投屏暂不可用'
  if (mirroring.value) return status.value.clientName || 'iPhone 已连接'
  if (status.value.running) return '在 iPhone 上选择这台电脑'
  return '将 iPhone 画面带到电脑'
})
const description = computed(() => {
  if (error.value) return '请刷新状态后重试。'
  if (!status.value) return '请稍候。'
  if (!status.value.ready) return status.value.lastError || '投屏组件缺失，请重新安装 Ariadne。'
  if (mirroring.value) return '画面与声音正在投送。可在手机上切换内容，在视频窗口中调整显示。'
  if (status.value.running) return '打开控制中心 → 屏幕镜像，选择下方接收名称。'
  return '启动接收后，在 iPhone 的「屏幕镜像」中连接。'
})

function changeFrameRate(event: Event) {
  const value = (event.target as HTMLSelectElement).value
  if (value === '30' || value === '60') void run(`fps ${value}` as AirPlayCommand)
}
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell airplay-shell" aria-label="iPhone 投屏">
        <header class="airplay-header">
          <div class="airplay-brand">
            <span class="airplay-brand-icon"><Airplay :size="21" /></span>
            <div><h1>iPhone 投屏</h1><p>屏幕与声音 · AirPlay</p></div>
          </div>
          <div class="airplay-header-actions">
            <span class="airplay-status" :class="{ active: status?.running && !error }" role="status">
              <span class="airplay-status-dot" />{{ stateLabel }}
            </span>
            <AriButton size="sm" variant="ghost" :disabled="refreshing || busy" @click="refresh">
              <RefreshCw :size="14" />刷新状态
            </AriButton>
            <AriButton size="sm" variant="ghost" :disabled="copyingDiagnostics" @click="copyDiagnostics">
              <Copy :size="14" />{{ copyingDiagnostics ? '正在读取…' : '复制诊断' }}
            </AriButton>
          </div>
        </header>

        <div class="airplay-workspace" data-no-drag>
          <div v-if="feedback" class="airplay-notice" :class="{ failure: !feedback.ok }" role="status">
            {{ feedback.message }}
          </div>
          <div v-if="error || status?.lastError" class="airplay-notice failure" role="alert">
            {{ error || status?.lastError }}
          </div>

          <div class="airplay-columns">
            <div class="airplay-main">
              <section class="airplay-connection" aria-label="连接状态">
                <div class="airplay-device-link" :class="{ connected: status?.running }" aria-hidden="true">
                  <Smartphone :size="30" :stroke-width="1.5" />
                  <span /><Monitor :size="45" :stroke-width="1.4" />
                </div>
                <h2>{{ heading }}</h2>
                <p class="airplay-description">{{ description }}</p>
                <div v-if="status?.receiverName" class="airplay-receiver">
                  <span>接收名称</span>
                  <div class="airplay-value-row">
                    <strong>{{ status.receiverName }}</strong>
                    <AriButton size="icon" variant="ghost" aria-label="复制接收名称" @click="copy(status.receiverName, '接收名称')"><Copy :size="14" /></AriButton>
                  </div>
                </div>
                <div v-if="status?.running && status.pin" class="airplay-pairing">
                  <div><span>配对码</span><strong>{{ status.pin }}</strong></div>
                  <AriButton size="sm" variant="ghost" @click="copy(status.pin, '配对码')"><Copy :size="14" />复制</AriButton>
                </div>
                <p v-if="status?.running && status.pin" class="airplay-pairing-hint">iPhone 提示输入 AirPlay 代码时填写。</p>
                <div class="airplay-connect-actions">
                  <AriButton v-if="status?.running" variant="secondary" :disabled="busy" @click="run('stop')">
                    <Square :size="14" />{{ pending === 'stop' ? '正在停止…' : '停止接收' }}
                  </AriButton>
                  <AriButton v-else variant="primary" :disabled="busy || !status?.ready || !!error" @click="run('start')">
                    <Play :size="15" />{{ pending === 'start' ? '正在启动…' : '启动投屏' }}
                  </AriButton>
                </div>
                <dl v-if="mirroring && status" class="airplay-stream-info">
                  <div><dt>画面尺寸</dt><dd>{{ status.width }} × {{ status.height }}</dd></div>
                  <div v-if="status.contentTitle"><dt>正在播放</dt><dd>{{ status.contentTitle }}</dd></div>
                </dl>
              </section>

              <section class="airplay-guide" aria-label="连接步骤">
                <h3>连接 iPhone</h3>
                <ol>
                  <li><span>1</span><div><strong>连接同一局域网</strong><p>iPhone 连接 Wi-Fi，电脑可使用同一网络的有线连接。</p></div></li>
                  <li><span>2</span><div><strong>打开屏幕镜像</strong><p>从 iPhone 右上角向下滑动，打开控制中心并点选「屏幕镜像」。</p></div></li>
                  <li><span>3</span><div><strong>选择接收名称</strong><p>选择这台电脑，按提示输入配对码。画面将显示在视频窗口中。</p></div></li>
                </ol>
              </section>
            </div>

            <aside class="airplay-sidebar" aria-label="投屏设置">
              <section class="airplay-settings-section">
                <h3>播放设置</h3>
                <label class="airplay-field" for="airplay-fps"><span>帧率上限</span>
                  <select id="airplay-fps" :value="status?.frameRate ?? ''" :disabled="busy || !status" @change="changeFrameRate">
                    <option disabled value="">正在读取</option>
                    <option value="30">30 fps · 节省带宽</option>
                    <option value="60">60 fps · 更流畅</option>
                  </select>
                </label>
                <p class="airplay-help">设置自动保存。运行中切换需要重新连接；实际帧率取决于手机、应用和网络。</p>
              </section>
              <section class="airplay-settings-section">
                <h3>窗口操作</h3>
                <dl class="airplay-shortcuts">
                  <div><dt>全屏 / 退出全屏</dt><dd><kbd>双击</kbd> / <kbd>Alt+Enter</kbd></dd></div>
                  <div><dt>退出全屏</dt><dd><kbd>Esc</kbd></dd></div>
                  <div><dt>置顶 / 跟随横竖屏</dt><dd>右键菜单</dd></div>
                </dl>
                <p class="airplay-help">横竖屏分别记忆窗口尺寸。视频可见时保持电脑亮屏。</p>
              </section>
              <section class="airplay-settings-section">
                <h3>搜不到这台电脑？</h3>
                <p class="airplay-help">允许专用网络内的设备连接，需要 Windows 管理员授权。</p>
                <AriButton size="sm" variant="secondary" :disabled="busy || !status?.ready" @click="run('firewall')">
                  <ShieldCheck :size="14" />{{ pending === 'firewall' ? '等待授权…' : '允许局域网投屏' }}
                </AriButton>
              </section>
              <AirPlayControlStatus v-if="status" :status="status" :busy="busy" @open="run('control')" />
            </aside>
          </div>
          <AirPlayLogs />
        </div>
        <footer class="airplay-footer"><span>关闭此窗口后仍可继续投屏；退出 Ariadne 会停止接收。</span><span>UxPlay</span></footer>
      </section>
    </div>
  </main>
</template>

<style scoped>
.airplay-shell { grid-template-rows: auto minmax(0, 1fr) auto; }
.airplay-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 22px 28px; border-bottom: 1px solid var(--border); }
.airplay-brand { display: flex; align-items: center; gap: 12px; }
.airplay-brand-icon { display: grid; place-items: center; width: 42px; height: 42px; border: 1px solid var(--border); border-radius: var(--radius-md); color: var(--primary); background: var(--surface-raised); }
h1, h2, h3, p { margin: 0; }
h1 { font-size: 18px; font-weight: 650; }
.airplay-brand p { margin-top: 3px; font-size: 12px; color: var(--muted-foreground); }
.airplay-header-actions { display: flex; align-items: center; gap: 14px; }
.airplay-status { display: inline-flex; align-items: center; gap: 7px; color: var(--muted); font-size: 12px; white-space: nowrap; }
.airplay-status-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--muted-foreground); }
.airplay-status.active { color: var(--primary); }
.airplay-status.active .airplay-status-dot { background: var(--primary); }
.airplay-workspace { min-height: 0; overflow: auto; padding: 24px 28px; }
.airplay-columns { display: grid; grid-template-columns: minmax(0, 1fr) 290px; gap: 32px; margin-bottom: 24px; }
.airplay-main { min-width: 0; }
.airplay-connection { border: 1px solid var(--border); border-radius: var(--radius-md); padding: 24px; background: var(--surface-raised); }
.airplay-device-link { display: flex; align-items: center; gap: 18px; margin-bottom: 18px; color: var(--muted-foreground); }
.airplay-device-link span { width: 40px; height: 1px; background: var(--border-strong); }
.airplay-device-link.connected { color: var(--primary); }
.airplay-device-link.connected span { background: var(--primary); }
h2 { font-size: 21px; font-weight: 600; line-height: 1.5; overflow-wrap: anywhere; }
.airplay-description { margin-top: 6px; font-size: 13px; color: var(--muted); line-height: 1.8; }
.airplay-receiver { margin-top: 22px; padding-top: 16px; border-top: 1px solid var(--border); }
.airplay-receiver > span, .airplay-pairing span { font-size: 11px; color: var(--muted-foreground); }
.airplay-value-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.airplay-value-row strong { font-size: 14px; font-weight: 550; overflow-wrap: anywhere; user-select: text; }
.airplay-pairing { display: flex; align-items: center; justify-content: space-between; margin-top: 10px; }
.airplay-pairing > div { display: flex; align-items: center; gap: 18px; }
.airplay-pairing strong { font-family: var(--font-mono); font-size: 29px; font-weight: 500; letter-spacing: 5px; user-select: text; }
.airplay-pairing-hint { margin-top: 4px; color: var(--muted-foreground); font-size: 11px; }
.airplay-connect-actions { margin-top: 20px; }
.airplay-stream-info { margin: 18px 0 0; padding-top: 14px; border-top: 1px solid var(--border); display: grid; gap: 8px; font-size: 12px; }
.airplay-stream-info div { display: grid; grid-template-columns: 70px minmax(0, 1fr); gap: 12px; }
dt { color: var(--muted-foreground); }
dd { margin: 0; overflow-wrap: anywhere; }
h3 { font-size: 13px; font-weight: 650; margin-bottom: 14px; }
.airplay-guide { margin-top: 24px; }
.airplay-guide ol { display: grid; gap: 14px; list-style: none; margin: 0; padding: 0; }
.airplay-guide li { display: flex; align-items: flex-start; gap: 12px; font-size: 12px; }
.airplay-guide li > span { display: grid; place-items: center; flex-shrink: 0; width: 22px; height: 22px; font-size: 11px; border: 1px solid var(--border); border-radius: 50%; color: var(--muted); }
.airplay-guide strong { font-weight: 550; line-height: 22px; }
.airplay-guide p { color: var(--muted-foreground); line-height: 1.7; margin-top: 2px; }
.airplay-sidebar { border-left: 1px solid var(--border); padding-left: 28px; min-width: 0; }
.airplay-settings-section { padding-bottom: 22px; margin-bottom: 22px; border-bottom: 1px solid var(--border); }
.airplay-field { display: grid; gap: 9px; font-size: 12px; }
select { width: 100%; height: 35px; padding: 0 10px; border: 1px solid var(--border-strong); border-radius: var(--radius-sm); background: var(--control-bg); color: var(--foreground); outline: none; }
select:focus-visible, summary:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }
select:disabled { opacity: 0.55; }
.airplay-help { font-size: 12px; line-height: 1.8; color: var(--muted-foreground); margin: 10px 0; }
.airplay-shortcuts { display: grid; gap: 12px; margin: 0; font-size: 11px; }
.airplay-shortcuts div { display: grid; gap: 5px; }
.airplay-shortcuts dd { font-size: 12px; color: var(--muted); }
.airplay-control summary { font-size: 12px; color: var(--muted); cursor: pointer; }
.airplay-notice { padding: 10px 14px; margin-bottom: 18px; border: 1px solid var(--success-border); border-radius: var(--radius-sm); background: var(--success-soft); color: var(--success-foreground); font-size: 12px; line-height: 1.7; overflow-wrap: anywhere; }
.airplay-notice.failure { border-color: var(--danger-border); background: var(--danger-soft); color: var(--danger-foreground); }
.airplay-footer { padding: 10px 28px; border-top: 1px solid var(--border); font-size: 11px; color: var(--muted-foreground); display: flex; justify-content: space-between; gap: 12px; }
@media (max-width: 900px) {
  .airplay-header { padding: 18px 22px; }
  .airplay-workspace { padding: 22px; }
  .airplay-columns { grid-template-columns: minmax(0, 1fr) 245px; gap: 22px; }
  .airplay-sidebar { padding-left: 22px; }
  .airplay-connection { padding: 20px; }
}
@media (max-width: 680px) {
  .airplay-header { align-items: flex-start; }
  .airplay-header-actions { align-items: flex-end; flex-direction: column; gap: 4px; }
  .airplay-columns { grid-template-columns: 1fr; }
  .airplay-sidebar { padding-left: 0; border-left: 0; }
  .airplay-footer { padding-inline: 22px; }
}
</style>
