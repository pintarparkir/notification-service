// Package template renders SMS bodies from event payloads.
//
// Templates are pure functions of (Event, routing-key) → string. No I/O.
// Adding a new routing key means: add the constant in model/event.go, then
// add a case here. Adding a translation = swap text.Template for a renderer
// keyed on locale; out of MVP scope.
package template

import (
	"fmt"

	"github.com/farid/notification-service/internal/notification/model"
)

// Render returns the SMS body for the given routing key + event payload.
// Returns "" when the routing key is unknown or the event is incomplete —
// the consumer should treat empty as "drop, don't send".
func Render(routingKey string, ev model.Event) string {
	switch routingKey {
	case model.EvtReservationConfirmed:
		if ev.SpotID == "" {
			return "Reservasi Anda dikonfirmasi. Silakan check-in dalam 1 jam."
		}
		return fmt.Sprintf("Reservasi Anda di slot %s dikonfirmasi. Silakan check-in dalam 1 jam.", ev.SpotID)

	case model.EvtReservationExpired:
		return "Anda melewatkan jendela check-in. Reservasi otomatis dibatalkan; biaya no-show diterapkan."

	case model.EvtReservationCancelled:
		return "Reservasi Anda telah dibatalkan."

	case model.EvtInvoiceClosed:
		if ev.TotalIDR > 0 {
			return fmt.Sprintf("Tagihan Anda sebesar Rp%s. Silakan bayar via QRIS di aplikasi.", formatIDR(ev.TotalIDR))
		}
		return "Tagihan Anda telah ditutup. Silakan bayar via QRIS di aplikasi."

	case model.EvtPaymentPaid:
		return "Pembayaran diterima. Terima kasih telah menggunakan ParkirPintar."
	}
	return ""
}

// formatIDR adds thousand-separator dots (Indonesian convention).
//   5000     → "5.000"
//   1234567  → "1.234.567"
func formatIDR(n int64) string {
	if n < 0 {
		return "-" + formatIDR(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return formatIDR(n/1000) + fmt.Sprintf(".%03d", n%1000)
}
