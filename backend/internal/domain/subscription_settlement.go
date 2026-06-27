package domain

// Settlement action types describe how a subscription settlement order changes the chain.
const (
	SettlementActionPurchase = "purchase"
	SettlementActionRenew    = "renew"
	SettlementActionUpgrade  = "upgrade"
	SettlementActionRefund   = "refund"
	SettlementActionRevoke   = "revoke"
)

// Settlement action sources describe where the settlement value came from.
const (
	SettlementActionSourceUserPurchase       = "user_purchase"
	SettlementActionSourceExchangeCode       = "exchange_code"
	SettlementActionSourceSubscriptionAssign = "subscription_assign"
	SettlementActionSourceAdminRevoke        = "admin_revoke"
)

// Settlement statuses model the single current settlement cursor invariant.
const (
	SettlementStatusEffective = "effective"
	SettlementStatusClosed    = "closed"
)

// Settlement trigger reference types identify the business object linked to a settlement order.
const (
	SettlementTriggerRefPaymentOrder    = "payment_order"
	SettlementTriggerRefRedeemCode      = "redeem_code"
	SettlementTriggerRefAdminAssignment = "admin_assignment"
	SettlementTriggerRefDirectAction    = "direct_action"
)
