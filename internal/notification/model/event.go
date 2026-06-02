// Package model defines notification domain models and events.
package model

// Event is the common shape we extract from RabbitMQ payloads. Producers
// emit slightly different JSON per routing key, but every payload carries
// at least one identifier (driver_id, reservation_id, or invoice_id) the
// consumer can use to resolve who to text.
type Event struct {
	DriverID      string `json:"driver_id,omitempty"`
	MSISDN        string `json:"msisdn,omitempty"`
	ReservationID string `json:"reservation_id,omitempty"`
	InvoiceID     string `json:"invoice_id,omitempty"`
	SpotID        string `json:"spot_id,omitempty"`
	TotalIDR      int64  `json:"total_idr,omitempty"`
}

// Routing keys we subscribe to.
const (
	EvtReservationConfirmed = "reservation.confirmed.v1"
	EvtReservationExpired   = "reservation.expired.v1"
	EvtReservationCancelled = "reservation.cancelled.v1"
	EvtInvoiceClosed        = "billing.invoice.closed.v1"
	EvtPaymentPaid          = "payment.paid.v1"
	EvtPaymentSuccess       = "billing.payment.success.v1"
	EvtPaymentFailed        = "billing.payment.failed.v1"
)

// AllRoutingKeys is the binding list for the consumer queue.
func AllRoutingKeys() []string {
	return []string{
		EvtReservationConfirmed,
		EvtReservationExpired,
		EvtReservationCancelled,
		EvtInvoiceClosed,
		EvtPaymentPaid,
		EvtPaymentSuccess,
		EvtPaymentFailed,
	}
}
