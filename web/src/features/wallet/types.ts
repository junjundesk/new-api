/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// ============================================================================
// Wallet Type Definitions
// ============================================================================

/**
 * Generic API response
 */
export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

/**
 * Standard API response types
 */
export type TopupInfoResponse = ApiResponse<TopupInfo>
export type RedemptionResponse = ApiResponse<number>
export type AmountResponse = ApiResponse<string>
export type PaymentResponse = ApiResponse<Record<string, unknown>> & {
  url?: string
}
export type StripePaymentResponse = ApiResponse<{ pay_link: string }>
export type AffiliateCodeResponse = ApiResponse<string>
export type AffiliateTransferResponse = ApiResponse
export type CreemPaymentResponse = ApiResponse<{ checkout_url: string }>
export type WaffoPaymentResponse = ApiResponse<
  { payment_url?: string } | string
>
export type WaffoPancakePaymentResponse = ApiResponse<
  | {
      checkout_url?: string
      session_id?: string
      expires_at?: number | string
      order_id?: string
      // Self-service session token + expiry — surfaced by the backend so
      // future flows (refund / cancel from new-api's own UI) can use them
      // without re-issuing checkout. Not consumed by the current handler.
      token?: string
      token_expires_at?: number | string
    }
  | string
>

/**
 * Creem product configuration
 */
export interface CreemProduct {
  /** Product display name */
  name: string
  /** Creem product ID */
  productId: string
  /** Product price */
  price: number
  /** Quota amount to credit */
  quota: number
  /** Currency (USD or EUR) */
  currency: 'USD' | 'EUR'
}

/**
 * Creem payment request
 */
export interface CreemPaymentRequest {
  /** Creem product ID */
  product_id: string
  /** Payment method identifier */
  payment_method: 'creem'
}

/**
 * Payment method configuration
 */
export interface PaymentMethod {
  /** Display name of payment method */
  name: string
  /** Payment method type identifier */
  type: string
  /** Legacy optional color for UI display */
  color?: string
  /** Minimum topup amount for this payment method */
  min_topup?: number
  /** Optional react-icons component name or safe icon URL */
  icon?: string
}

/**
 * Waffo payment method configuration
 */
export interface WaffoPayMethod {
  /** Display name of payment method */
  name: string
  /** Optional icon path */
  icon?: string
  /** Waffo pay method type */
  payMethodType?: string
  /** Waffo pay method name */
  payMethodName?: string
}

/**
 * Topup configuration information
 */
export interface TopupInfo {
  /** Whether online topup is enabled */
  enable_online_topup: boolean
  /** Whether Stripe topup is enabled */
  enable_stripe_topup: boolean
  /** Available payment methods */
  pay_methods: PaymentMethod[]
  /** Minimum topup amount for online topup */
  min_topup: number
  /** Minimum topup amount for Stripe */
  stripe_min_topup: number
  /** Preset amount options */
  amount_options: number[]
  /** Discount rates by amount */
  discount: Record<number, number>
  /** Optional topup link for purchasing codes */
  topup_link?: string
  /** Whether Creem topup is enabled */
  enable_creem_topup?: boolean
  /** Available Creem products */
  creem_products?: CreemProduct[]
  /** Whether Waffo topup is enabled */
  enable_waffo_topup?: boolean
  /** Available Waffo payment methods */
  waffo_pay_methods?: WaffoPayMethod[]
  /** Minimum topup amount for Waffo */
  waffo_min_topup?: number
  /** Whether Waffo Pancake topup is enabled */
  enable_waffo_pancake_topup?: boolean
  /** Minimum topup amount for Waffo Pancake */
  waffo_pancake_min_topup?: number
  /** Whether redemption code usage is enabled */
  enable_redemption?: boolean
  /** Whether compliance confirmation has been completed */
  payment_compliance_confirmed?: boolean
  /** Current compliance terms version */
  payment_compliance_terms_version?: string
}

/**
 * Preset amount option with optional discount
 */
export interface PresetAmount {
  /** Preset amount value */
  value: number
  /** Optional discount rate (0-1) */
  discount?: number
}

/**
 * Redemption code request
 */
export interface RedemptionRequest {
  /** Redemption code key */
  key: string
}

/**
 * Payment request parameters
 */
export interface PaymentRequest {
  /** Topup amount */
  amount: number
  /** Payment method identifier */
  payment_method: string
}

/**
 * Waffo payment request parameters
 */
export interface WaffoPaymentRequest {
  /** Topup amount */
  amount: number
  /** Optional server-side Waffo payment method index */
  pay_method_index?: number
}

/**
 * Waffo Pancake payment request parameters
 */
export interface WaffoPancakePaymentRequest {
  /** Topup amount */
  amount: number
}

