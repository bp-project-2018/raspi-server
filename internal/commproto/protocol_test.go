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

	result, err := MakeMessageWithDetails("master", "passphrase", key, iv, 0x00112233, PayloadEncodingJSON, payload)

	expected := decodeHex("4d304a066d617374657200420011001100110011001100110011001100304d3a268a1b62b8fa73b46b1338c78e3b6e70cf3ffa018cb6ba" +
		"20053d9efd1bd85ec2500ecc4435a5b8636855dfbf2ac888aa424023b5f628fccd50d32663a6a10ac7eca3717acca2001a1947253ae7a4")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, expected) {
		t.Fatalf("expected (top) vs actual (bottom):\n%x\n%x\n", expected, result)
	}
}

func TestGetDatagramPublicInformation(t *testing.T) {
	datagram := decodeHex("4d304a066d617374657200420011001100110011001100110011001100304d3a268a1b62b8fa73b46b1338c78e3b6e70cf3ffa018cb6ba" +
		"20053d9efd1bd85ec2500ecc4435a5b8636855dfbf2ac888aa424023b5f628fccd50d32663a6a10ac7eca3717acca2001a1947253ae7a4")

	datagramType, version, encoding, sourceAddress, err := GetDatagramPublicInformation(datagram)

	expectedSourceAddress := "master"
	if err != nil {
		t.Fatal(err)
	}
	if datagramType != DatagramTypeMessage {
		t.Fatal("wrong datagram type")
	}
	if version != 0 {
		t.Fatal("wrong version")
	}
	if encoding != PayloadEncodingJSON {
		t.Fatal("wrong payload encoding")
	}
	if sourceAddress != expectedSourceAddress {
		t.Fatalf("expected '%s', actual '%s'", expectedSourceAddress, sourceAddress)
	}
}

func TestDecodeMessageWithDetails(t *testing.T) {
	key := decodeHex("00112233445566778899aabbccddeeff")
	datagram := decodeHex("4d304a066d617374657200420011001100110011001100110011001100304d3a268a1b62b8fa73b46b1338c78e3b6e70cf3ffa018cb6ba" +
		"20053d9efd1bd85ec2500ecc4435a5b8636855dfbf2ac888aa424023b5f628fccd50d32663a6a10ac7eca3717acca2001a1947253ae7a4")

	timestamp, payload, err := DecodeMessageWithDetails(datagram, "passphrase", key)

	expectedTimestamp := int32(0x00112233)
	expectedPayload := `{ value: "Hello, Sailor!" }`
	if err != nil {
		t.Fatal(err)
	}
	if timestamp != expectedTimestamp {
		t.Fatalf("expected %08x, actual %08x", expectedTimestamp, timestamp)
	}
	if string(payload) != expectedPayload {
		t.Fatalf("expected '%s', actual '%s'", expectedPayload, string(payload))
	}
}

func decodeHex(source string) []byte {
	result, err := hex.DecodeString(source)
	if err != nil {
		panic(err)
	}
	return result
}
