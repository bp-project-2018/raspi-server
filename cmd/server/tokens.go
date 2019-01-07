package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
)

var tokenStorage struct {
	Tokens []string `json:"tokens"`
}

func verifyToken(token string) (valid bool) {
	for _, t := range tokenStorage.Tokens {
		if token == t {
			return true
		}
	}
	return false
}

func generateToken() (token string) {
	b := make([]byte, 16)
	rand.Read(b)
	token = fmt.Sprintf("%x", b)
	addToken(token)
	return token
}

func addToken(token string) {
	tokenStorage.Tokens = append(tokenStorage.Tokens, token)
	log.Println("[tokens] new token added to token storage")
	saveTokens()
}

func loadTokens() {
	data, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		tokenStorage.Tokens = make([]string, 0)
		log.Println("[tokens] empty token list created")
	} else {
		json.Unmarshal(data, &tokenStorage)
		log.Println("[tokens] token list loaded")
	}
}

func saveTokens() {
	data, err := json.MarshalIndent(tokenStorage, "", "\t")
	if err != nil {
		log.Println("[tokens] could not encode token storage")
		log.Panicln(err)
	}
	err = ioutil.WriteFile(tokenFile, data, 0644)
	if err != nil {
		log.Println("[tokens] failed to write tokens file")
		log.Println(err)
	}
}