/**
 * Amount calculation request
 */
export interface AmountRequest {
  /** Topup amount to calculate */
  amount: number
}

/**
 * Affiliate quota transfer request
 */
export interface AffiliateTransferRequest {
  /** Quota amount to transfer */
  quota: number
}

/**
 * One inviter commission bracket of the referral program
 */
export interface ReferralTier {
  /** Valid invitee count required for this tier */
  min_invitees: number
  /** Commission rate in basis points (100 = 1%) */
  rate_bps: number
}

/**
 * Invite page summary returned by /api/user/referral/summary
 */
export interface ReferralSummary {
  /** Whether the referral program is enabled site-wide */
  enabled: boolean
  /** The caller's invite code */
  aff_code?: string
  /** Reward recorded but not yet paid out */
  pending_quota: number
  /** Reward paid out to date */
  total_quota: number
  /** Reward in the claimable referral balance */
  claimable_quota: number
  /** Lifetime reward accrued */
  history_quota: number
  /** Invitees whose first top-up counted as valid */
  valid_invitees: number
  /** Users who registered with this invite code */
  registered_count: number
  /** Commission rate that applies right now */
  current_rate_bps: number
  /** Invitees needed for the next tier (0 = top tier) */
  next_tier_invitees: number
  /** Commission rate at the next tier */
  next_tier_rate_bps: number
  /** The invitee's own first top-up reward rate */
  invitee_rate_bps: number
  /** Whether the invitee-side reward is enabled */
  invitee_reward_enabled: boolean
  /** Whether the inviter-side commission is enabled */
  inviter_reward_enabled: boolean
  /** Reward bonus validity period, as a duration string */
  bonus_duration: string
  /** Whether redemption codes count toward tier progress */
  count_redeem_as_valid: boolean
  /** Commission ladder */
  tiers: ReferralTier[]
  /** Whether the admin confirmed payment compliance */
  compliance_confirmed: boolean
  /** Minimum quota for a balance transfer */
  transfer_min_quota: number
  /** Whether the withdrawal entry point is live (currently a placeholder) */
  withdrawal_available: boolean
}

/**
 * One referral reward record
 */
export interface ReferralCharge {
  id: number
  /** Recipient of the reward */
  user_id: number
  /** Inviter user id, 0 for invitee-side rewards */
  inviter_id: number
  /** Paying invitee */
  invitee_id: number
  /** 1 = inviter commission, 2 = invitee first-top-up reward */
  kind: number
  /** Quota base the percentage was applied to */
  base_quota: number
  /** Commission rate in basis points */
  rate_bps: number
  /** Reward amount in quota */
  amount: number
  /** 0 = pending, 1 = paid, 2 = canceled */
  status: number
  create_time: number
  settle_time: number
  remark: string
}

/**
 * Paginated referral reward history
 */
export interface ReferralChargePage {
  items: ReferralCharge[]
  total: number
  page: number
  page_size: number
}

export type ReferralSummaryResponse = ApiResponse<ReferralSummary>
export type ReferralChargeResponse = ApiResponse<ReferralChargePage>

/**
 * User wallet data
 */
export interface UserWalletData {
  /** User ID */
  id: number
  /** Username */
  username: string
  /** Current quota balance */
  quota: number
  /** Total used quota */
  used_quota: number
  /** Expiring bonus quota (赠金) */
  bonus_quota?: number
  /** Remaining bonus that expires */
  bonus_expiring_quota?: number
  /** Remaining bonus that never expires */
  bonus_permanent_quota?: number
  /** Nearest bonus expiry timestamp in seconds (0 = never / no bonus) */
  bonus_expire_time?: number
  /** Total request count */
  request_count: number
  /** Affiliate quota (pending rewards) */
  aff_quota: number
  /** Total affiliate quota earned (historical) */
  aff_history_quota: number
  /** Number of successful affiliate invites */
  aff_count: number
  /** User group */
  group: string
}

/**
 * Topup record status
 */
export type TopupStatus = 'success' | 'pending' | 'expired'

/**
 * Topup billing record
 */
export interface TopupRecord {
  /** Record ID */
  id: number
  /** User ID */
  user_id: number
  /** Topup amount (quota) */
  amount: number
  /** Payment amount (actual money paid) */
  money: number
  /** Trade/order number */
  trade_no: string
  /** Payment method type */
  payment_method: string
  /** Creation timestamp */
  create_time: number
  /** Completion timestamp */
  complete_time?: number
  /** Payment status */
  status: TopupStatus
}

/**
 * Billing history response
 */
export interface BillingHistoryResponse {
  items: TopupRecord[]
  total: number
}

/**
 * Complete order request (admin only)
 */
export interface CompleteOrderRequest {
  trade_no: string
}
