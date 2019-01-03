package commproto

// This file handles the encoding and decoding of individual datagrams.

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

// Len returns the length in bytes of the header when encoded using the Encode()
// function.
func (d *DatagramHeader) Len() int {
	// 3 bytes for the format, 1 for the length of the address and the length of
	// the address itself.
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

// ExtractPublicHeader extracts the header from the unencrypted parts of a
// datagram buffer.
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

// AssembleDatagram creates a datagram from the given data using the provided
// encryption secrets.
//
// The length of the fixedPayload must be deducible from the datagram header to
// be able to disassemble the datagram correctly.
func AssembleDatagram(header *DatagramHeader, fixedPayload []byte, variablePayload []byte, key []byte, iv []byte, passphrase string) ([]byte, error) {
	if len(key) != 16 {
		panic("key has wrong length")
	}

	if len(iv) != 16 {
		panic("iv has wrong length")
	}

	headerBuffer := header.Encode()

	var aesBuffer bytes.Buffer
	{ // Write plaintext.
		aesBuffer.Write(headerBuffer)
		aesBuffer.Write(fixedPayload)
		aesBuffer.Write(variablePayload)
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
			panic(err)
		}
		mode := cipher.NewCBCEncrypter(block, iv)
		mode.CryptBlocks(aesBuffer.Bytes(), aesBuffer.Bytes()) // encrypt aesBuffer in-place
	}

	{ // Check payload length.
		expectedHmacLength := 16 + 2 + aesBuffer.Len() // iv + aes ciphertext size + aes ciphertext
		if expectedHmacLength > math.MaxUint16 {
			return nil, errors.New("payload too long")
		}
	}

	var hmacBuffer bytes.Buffer
	{ // Write HMAC message.
		hmacBuffer.Write(iv)
		hmacBuffer.WriteByte(byte(aesBuffer.Len() >> 8))
		hmacBuffer.WriteByte(byte(aesBuffer.Len() >> 0))
		hmacBuffer.Write(aesBuffer.Bytes())
	}

	var mac []byte
	{ // Calculate MAC.
		hash := hmac.New(sha256.New, []byte(passphrase))
		hash.Write(hmacBuffer.Bytes())
		mac = hash.Sum(nil)
	}

	var datagramBuffer bytes.Buffer
	{ // Write final datagram.
		datagramBuffer.Write(headerBuffer)
		datagramBuffer.WriteByte(byte(hmacBuffer.Len() >> 8))
		datagramBuffer.WriteByte(byte(hmacBuffer.Len() >> 0))
		datagramBuffer.Write(hmacBuffer.Bytes())
		datagramBuffer.Write(mac)
	}

	return datagramBuffer.Bytes(), nil
}

