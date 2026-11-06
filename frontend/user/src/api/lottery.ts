import { userApi } from './client'

export type LotteryPrizeType = 'none' | 'balance' | 'coupon' | 'product' | 'custom'
export type LotteryDeliveryStatus = 'none' | 'pending' | 'delivered'

/** 服务端返回的"未中奖"兜底扇区名，前台用它换成本地化文案。 */
export const LOTTERY_MISS_SLICE = '__miss__'

export interface LotteryWheelSlice {
    index: number
    /** 0 表示兜底的未中奖扇区 */
    prize_id: number
    name: string
    type: LotteryPrizeType
    icon: string
    color: string
    description: string
    amount: string
    sold_out: boolean
}

export interface LotteryWheel {
    enabled: boolean
    title: string
    notice: string
    currency: string
    min_order_amount: string
    draws_per_order: number
    daily_draw_limit: number
    valid_hours: number
    slices: LotteryWheelSlice[]
}

export interface LotteryDraw {
    id: number
    user_id?: number
    order_no: string
    seq: number
    prize_id: number
    prize_name: string
    prize_type: LotteryPrizeType
    is_win: boolean
    amount: string
    currency: string
    coupon_code?: string
    product_id?: number | null
    product_name?: string
    delivery_status: LotteryDeliveryStatus
    delivery_remark?: string
    delivered_at?: string
    created_at: string
}

/** reason 说明为什么不能抽，前台据此展示对应文案。 */
export type LotteryChanceReason =
    | 'ok'
    | 'disabled'
    | 'order_not_found'
    | 'not_paid'
    | 'amount_too_low'
    | 'expired'
    | 'exhausted'
    | 'daily_limit'
    | 'no_prize'

export interface LotteryChance {
    order_no: string
    eligible: boolean
    reason: LotteryChanceReason
    total: number
    used: number
    remaining: number
    expires_at?: string
    draws: LotteryDraw[]
}

export interface LotteryDrawResult {
    draw: LotteryDraw
    /** 命中扇区由服务端决定，前端只负责把指针转到这个下标 */
    slice_index: number
    remaining: number
}

export interface LotteryDrawableOrder {
    order_no: string
    status: string
    total_amount: string
    currency: string
    paid_at?: string
    remaining: number
    expires_at?: string
}

export const lotteryAPI = {
    drawableOrders: () => userApi.get('/lottery/drawable-orders'),
    wheel: () => userApi.get('/lottery/wheel'),
    chance: (orderNo: string) =>
        userApi.get(`/lottery/orders/${encodeURIComponent(orderNo)}/chance`),
    draw: (orderNo: string) =>
        userApi.post(`/lottery/orders/${encodeURIComponent(orderNo)}/draw`),
    records: (params?: Record<string, unknown>) => userApi.get('/lottery/draws', { params }),
}
