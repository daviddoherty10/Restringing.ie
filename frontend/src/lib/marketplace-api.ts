import type { components } from './marketplace-contract'

export type BusinessUsage = components['schemas']['BusinessUsage']

type Schemas = components['schemas']
export type GripProduct = Schemas['GripProduct']
export type GripOffering = Schemas['GripOffering']
export type GripStock = Schemas['GripStock']
export type GripFilters = { q?: string; brand?: string; kind?: 'overgrip' | 'replacement_grip'; active?: boolean; grip_product_id?: string; page?: number; page_size?: number }
export type Product = Schemas['Product']
export type Racket = Schemas['Racket']
export type CatalogueVariant = Schemas['CatalogueVariant']
export type CatalogueFilters = SearchFilters & { active?: boolean; verified?: boolean; product_id?: string }
export type Booking = Schemas['Booking']
export type Quote = Schemas['Quote']
export type Order = Schemas['Order']
export type Variant = Schemas['Variant']
export type Service = Schemas['Service']
export type Stringer = Schemas['Stringer']
export type ManagementProfile = Schemas['ManagementProfile']
export type StockLot = Schemas['StockLot']
export type RecommendationRequest = Schemas['RecommendationRequest']
export type OrderStatus = 'placed' | 'accepted' | 'received' | 'in_progress' | 'ready' | 'completed' | 'cancelled'
export type SearchFilters = {
  q?: string
  page?: number
  page_size?: number
  sport?: 'tennis' | 'badminton' | 'squash'
  brand?: string
  material?: Variant['material']
  gauge_mm?: number
  county?: string
  variant_id?: string
  status?: OrderStatus
}

export class MarketplaceError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>
  readonly retryAfterSeconds: number

  constructor(status: number, details?: Schemas['Problem'], retryAfterSeconds = 60) {
    super(details?.message ?? 'The request could not be completed.')
    this.name = 'MarketplaceError'
    this.status = status
    this.retryAfterSeconds = retryAfterSeconds
    this.code = details?.code ?? 'request_failed'
    this.fields = details?.fields ?? {}
  }
}

type ClientOptions = {
  baseURL?: string
  // Prefer the HttpOnly session cookie. This hook is for explicit bearer clients;
  // it does not store tokens or introduce a development authentication bypass.
  getAccessToken?: () => string | undefined
}

