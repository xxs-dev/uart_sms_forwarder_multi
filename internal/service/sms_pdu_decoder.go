package service

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/warthog618/sms"
	"github.com/warthog618/sms/encoding/pdumode"
	"github.com/warthog618/sms/encoding/tpdu"
)

const smsPDUReassemblyTTL = 30 * time.Minute

type SMSDecodeResult struct {
	Content  string
	From     string
	Alphabet string
	DCS      int
	PDUHexes []string
	Complete bool
	Segment  int
	Segments int
}

type pendingSMSPDU struct {
	segments []*tpdu.TPDU
	pduHexes []string
	updated  time.Time
}

type SMSPDUDecoder struct {
	mu      sync.Mutex
	pending map[string]*pendingSMSPDU
	now     func() time.Time
}

func NewSMSPDUDecoder() *SMSPDUDecoder {
	return &SMSPDUDecoder{
		pending: make(map[string]*pendingSMSPDU),
		now:     time.Now,
	}
}

func (d *SMSPDUDecoder) Decode(pduHex string) (SMSDecodeResult, error) {
	normalized, err := normalizePDUHex(pduHex)
	if err != nil {
		return SMSDecodeResult{}, err
	}

	modemPDU, err := pdumode.UnmarshalHexString(normalized)
	if err != nil {
		return SMSDecodeResult{}, fmt.Errorf("解析 PDU 模式数据失败: %w", err)
	}
	segment, err := sms.Unmarshal(modemPDU.TPDU)
	if err != nil {
		return SMSDecodeResult{}, fmt.Errorf("解析短信 PDU 失败: %w", err)
	}
	alphabet, err := segment.Alphabet()
	if err != nil {
		return SMSDecodeResult{}, fmt.Errorf("解析短信 PDU 编码失败: %w", err)
	}

	result := SMSDecodeResult{
		From:     segment.OA.Number(),
		Alphabet: alphabetName(alphabet),
		DCS:      int(segment.DCS),
		PDUHexes: []string{normalized},
		Complete: true,
		Segment:  1,
		Segments: 1,
	}
	segments, sequence, reference, concatenated := segment.ConcatInfo()
	if !concatenated || segments < 2 {
		content, decodeErr := sms.Decode([]*tpdu.TPDU{segment})
		if decodeErr != nil {
			return SMSDecodeResult{}, fmt.Errorf("解码短信 PDU 正文失败: %w", decodeErr)
		}
		result.Content = string(content)
		return result, nil
	}
	if sequence < 1 || sequence > segments {
		return SMSDecodeResult{}, fmt.Errorf("短信 PDU 分片序号无效: %d/%d", sequence, segments)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	d.expireLocked(now)

	key := fmt.Sprintf("%02x:%s:%d:%d", segment.OA.TOA, segment.OA.Addr, reference, segments)
	pending := d.pending[key]
	if pending == nil {
		pending = &pendingSMSPDU{
			segments: make([]*tpdu.TPDU, segments),
			pduHexes: make([]string, segments),
		}
		d.pending[key] = pending
	}
	index := sequence - 1
	if pending.segments[index] != nil && pending.pduHexes[index] != normalized {
		return SMSDecodeResult{}, fmt.Errorf("短信 PDU 分片冲突: %d/%d", sequence, segments)
	}
	pending.segments[index] = segment
	pending.pduHexes[index] = normalized
	pending.updated = now

	result.Complete = false
	result.Segment = sequence
	result.Segments = segments
	result.PDUHexes = nil
	for _, item := range pending.segments {
		if item == nil {
			return result, nil
		}
	}

	content, decodeErr := sms.Decode(pending.segments)
	if decodeErr != nil {
		delete(d.pending, key)
		return SMSDecodeResult{}, fmt.Errorf("重组短信 PDU 正文失败: %w", decodeErr)
	}
	delete(d.pending, key)
	result.Content = string(content)
	result.PDUHexes = append([]string(nil), pending.pduHexes...)
	result.Complete = true
	return result, nil
}

func (d *SMSPDUDecoder) expireLocked(now time.Time) {
	for key, pending := range d.pending {
		if now.Sub(pending.updated) > smsPDUReassemblyTTL {
			delete(d.pending, key)
		}
	}
}

func normalizePDUHex(value string) (string, error) {
	normalized := strings.ToUpper(strings.Join(strings.Fields(value), ""))
	if normalized == "" {
		return "", fmt.Errorf("PDU 为空")
	}
	if len(normalized)%2 != 0 {
		return "", fmt.Errorf("PDU 十六进制长度必须为偶数")
	}
	if _, err := hex.DecodeString(normalized); err != nil {
		return "", fmt.Errorf("PDU 不是有效的十六进制数据: %w", err)
	}
	return normalized, nil
}

func alphabetName(alphabet tpdu.Alphabet) string {
	switch alphabet {
	case tpdu.Alpha7Bit:
		return "gsm7"
	case tpdu.AlphaUCS2:
		return "ucs2"
	case tpdu.Alpha8Bit:
		return "8bit"
	default:
		return "unknown"
	}
}
