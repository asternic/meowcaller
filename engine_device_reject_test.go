package meowcaller

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestDeviceEncryptionReject(t *testing.T) {
	primary := peerJID()
	linked := primary
	linked.Device = 52
	unknown := primary
	unknown.Device = 99

	reject := func(device types.JID, reason string) *events.CallReject {
		return &events.CallReject{
			BasicCallMeta: types.BasicCallMeta{CallID: "CID", From: device},
			Data:          &waBinary.Node{Tag: "reject", Attrs: waBinary.Attrs{"reason": reason}},
		}
	}
	setup := func() (*engine, *Call) {
		eng, call := testEngineWithOutgoingCall()
		eng.calls["CID"].deviceEncryptionFailures = map[types.JID]bool{primary: false, linked: false}
		return eng, call
	}

	t.Run("linked device fails but primary answers", func(t *testing.T) {
		eng, call := setup()
		eng.onReject(reject(linked, "enc"))
		eng.onReject(reject(linked, "enc"))
		if call.State() == CallPhaseEnded {
			t.Fatal("linked device failure ended the whole call")
		}
		eng.onAccept(&events.CallAccept{
			BasicCallMeta: types.BasicCallMeta{CallID: "CID", From: primary},
			Data:          &waBinary.Node{Tag: "accept"},
		})
		eng.onReject(reject(primary, "enc"))
		eng.onReject(reject(linked, "enc"))
		if call.State() == CallPhaseEnded || !eng.calls["CID"].peerAnswered {
			t.Fatal("encryption rejection ended an answered call")
		}
		eng.onTerminate("CID", "hangup")
		if call.State() != CallPhaseEnded {
			t.Fatal("normal termination must still end the call")
		}
	})

	t.Run("all invited devices fail once", func(t *testing.T) {
		eng, call := setup()
		count := 0
		reason := ""
		call.OnEnd(func(value string) { count++; reason = value })
		eng.onReject(reject(linked, "enc"))
		eng.onReject(reject(unknown, "enc"))
		if call.State() == CallPhaseEnded {
			t.Fatal("unknown device must not change the pending device count")
		}
		eng.onReject(reject(primary, "enc"))
		eng.onReject(reject(primary, "enc"))
		if call.State() != CallPhaseEnded || reason != "encryption_failed" || count != 1 {
			t.Fatalf("state=%v reason=%q callbacks=%d", call.State(), reason, count)
		}
	})

	t.Run("ordinary rejection still ends call", func(t *testing.T) {
		eng, call := setup()
		eng.onReject(reject(primary, "busy"))
		if call.State() != CallPhaseEnded {
			t.Fatal("normal rejection must end the call")
		}
	})

	t.Run("unknown encryption rejection waits for normal timeout", func(t *testing.T) {
		eng, call := setup()
		eng.onReject(reject(unknown, "enc"))
		if call.State() == CallPhaseEnded {
			t.Fatal("unknown encryption rejection ended the call")
		}
		eng.onTerminate("CID", "timeout")
		if call.State() != CallPhaseEnded {
			t.Fatal("timeout must end the call")
		}
	})

	t.Run("accept wins before encryption cleanup", func(t *testing.T) {
		eng, call := setup()
		eng.calls["CID"].peerAnswered = true
		eng.finishCall("CID", "encryption_failed")
		if call.State() == CallPhaseEnded {
			t.Fatal("encryption cleanup must preserve an accepted call")
		}
	})

	t.Run("single invited device fails", func(t *testing.T) {
		eng, call := setup()
		delete(eng.calls["CID"].deviceEncryptionFailures, linked)
		eng.onReject(reject(primary, "enc"))
		if call.State() != CallPhaseEnded {
			t.Fatal("only device failed, call must end")
		}
	})
}
