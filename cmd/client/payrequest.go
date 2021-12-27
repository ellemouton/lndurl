package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ellemouton/lndurl"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/zpay32"
	"github.com/urfave/cli/v2"
)

var payRequestCommand = &cli.Command{
	Name:        "pay",
	Usage:       "Pay to LNURL",
	Description: `Pay to a static LNURL`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "lnurl",
			Usage: "The LNURL to pay too.",
		},
		&cli.Int64Flag{
			Name:  "amt",
			Usage: "The amt of millisats to pay",
		},
		&cli.Int64Flag{
			Name:  "maxfee",
			Usage: "max fee to pay for this payment (in millisats)",
			Value: 1000,
		},
		&cli.BoolFlag{
			Name:  "notls",
			Usage: "set to true to use http instead of https",
		},
	},
	Action: payToLNURL,
}

func payToLNURL(ctx *cli.Context) error {
	// LNURL must be specified.
	lnurl := ctx.String("lnurl")
	if lnurl == "" {
		return fmt.Errorf("missing '--lnurl' flag")
	}

	protocol := "https"
	if ctx.Bool("notls") {
		protocol = "http"
	}

	var (
		url string
		err error
	)
	switch {
	case strings.HasPrefix(lnurl, "LNURL"):
		url, err = lndurl.DecodeURL(lnurl)
		if err != nil {
			return fmt.Errorf("error decoding LNURL: %w", err)
		}

	case strings.HasPrefix(lnurl, "lightning:"):
		fmt.Println(lnurl)
		url, err = lndurl.DecodeURL(
			strings.TrimPrefix(lnurl, "lightning:"),
		)
		if err != nil {
			return fmt.Errorf("error decoding LNURL: %w", err)
		}

	case strings.HasPrefix(lnurl, "lnurlp://"):
		url = strings.Replace(lnurl, "lnurlp", protocol, 1)

	case strings.Contains(lnurl, "@"):
		// This is an LN Address:
		parts := strings.Split(lnurl, "@")
		if len(parts) != 2 {
			return fmt.Errorf("invalid LN address. Expected" +
				"the form <username>@<domain>")
		}

		username, domain := parts[0], parts[1]
		url = fmt.Sprintf("%s://%s/.well-known/lnurlp/%s",
			protocol, domain, username)

	default:
		return fmt.Errorf("unsupported scheme")
	}

	// Ensure that the url uses the tls if we have not set --notls
	if !ctx.Bool("notls") && !strings.HasPrefix(url, "https") {
		return fmt.Errorf("url is not https")
	}

	// Make a GET request to the decoded LNURL.
	var payResp lndurl.PayResponse
	if err := get(url, &payResp); err != nil {
		return err
	}

	// Ensure that the response contains the necessary metadata field.
	var meta string
	for _, d := range payResp.Metadata {
		if d[0] == "text/plain" {
			meta = d[1]
		}
	}
	if meta == "" {
		return fmt.Errorf("response metadata does not contain the " +
			"required 'text/plain' field")
	}

	minSendable, err := strconv.ParseInt(payResp.MinSendable, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse MinSendable: %w", err)
	}

	maxSendable, err := strconv.ParseInt(payResp.MaxSendable, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse MaxSendable: %w", err)
	}

	// Check if the user specified an amount in the original call. If they
	// did not or if the specified amount is not within the bounds specified
	// in the server response, ask the user to enter a valid amount.
	millisats := ctx.Int64("amt")
	for millisats < minSendable || millisats > maxSendable {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Enter an amount (in millisatoshis) between "+
			"%d and %d\n", minSendable, maxSendable)

		userInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("could not read from console: %w",
				err)
		}
		userInput = strings.TrimSpace(userInput)

		millisats, err = strconv.ParseInt(userInput, 10, 64)
		if err != nil {
			fmt.Printf("error parsing input: %v", err)
			continue
		}

		if millisats < minSendable || millisats > maxSendable {
			fmt.Printf("Invalid amount. Expected an amount "+
				"between %d and %d, got %d\n", minSendable,
				maxSendable, millisats)
		}
	}

	delim := "?"
	if strings.Contains(payResp.Callback, "?") {
		delim = "&"
	}

	getInvoice := fmt.Sprintf(
		"%s%samount=%d", payResp.Callback, delim, 2000,
	)

	var invoice lndurl.InvoiceResponse
	if err := get(getInvoice, &invoice); err != nil {
		return err
	}

	inv, err := zpay32.Decode(
		invoice.PayRequest, &chaincfg.RegressionNetParams,
	)
	if err != nil {
		return err
	}

	// Ensure that the invoice description hash matches the metadata
	// received before.
	hash := sha256.Sum256([]byte(meta))
	if !bytes.Equal(inv.DescriptionHash[:], hash[:]) {
		return fmt.Errorf("invalid invoice description hash")
	}

	lndClient, err := getLND(ctx)
	if err != nil {
		return fmt.Errorf("could not connect to LND: %w", err)
	}

	res := <-lndClient.Client.PayInvoice(
		ctx.Context, invoice.PayRequest,
		btcutil.Amount(ctx.Int64("maxfee")), nil,
	)

	if res.Err != nil {
		return fmt.Errorf("could not pay invoice: %w", res.Err)
	}

	fmt.Printf("Successful payment! Preimage: %s\n", res.Preimage)

	return nil
}
