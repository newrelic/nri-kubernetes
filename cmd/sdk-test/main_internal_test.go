package main

import (
	"os"
	"os/exec"
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

	cmd := exec.Command(os.Args[0], "-test.main", "-timeout=10")
	go func() {
		output, err := cmd.CombinedOutput()
		outputCh <- output
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("should be still running: %v", err)
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
	}

	if output := <-outputCh; len(output) != 0 {
		t.Log(string(output))
	}
}

func Test_run(t *testing.T) {
	run()
	run()
}
