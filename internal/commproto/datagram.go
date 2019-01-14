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

// ExtractAddress returns the address that is stored at the beginning of the message.
func ExtractAddress(message []byte) (address string, ok bool) {
	if len(message) == 0 {
		return
	}

	length := int(message[0])

	if len(message) < 1+length {
		return
	}

	address = string(message[1 : 1+length])
	ok = true
	return
}

// checkMAC validates the MAC that is stored at the end of the message.
func checkMAC(message []byte, passphrase string) bool {
	if len(message) < sha256.Size {
		return false
	}

	index := len(message) - sha256.Size
	content, actualMAC := message[:index], message[index:]

	var expectedMAC []byte
	{ // Calculate expected MAC.
		hash := hmac.New(sha256.New, []byte(passphrase))
		hash.Write(content)
		expectedMAC = hash.Sum(nil)
	}

	return hmac.Equal(actualMAC, expectedMAC)
}

// AssembleDatagram creates a datagram from the given data using the provided encryption secrets.
func AssembleDatagram(address string, iv []byte, timestamp int64, data []byte, key []byte, passphrase string) []byte {
	if len(address) > 255 {
		panic("address too long")
	}

	if len(iv) != 16 {
		panic("iv has wrong length")
	}

	if len(key) != 16 {
		panic("key has wrong length")
	}

	var buffer bytes.Buffer
	buffer.WriteByte(byte(len(address))) // length of address
	buffer.WriteString(address)          // address
	buffer.Write(iv)                     // initialization vector

	aesStart := buffer.Len()

	// Write timestamp in network byte order (big-endian).
	buffer.WriteByte(byte(timestamp >> 56))
	buffer.WriteByte(byte(timestamp >> 48))
	buffer.WriteByte(byte(timestamp >> 40))
	buffer.WriteByte(byte(timestamp >> 32))
	buffer.WriteByte(byte(timestamp >> 24))
	buffer.WriteByte(byte(timestamp >> 16))
	buffer.WriteByte(byte(timestamp >> 8))
	buffer.WriteByte(byte(timestamp >> 0))

	// Write the actual payload data.
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

	{ // Append MAC.
		hash := hmac.New(sha256.New, []byte(passphrase))
		hash.Write(buffer.Bytes())
		mac := hash.Sum(nil)
		buffer.Write(mac)
	}

	return buffer.Bytes()
}

// DisassembleDatagram validates and decrypts a datagram using the provided encryption secrets.
func DisassembleDatagram(datagram []byte, address string, key []byte, passphrase string) (timestamp int64, data []byte, err error) {
	if len(key) != 16 {
		panic("key has wrong length")
	}

	if !checkMAC(datagram, passphrase) {
		err = errors.New("invalid datagram")
		return
	}

	ivStart := 1 /* address length specification */ + len(address)
	ivEnd := ivStart + 16
	aesStart := ivEnd
	aesEnd := len(datagram) - sha256.Size

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

	if len(aesBuffer)-padding < 8 {
		err = errors.New("invalid datagram")
		return
	}

	timestamp += int64(aesBuffer[0]) << 56
	timestamp += int64(aesBuffer[1]) << 48
	timestamp += int64(aesBuffer[2]) << 40
	timestamp += int64(aesBuffer[3]) << 32
	timestamp += int64(aesBuffer[4]) << 24
	timestamp += int64(aesBuffer[5]) << 16
	timestamp += int64(aesBuffer[6]) << 8
	timestamp += int64(aesBuffer[7]) << 0

	data = aesBuffer[8 : len(aesBuffer)-padding]

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
