package order

const (
	// MaxPickupDaysAhead is the furthest a customer may schedule pickup_date
	// into the future, measured from today.
	MaxPickupDaysAhead = 7

	// FreeDeliveryRadiusKM is the max outlet-to-pickup distance still eligible
	// for free delivery; beyond it, FlatDeliveryFee applies.
	FreeDeliveryRadiusKM = 5.0

	// FlatDeliveryFee is charged whenever distance exceeds FreeDeliveryRadiusKM.
	FlatDeliveryFee = 10_000

	StatusWaitingPickupDriver = "waiting_pickup_driver"
	StatusReceivedByCustomer  = "received_by_customer"

	invoiceNumberMaxAttempts = 5
)