export function createMarketplaceClient(options: ClientOptions = {}) {
  const base = (options.baseURL ?? import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8081/api/v1').replace(/\/$/, '')
  const id = encodeURIComponent

  async function request<T>(method: string, path: string, body?: unknown, query?: SearchFilters | CatalogueFilters | GripFilters | Record<string, string>, key?: string, signal?: AbortSignal): Promise<T> {
    const params = new URLSearchParams()
    for (const [name, value] of Object.entries(query ?? {})) {
      if (value !== undefined && value !== '') params.set(name, String(value))
    }
    const suffix = params.size ? `?${params}` : ''
    const headers = new Headers({ Accept: 'application/json' })
    if (body !== undefined) headers.set('Content-Type', 'application/json')
    if (key) headers.set('Idempotency-Key', key)
    const token = options.getAccessToken?.()
    if (token) headers.set('Authorization', `Bearer ${token}`)

    const response = await fetch(`${base}${path}${suffix}`, {
      method, headers, credentials: 'include', signal,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    const result: unknown = await response.json().catch(() => null)
    if (!response.ok) {
      const error = (result as { error?: Schemas['Problem'] } | null)?.error
      throw new MarketplaceError(response.status, error, Math.max(1, Math.min(3600, Number(response.headers.get('Retry-After')) || 60)))
    }
    return result as T
  }

  return {
    auth: {
      config: () => request<Schemas['AuthConfig']>('GET', '/auth/config'),
      requestCode: (phone: string) => request<Schemas['AuthChallenge']>('POST', '/auth/phone/request', { phone_number: phone }),
      verifyCode: (challengeID: string, code: string) => request<Schemas['Me']>('POST', '/auth/phone/verify', { challenge_id: challengeID, code }),
      logout: () => request<Schemas['LogoutResponse']>('POST', '/auth/logout'),
    },
    me: () => request<Schemas['Me']>('GET', '/me'),
    myStringers: () => request<Schemas['ManagementProfileList']>('GET', '/me/stringers'),
    grips: (filters: GripFilters = {}) => request<Schemas['GripProductPage']>('GET', '/grips', undefined, filters),
    offeredGrips: (stringerID: string, filters: GripFilters = {}) => request<Schemas['GripOfferingPage']>('GET', `/stringers/${id(stringerID)}/grips`, undefined, filters),
    manageGrips: (stringerID: string, filters: GripFilters = {}) => request<Schemas['GripStockPage']>('GET', `/stringers/${id(stringerID)}/manage/grips`, undefined, filters),
    saveGripOffering: (stringerID: string, productID: string, input: Schemas['GripOfferingInput']) => request<Schemas['GripStock']>('PUT', `/stringers/${id(stringerID)}/grips/${id(productID)}`, input),
    adjustGripStock: (stringerID: string, productID: string, input: Schemas['GripStockInput'], key: string) => request<Schemas['GripMovement']>('POST', `/stringers/${id(stringerID)}/grips/${id(productID)}/stock-events`, input, undefined, key),
    gripMovements: (stringerID: string, productID: string, filters: SearchFilters = {}) => request<Schemas['GripMovementPage']>('GET', `/stringers/${id(stringerID)}/grips/${id(productID)}/stock-events`, undefined, filters),
    strings: (filters: SearchFilters = {}, signal?: AbortSignal) => request<Schemas['VariantPage']>('GET', '/strings', undefined, filters, undefined, signal),
    rackets: (filters: SearchFilters = {}, signal?: AbortSignal) => request<Schemas['RacketPage']>('GET', '/rackets', undefined, filters, undefined, signal),
    stringers: (filters: SearchFilters = {}, signal?: AbortSignal) => request<Schemas['StringerPage']>('GET', '/stringers', undefined, filters, undefined, signal),
    stringer: (stringerID: string) => request<Stringer>('GET', `/stringers/${id(stringerID)}`),
    services: (stringerID: string) => request<Schemas['ServiceList']>('GET', `/stringers/${id(stringerID)}/services`),
    inventory: (stringerID: string, filters: SearchFilters = {}, signal?: AbortSignal) => request<Schemas['VariantPage']>('GET', `/stringers/${id(stringerID)}/inventory`, undefined, filters, undefined, signal),
    bookingConfiguration: (stringerID: string, racketID: string) => request<Schemas['BookingOptions']>('GET', `/stringers/${id(stringerID)}/booking-configuration`, undefined, { racket_model_id: racketID }),
    recommend: (stringerID: string, answers: RecommendationRequest, signal?: AbortSignal) => request<Schemas['Recommendations']>('POST', `/stringers/${id(stringerID)}/recommendations`, answers, undefined, undefined, signal),
    quote: (stringerID: string, booking: Booking, signal?: AbortSignal) => request<Quote>('POST', `/stringers/${id(stringerID)}/quotes`, booking, undefined, undefined, signal),
    // Generate a key once per attempted purchase and reuse it after timeouts.
    createOrder: (quoteID: string, idempotencyKey: string) => request<Order>('POST', '/orders', { quote_id: quoteID, payment_method: 'cash' }, undefined, idempotencyKey),
    orders: (filters: SearchFilters = {}) => request<Schemas['OrderPage']>('GET', '/orders', undefined, filters),
    order: (orderID: string) => request<Order>('GET', `/orders/${id(orderID)}`),
    tracking: (orderID: string) => request<Schemas['OrderTracking']>('GET', `/orders/${id(orderID)}/tracking`),
    trackingUpdates: (orderID: string, filters: SearchFilters = {}) => request<Schemas['TrackingUpdatePage']>('GET', `/orders/${id(orderID)}/tracking-updates`, undefined, filters),
    requestCollection: (orderID: string, input: Schemas['CollectionRequest'], key: string) => request<Schemas['TrackingUpdate']>('POST', `/orders/${id(orderID)}/collection-request`, input, undefined, key),
    updateTracking: (orderID: string, input: Schemas['TrackingInput'], key: string) => request<Schemas['TrackingUpdate']>('POST', `/orders/${id(orderID)}/tracking-updates`, input, undefined, key),
    messages: (orderID: string, filters: SearchFilters = {}) => request<Schemas['MessagePage']>('GET', `/orders/${id(orderID)}/messages`, undefined, filters),
    sendMessage: (orderID: string, body: string, key: string) => request<Schemas['OrderMessage']>('POST', `/orders/${id(orderID)}/messages`, { body }, undefined, key),
    readMessages: (orderID: string, lastMessageID: string) => request<Schemas['ReadResult']>('POST', `/orders/${id(orderID)}/messages/read`, { last_message_id: lastMessageID }),
    notifications: (filters: SearchFilters = {}) => request<Schemas['NotificationPage']>('GET', '/notifications', undefined, filters),
    readNotification: (notificationID: string) => request<Schemas['ReadResult']>('POST', `/notifications/${id(notificationID)}/read`, {}),
    incomingOrder: (stringerID: string, orderID: string) => request<Schemas['IncomingOrder']>('GET', `/stringers/${id(stringerID)}/orders/${id(orderID)}`),
    incomingOrders: (stringerID: string, filters: SearchFilters = {}) => request<Schemas['OrderPage']>('GET', `/stringers/${id(stringerID)}/orders`, undefined, filters),
    updateOrderStatus: (orderID: string, status: OrderStatus) => request<Order>('PATCH', `/orders/${id(orderID)}/status`, { status }),
    applyStringer: (profile: Schemas['ProfileInput']) => request<Stringer>('POST', '/stringers', profile),
    updateProfile: (stringerID: string, profile: Schemas['ProfileInput']) => request<Stringer>('PUT', `/stringers/${id(stringerID)}`, profile),
    managementProfile: (stringerID: string) => request<ManagementProfile>('GET', `/stringers/${id(stringerID)}/manage/profile`),
    businessUsage: (stringerID: string) => request<BusinessUsage>('GET', `/stringers/${id(stringerID)}/billing`),
    subscriptionCheckout: (stringerID: string) => request<Schemas['RedirectURL']>('POST', `/stringers/${id(stringerID)}/billing/checkout`, {}),
    subscriptionPortal: (stringerID: string) => request<Schemas['RedirectURL']>('POST', `/stringers/${id(stringerID)}/billing/portal`, {}),
    confirmCashReceipt: (orderID: string) => request<Order>('POST', `/orders/${id(orderID)}/cash-receipt`, {}),
    orderSummary: (stringerID: string) => request<Schemas['OrderSummary']>('GET', `/stringers/${id(stringerID)}/manage/summary`),
    manageServices: (stringerID: string) => request<Schemas['ServiceList']>('GET', `/stringers/${id(stringerID)}/manage/services`),
    saveService: (stringerID: string, policy: Schemas['ServiceInput']) => request<Service>('PUT', `/stringers/${id(stringerID)}/services/${policy.sport}`, policy),
    manageInventory: (stringerID: string, filters: SearchFilters = {}) => request<Schemas['VariantPage']>('GET', `/stringers/${id(stringerID)}/manage/inventory`, undefined, filters),
    saveInventory: (stringerID: string, variantID: string, offering: Schemas['InventoryInput']) => request<Schemas['IDResponse']>('PUT', `/stringers/${id(stringerID)}/inventory/${id(variantID)}`, offering),
    lots: (stringerID: string, filters: SearchFilters = {}) => request<Schemas['LotPage']>('GET', `/stringers/${id(stringerID)}/inventory/lots`, undefined, filters),
    receiveStock: (stringerID: string, variantID: string, lot: Schemas['LotInput']) => request<Schemas['Lot']>('POST', `/stringers/${id(stringerID)}/inventory/${id(variantID)}/lots`, lot),
    adjustStock: (stringerID: string, lotID: string, adjustment: Schemas['AdjustmentInput']) => request<Schemas['Lot']>('POST', `/stringers/${id(stringerID)}/inventory/lots/${id(lotID)}/adjustments`, adjustment),
    admin: {
      grips: (filters: GripFilters = {}) => request<Schemas['GripProductPage']>('GET', '/admin/grips', undefined, filters),
      createGrip: (input: Schemas['GripInput'], key: string) => request<GripProduct>('POST', '/admin/grips', input, undefined, key),
      updateGrip: (productID: string, input: Schemas['GripInput']) => request<GripProduct>('PUT', `/admin/grips/${id(productID)}`, input),
      products: (filters: CatalogueFilters = {}) => request<Schemas['ProductPage']>('GET', '/admin/string-products', undefined, filters),
      product: (productID: string) => request<Product>('GET', `/admin/string-products/${id(productID)}`),
      variants: (filters: CatalogueFilters = {}) => request<Schemas['CatalogueVariantPage']>('GET', '/admin/string-variants', undefined, filters),
      variant: (variantID: string) => request<CatalogueVariant>('GET', `/admin/string-variants/${id(variantID)}`),
      rackets: (filters: CatalogueFilters = {}) => request<Schemas['RacketPage']>('GET', '/admin/rackets', undefined, filters),
      racket: (racketID: string) => request<Racket>('GET', `/admin/rackets/${id(racketID)}`),
      applications: (filters: SearchFilters = {}) => request<Schemas['StringerPage']>('GET', '/admin/stringer-applications', undefined, filters),
      reviewStringer: (stringerID: string, status: 'approved' | 'rejected' | 'suspended', reason: string) => request<Stringer>('PATCH', `/admin/stringers/${id(stringerID)}/approval`, { status, reason }),
      createProduct: (product: Schemas['ProductInput']) => request<Schemas['Product']>('POST', '/admin/string-products', product),
      updateProduct: (productID: string, product: Schemas['ProductInput']) => request<Schemas['Product']>('PUT', `/admin/string-products/${id(productID)}`, product),
      createVariant: (variant: Schemas['VariantInput']) => request<Schemas['IDResponse']>('POST', '/admin/string-variants', variant),
      updateVariant: (variantID: string, variant: Schemas['VariantInput']) => request<Schemas['IDResponse']>('PUT', `/admin/string-variants/${id(variantID)}`, variant),
      createRacket: (racket: Schemas['RacketInput']) => request<Schemas['Racket']>('POST', '/admin/rackets', racket),
      updateRacket: (racketID: string, racket: Schemas['RacketInput']) => request<Schemas['Racket']>('PUT', `/admin/rackets/${id(racketID)}`, racket),
    },
  }
}

export const marketplaceAPI = createMarketplaceClient()
