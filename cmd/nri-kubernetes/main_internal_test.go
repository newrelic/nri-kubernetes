package main

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	for i, arg := range os.Args {
		if arg == "-test.main" {
			os.Args = append(os.Args[:i], os.Args[i+1:]...)
			main()
			return
		}
	}

	os.Exit(m.Run())
}

func Test_main_accepts_CLI_flags(t *testing.T) {
	outputCh := make(chan []byte, 1)
	errCh := make(chan error)

	cmd := exec.Command(os.Args[0], "-test.main", "-interval_seconds=30")
	go func() {
		output, err := cmd.CombinedOutput()
		outputCh <- output
		errCh <- err
	}()

	timeout := time.NewTimer(time.Second)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("should be still running: %v", err)
		}
	case <-timeout.C:
		if err := cmd.Process.Kill(); err != nil {
			t.Fatalf("Sending signal to process failed: %v", err)
		}
	}

	if output := <-outputCh; len(output) != 0 {
		t.Log(string(output))
	}
}

func Test_main_gracefully_handles(t *testing.T) {
	t.Parallel()
	for name, sig := range map[string]syscall.Signal{
		"interrupt_signal": syscall.SIGINT,
		"terminate_signal": syscall.SIGTERM,
	} {
		sig := sig
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			outputCh := make(chan []byte, 1)
			errCh := make(chan error)

			cmd := exec.Command(os.Args[0], "-test.main")
			go func() {
				output, err := cmd.CombinedOutput()
				outputCh <- output
				errCh <- err
			}()

			func() {
				termSent := false

				startTimeout := time.NewTimer(time.Second)
				termTimeout := time.NewTimer(2 * time.Second)

				for {
					select {
					case err := <-errCh:
						if err != nil {
							t.Fatalf("Executing process failed: %v", err)
						}
						return
					case <-startTimeout.C:
						if !termSent {
							if err := cmd.Process.Signal(sig); err != nil {
								t.Fatalf("Sending TERM signal to process failed: %v", err)
							}
							termSent = true
						}
					case <-termTimeout.C:
						if err := cmd.Process.Kill(); err != nil {
							t.Fatalf("Killing process failed: %v", err)
						}
						return
					}
				}
			}()

			if output := <-outputCh; len(output) != 0 {
				t.Log(string(output))
			}
		})
	}
}