// DisassembleDatagram validates and decrypts a datagram using the provided
// encryption secrets.
func DisassembleDatagram(datagram []byte, header *DatagramHeader, fixedPayloadLength int, key []byte, passphrase string) (fixedPayload []byte, variablePayload []byte, err error) {
	if len(key) != 16 {
		panic("key has wrong length")
	}

	remainder := datagram

	{ // Skip the header.
		// The header must fit the data, so we do not need to check the length
		// here, as this has already happened in ExtractPublicHeader.
		remainder = remainder[header.Len():]
	}

	var hmacLength int
	{ // Extract hmac content length.
		if len(remainder) < 2 {
			err = errors.New("invalid datagram")
			return
		}
		high, low := int(remainder[0]), int(remainder[1])
		hmacLength = (high << 8) + low
		remainder = remainder[2:]
	}

	var hmacContent []byte
	{ // Save hmac content for later.
		if len(remainder) < hmacLength {
			err = errors.New("invalid datagram")
			return
		}
		hmacContent = remainder[:hmacLength]
		remainder = remainder[hmacLength:]
	}

	var messageMAC []byte
	{ // Extract MAC stored in datagram.
		if len(remainder) < sha256.Size {
			err = errors.New("invalid datagram")
			return
		}
		messageMAC = remainder[:sha256.Size]
		remainder = remainder[sha256.Size:]
	}

	if len(remainder) != 0 {
		// More data in datagram than expected.
		err = errors.New("invalid datagram")
		return
	}

	var expectedMAC []byte
	{ // Calculate MAC of received data.
		hash := hmac.New(sha256.New, []byte(passphrase))
		hash.Write(hmacContent)
		expectedMAC = hash.Sum(nil)
	}

	if !hmac.Equal(messageMAC, expectedMAC) {
		err = errors.New("invalid datagram")
		return
	}

	// Disassemble hmacContent now.
	remainder = hmacContent

	var iv []byte
	{ // Extract initialization vector.
		if len(remainder) < 16 {
			err = errors.New("invalid datagram")
			return
		}
		iv = remainder[:16]
		remainder = remainder[16:]
	}

	var aesLength int
	{ // Extract aes content length.
		if len(remainder) < 2 {
			err = errors.New("invalid datagram")
			return
		}
		high, low := int(remainder[0]), int(remainder[1])
		aesLength = (high << 8) + low
		remainder = remainder[2:]
	}

	// We expect at least one block of data because of padding.
	// Also, the length must be a multiple of the block size.
	if aesLength < aes.BlockSize || aesLength%aes.BlockSize != 0 {
		err = errors.New("invalid datagram")
		return
	}

	var aesContent []byte
	{
		if len(remainder) < aesLength {
			err = errors.New("invalid datagram")
			return
		}
		aesContent = remainder[:aesLength]
		remainder = remainder[aesLength:]
	}

	if len(remainder) != 0 {
		// More data in hmac content than expected.
		err = errors.New("invalid datagram")
		return
	}

	{ // Decrypt data.
		block, aesErr := aes.NewCipher(key)
		if aesErr != nil {
			panic(aesErr)
		}
		mode := cipher.NewCBCDecrypter(block, iv)
		mode.CryptBlocks(aesContent, aesContent) // decrypt in-place
	}

	var padding int
	{ // Check padding.
		// Accesses in this block are safe, because of the check directly after
		// the length has been extracted.
		padding = int(aesContent[aesLength-1])
		if padding > aes.BlockSize {
			err = errors.New("invalid datagram")
			return
		}
		for i := 0; i < padding; i++ {
			if int(aesContent[aesLength-i-1]) != padding {
				err = errors.New("invalid datagram")
				return
			}
		}
	}

	// Check that the decrypted data is long enough.
	if aesLength-padding < header.Len()+fixedPayloadLength {
		err = errors.New("invalid datagram")
		return
	}

	// Make sure the public header has not been tampered with.
	if !bytes.Equal(datagram[:header.Len()], aesContent[:header.Len()]) {
		err = errors.New("invalid datagram")
		return
	}

	fixedPayload = aesContent[header.Len() : header.Len()+fixedPayloadLength]
	variablePayload = aesContent[header.Len()+fixedPayloadLength : aesLength-padding]
	return
}

func AssembleTime(time int32, passphrase string) []byte {
	var buffer bytes.Buffer

	// @Todo: Length is not really needed, as this a fixed size datagram, but
	// this has to be defined in the specification first.
	length := 4
	buffer.WriteByte(byte(length >> 8))
	buffer.WriteByte(byte(length >> 0))

	buffer.WriteByte(byte(time >> 24))
	buffer.WriteByte(byte(time >> 16))
	buffer.WriteByte(byte(time >> 8))
	buffer.WriteByte(byte(time >> 0))

	var mac []byte
	{ // Calculate MAC.
		hash := hmac.New(sha256.New, []byte(passphrase))
		hash.Write(buffer.Bytes()[2 : 2+length])
		mac = hash.Sum(nil)
	}

	buffer.Write(mac)
	return buffer.Bytes()
}

func DisassembleTime(data []byte, passphrase string) (int32, error) {
	if len(data) != 2+4+sha256.Size /* length, payload, mac */ {
		return 0, errors.New("invalid datagram")
	}

	high, low := int(data[0]), int(data[1])
	length := high<<8 + low
	if length != 4 {
		return 0, errors.New("invalid datagram")
	}

	messageMAC := data[2+4:]

	var expectedMAC []byte
	{ // Calculate MAC of received data.
		hash := hmac.New(sha256.New, []byte(passphrase))
		hash.Write(data[2 : 2+length])
		expectedMAC = hash.Sum(nil)
	}

	if !hmac.Equal(messageMAC, expectedMAC) {
		return 0, errors.New("invalid datagram")
	}

	time := int32(0)
	time += int32(data[2]) << 24
	time += int32(data[3]) << 16
	time += int32(data[4]) << 8
	time += int32(data[5]) << 0

	return time, nil
}
