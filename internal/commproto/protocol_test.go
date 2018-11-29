package commproto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestMakeMessageDetails(t *testing.T) {
	key := decodeHex("00112233445566778899aabbccddeeff")
	iv := decodeHex("00110011001100110011001100110011")
	payload := []byte(`{ value: "Hello, Sailor!" }`)
	expected := decodeHex("4d304a066d617374657200420011001100110011001100110011001100304d3a268a1b62b8fa73b46b1338c78e3b6e70cf3ffa018cb6ba" +
		"20053d9efd1bd85ec2500ecc4435a5b8636855dfbf2ac888aa424023b5f628fccd50d32663a6a10ac7eca3717acca2001a1947253ae7a4")
	result, err := MakeMessageWithDetails("master", "passphrase", key, iv, 0x00112233, PayloadEncodingJSON, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, expected) {
		t.Fatalf("expected (top) vs actual (bottom):\n%x\n%x\n", expected, result)
	}
}

func decodeHex(source string) []byte {
	result, err := hex.DecodeString(source)
	if err != nil {
		panic(err)
	}
	return result
}
