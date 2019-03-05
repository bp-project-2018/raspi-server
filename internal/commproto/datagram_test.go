package commproto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestExtractAddressEmpty(t *testing.T) {
	var empty []byte

	_, ok := ExtractAddress(empty)

	if ok {
		t.Fatal("ExtractAddress returned ok for empty message")
	}
}

func TestExtractAddressTooShort(t *testing.T) {
	message := []byte{1}

	_, ok := ExtractAddress(message)

	if ok {
		t.Fatal("ExtractAddress returned ok for short message")
	}
}

func TestExtractAddressValid(t *testing.T) {
	message := []byte{6, 'm', 'a', 's', 't', 'e', 'r', 'g', 'a', 'r', 'b', 'a', 'g', 'e'}

	address, ok := ExtractAddress(message)

	if !ok {
		t.Fatal("ExtractAddress returned !ok for valid message")
	}
	expected := "master"
	if address != expected {
		t.Fatalf("ExtractAddress: expected '%s', but was '%s'", expected, address)
	}
}

func TestAssembleDatagram(t *testing.T) {
	address := "master"
	iv := decodeHex("00110011001100110011001100110011")
	timestamp := int64(0x0123456701234567)
	data := []byte(`{ value: "Hello, Sailor!" }`)
	key := decodeHex("00112233445566778899aabbccddeeff")

	result := AssembleDatagram(address, iv, timestamp, data, key, "passphrase")

	expected := decodeHex("066d617374657200110011001100110011001100110011b349503ac3f01a2cfb742313fa1cd6f26785b42e71dde6ac66c9f28269b18d7d6d01e92ddb3b411dab40e6b0144487138561ec2353ce7c30c79b7b18312a1c0d7f67160a53c7e905b465ef2ac6c3c49c")
	if !bytes.Equal(result, expected) {
		t.Fatalf("AssembleDatagram: expected (top) vs actual (bottom):\n%x\n%x\n", expected, result)
	}
}

func TestDisassembleDatagramValid(t *testing.T) {
	datagram := decodeHex("066d617374657200110011001100110011001100110011b349503ac3f01a2cfb742313fa1cd6f26785b42e71dde6ac66c9f28269b18d7d6d01e92ddb3b411dab40e6b0144487138561ec2353ce7c30c79b7b18312a1c0d7f67160a53c7e905b465ef2ac6c3c49c")
	address := "master"
	key := decodeHex("00112233445566778899aabbccddeeff")

	timestamp, data, err := DisassembleDatagram(datagram, address, key, "passphrase")

	if err != nil {
		t.Fatalf("DisassembleDatagram returned err for valid datagram: %v", err)
	}
	expectedTimestamp := int64(0x0123456701234567)
	if timestamp != expectedTimestamp {
		t.Fatalf("DisassembleDatagram: expected %16x, actual %16x", expectedTimestamp, timestamp)
	}
	expectedData := `{ value: "Hello, Sailor!" }`
	if string(data) != expectedData {
		t.Fatalf("expected '%s', actual '%s'", expectedData, string(data))
	}
}

func TestAssembleTimeRequestValid(t *testing.T) {
	result := AssembleTimeRequest("master", []byte{ 0, 1, 2, 3, 4, 5, 6, 7 }, "passphrase")

	expected := decodeHex("066d61737465720001020304050607076cf58d9a1ef7f29e4c7cc82f470273a1049d3d0df81ce706f8c21b8271be3e")
	if !bytes.Equal(result, expected) {
		t.Fatalf("AssembleTimeRequest: expected (top) vs actual (bottom):\n%x\n%x\n", expected, result)
	}
}

func TestDisassembleTimeRequestValid(t *testing.T) {
	request := decodeHex("066d61737465720001020304050607076cf58d9a1ef7f29e4c7cc82f470273a1049d3d0df81ce706f8c21b8271be3e")

	resultNonce, err := DisassembleTimeRequest(request, "master", "passphrase")

	if err != nil {
		t.Fatalf("DisassembleTimeRequest returned err for valid request: %v", err)
	}
	expectedNonce := []byte{ 0, 1, 2, 3, 4, 5, 6, 7 }
	if !bytes.Equal(resultNonce, expectedNonce) {
		t.Fatalf("DisassembleTimeRequest: expected %8x, actual %8x", expectedNonce, resultNonce)
	}
}

func TestAssembleTimeResponseValid(t *testing.T) {
	result := AssembleTimeResponse("master", 0x0123456701234567, []byte{ 0, 1, 2, 3, 4, 5, 6, 7 }, "passphrase")

	expected := decodeHex("066d6173746572012345670123456700010203040506078320414e9fefc84ea3a4b6c96adc4517833941b6e80735bca56eb54a6cfdee32")
	if !bytes.Equal(result, expected) {
		t.Fatalf("AssembleTimeResponse: expected (top) vs actual (bottom):\n%x\n%x\n", expected, result)
	}
}

func TestDisassembleTimeResponseValid(t *testing.T) {
	response := decodeHex("066d6173746572012345670123456700010203040506078320414e9fefc84ea3a4b6c96adc4517833941b6e80735bca56eb54a6cfdee32")

	timestamp, nonce, err := DisassembleTimeResponse(response, "master", "passphrase")

	if err != nil {
		t.Fatalf("DisassembleTimeResponse returned err for valid response: %v", err)
	}
	expectedTimestamp := int64(0x0123456701234567)
	if timestamp != expectedTimestamp {
		t.Fatalf("DisassembleTimeResponse: expected timestamp %16x, actual timestamp %16x", expectedTimestamp, timestamp)
	}
	expectedNonce := []byte{ 0, 1, 2, 3, 4, 5, 6, 7 }
	if !bytes.Equal(nonce, expectedNonce) {
		t.Fatalf("DisassembleTimeResponse: expected nonce %8x, actual nonce %8x", expectedNonce, nonce)
	}
}

func TestCheckMACTooShort(t *testing.T) {
	message := []byte{0, 1, 2, 3}

	ok := checkMAC(message, "passphrase")

	if ok {
		t.Fatal("checkMAC returned ok for short message")
	}
}

func TestCheckMACInvalid(t *testing.T) {
	message := []byte{0, 1, 2, 3}
	for i := 0; i < macSize; i++ {
		message = append(message, 0)
	}

	ok := checkMAC(message, "passphrase")

	if ok {
		t.Fatal("checkMAC returned ok for invalid message")
	}
}

func TestCheckMACValid(t *testing.T) {
	// Test Case 2 from RFC 4231.
	// See: https://tools.ietf.org/html/rfc4231#section-4.3
	passphrase := "Jefe"
	content := []byte("what do ya want for nothing?")
	mac := decodeHex("5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843")
	message := append(content, mac...)

	ok := checkMAC(message, passphrase)

	if !ok {
		t.Fatal("checkMAC returned !ok for valid message")
	}
}

func decodeHex(source string) []byte {
	result, err := hex.DecodeString(source)
	if err != nil {
		panic(err)
	}
	return result
}
