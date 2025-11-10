package tests

import (
	"testing"

	"github.com/Vova4o/VPSSetup/internal/interactive"
)

// TestShowSpinner tests the spinner creation
func TestShowSpinner(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "simple spinner",
			message: "Loading...",
		},
		{
			name:    "empty message",
			message: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := interactive.ShowSpinner(tt.message)

			if s == nil {
				t.Error("ShowSpinner() returned nil")
				return
			}

			// Stop the spinner immediately
			s.Stop()
		})
	}
}

// TestPublicFunctionsExist verifies all public functions are exported
func TestPublicFunctionsExist(t *testing.T) {
	tests := []struct {
		name string
		fn   interface{}
	}{
		{"ConfirmAction", interactive.ConfirmAction},
		{"SelectProfile", interactive.SelectProfile},
		{"SetupWizard", interactive.SetupWizard},
		{"ShowSpinner", interactive.ShowSpinner},
		{"Success", interactive.Success},
		{"Error", interactive.Error},
		{"Info", interactive.Info},
		{"Warning", interactive.Warning},
		{"AskMultiline", interactive.AskMultiline},
		{"AskInput", interactive.AskInput},
		{"AskDeploymentDetails", interactive.AskDeploymentDetails},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fn == nil {
				t.Errorf("Function %s is nil or not exported", tt.name)
			}
		})
	}
}

// BenchmarkShowSpinner benchmarks spinner creation
func BenchmarkShowSpinner(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := interactive.ShowSpinner("Loading...")
		s.Stop()
	}
}
