import { api } from './client'

export interface TrackVisitPayload {
    scope: 'home' | 'product'
    target?: string
    visitor_key?: string
}

export const visitAPI = {
    // 埋点失败不应影响前台流程，调用方自行忽略异常。
    track: (data: TrackVisitPayload) =>
        api.post('/public/visits', data, { silentBusinessError: true }),
}
