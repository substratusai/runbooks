package tui

import tea "github.com/charmbracelet/bubbletea"

// Keeping track of whats happening
type operationMap map[operation]status

func (m operationMap) UpdateOperations(msg tea.Msg) {
	op, ok := msg.(operationMsg)
	if ok {
		m[op.operation] = op.status
	}
}

type operation string

const (
	tarring        = operation("Tarring")
	applying       = operation("Applying")
	creating       = operation("Creating")
	uploading      = operation("Uploading")
	waitingReady   = operation("WaitingReady")
	syncingFiles   = operation("SyncingFiles")
	portForwarding = operation("PortForwarding")
)

type status int

const (
	notStarted = status(0)
	inProgress = status(1)
	completed  = status(2)
)
