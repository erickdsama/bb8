package protocol

import (
	"errors"
	"testing"
)

func TestParseStatus(t *testing.T) {
	st := ParseStatus("V=11.62 T=3.1 Y=-12.4 E=1420,1398 H=-5 D=1830 F=obstacle,tilt X=new")
	if st.BatteryV != 11.62 || st.TiltDeg != 3.1 || st.YawDeg != -12.4 {
		t.Fatalf("floats: %+v", st)
	}
	if st.EncLeft != 1420 || st.EncRight != 1398 || st.HeadDeg != -5 || st.ToFMM != 1830 {
		t.Fatalf("ints: %+v", st)
	}
	if !st.Has("obstacle") || !st.Has("tilt") || st.Has("sleep") {
		t.Fatalf("flags: %v", st.Flags)
	}
	if len(ParseStatus("F=-").Flags) != 0 {
		t.Fatal("F=- must mean no flags")
	}
}

func TestParseReply(t *testing.T) {
	if s, err := ParseReply("OK BB8 fw=0.1\r\n"); err != nil || s != "BB8 fw=0.1" {
		t.Fatal(s, err)
	}
	_, err := ParseReply("ERR obstacle")
	var pe *Error
	if !errors.As(err, &pe) || pe.Reason != "obstacle" {
		t.Fatal(err)
	}
	if _, err := ParseReply("hello"); err == nil {
		t.Fatal("garbage must fail")
	}
}

func TestCommands(t *testing.T) {
	if Motors(300, -12.6) != "M 255 -13" || Head(-120) != "H -90" || Distance(9000) != "D 4000" {
		t.Fatal(Motors(300, -12.6), Head(-120), Distance(9000))
	}
}
