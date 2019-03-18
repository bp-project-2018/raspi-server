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

func TestDisassembleDatagramInvalidMAC(t *testing.T) {
	datagram := decodeHex("066d617374657200110011001100110011001100110011b349503ac3f01a2cfb742313fa1cd6f26785b42e71dde6ac66c9f28269b18d7d6d01e92ddb3b411dab40e6b0144487130000000000000000000000000000000000000000000000000000000000000000")
	address := "master"
	key := decodeHex("00112233445566778899aabbccddeeff")

	_, _, err := DisassembleDatagram(datagram, address, key, "passphrase")

	if err == nil {
		t.Fatalf("DisassembleDatagram failed to report invalid mac")
	}
}

func TestDisassembleDatagramInvalidPaddingLength(t *testing.T) {
	datagram := decodeHex("066d617374657200110011001100110011001100110011b349503ac3f01a2cfb742313fa1cd6f26785b42e71dde6ac66c9f28269b18d7d8f8748f9eb6e4cd3ce29afeb8b38942ee4d2e6999c49d73b02315be04ef61495a488fa2bec0d0b70899cde15875d1273")
	address := "master"
	key := decodeHex("00112233445566778899aabbccddeeff")

	_, _, err := DisassembleDatagram(datagram, address, key, "passphrase")

	if err == nil {
		t.Fatalf("DisassembleDatagram failed to report invalid padding")
	}
}

func TestDisassembleDatagramInvalidPadding(t *testing.T) {
	datagram := decodeHex("066d617374657200110011001100110011001100110011b349503ac3f01a2cfb742313fa1cd6f26785b42e71dde6ac66c9f28269b18d7d70530e11d84f90f125de99c7a1f0ff1b3da274df59ab98d07e24e9ce3e75ba6601ecc5c4c90a1df529ef5d9975b49d07")
	address := "master"
	key := decodeHex("00112233445566778899aabbccddeeff")

	_, _, err := DisassembleDatagram(datagram, address, key, "passphrase")

	if err == nil {
		t.Fatalf("DisassembleDatagram failed to report invalid padding")
	}
}

func TestDisassembleDatagramInvalidAESPayload(t *testing.T) {
	datagram := decodeHex("066d617374657200110011001100110011001100110011ffe0c6c579c4e6ccc18ce0ea17f3c7725fc1f301081ffaa26fd95a205f276e783f6ad5ec2c52fd398bc75c8ff208d17c03dec174b42d96474ea437b9a3e23a7273dd2bbac0b331f49d82498cc122b3485fc55cc390d14d0d")
	address := "master"
	key := decodeHex("00112233445566778899aabbccddeeff")

	_, _, err := DisassembleDatagram(datagram, address, key, "passphrase")

	if err == nil {
		t.Fatalf("DisassembleDatagram failed to report invalid aes payload")
	}
}

func TestDisassembleDatagramEmptyPayload(t *testing.T) {
	datagram := decodeHex("066d6173746572001100110011001100110011001100115c405f9e049d560929aa439252acbc82e99bc83f86fd517c7243f1b0531c5dd804958bc750dd2b716e1c67b73c2f76d1")
	address := "master"
	key := decodeHex("00112233445566778899aabbccddeeff")

	_, _, err := DisassembleDatagram(datagram, address, key, "passphrase")

	if err == nil {
		t.Fatalf("DisassembleDatagram failed to report empty payload")
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
	result := AssembleTimeRequest("master", []byte{0, 1, 2, 3, 4, 5, 6, 7}, "passphrase")

	expected := decodeHex("066d61737465720001020304050607076cf58d9a1ef7f29e4c7cc82f470273a1049d3d0df81ce706f8c21b8271be3e")
	if !bytes.Equal(result, expected) {
		t.Fatalf("AssembleTimeRequest: expected (top) vs actual (bottom):\n%x\n%x\n", expected, result)
	}
}

func TestDisassembleTimeRequestInvalidMAC(t *testing.T) {
	request := decodeHex("066d617374657200010203040506070000000000000000000000000000000000000000000000000000000000000000")

	_, err := DisassembleTimeRequest(request, "master", "passphrase")

	if err == nil {
		t.Fatalf("DisassembleTimeRequest failed to report invalid mac")
	}
}

func TestDisassembleTimeRequestInvalidSize(t *testing.T) {
	request := decodeHex("066d617374657200010203572b77369c93ac1c91020ea544af827123ace671e5cfb44c2db4a96758075c68")

	_, err := DisassembleTimeRequest(request, "master", "passphrase")

	if err == nil {
		t.Fatalf("DisassembleTimeRequest failed to report invalid size")
	}
}

func TestDisassembleTimeRequestValid(t *testing.T) {
	request := decodeHex("066d61737465720001020304050607076cf58d9a1ef7f29e4c7cc82f470273a1049d3d0df81ce706f8c21b8271be3e")

	resultNonce, err := DisassembleTimeRequest(request, "master", "passphrase")

	if err != nil {
		t.Fatalf("DisassembleTimeRequest returned err for valid request: %v", err)
	}
	expectedNonce := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	if !bytes.Equal(resultNonce, expectedNonce) {
		t.Fatalf("DisassembleTimeRequest: expected %8x, actual %8x", expectedNonce, resultNonce)
	}
}

func TestAssembleTimeResponseValid(t *testing.T) {
	result := AssembleTimeResponse("master", 0x0123456701234567, []byte{0, 1, 2, 3, 4, 5, 6, 7}, "passphrase")

	expected := decodeHex("066d6173746572012345670123456700010203040506078320414e9fefc84ea3a4b6c96adc4517833941b6e80735bca56eb54a6cfdee32")
	if !bytes.Equal(result, expected) {
		t.Fatalf("AssembleTimeResponse: expected (top) vs actual (bottom):\n%x\n%x\n", expected, result)
	}
}

func TestDisassembleTimeResponseInvalidMAC(t *testing.T) {
	response := decodeHex("066d6173746572012345670123456700010203040506070000000000000000000000000000000000000000000000000000000000000000")

	_, _, err := DisassembleTimeResponse(response, "master", "passphrase")

	if err == nil {
		t.Fatalf("DisassembleTimeResponse failed to report invalid mac")
	}
}

func TestDisassembleTimeResponseInvalidSize(t *testing.T) {
	response := decodeHex("066d61737465720123456701234567000102039add3d394cbdb84fb0af8fc9fb820a4d6c8b16bddd2a3191d502821fff3cf392")

	_, _, err := DisassembleTimeResponse(response, "master", "passphrase")

	if err == nil {
		t.Fatalf("DisassembleTimeResponse failed to report invalid size")
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
	expectedNonce := []byte{0, 1, 2, 3, 4, 5, 6, 7}
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
