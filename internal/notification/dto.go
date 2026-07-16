package notification

// Notification type constants — plain strings, no DB CHECK constraint,
// mirroring the TS source's union type (validated at the application layer
// only, not the schema).
const (
	TypeOrderDetails          = "order_details"
	TypePayment               = "payment"
	TypePaymentCompleted      = "payment_completed"
	TypeOrderUpdate           = "order_update"
	TypeDriverPickupStarted   = "driver_pickup_started"
	TypeDriverDeliveryStarted = "driver_delivery_started"
	TypeDriverArrivedOutlet   = "driver_arrived_outlet"
	TypeDriverArrivedCustomer = "driver_arrived_customer"
	TypeComplaintUpdate       = "complaint_update"
)

type NotificationResponse struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	Type            string `json:"type"`
	RelatedEntityID string `json:"related_entity_id,omitempty"`
	IsRead          bool   `json:"is_read"`
	CreatedAt       string `json:"created_at"`
}

type NotificationListResponse struct {
	Data       []NotificationResponse `json:"data"`
	TotalCount int64                  `json:"total_count"`
}

type UnreadCountResponse struct {
	UnreadCount int64 `json:"unread_count"`
}
