package grainactor

import (
	"errors"
	"fmt"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

var (
	// ErrPeerSessionNotBound reports that a grain actor has no active peer session.
	ErrPeerSessionNotBound = errors.New("grainactor: peer session not bound")
	// ErrPeerSessionActorNotLocal reports that the peer session binding target is not local.
	ErrPeerSessionActorNotLocal = errors.New("grainactor: peer session actor not local")
	// ErrPeerSessionIdentityMismatch reports that a binding message targeted a different grain identity.
	ErrPeerSessionIdentityMismatch = errors.New("grainactor: peer session identity mismatch")
)

const peerSessionMessageTimeout = time.Second

// PeerSession is a local runtime capability for writing to and closing a peer.
type PeerSession interface {
	SendData(payload []byte) error
	Close(reason string) error
}

// PeerSessionReceiver receives direct peer session assignment callbacks.
type PeerSessionReceiver interface {
	SetPeerSession(session PeerSession)
	ClearPeerSession(session PeerSession)
}

// PeerSessionBind binds a local peer session to a grain actor.
//
// The message carries a process-local handle and is not suitable for remote
// transport serialization. Prefer BindPeerSession, which rejects non-local PIDs.
type PeerSessionBind struct {
	Identity string
	Kind     string
	Session  PeerSession
}

// PeerSessionClear clears a local peer session from a grain actor if it is current.
//
// The message carries a process-local handle and is not suitable for remote
// transport serialization. Prefer ClearPeerSession, which rejects non-local PIDs.
type PeerSessionClear struct {
	Identity string
	Kind     string
	Session  PeerSession
}

type peerSessionResult struct {
	err error
}

// BindPeerSession binds session to a locally activated grain actor through its mailbox.
func BindPeerSession(sender actor.SenderContext, pid *actor.PID, identity string, kind string, session PeerSession) error {
	if err := validateLocalPeerSessionTarget(sender, pid); err != nil {
		return err
	}
	result, err := sender.RequestFuture(pid, &PeerSessionBind{
		Identity: identity,
		Kind:     kind,
		Session:  session,
	}, peerSessionMessageTimeout).Result()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPeerSessionActorNotLocal, err)
	}
	return peerSessionResultError(result)
}

// ClearPeerSession clears session from a locally activated grain actor through its mailbox.
func ClearPeerSession(sender actor.SenderContext, pid *actor.PID, identity string, kind string, session PeerSession) error {
	if err := validateLocalPeerSessionTarget(sender, pid); err != nil {
		return err
	}
	result, err := sender.RequestFuture(pid, &PeerSessionClear{
		Identity: identity,
		Kind:     kind,
		Session:  session,
	}, peerSessionMessageTimeout).Result()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPeerSessionActorNotLocal, err)
	}
	return peerSessionResultError(result)
}

func validateLocalPeerSessionTarget(sender actor.SenderContext, pid *actor.PID) error {
	if sender == nil || sender.ActorSystem() == nil || pid == nil {
		return ErrPeerSessionActorNotLocal
	}
	if pid.GetAddress() != sender.ActorSystem().Address() {
		return ErrPeerSessionActorNotLocal
	}
	if _, ok := sender.ActorSystem().ProcessRegistry.Get(pid); !ok {
		return ErrPeerSessionActorNotLocal
	}
	return nil
}

func peerSessionResultError(result interface{}) error {
	peerSessionResult, ok := result.(*peerSessionResult)
	if !ok {
		return fmt.Errorf("%w: unexpected response %T", ErrPeerSessionActorNotLocal, result)
	}
	return peerSessionResult.err
}

func peerSessionNilError() error {
	return fmt.Errorf("%w: nil session", ErrPeerSessionNotBound)
}
