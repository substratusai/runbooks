package tui

import (
	"context"
	"log"
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/substratusai/substratus/internal/cli/client"
)

type portForwardReadyMsg struct{}

func portForwardCmd(ctx context.Context, c client.Interface, podRef types.NamespacedName) tea.Cmd {
	return func() tea.Msg {
		const maxRetries = 3
		for i := 0; i < maxRetries; i++ {
			portFwdCtx, cancelPortFwd := context.WithCancel(ctx)
			defer cancelPortFwd() // Avoid a context leak
			runtime.ErrorHandlers = []func(err error){
				func(err error) {
					// Cancel a broken port forward to attempt to restart the port-forward.
					log.Printf("Port-forward error: %v", err)
					cancelPortFwd()
				},
			}

			// portForward will close the ready channel when it returns.
			// so we only use the outer ready channel once. On restart of the portForward,
			// we use a new channel.
			ready := make(chan struct{})
			go func() {
				log.Println("Waiting for port-forward to be ready")
				<-ready
				log.Println("Port-forward ready")
				P.Send(portForwardReadyMsg{})
			}()

			if err := c.PortForward(portFwdCtx, LogFile, podRef, ready); err != nil {
				log.Printf("Port-forward returned an error: %v", err)
			}

			// Check if the command's context is cancelled, if so,
			// avoid restarting the port forward.
			if err := ctx.Err(); err != nil {
				log.Printf("Context done, not attempting to restart port-forward: %v", err.Error())
				return nil
			}

			cancelPortFwd() // Avoid a build up of contexts before returning.
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			log.Printf("Restarting port forward (index = %v), after backoff: %s", i, backoff)
			time.Sleep(backoff)
		}
		log.Println("Done trying to port-forward")

		return nil
	}
}
