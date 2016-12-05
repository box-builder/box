package signal

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var currentChannel chan os.Signal

// SetSignal It handles INT and TERM signals. If a hook is provided, it will be
// run at signal handling time, before terminating the program.
//
// This is not threadsafe code.
func SetSignal(hook func()) {
	intSig := make(chan os.Signal)
	signal.Notify(intSig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_, ok := <-intSig
		if ok {
			if hook != nil {
				hook()
			}

			fmt.Println("\n\n!!! SIGINT or SIGTERM recieved, crashing container...")
			os.Exit(1)
		}
	}()

	if currentChannel != nil {
		signal.Stop(currentChannel)
		close(currentChannel)
	}

	currentChannel = intSig
}
