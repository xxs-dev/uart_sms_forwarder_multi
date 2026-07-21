package service

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/warthog618/sms"
	"github.com/warthog618/sms/encoding/tpdu"
)

func TestSMSPDUDecoderDecodesGSM7Deliver(t *testing.T) {
	decoder := NewSMSPDUDecoder()

	result, err := decoder.Decode("07911614220991F1040B911605935713F200008140806113912304D7F79B0E")
	if err != nil {
		t.Fatalf("decode PDU: %v", err)
	}
	if !result.Complete {
		t.Fatal("single-part PDU reported as incomplete")
	}
	if result.Content != "Woot" {
		t.Fatalf("content = %q, want %q", result.Content, "Woot")
	}
	if result.From != "+61503975312" {
		t.Fatalf("from = %q, want %q", result.From, "+61503975312")
	}
	if result.Alphabet != "gsm7" {
		t.Fatalf("alphabet = %q, want gsm7", result.Alphabet)
	}
	if len(result.PDUHexes) != 1 || result.PDUHexes[0] != "07911614220991F1040B911605935713F200008140806113912304D7F79B0E" {
		t.Fatalf("raw PDUs = %#v", result.PDUHexes)
	}
}

func TestSMSPDUDecoderReassemblesConcatenatedUCS2(t *testing.T) {
	decoder := NewSMSPDUDecoder()
	pdus := []string{
		"00400B911605935713F20008814080611373238C050003C00301007400680069007300200069007300200061002000760065007200790020006C006F006E00670020006D0065007300730061006700650020007400680061007400200064006F006500730020006E006F0074002000660069007400200069006E00200061002000730069006E0067006C006500200053004D00530020006D0065007300730061",
		"00400B911605935713F20008814080611373238C050003C0030200670065002C0020006100740020006C0065006100730074002000690074002000770069006C006C002000690066002000490020006B00650065007000200061006400640069006E00670020006D006F0072006500200074006F0020006900740020006100730020003100360030002000630068006100720061006300740065007200730020",
		"00440B911605935713F200088140806113832344050003C00303006900730020006D006F007200650020007400680061006E00200079006F00750020006D00690067006800740020007400680069006E006B0020D83DDE01",
	}

	for _, i := range []int{1, 0} {
		result, err := decoder.Decode(pdus[i])
		if err != nil {
			t.Fatalf("decode segment %d: %v", i+1, err)
		}
		if result.Complete {
			t.Fatalf("segment %d completed message too early", i+1)
		}
	}

	result, err := decoder.Decode(pdus[len(pdus)-1])
	if err != nil {
		t.Fatalf("decode final segment: %v", err)
	}
	want := "this is a very long message that does not fit in a single SMS message, at least it will if I keep adding more to it as 160 characters is more than you might think 😁"
	if !result.Complete || result.Content != want {
		t.Fatalf("result = complete:%t content:%q", result.Complete, result.Content)
	}
	if result.Alphabet != "ucs2" {
		t.Fatalf("alphabet = %q, want ucs2", result.Alphabet)
	}
	if len(result.PDUHexes) != len(pdus) {
		t.Fatalf("raw PDU count = %d, want %d", len(result.PDUHexes), len(pdus))
	}
}

func TestSMSPDUDecoderRejectsInvalidPDU(t *testing.T) {
	decoder := NewSMSPDUDecoder()

	_, err := decoder.Decode("not-a-pdu")
	if err == nil || !strings.Contains(err.Error(), "PDU") {
		t.Fatalf("error = %v, want descriptive PDU error", err)
	}
}

func TestSMSPDUDecoderHandlesGiffgaffSenderAndPoundSign(t *testing.T) {
	sender := tpdu.NewAddress()
	sender.Addr = "giffgaff"
	sender.SetTypeOfNumber(tpdu.TonAlphanumeric)
	encoded, err := sms.Encode(
		[]byte("Your balance is £12.15"),
		sms.AsDeliver,
		sms.WithTemplateOption(tpdu.WithOA(sender)),
	)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	tpduBytes, err := encoded[0].MarshalBinary()
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	pduHex := "00" + strings.ToUpper(hex.EncodeToString(tpduBytes))

	result, err := NewSMSPDUDecoder().Decode(pduHex)
	if err != nil {
		t.Fatalf("decode giffgaff PDU: %v", err)
	}
	if result.From != "giffgaff" {
		t.Fatalf("from = %q, want giffgaff", result.From)
	}
	if result.Content != "Your balance is £12.15" {
		t.Fatalf("content = %q", result.Content)
	}
	if result.Alphabet != "gsm7" {
		t.Fatalf("alphabet = %q, want gsm7", result.Alphabet)
	}
}
