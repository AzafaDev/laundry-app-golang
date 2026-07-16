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

	StatusWaitingPickupDriver  = "waiting_pickup_driver"
	StatusLaundryToOutlet      = "laundry_to_outlet"
	StatusLaundryArrivedOutlet = "laundry_arrived_outlet"
	StatusWashing              = "washing"
	StatusIroning              = "ironing"
	StatusPacking              = "packing"
	StatusWaitingPayment       = "waiting_payment"
	StatusReadyForDelivery     = "ready_for_delivery"
	StatusDeliveryToCustomer   = "delivery_to_customer"
	StatusReceivedByCustomer   = "received_by_customer"
	StatusCompleted            = "completed"

	invoiceNumberMaxAttempts = 5

	// MaxBypassAttemptsPerStation is the max number of non-pending bypass
	// requests allowed per (order, station) before further requests are
	// rejected outright.
	MaxBypassAttemptsPerStation = 2
)

// stationNextStatus maps a worker station's current order status to the
// status it transitions to on completion. Packing always goes to
// waiting_payment — payment (ticket #2) doesn't exist yet, so the TS
// "skip to ready_for_delivery if already paid" override is not replicated.
var stationNextStatus = map[string]string{
	StatusWashing: StatusIroning,
	StatusIroning: StatusPacking,
	StatusPacking: StatusWaitingPayment,
}

// stationForRole maps a worker role to the single station it's allowed to
// operate on.
var stationForRole = map[string]string{
	"washing_worker": StatusWashing,
	"ironing_worker": StatusIroning,
	"packing_worker": StatusPacking,
}

// claimNextStatus/claimOldStatus and completeNextStatus/completeOldStatus
// map a driver_tasks.task_type to the order status transition it drives.
var claimNextStatus = map[string]string{
	"pickup":   StatusLaundryToOutlet,
	"delivery": StatusDeliveryToCustomer,
}

var claimOldStatus = map[string]string{
	"pickup":   StatusWaitingPickupDriver,
	"delivery": StatusReadyForDelivery,
}

var completeNextStatus = map[string]string{
	"pickup":   StatusLaundryArrivedOutlet,
	"delivery": StatusReceivedByCustomer,
}

var completeOldStatus = map[string]string{
	"pickup":   StatusLaundryToOutlet,
	"delivery": StatusDeliveryToCustomer,
}
