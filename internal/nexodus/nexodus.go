package nexodus

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.uber.org/zap"
)

const (
	pollInterval = 5 * time.Second
)

const (
	// when nexd is first starting up
	NexdStatusStarting = iota
	// when nexd is waiting for auth and the user must complete the OTP auth flow
	NexdStatusAuth
	// nexd is up and running normally
	NexdStatusRunning
)

var (
	invalidTokenGrant = errors.New("invalid_grant")
)

type Nexodus struct {
	organization         uuid.UUID
	requestedIP          string
	userProvidedLocalIP  string
	LocalIP              string
	stun                 bool
	relayWgIP            string
	deviceCache          map[uuid.UUID]models.Device
	nodeReflexiveAddress string
	hostname             string
	logger               *zap.SugaredLogger

	// See the NexdStatus* constants
	status    int
	statusMsg string
	version   string
}
