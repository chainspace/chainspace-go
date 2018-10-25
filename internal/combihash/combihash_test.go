package combihash

import (
	"testing"
)

var hashes = map[string]string{
	"":      "djEAAAAARrndKwuojRMjOz_rdD7rJD_NUupiuBuCtQwnZG7Vdi_XXcTd2MDyAMsFAZ1ntZL2_IIcSUeatIZAKS6ss7fEvsZyuNHvVu0oq4fDYixRFAab3TrXuPlzdJjQwB7O8JZ6",
	"a":     "djEAAAAAhn4ssE9aBNy9WSUBpej-nOqvylAlVibKc2wTgEJTC6Q2t7HsDgaiebx5BzO7Cu5vqAJoPHs1UGPENOkRibDGUUVeUYgkvAYB-fuFj_XDfUF9Z8L44N8rq-SAiFiuqDD4",
	"hello": "djEAAAAAEjQHWuSh53MWzy2AAJdFgaNDueu8p-PR24M5TDDyIWJvWU5PDeY5AjSaXqV4EhMhWBORn5Kk2G0SdGbj0H6L4-MNh8-ip121RerE1huvlwNmqDV8f3L6lbUtCsy2mPE6",
}

func TestBase64(t *testing.T) {
	hasher := New()
	for input, output := range hashes {
		t.Run(input, func(t *testing.T) {
			hasher.Reset()
			if _, err := hasher.Write([]byte(input)); err != nil {
				t.Errorf("received unexpected error when hashing %q: %s", input, err)
				return
			}
			digest := string(hasher.Base64())
			if digest != output {
				t.Errorf("got %s, want %s", digest, output)
			}
		})
	}
}
