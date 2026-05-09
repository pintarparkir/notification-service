package template

import (
	"strings"
	"testing"

	"github.com/farid/notification-service/internal/notification/model"
)

func TestRender_AllSupportedRoutingKeys(t *testing.T) {
	cases := []struct {
		key   string
		event model.Event
		// substrings the rendered body MUST contain so we know the right
		// template fired (avoids hard-coding the full Indonesian copy).
		musts []string
	}{
		{model.EvtReservationConfirmed, model.Event{SpotID: "F1-C-001"}, []string{"F1-C-001", "check-in"}},
		{model.EvtReservationConfirmed, model.Event{}, []string{"check-in"}}, // graceful when SpotID missing
		{model.EvtReservationExpired, model.Event{}, []string{"melewatkan", "no-show"}},
		{model.EvtReservationCancelled, model.Event{}, []string{"dibatalkan"}},
		{model.EvtInvoiceClosed, model.Event{TotalIDR: 25000}, []string{"25.000", "QRIS"}},
		{model.EvtInvoiceClosed, model.Event{}, []string{"QRIS"}}, // amount missing → still meaningful
		{model.EvtPaymentPaid, model.Event{}, []string{"Pembayaran", "diterima"}},
	}
	for _, tc := range cases {
		got := Render(tc.key, tc.event)
		if got == "" {
			t.Errorf("%s: got empty body", tc.key)
			continue
		}
		for _, m := range tc.musts {
			if !strings.Contains(got, m) {
				t.Errorf("%s: %q missing from %q", tc.key, m, got)
			}
		}
	}
}

func TestRender_UnknownKeyReturnsEmpty(t *testing.T) {
	if Render("user.phone_changed.v1", model.Event{}) != "" {
		t.Errorf("unknown routing key should return empty body so consumer drops it")
	}
}

func TestFormatIDR(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{50, "50"},
		{999, "999"},
		{1000, "1.000"},
		{1234, "1.234"},
		{10000, "10.000"},
		{1234567, "1.234.567"},
	}
	for _, tc := range cases {
		if got := formatIDR(tc.in); got != tc.want {
			t.Errorf("formatIDR(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
