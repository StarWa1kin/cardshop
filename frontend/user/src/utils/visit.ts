import type { RouteLocationNormalized, RouteLocationNormalizedLoaded } from 'vue-router'
import { visitAPI } from '../api'
import type { TrackVisitPayload } from '../api/visit'

const VISITOR_STORAGE_KEY = 'dj_visitor_key'

// 同一个目标在这个窗口内只上报一次：路由重定向、hash 变化都会触发 afterEach，
// 不去重会把 PV 打虚。真实的二次浏览间隔远大于这个窗口，不受影响。
const DEDUPE_WINDOW_MS = 1500

type TrackedRoute = RouteLocationNormalized | RouteLocationNormalizedLoaded

let lastTrackedKey = ''
let lastTrackedAt = 0

const ensureVisitorKey = () => {
    if (typeof window === 'undefined') return ''
    try {
        const existing = String(localStorage.getItem(VISITOR_STORAGE_KEY) || '').trim()
        if (existing) return existing
        const next = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 12)}`
        localStorage.setItem(VISITOR_STORAGE_KEY, next)
        return next
    } catch {
        // 隐私模式下 localStorage 不可用，后端会退化为 IP + UA 指纹。
        return ''
    }
}

const resolvePayload = (route: TrackedRoute): TrackVisitPayload | null => {
    if (route.name === 'home') {
        return { scope: 'home' }
    }
    if (route.name === 'product-detail') {
        const raw = Array.isArray(route.params.slug) ? route.params.slug[0] : route.params.slug
        const slug = String(raw || '').trim()
        if (!slug) return null
        return { scope: 'product', target: slug }
    }
    return null
}

export const trackRouteVisit = (route: TrackedRoute) => {
    if (typeof window === 'undefined') return
    const payload = resolvePayload(route)
    if (!payload) return

    const key = `${payload.scope}:${payload.target || ''}`
    const now = Date.now()
    if (key === lastTrackedKey && now - lastTrackedAt < DEDUPE_WINDOW_MS) return
    lastTrackedKey = key
    lastTrackedAt = now

    const visitorKey = ensureVisitorKey()
    void visitAPI
        .track(visitorKey ? { ...payload, visitor_key: visitorKey } : payload)
        .catch(() => {})
}
