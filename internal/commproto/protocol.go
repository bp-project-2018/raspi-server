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

type DatagramHeader struct {
	Type          DatagramType
	Version       int
	Encoding      PayloadEncoding
	SourceAddress string
}

// Returns the length in bytes of the header when encoded using the Encode() function.
func (d *DatagramHeader) Len() int {
	// 3 bytes for the format, 1 for the length of the address and the length of the address itself.
	return 3 + 1 + len(d.SourceAddress)
}

func (d *DatagramHeader) Encode() []byte {
	var result bytes.Buffer

	// Reserve space.
	result.Grow(d.Len())

	switch d.Type {
	case DatagramTypeMessage:
		result.WriteByte('M')
	case DatagramTypeCommand:
		result.WriteByte('C')
	default:
		panic(fmt.Sprintf("invalid datagram type: %d", d.Type))
	}

	if d.Version < 0 {
		panic(fmt.Sprintf("invalid datagram version: %d", d.Version))
	}
	if d.Version >= 10 {
		panic(fmt.Sprintf("datagram version too big to encode using current format: %d", d.Version))
	}
	result.WriteByte(byte('0' + d.Version))

	switch d.Encoding {
	case PayloadEncodingBinary:
		result.WriteByte('B')
	case PayloadEncodingJSON:
		result.WriteByte('J')
	default:
		panic(fmt.Sprintf("invalid payload encoding: %d", d.Encoding))
	}

	length := len(d.SourceAddress)
	if length > math.MaxUint8 {
		panic("address too long")
	}
	result.WriteByte(byte(length))
	result.WriteString(d.SourceAddress)

	return result.Bytes()
}

// Extracts the header from the unencrypted parts of a datagram buffer.
func ExtractPublicHeader(datagram []byte) (*DatagramHeader, error) {
	if len(datagram) < 4 {
		return nil, errors.New("datagram too short")
	}

	var header DatagramHeader

	switch datagram[0] {
	case 'M':
		header.Type = DatagramTypeMessage
	case 'C':
		header.Type = DatagramTypeCommand
	default:
		return nil, errors.New("unknown datagram type")
	}

	switch datagram[1] {
	case '0':
		header.Version = 0
	default:
		return nil, errors.New("unknown version")
	}

	switch datagram[2] {
	case 'B':
		header.Encoding = PayloadEncodingBinary
	case 'J':
		header.Encoding = PayloadEncodingJSON
	default:
		return nil, errors.New("unknown payload encoding")
	}

	length := int(datagram[3])
	if len(datagram) < 4+length {
		return nil, errors.New("invalid address length")
	}
	header.SourceAddress = string(datagram[4 : 4+length])

	return &header, nil
}

func MakeMessageWithDetails(sourceAddress string, passphrase string, key []byte, iv []byte, timestamp int32, encoding PayloadEncoding, payload []byte) ([]byte, error) {
	if len(key) != 16 {
		panic("key has wrong length")
	}

	if len(iv) != 16 {
		panic("iv has wrong length")
	}

	header := DatagramHeader{
		Type:          DatagramTypeMessage,
		Version:       0,
		Encoding:      encoding,
		SourceAddress: sourceAddress,
	}
	headerBuffer := header.Encode()

	var aesBuffer bytes.Buffer
	{ // Write plaintext.
		aesBuffer.Write(headerBuffer)
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
		datagramBuffer.Write(headerBuffer)
		datagramBuffer.WriteByte(byte(hmacLength >> 8))
		datagramBuffer.WriteByte(byte(hmacLength >> 0))
		datagramBuffer.Write(hmacBuffer.Bytes())
		datagramBuffer.Write(mac)
	}

	return datagramBuffer.Bytes(), nil
}

// @Todo: Assumes well-formed data right now!!!
func DecodeMessageWithDetails(datagram []byte, passphrase string, key []byte) (timestamp int32, payload []byte, err error) {
	if len(key) != 16 {
		panic("key has wrong length")
	}

	publicLength := 4 + int(datagram[3])

	hmacLengthHigh := int(datagram[publicLength])
	hmacLengthLow := int(datagram[publicLength+1])
	hmacLength := (hmacLengthHigh << 8) + hmacLengthLow
	hmacStart := publicLength + 2

	var expectedMAC []byte
	{ // Calculate MAC.
		hash := hmac.New(sha256.New, []byte(passphrase))
		hash.Write(datagram[hmacStart : hmacStart+hmacLength])
		expectedMAC = hash.Sum(nil)
	}

	messageMAC := datagram[hmacStart+hmacLength : hmacStart+hmacLength+sha256.Size]

	if len(datagram) != hmacStart+hmacLength+sha256.Size {
		err = errors.New("invalid datagram")
		return
	}

	if !hmac.Equal(messageMAC, expectedMAC) {
		err = errors.New("invalid datagram")
		return
	}

	iv := datagram[hmacStart : hmacStart+16]
	aesLengthHigh := int(datagram[hmacStart+16])
	aesLengthLow := int(datagram[hmacStart+17])
	aesLength := (aesLengthHigh << 8) + aesLengthLow
	aesStart := hmacStart + 16 + 2

	{ // Decrypt data.
		aesBuffer := datagram[aesStart : aesStart+aesLength]
		block, aesErr := aes.NewCipher(key)
		if aesErr != nil {
			err = aesErr
			return
		}
		mode := cipher.NewCBCDecrypter(block, iv)
		mode.CryptBlocks(aesBuffer, aesBuffer) // decrypt in-place
	}

	var padding int
	{ // Check padding.
		padding = int(datagram[aesStart+aesLength-1])
		if padding > aes.BlockSize {
			err = errors.New("invalid datagram")
			return
		}
		for i := 0; i < padding; i++ {
			if int(datagram[aesStart+aesLength-i-1]) != padding {
				err = errors.New("invalid datagram")
				return
			}
		}
	}

	if !bytes.Equal(datagram[0:publicLength], datagram[aesStart:aesStart+publicLength]) {
		err = errors.New("invalid datagram")
		return
	}

	if aesStart+aesLength != hmacStart+hmacLength {
		err = errors.New("invalid datagram")
		return
	}

	timestamp += int32(datagram[aesStart+publicLength+0]) << 24
	timestamp += int32(datagram[aesStart+publicLength+1]) << 16
	timestamp += int32(datagram[aesStart+publicLength+2]) << 8
	timestamp += int32(datagram[aesStart+publicLength+3]) << 0

	payload = datagram[aesStart+publicLength+4 : aesStart+aesLength-padding]
	return
}
