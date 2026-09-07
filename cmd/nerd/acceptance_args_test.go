package main

import (
	"github.com/spf13/cobra"
	"testing"
)

func TestFixContractSuppliesTheTask(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("acceptance", "", "")
	if err := validateFixArgs(cmd, nil); err == nil {
		t.Fatal("ordinary fix lost required target")
	}
	if err := cmd.Flags().Set("acceptance", "contract.json"); err != nil {
		t.Fatal(err)
	}
	if err := validateFixArgs(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if err := validateFixArgs(cmd, []string{"a different task"}); err == nil {
		t.Fatal("ambiguous acceptance authority accepted")
	}
}
