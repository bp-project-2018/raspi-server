package commproto

// This file handles the encoding and decoding of individual datagrams.

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
)

const (
	// The size of initialization vectors in bytes.
	IVSize = 16
	// The size of keys in bytes.
	KeySize = 16
	// The size of nonces in bytes.
	NonceSize = 8
)

const (
	addressLengthSize = 1
	timestampSize     = 8
	macSize           = sha256.Size
)

// ExtractAddress returns the address that is stored at the beginning of the message.
func ExtractAddress(message []byte) (address string, ok bool) {
	if len(message) == 0 {
		return
	}

	length := int(message[0])

	if len(message) < addressLengthSize+length {
		return
	}

	address = string(message[addressLengthSize : addressLengthSize+length])
	ok = true
	return
}

// AssembleDatagram creates a datagram from the given data using the provided encryption secrets.
func AssembleDatagram(address string, iv []byte, timestamp int64, data []byte, key []byte, passphrase string) []byte {
	if len(address) > 255 {
		panic("address too long")
	}

	if len(iv) != IVSize {
		panic("iv has wrong length")
	}

	if len(key) != KeySize {
		panic("key has wrong length")
	}

	var buffer bytes.Buffer
	buffer.WriteByte(byte(len(address)))
	buffer.WriteString(address)
	buffer.Write(iv)

	aesStart := buffer.Len()

	writeTimestamp(&buffer, timestamp)
	buffer.Write(data)

	{ // Add PKCS#7 padding.
		length := buffer.Len() - aesStart
		padding := aes.BlockSize - length%aes.BlockSize
		for i := 0; i < padding; i++ {
			buffer.WriteByte(byte(padding))
		}
	}

	aesEnd := buffer.Len()

	{ // Encrypt buffer.
		block, err := aes.NewCipher(key)
		if err != nil {
			panic(err)
		}
		mode := cipher.NewCBCEncrypter(block, iv)
		aesBuffer := buffer.Bytes()[aesStart:aesEnd] // aliases the content of buffer
		mode.CryptBlocks(aesBuffer, aesBuffer)       // encrypt in-place
	}

	writeMAC(&buffer, passphrase)

	return buffer.Bytes()
}

// DisassembleDatagram validates and decrypts a datagram using the provided encryption secrets.
func DisassembleDatagram(datagram []byte, address string, key []byte, passphrase string) (timestamp int64, data []byte, err error) {
	if len(key) != KeySize {
		panic("key has wrong length")
	}

	if !checkMAC(datagram, passphrase) {
		err = errors.New("invalid datagram")
		return
	}

	ivStart := addressLengthSize + len(address)
	ivEnd := ivStart + IVSize
	aesStart := ivEnd
	aesEnd := len(datagram) - macSize

	// We expect at least one block of data because of padding and the timestamp.
	// Also, the length must be a multiple of the block size.
	if aesLength := aesEnd - aesStart; aesLength < aes.BlockSize || aesLength%aes.BlockSize != 0 {
		err = errors.New("invalid datagram")
		return
	}

	// Only do this after the length check above!
	ivBuffer := datagram[ivStart:ivEnd]
	aesBuffer := datagram[aesStart:aesEnd]

	{ // Decrypt data.
		block, aesErr := aes.NewCipher(key)
		if aesErr != nil {
			panic(aesErr)
		}
		mode := cipher.NewCBCDecrypter(block, ivBuffer)
		mode.CryptBlocks(aesBuffer, aesBuffer) // decrypt in-place
	}

	var padding int
	{ // Check padding.
		// Accesses in this block are safe, because of the aes length check above.
		padding = int(aesBuffer[len(aesBuffer)-1])
		if padding > aes.BlockSize {
			err = errors.New("invalid datagram")
			return
		}
		for i := 0; i < padding; i++ {
			if int(aesBuffer[len(aesBuffer)-i-1]) != padding {
				err = errors.New("invalid datagram")
				return
			}
		}
	}

	if len(aesBuffer)-padding < timestampSize {
		err = errors.New("invalid datagram")
		return
	}

	timestamp = decodeTimestamp(aesBuffer[0:timestampSize])
	data = aesBuffer[timestampSize : len(aesBuffer)-padding]
	return
}

