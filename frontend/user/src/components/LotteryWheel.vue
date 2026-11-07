<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { LOTTERY_MISS_SLICE, type LotteryWheelSlice } from '../api/lottery'

const props = withDefaults(defineProps<{
  slices: LotteryWheelSlice[]
  /** 指针当前的累计旋转角度，由父组件推进，保证转盘总是向前转 */
  rotation: number
  spinning: boolean
  disabled?: boolean
  /** 转盘中心按钮的文案，留空用默认「抽奖」 */
  actionLabel?: string
  /** 命中扇区，停转后高亮；null 表示还没有结果 */
  highlightIndex?: number | null
}>(), {
  disabled: false,
  actionLabel: '',
  highlightIndex: null,
})

const emit = defineEmits<{ spin: [] }>()

const { t } = useI18n()

/** 没配颜色时按扇区序号取色；这组值在深浅两种主题下都够清晰。 */
const palette = ['#f97316', '#fbbf24', '#10b981', '#3b82f6', '#8b5cf6', '#ec4899', '#14b8a6', '#f43f5e']

const VIEW = 240
const CENTER = VIEW / 2
const RADIUS = 96
const LABEL_RADIUS = RADIUS * 0.63
const BULB_RADIUS = RADIUS + 13
const BULB_COUNT = 24

const sectorAngle = computed(() => (props.slices.length > 0 ? 360 / props.slices.length : 360))

const sliceLabel = (slice: LotteryWheelSlice) =>
  slice.name === LOTTERY_MISS_SLICE ? t('lottery.missLabel') : slice.name

const sliceColor = (slice: LotteryWheelSlice, index: number): string => {
  if (slice.prize_id === 0) return '#94a3b8'
  return slice.color || palette[index % palette.length] || '#64748b'
}

/**
 * 扇区底色由后台配置，深浅不可控，所以文字颜色按底色亮度自动取深或浅。
 * 否则浅色奖品（例如浅灰的"谢谢参与"）配白字会糊成一片。
 */
