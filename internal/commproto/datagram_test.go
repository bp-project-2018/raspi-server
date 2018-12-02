package commproto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestEncodeHeader(t *testing.T) {
	header := DatagramHeader{
		Type:          DatagramTypeMessage,
		Version:       0,
		Encoding:      PayloadEncodingBinary,
		SourceAddress: "test",
	}

	result := header.Encode()
	length := header.Len()

	expected := []byte{'M', '0', 'B', 4, 't', 'e', 's', 't'}
	if !bytes.Equal(result, expected) {
		t.Fatalf("expected %v, actual %v", expected, result)
	}
	if length != len(result) {
		t.Fatalf("Len() returned %d, but actual length is %d", length, len(result))
	}
}

func TestExtractPublicHeader(t *testing.T) {
	datagram := decodeHex("4d304a066d617374657200420011001100110011001100110011001100304d3a268a1b62b8fa73b46b1338c78e3b6e70cf3ffa018cb6ba" +
		"20053d9efd1bd85ec2500ecc4435a5b8636855dfbf2ac888aa424023b5f628fccd50d32663a6a10ac7eca3717acca2001a1947253ae7a4")

	header, err := ExtractPublicHeader(datagram)

	expectedSourceAddress := "master"
	if err != nil {
		t.Fatal(err)
	}
	if header.Type != DatagramTypeMessage {
		t.Fatal("wrong datagram type")
	}
	if header.Version != 0 {
		t.Fatal("wrong version")
	}
	if header.Encoding != PayloadEncodingJSON {
		t.Fatal("wrong payload encoding")
	}
	if header.SourceAddress != expectedSourceAddress {
		t.Fatalf("expected '%s', actual '%s'", expectedSourceAddress, header.SourceAddress)
	}
}

func TestAssembleDatagram(t *testing.T) {
	header := DatagramHeader{
		Type:          DatagramTypeMessage,
		Version:       0,
		Encoding:      PayloadEncodingJSON,
		SourceAddress: "master",
	}
	variablePayload := []byte{0x00, 0x11, 0x22, 0x33}
	fixedPayload := []byte(`{ value: "Hello, Sailor!" }`)
	key := decodeHex("00112233445566778899aabbccddeeff")
	iv := decodeHex("00110011001100110011001100110011")

	result, err := AssembleDatagram(&header, variablePayload, fixedPayload, key, iv, "passphrase")

	expected := decodeHex("4d304a066d617374657200420011001100110011001100110011001100304d3a268a1b62b8fa73b46b1338c78e3b6e70cf3ffa018cb6ba" +
		"20053d9efd1bd85ec2500ecc4435a5b8636855dfbf2ac888aa424023b5f628fccd50d32663a6a10ac7eca3717acca2001a1947253ae7a4")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, expected) {
		t.Fatalf("expected (top) vs actual (bottom):\n%x\n%x\n", expected, result)
	}
}

func TestDisassembleDatagram(t *testing.T) {
	datagram := decodeHex("4d304a066d617374657200420011001100110011001100110011001100304d3a268a1b62b8fa73b46b1338c78e3b6e70cf3ffa018cb6ba" +
		"20053d9efd1bd85ec2500ecc4435a5b8636855dfbf2ac888aa424023b5f628fccd50d32663a6a10ac7eca3717acca2001a1947253ae7a4")
	header := DatagramHeader{
		Type:          DatagramTypeMessage,
		Version:       0,
		Encoding:      PayloadEncodingJSON,
		SourceAddress: "master",
	}
	key := decodeHex("00112233445566778899aabbccddeeff")

	fixed, variable, err := DisassembleDatagram(datagram, &header, 4, key, "passphrase")

	expectedFixed := []byte{0x00, 0x11, 0x22, 0x33}
	expectedVariable := `{ value: "Hello, Sailor!" }`
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fixed, expectedFixed) {
		t.Fatalf("expected %08x, actual %08x", expectedFixed, fixed)
	}
	if string(variable) != expectedVariable {
		t.Fatalf("expected '%s', actual '%s'", expectedVariable, string(variable))
	}
}

func decodeHex(source string) []byte {
	result, err := hex.DecodeString(source)
	if err != nil {
		panic(err)
	}
	return result
}
