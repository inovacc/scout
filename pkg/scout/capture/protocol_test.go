package capture

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, Msg{V: 1, Type: "ack", ID: "x"}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if got.Type != "ack" || got.ID != "x" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestReadFrameRejectsOversize(t *testing.T) {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], maxFrame+1)
	buf.Write(hdr[:])
	if _, err := ReadFrame(&buf); err == nil {
		t.Fatal("oversize frame accepted")
	}
}

func TestReadFrameRejectsGarbageJSON(t *testing.T) {
	var buf bytes.Buffer
	body := []byte("not json")
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(body)))
	buf.Write(hdr[:])
	buf.Write(body)
	if _, err := ReadFrame(&buf); err == nil {
		t.Fatal("garbage JSON accepted")
	}
}

func TestValidateRejectsBadVersionAndType(t *testing.T) {
	if err := Validate(Msg{V: 2, Type: "hello"}); err == nil {
		t.Error("bad version accepted")
	}
	if err := Validate(Msg{V: 1, Type: "bogus"}); err == nil {
		t.Error("unknown type accepted")
	}
	if err := Validate(Msg{V: 1, Type: "capture_login", Site: "s", Username: "u", Password: "p"}); err != nil {
		t.Errorf("valid capture_login rejected: %v", err)
	}
}

func TestErrorMsgNeverEchoesSecret(t *testing.T) {
	// A login with a bad (empty) site must error WITHOUT including the password.
	err := Validate(Msg{V: 1, Type: "capture_login", Site: "", Username: "u", Password: "hunter2"})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("validation error leaked the password: %v", err)
	}
}
