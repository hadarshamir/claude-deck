package terminal

import (
	"os/exec"
	"testing"
)

func TestKittyAvailable(t *testing.T) {
	// Skip if kitty is not available
	if _, err := exec.LookPath("kitty"); err != nil {
		t.Skip("kitty not available, skipping kitty tests")
	}

	// Just verify kitty @ commands don't panic with invalid window IDs
	// These should fail gracefully
	_ = focusKittyWindow(-1)
	_ = CloseKittyWindow(-1)
	_ = CloseKittyWindow(0)
	_ = SetKittyTabTitle(-1, "test")
	_ = SetKittyTabTitle(0, "test")
	_ = SetKittyTabTitle(1, "")
	_ = ResetKittyTabTitle(-1)
	_ = ResetKittyTabTitle(0)
}

func TestCloseKittyWindowInvalidID(t *testing.T) {
	// Should return nil for invalid IDs (early return)
	if err := CloseKittyWindow(0); err != nil {
		t.Errorf("CloseKittyWindow(0) = %v, want nil", err)
	}
	if err := CloseKittyWindow(-1); err != nil {
		t.Errorf("CloseKittyWindow(-1) = %v, want nil", err)
	}
}

func TestSetKittyTabTitleInvalidArgs(t *testing.T) {
	// Should return nil for invalid arguments (early return)
	if err := SetKittyTabTitle(0, "test"); err != nil {
		t.Errorf("SetKittyTabTitle(0, 'test') = %v, want nil", err)
	}
	if err := SetKittyTabTitle(-1, "test"); err != nil {
		t.Errorf("SetKittyTabTitle(-1, 'test') = %v, want nil", err)
	}
	if err := SetKittyTabTitle(1, ""); err != nil {
		t.Errorf("SetKittyTabTitle(1, '') = %v, want nil", err)
	}
}

func TestResetKittyTabTitleInvalidID(t *testing.T) {
	// Should return nil for invalid IDs (early return)
	if err := ResetKittyTabTitle(0); err != nil {
		t.Errorf("ResetKittyTabTitle(0) = %v, want nil", err)
	}
	if err := ResetKittyTabTitle(-1); err != nil {
		t.Errorf("ResetKittyTabTitle(-1) = %v, want nil", err)
	}
}