const parseColor = (value: string): number[] | null => {
  const raw = String(value || '').trim()
  let hex = raw.replace(/^#/, '')
  if (/^[0-9a-f]{3}$/i.test(hex)) {
    hex = hex.split('').map((char) => char + char).join('')
  }
  if (/^[0-9a-f]{6}$/i.test(hex)) {
    const int = parseInt(hex, 16)
    return [(int >> 16) & 255, (int >> 8) & 255, int & 255]
  }
  const match = raw.match(/rgba?\(\s*(\d+)[,\s]+(\d+)[,\s]+(\d+)/i)
  if (match) return [Number(match[1]), Number(match[2]), Number(match[3])]
  return null
}

const channelLuminance = (channel: number) => {
  const ratio = channel / 255
  return ratio <= 0.03928 ? ratio / 12.92 : ((ratio + 0.055) / 1.055) ** 2.4
}

const isLightColor = (value: string) => {
  const rgb = parseColor(value)
  if (!rgb || rgb.length < 3) return false
  // W3C 相对亮度，阈值取 0.6：比纯中点略高，偏向让中等深浅的底色也用白字。
  const weights = [0.2126, 0.7152, 0.0722]
  const luminance = rgb.reduce(
    (sum, channel, index) => sum + channelLuminance(channel) * (weights[index] ?? 0),
    0
  )
  return luminance > 0.6
}

const labelFill = (slice: LotteryWheelSlice, index: number) =>
  isLightColor(sliceColor(slice, index)) ? '#1f2937' : '#ffffff'

const labelStroke = (slice: LotteryWheelSlice, index: number) =>
  isLightColor(sliceColor(slice, index)) ? 'rgba(255, 255, 255, 0.55)' : 'rgba(15, 23, 42, 0.22)'

const polar = (angleDeg: number, radius = RADIUS) => {
  // 0° 指向 12 点，顺时针为正，和指针方向一致。
  const radians = ((angleDeg - 90) * Math.PI) / 180
  return {
    x: CENTER + radius * Math.cos(radians),
    y: CENTER + radius * Math.sin(radians),
  }
}

const sectorPath = (index: number) => {
  const start = index * sectorAngle.value
  const end = start + sectorAngle.value
  if (props.slices.length === 1) {
    // 单扇区画不出扇形，直接用整圆。
    return `M ${CENTER} ${CENTER - RADIUS} A ${RADIUS} ${RADIUS} 0 1 1 ${CENTER - 0.01} ${CENTER - RADIUS} Z`
  }
  const from = polar(start)
  const to = polar(end)
  const largeArc = sectorAngle.value > 180 ? 1 : 0
  return `M ${CENTER} ${CENTER} L ${from.x} ${from.y} A ${RADIUS} ${RADIUS} 0 ${largeArc} 1 ${to.x} ${to.y} Z`
}

/**
 * 标签跟着扇区旋转，落在下半圈的再自转 180°。
 * 否则扇区少的时候底部标签会整个倒过来，没法读。
 */
const labelTransform = (index: number) => {
  const center = index * sectorAngle.value + sectorAngle.value / 2
  const base = `rotate(${center} ${CENTER} ${CENTER})`
  if (center > 90 && center < 270) {
    return `${base} rotate(180 ${CENTER} ${CENTER - LABEL_RADIUS})`
  }
  return base
}

const labelFontSize = computed(() => {
  if (props.slices.length > 10) return 8
  if (props.slices.length > 7) return 9
  return 11
})

const truncate = (value: string) => {
  const text = String(value || '')
  const limit = props.slices.length > 8 ? 5 : 7
  return text.length > limit ? `${text.slice(0, limit)}…` : text
}

const bulbs = computed(() =>
  Array.from({ length: BULB_COUNT }, (_, index) => {
    const point = polar((360 / BULB_COUNT) * index, BULB_RADIUS)
    return { ...point, index }
  })
)

const hubLabel = computed(() => props.actionLabel || t('lottery.drawButton'))

const onSpin = () => {
  if (props.disabled || props.spinning) return
  emit('spin')
}
</script>

<template>
  <div class="lottery-wheel" :class="{ 'is-spinning': spinning }">
    <!-- 外圈灯带与金属边框，不跟着转 -->
    <div class="lottery-wheel__bezel" aria-hidden="true"></div>

    <svg :viewBox="`0 0 ${VIEW} ${VIEW}`" class="lottery-wheel__svg" role="img" :aria-label="t('lottery.wheelLabel')">
      <defs>
        <!-- 中心到边缘的高光，让平涂扇区有厚度 -->
        <radialGradient :id="`sheen-${VIEW}`" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stop-color="#ffffff" stop-opacity="0.26" />
          <stop offset="45%" stop-color="#ffffff" stop-opacity="0.05" />
          <stop offset="100%" stop-color="#000000" stop-opacity="0.14" />
        </radialGradient>
        <filter :id="`face-shadow-${VIEW}`" x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow dx="0" dy="3" stdDeviation="5" flood-color="#0f172a" flood-opacity="0.28" />
        </filter>
      </defs>

      <!-- 静止的灯珠 -->
      <g class="lottery-wheel__bulbs">
        <circle
          v-for="bulb in bulbs"
          :key="`bulb-${bulb.index}`"
          :cx="bulb.x"
          :cy="bulb.y"
          r="2.6"
          :class="bulb.index % 2 === 0 ? 'bulb bulb--odd' : 'bulb bulb--even'"
        />
      </g>

      <!-- 盘面：只有这一层跟着转 -->
      <g
        class="lottery-wheel__face"
        :style="{
          transform: `rotate(${rotation}deg)`,
          transition: spinning ? 'transform 4.2s cubic-bezier(0.12, 0.72, 0.1, 1)' : 'none',
        }"
        :filter="`url(#face-shadow-${VIEW})`"
      >
        <circle :cx="CENTER" :cy="CENTER" :r="RADIUS + 1" fill="#ffffff" />
        <g v-for="(slice, index) in slices" :key="`${slice.index}-${slice.prize_id}`">
          <path
            :d="sectorPath(index)"
            :fill="sliceColor(slice, index)"
            :fill-opacity="slice.sold_out ? 0.3 : 1"
          />
          <path :d="sectorPath(index)" :fill="`url(#sheen-${VIEW})`" />
          <path
            :d="sectorPath(index)"
            fill="none"
            stroke="#ffffff"
            stroke-opacity="0.75"
            stroke-width="1.5"
          />
          <path
            v-if="highlightIndex === index && !spinning"
            :d="sectorPath(index)"
            fill="#ffffff"
            class="lottery-wheel__hit"
          />
          <text
            :x="CENTER"
            :y="CENTER - LABEL_RADIUS"
            :transform="labelTransform(index)"
            text-anchor="middle"
            :font-size="labelFontSize"
            font-weight="700"
            :fill="labelFill(slice, index)"
            :stroke="labelStroke(slice, index)"
            class="lottery-wheel__label"
          >
            {{ truncate(sliceLabel(slice)) }}
          </text>
        </g>
      </g>
    </svg>

    <!-- 指针：固定在 12 点，针尖压住盘面 -->
    <div class="lottery-wheel__pointer" aria-hidden="true">
      <svg viewBox="0 0 34 42" width="34" height="42">
        <path d="M17 41 L4 14 A14 14 0 1 1 30 14 Z" fill="var(--ui-accent)" />
        <path d="M17 41 L4 14 A14 14 0 1 1 30 14 Z" fill="url(#pointer-sheen)" />
        <circle cx="17" cy="14" r="5" fill="#ffffff" fill-opacity="0.9" />
        <defs>
          <linearGradient id="pointer-sheen" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#ffffff" stop-opacity="0.35" />
            <stop offset="100%" stop-color="#000000" stop-opacity="0.12" />
          </linearGradient>
        </defs>
      </svg>
    </div>

    <!-- 中心即抽奖按钮 -->
    <button
      type="button"
      class="lottery-wheel__hub"
      :disabled="disabled || spinning"
      :aria-label="hubLabel"
      @click="onSpin"
    >
      <span class="lottery-wheel__hub-text">{{ hubLabel }}</span>
    </button>
  </div>
</template>

<style scoped>
.lottery-wheel {
  position: relative;
  width: 100%;
  max-width: 300px;
  margin-inline: auto;
  aspect-ratio: 1;
  container-type: inline-size;
}

/*
 * 金属外圈。
 * 颜色取设计系统的 --ui-* 令牌（深浅主题各自覆盖），不要用 hsl(var(--primary))：
 * 本项目的 token 存的是完整色值，不是 HSL 三元组，那样写会解析成空值。
 */
.lottery-wheel__bezel {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: var(--ui-accent);
  background: conic-gradient(
    from 210deg,
    color-mix(in srgb, var(--ui-accent) 88%, #ffffff),
    color-mix(in srgb, var(--ui-accent) 62%, #000000) 25%,
    color-mix(in srgb, var(--ui-accent) 92%, #ffffff) 50%,
    color-mix(in srgb, var(--ui-accent) 58%, #000000) 75%,
    color-mix(in srgb, var(--ui-accent) 88%, #ffffff)
  );
  box-shadow:
    0 14px 34px -12px var(--ui-accent-soft),
    inset 0 2px 6px rgba(255, 255, 255, 0.4),
    inset 0 -3px 9px rgba(15, 23, 42, 0.32);
}

.lottery-wheel__svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.lottery-wheel__face {
  transform-box: view-box;
  transform-origin: center;
}

.lottery-wheel__label {
  letter-spacing: 0.03em;
  paint-order: stroke;
  stroke-width: 1.8px;
  stroke-linejoin: round;
}

/* 灯珠压在金属圈上，暖白才看得出来 */
.bulb {
  fill: #fff6d8;
  opacity: 0.95;
}

.bulb--even {
  opacity: 0.45;
}

.is-spinning .bulb--odd {
  animation: wheel-bulb 0.42s steps(1, end) infinite;
}

.is-spinning .bulb--even {
  animation: wheel-bulb 0.42s steps(1, end) infinite reverse;
}

@keyframes wheel-bulb {
  0%,
  49% {
    opacity: 1;
  }
  50%,
  100% {
    opacity: 0.3;
  }
}

/* 停转后命中扇区闪两下 */
.lottery-wheel__hit {
  animation: wheel-hit 0.55s ease-in-out 3;
  pointer-events: none;
}

@keyframes wheel-hit {
  0%,
  100% {
    opacity: 0;
  }
  50% {
    opacity: 0.4;
  }
}

.lottery-wheel__pointer {
  position: absolute;
  top: -4px;
  left: 50%;
  transform: translateX(-50%);
  filter: drop-shadow(0 3px 5px rgba(15, 23, 42, 0.45));
  pointer-events: none;
  z-index: 2;
}

.lottery-wheel__hub {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 29%;
  aspect-ratio: 1;
  border-radius: 50%;
  border: 3px solid rgba(255, 255, 255, 0.95);
  background: var(--ui-accent);
  background: radial-gradient(
    circle at 32% 26%,
    color-mix(in srgb, var(--ui-accent) 70%, #ffffff),
    var(--ui-accent) 58%,
    color-mix(in srgb, var(--ui-accent) 78%, #000000)
  );
  color: var(--ui-text-on-accent);
  font-weight: 800;
  letter-spacing: 0.04em;
  cursor: pointer;
  z-index: 3;
  box-shadow:
    0 8px 18px -5px rgba(15, 23, 42, 0.45),
    inset 0 2px 5px rgba(255, 255, 255, 0.4);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.lottery-wheel__hub:hover:not(:disabled) {
  transform: translate(-50%, -50%) scale(1.06);
  box-shadow:
    0 12px 24px -5px rgba(15, 23, 42, 0.5),
    inset 0 2px 5px rgba(255, 255, 255, 0.45);
}

.lottery-wheel__hub:active:not(:disabled) {
  transform: translate(-50%, -50%) scale(0.97);
}

.lottery-wheel__hub:disabled {
  cursor: not-allowed;
  filter: grayscale(0.5);
  opacity: 0.7;
}

.lottery-wheel__hub-text {
  display: block;
  font-size: clamp(0.72rem, 4.6cqw, 1rem);
  line-height: 1.15;
  padding-inline: 2px;
}

@media (prefers-reduced-motion: reduce) {
  .is-spinning .bulb--odd,
  .is-spinning .bulb--even,
  .lottery-wheel__hit {
    animation: none;
  }
}
</style>
