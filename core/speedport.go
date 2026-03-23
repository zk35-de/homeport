package core

import (
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SpeedportFetcher fetches router status from a Telekom Speedport Hybrid router.
// The router serves /data/Status.json encrypted with AES-256-CCM using a
// hardcoded key (the key is public / baked into official firmware).
type SpeedportFetcher struct {
	BaseURL  string
	Password string // unused for Status.json – kept for interface consistency
}

// speedportKey is the fixed AES-256 key used by the Speedport Hybrid firmware.
const speedportKeyHex = "cdc0cac1280b516e674f0057e4929bca84447cca8425007e33a88a5cf598a190"

// Fetch retrieves and decodes the router status.
func (s *SpeedportFetcher) Fetch() (*RouterStatus, error) {
	url := s.BaseURL + "/data/Status.json"

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("speedport: fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("speedport: HTTP %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("speedport: read body: %w", err)
	}

	plaintext, err := speedportDecrypt(body)
	if err != nil {
		return nil, fmt.Errorf("speedport: decrypt: %w", err)
	}

	return parseSpeedportStatus(plaintext)
}

// speedportDecrypt decrypts the AES-256-CCM ciphertext produced by the
// Speedport Hybrid firmware. Since Go's standard library has no CCM mode,
// we replicate the CCM counter (CTR) layer manually. The auth-tag
// verification is intentionally skipped – we trust the local router.
//
// CCM counter format (SJCL convention used by the firmware):
//
//	flags byte = L - 1, where L = 15 - len(nonce)
//	nonce     = 8 bytes (first 8 bytes of key)
//	counter   = 7 bytes big-endian, starting at 1 for the first data block
//
// Block 0 is reserved for the MAC/tag encryption.
func speedportDecrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 16 {
		return nil, fmt.Errorf("ciphertext too short")
	}

	keyBytes, err := hex.DecodeString(speedportKeyHex)
	if err != nil {
		return nil, fmt.Errorf("bad key hex: %w", err)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("aes init: %w", err)
	}

	// nonce = first 8 bytes of key
	nonce := keyBytes[:8]

	// Strip last 16 bytes (auth tag) – we do not verify it
	data := ciphertext[:len(ciphertext)-16]

	// CCM counter flags byte: L - 1 = 7 - 1 = 6 = 0x06
	// (L = 15 - nonceLen = 15 - 8 = 7)
	const flagsByte = 0x06

	plaintext := make([]byte, len(data))

	blockBuf := make([]byte, aes.BlockSize)
	keystream := make([]byte, aes.BlockSize)

	for offset := 0; offset < len(data); offset += aes.BlockSize {
		// counter block for this segment (1-based)
		ctr := uint64(offset/aes.BlockSize) + 1

		blockBuf[0] = flagsByte
		copy(blockBuf[1:9], nonce) // bytes 1..8
		// bytes 9..15: 7-byte big-endian counter
		blockBuf[9] = byte(ctr >> 48)
		blockBuf[10] = byte(ctr >> 40)
		blockBuf[11] = byte(ctr >> 32)
		blockBuf[12] = byte(ctr >> 24)
		blockBuf[13] = byte(ctr >> 16)
		blockBuf[14] = byte(ctr >> 8)
		blockBuf[15] = byte(ctr)

		block.Encrypt(keystream, blockBuf)

		end := offset + aes.BlockSize
		if end > len(data) {
			end = len(data)
		}
		for i := offset; i < end; i++ {
			plaintext[i] = data[i] ^ keystream[i-offset]
		}
	}

	return plaintext, nil
}

// speedportVar is one entry in the Status.json array.
type speedportVar struct {
	VarType  string `json:"vartype"`
	VarID    string `json:"varid"`
	VarValue string `json:"varvalue"`
}

func parseSpeedportStatus(data []byte) (*RouterStatus, error) {
	// The plaintext may still be hex-encoded JSON – some firmware variants
	// return hex, others return raw bytes. Try raw JSON first.
	var vars []speedportVar
	if err := json.Unmarshal(data, &vars); err != nil {
		// Try hex-decode → JSON
		hexStr := strings.TrimSpace(string(data))
		decoded, herr := hex.DecodeString(hexStr)
		if herr != nil {
			return nil, fmt.Errorf("json decode: %w (hex decode also failed: %v)", err, herr)
		}
		if err2 := json.Unmarshal(decoded, &vars); err2 != nil {
			return nil, fmt.Errorf("json decode after hex: %w", err2)
		}
	}

	m := make(map[string]string, len(vars))
	for _, v := range vars {
		m[v.VarID] = v.VarValue
	}

	status := &RouterStatus{}

	// DSL downstream / upstream are in bit/s
	if v, ok := m["dsl_downstream"]; ok {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			status.DSLDownMbit = n / 1_000_000
		}
	}
	if v, ok := m["dsl_upstream"]; ok {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			status.DSLUpMbit = n / 1_000_000
		}
	}

	status.DSLOnline = strings.EqualFold(m["dsl_link_status"], "online")

	lteStatus := m["lte_status"]
	status.LTEActive = lteStatus == "10" || lteStatus == "11"

	if v, ok := m["ex5g_signal_lte"]; ok {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			status.LTESignalDBm = n
		}
	}
	status.LTEBand = m["ex5g_freq_lte"]

	// Mode detection based on tunnel flags
	hybridTunnel := m["hybrid_tunnel"] == "1"
	lteTunnel := m["lte_tunnel"] == "1"
	dslTunnel := m["dsl_tunnel"] == "1"

	switch {
	case hybridTunnel && lteTunnel && dslTunnel:
		status.Mode = "Hybrid"
	case lteTunnel && !dslTunnel:
		status.Mode = "LTE"
	default:
		status.Mode = "DSL"
	}

	status.Online = status.DSLOnline || status.LTEActive

	return status, nil
}
