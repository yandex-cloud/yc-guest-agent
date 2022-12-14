package linux

import (
	"crypto/ed25519"
	"encoding/pem"
	"github.com/gruntwork-io/terratest/modules/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

func PairFromED25519(public ed25519.PublicKey, private ed25519.PrivateKey) (*ssh.KeyPair, error) {
	// see https://github.com/golang/crypto/blob/7f63de1d35b0f77fa2b9faea3e7deb402a2383c8/ssh/keys.go#L1273-L1443
	// and https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.key
	// Also the right cipher block size for padding could be found here
	// https://github.com/openssh/openssh-portable/blob/eba523f0a130f1cce829e6aecdcefa841f526a1a/cipher.c#L112
	const noneCipherBlockSize = 8

	key := struct {
		Pub     []byte
		Priv    []byte
		Comment string
		Pad     []byte `ssh:"rest"`
	}{
		Pub:  public,
		Priv: private,
	}
	keyBytes := cryptossh.Marshal(key)

	pk1 := struct {
		Check1  uint32
		Check2  uint32
		Keytype string
		Rest    []byte `ssh:"rest"`
	}{
		Keytype: cryptossh.KeyAlgoED25519,
		Rest:    keyBytes,
	}
	pk1Bytes := cryptossh.Marshal(pk1)

	pubk1 := struct {
		Keytype string
		Key     []byte
	}{
		Keytype: cryptossh.KeyAlgoED25519,
		Key:     public,
	}
	pubk1Bytes := cryptossh.Marshal(pubk1)

	padLen := noneCipherBlockSize - (len(pk1Bytes) % noneCipherBlockSize)
	for i := 1; i <= padLen; i++ {
		pk1Bytes = append(pk1Bytes, byte(i))
	}

	k := struct {
		CipherName   string
		KdfName      string
		KdfOpts      string
		NumKeys      uint32
		PubKey       []byte
		PrivKeyBlock []byte
	}{
		CipherName:   "none",
		KdfName:      "none",
		KdfOpts:      "",
		NumKeys:      1,
		PubKey:       pubk1Bytes,
		PrivKeyBlock: pk1Bytes,
	}

	const opensshV1Magic = "openssh-key-v1\x00"

	privBlk := &pem.Block{
		Type:    "OPENSSH PRIVATE KEY",
		Headers: nil,
		Bytes:   append([]byte(opensshV1Magic), cryptossh.Marshal(k)...),
	}
	publicKey, err := cryptossh.NewPublicKey(public)
	if err != nil {
		return nil, err
	}
	return &ssh.KeyPair{
		PrivateKey: string(pem.EncodeToMemory(privBlk)),
		PublicKey:  string(cryptossh.MarshalAuthorizedKey(publicKey)),
	}, nil
}