// AssembleTimeRequest creates a time request from the given nonce and passphrase.
func AssembleTimeRequest(address string, nonce []byte, passphrase string) []byte {
	if len(nonce) != NonceSize {
		panic("nonce has wrong length")
	}

	var buffer bytes.Buffer
	buffer.WriteByte(byte(len(address)))
	buffer.WriteString(address)
	buffer.Write(nonce)
	writeMAC(&buffer, passphrase)

	return buffer.Bytes()
}

// DisassembleTimeRequest validates and extracts a time request using the provided passphrase.
func DisassembleTimeRequest(message []byte, address string, passphrase string) (nonce []byte, err error) {
	if !checkMAC(message, passphrase) {
		err = errors.New("invalid message")
		return
	}

	if len(message) != addressLengthSize+len(address)+NonceSize+macSize {
		err = errors.New("invalid message")
		return
	}

	nonceStart := addressLengthSize + len(address)
	nonce = message[nonceStart : nonceStart+NonceSize]
	return
}

// AssembleTimeResponse creates a time response from the given data and passphrase.
func AssembleTimeResponse(address string, timestamp int64, nonce []byte, passphrase string) []byte {
	if len(nonce) != NonceSize {
		panic("nonce has wrong length")
	}

	var buffer bytes.Buffer
	buffer.WriteByte(byte(len(address)))
	buffer.WriteString(address)
	writeTimestamp(&buffer, timestamp)
	buffer.Write(nonce)
	writeMAC(&buffer, passphrase)

	return buffer.Bytes()
}

// DisassembleTimeResponse validates and extracts a time response using the provided passphrase.
func DisassembleTimeResponse(message []byte, address string, passphrase string) (timestamp int64, nonce []byte, err error) {
	if !checkMAC(message, passphrase) {
		err = errors.New("invalid message")
		return
	}

	if len(message) != addressLengthSize+len(address)+timestampSize+NonceSize+macSize {
		err = errors.New("invalid message")
		return
	}

	timestampStart := addressLengthSize + len(address)
	timestamp = decodeTimestamp(message[timestampStart : timestampStart+timestampSize])
	nonceStart := timestampStart + timestampSize
	nonce = message[nonceStart : nonceStart+NonceSize]
	return
}

// writeTimestamp appends the timestamp to the buffer.
func writeTimestamp(buffer *bytes.Buffer, timestamp int64) {
	// Write timestamp in network byte order (big-endian).
	buffer.WriteByte(byte(timestamp >> 56))
	buffer.WriteByte(byte(timestamp >> 48))
	buffer.WriteByte(byte(timestamp >> 40))
	buffer.WriteByte(byte(timestamp >> 32))
	buffer.WriteByte(byte(timestamp >> 24))
	buffer.WriteByte(byte(timestamp >> 16))
	buffer.WriteByte(byte(timestamp >> 8))
	buffer.WriteByte(byte(timestamp >> 0))
}

// decodeTimestamp decodes a timestamp written by writeTimestamp.
// raw must have a length of timestampSize.
func decodeTimestamp(raw []byte) (timestamp int64) {
	timestamp += int64(raw[0]) << 56
	timestamp += int64(raw[1]) << 48
	timestamp += int64(raw[2]) << 40
	timestamp += int64(raw[3]) << 32
	timestamp += int64(raw[4]) << 24
	timestamp += int64(raw[5]) << 16
	timestamp += int64(raw[6]) << 8
	timestamp += int64(raw[7]) << 0
	return
}

// writeMAC calculates the MAC of the current contents of buffer and appends it to the end of buffer itself.
func writeMAC(buffer *bytes.Buffer, passphrase string) {
	hash := hmac.New(sha256.New, []byte(passphrase))
	hash.Write(buffer.Bytes())
	mac := hash.Sum(nil)
	buffer.Write(mac)
}

// checkMAC validates the MAC that is stored at the end of the message.
func checkMAC(message []byte, passphrase string) bool {
	if len(message) < macSize {
		return false
	}

	index := len(message) - macSize
	content, actualMAC := message[:index], message[index:]

	var expectedMAC []byte
	{ // Calculate expected MAC.
		hash := hmac.New(sha256.New, []byte(passphrase))
		hash.Write(content)
		expectedMAC = hash.Sum(nil)
	}

	return hmac.Equal(actualMAC, expectedMAC)
}
