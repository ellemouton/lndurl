package lndurl

import (
	"fmt"
	"strings"

	"github.com/ellemouton/lndurl/bech32"
)

const humanReadablePart = "lnurl"

func DecodeURL(lnurl string) (string, error) {
	hrp, data, err := bech32.Decode(lnurl)
	if err != nil {
		return "", err
	}

	if hrp != humanReadablePart {
		return "", fmt.Errorf("incorrect hrp for LNURL. Expected "+
			"'%s', got '%s'", hrp, humanReadablePart)
	}

	data, err = bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func EncodeURL(url string) (string, error) {
	converted, err := bech32.ConvertBits([]byte(url), 8, 5, true)
	if err != nil {
		return "", err
	}

	str, err := bech32.Encode(humanReadablePart, converted)
	if err != nil {
		return "", err
	}

	return strings.ToUpper(str), nil
}
