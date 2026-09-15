package autopoiesis

import (
	"testing"
)

// Adversarial probes against the self-modification safety gate.
// Default config denies filesystem, networking, and exec.

func checkWithDefaultConfig(t *testing.T, code string) *SafetyReport {
	t.Helper()
	sc := NewSafetyChecker(OuroborosConfig{})
	if err := sc.PolicyLoadError(); err != nil {
		t.Fatalf("policy failed to load: %v", err)
	}
	return sc.Check(code)
}

func TestSafetyBenignPasses(t *testing.T) {
	report := checkWithDefaultConfig(t, `package main

import ("fmt"; "strings")

func Run() string { return strings.ToUpper(fmt.Sprintf("%s", "ok")) }
`)
	if !report.Safe {
		t.Fatalf("benign code rejected: %+v", report.Violations)
	}
}

func TestSafetyRejectsDangerClasses(t *testing.T) {
	cases := map[string]string{
		"os import": `package main
import "os"
func Run() string { return os.Getenv("X") }
`,
		"os/exec import": `package main
import "os/exec"
func Run() string { out, _ := exec.Command("id").Output(); return string(out) }
`,
		"net import": `package main
import "net"
func Run() string { c, _ := net.Dial("tcp", "x:1"); _ = c; return "x" }
`,
		"panic call": `package main
import "fmt"
func Run() string { panic(fmt.Sprintf("boom")) }
`,
		"goroutine without context": `package main
func Run() string { go func() {}(); return "x" }
`,
		"log.Fatal exits process": `package main
import "log"
func Run() string { log.Fatal("boom"); return "x" }
`,
		"syscall import": `package main
import "syscall"
func Run() string { syscall.Exit(1); return "x" }
`,
		"unsafe import": `package main
import "unsafe"
func Run() string { var x int; return string((*[1]byte)(unsafe.Pointer(&x)))[:] }
`,
	}
	for name, code := range cases {
		report := checkWithDefaultConfig(t, code)
		if report.Safe {
			t.Errorf("%s: ACCEPTED, want rejected", name)
		}
	}
}

func TestSafetyUnparseableFailsClosed(t *testing.T) {
	report := checkWithDefaultConfig(t, `this is not go code {{{`)
	if report.Safe {
		t.Fatal("unparseable code accepted (fail-open!)")
	}
}
