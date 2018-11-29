package commproto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
)

type DatagramType int

const (
	DatagramTypeMessage DatagramType = 0
	DatagramTypeCommand DatagramType = 1
)

type PayloadEncoding int

const (
	PayloadEncodingBinary PayloadEncoding = 0
	PayloadEncodingJSON   PayloadEncoding = 1
)

func MakeMessageWithDetails(sourceAddress string, passphrase string, key []byte, iv []byte, timestamp int32, encoding PayloadEncoding, payload []byte) ([]byte, error) {
	if len(key) != 16 {
		panic("key has wrong length")
	}

	if len(iv) != 16 {
		panic("iv has wrong length")
	}

	format := makeDatagramFormat(DatagramTypeMessage, 0, encoding)

	var aesBuffer bytes.Buffer
	{ // Write plaintext.
		aesBuffer.Write(format[:])
		writeAddress(&aesBuffer, sourceAddress)
		aesBuffer.WriteByte(byte(timestamp >> 24))
		aesBuffer.WriteByte(byte(timestamp >> 16))
		aesBuffer.WriteByte(byte(timestamp >> 8))
		aesBuffer.WriteByte(byte(timestamp >> 0))
		aesBuffer.Write(payload)
	}

	{ // Add PKCS#7 padding.
		padding := aes.BlockSize - aesBuffer.Len()%aes.BlockSize
		for i := 0; i < padding; i++ {
			aesBuffer.WriteByte(byte(padding))
		}
	}

	{ // Encrypt buffer.
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		mode := cipher.NewCBCEncrypter(block, iv)
		mode.CryptBlocks(aesBuffer.Bytes(), aesBuffer.Bytes()) // encrypt aesBuffer in-place
	}

	aesLength := aesBuffer.Len()
	{ // Check payload length.
		expectedHmacLength := 16 + 2 + aesLength // iv + aes ciphertext size + aes ciphertext
		if expectedHmacLength > math.MaxUint16 {
			return nil, errors.New("payload too long")
		}
	}

	var hmacBuffer bytes.Buffer
	{ // Write HMAC message.
		hmacBuffer.Write(iv)
		hmacBuffer.WriteByte(byte(aesLength >> 8))
		hmacBuffer.WriteByte(byte(aesLength >> 0))
		hmacBuffer.Write(aesBuffer.Bytes())
	}

	hmacLength := hmacBuffer.Len()

	var mac []byte
	{ // Calculate MAC.
		hash := hmac.New(sha256.New, []byte(passphrase))
		hash.Write(hmacBuffer.Bytes())
		mac = hash.Sum(nil)
	}

	var datagramBuffer bytes.Buffer
	{ // Write final datagram.
		datagramBuffer.Write(format[:])
		writeAddress(&datagramBuffer, sourceAddress)
		datagramBuffer.WriteByte(byte(hmacLength >> 8))
		datagramBuffer.WriteByte(byte(hmacLength >> 0))
		datagramBuffer.Write(hmacBuffer.Bytes())
		datagramBuffer.Write(mac)
	}

	return datagramBuffer.Bytes(), nil
}

func writeAddress(buffer *bytes.Buffer, address string) {
	length := len(address)
	if length > math.MaxUint8 {
		panic("address too long")
	}
	buffer.WriteByte(byte(length))
	buffer.WriteString(address)
}

func makeDatagramFormat(datagramType DatagramType, version int, encoding PayloadEncoding) (format [3]byte) {
	switch datagramType {
	case DatagramTypeMessage:
		format[0] = 'M'
	case DatagramTypeCommand:
		format[0] = 'C'
	default:
		panic(fmt.Sprintf("invalid datagram type: %d", datagramType))
	}

	if version < 0 {
		panic(fmt.Sprintf("invalid datagram version: %d", version))
	}
	if version >= 10 {
		panic(fmt.Sprintf("datagram version too big to encode using current format: %d", version))
	}

	format[1] = byte('0' + version)

	switch encoding {
	case PayloadEncodingBinary:
		format[2] = 'B'
	case PayloadEncodingJSON:
		format[2] = 'J'
	default:
		panic(fmt.Sprintf("invalid payload encoding: %d", encoding))
	}

	return
}
