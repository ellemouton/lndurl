package lndurl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

type Server struct {
	cfg       *Config
	lndClient lndclient.LightningClient

	paymentMetadata map[string]*metadata
	metadataMu      sync.Mutex
}

type metadata struct {
	data      string
	createdAt time.Time
}

type Config struct {
	Protocol        string
	Username        string
	Host            string
	Port            int
	LndAddr         string
	Network         lndclient.Network
	MacaroonDir     string
	TLSPath         string
	MinMsatSendable int64
	MaxMsatSendable int64
}

func NewServer(cfg *Config) (*Server, error) {
	s := Server{
		cfg:             cfg,
		paymentMetadata: make(map[string]*metadata),
	}

	// Connect to LND.
	lnd, err := lndclient.NewLndServices(&lndclient.LndServicesConfig{
		LndAddress:  cfg.LndAddr,
		Network:     cfg.Network,
		MacaroonDir: cfg.MacaroonDir,
		TLSPath:     cfg.TLSPath,
	})
	if err != nil {
		return nil, err
	}
	s.lndClient = lnd.Client

	// Register routes with the http default mux.
	http.HandleFunc("/pay", s.pay(false))
	http.HandleFunc("/invoice", s.invoice)
	http.HandleFunc(
		fmt.Sprintf("/.well-known/lnurlp/%s", cfg.Username),
		s.pay(true),
	)

	return &s, nil
}

func (s *Server) Run() error {
	if err := s.printHello(); err != nil {
		return err
	}

	info, err := s.lndClient.GetInfo(context.Background())
	if err != nil {
		return err
	}

	fmt.Println("Connected to node with alias:", info.Alias)

	return http.ListenAndServe(":8080", nil)
}

func (s *Server) printHello() error {
	payCode := fmt.Sprintf(
		"%s://%s:%d/pay", s.cfg.Protocol, s.cfg.Host, s.cfg.Port,
	)

	payLNURL, err := EncodeURL(payCode)
	if err != nil {
		return err
	}

	lnAddress := fmt.Sprintf("%s@%s", s.cfg.Username, s.cfg.Host)
	if s.cfg.Port != 80 {
		lnAddress += fmt.Sprintf(":%d", s.cfg.Port)
	}

	fmt.Printf(
		""+
			"=======================================\n"+
			"Welcome to LNDURL!\n"+
			"Your static LNURL-pay code is: \n"+
			"- %s\n"+
			"- lightning:%s\n"+
			"- %s\n"+
			"- %s\n"+
			"=======================================\n",
		payLNURL, payLNURL, strings.Replace(
			payCode, s.cfg.Protocol, "lnurlp", 1,
		), lnAddress,
	)

	return nil
}

func (s *Server) pay(lnAddress bool) func(w http.ResponseWriter,
	r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		// TODO(elle): checkout client IP here to throttle requests.

		var hash [32]byte
		if _, err := rand.Read(hash[:]); err != nil {
			http.Error(
				w, err.Error(), http.StatusInternalServerError,
			)
			return
		}

		h := hex.EncodeToString(hash[:])
		id := hex.EncodeToString(hash[:10])
		meta := &metadata{
			data:      fmt.Sprintf("[[\"text/plain\",\"%s\"]]", h),
			createdAt: time.Now(),
		}

		if lnAddress {
			addr := fmt.Sprintf("%s@%s", s.cfg.Username, s.cfg.Host)
			if s.cfg.Port != 80 {
				addr += fmt.Sprintf(":%d", s.cfg.Port)
			}

			meta.data = fmt.Sprintf("[[\"text/plain\",\"%s\"],[\"text/identifier\",\"%s\"]]", h, addr)
		}

		// TODO(elle): kick off a goroutine to expire & delete this
		//  metadata after x amount of time.
		s.metadataMu.Lock()
		s.paymentMetadata[id] = meta
		s.metadataMu.Unlock()

		getInvoice := fmt.Sprintf(
			"%s://%s:%d/invoice?id=%s", s.cfg.Protocol, s.cfg.Host,
			s.cfg.Port, id,
		)

		resp := &PayResponse{
			Callback:    getInvoice,
			MinSendable: s.cfg.MinMsatSendable,
			MaxSendable: s.cfg.MaxMsatSendable,
			Metadata:    meta.data,
			Tag:         TypePayRequest,
		}

		b, _ := json.Marshal(resp)
		fmt.Fprintf(w, string(b))
	}
}

func (s *Server) invoice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := r.Form.Get("id")
	if id == "" {
		http.Error(w, "expected 'id' field", http.StatusBadRequest)
		return
	}

	s.metadataMu.Lock()
	meta, ok := s.paymentMetadata[id]
	if !ok {
		s.metadataMu.Unlock()
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	delete(s.paymentMetadata, id)
	s.metadataMu.Unlock()

	amt := r.Form.Get("amount")
	if amt == "" {
		http.Error(w, "expected 'amount' field", http.StatusBadRequest)
		return
	}

	milliSats, err := strconv.ParseInt(amt, 10, 64)
	if err != nil {
		http.Error(w, "expected 'amount' field", http.StatusBadRequest)
		return
	}

	h := sha256.Sum256([]byte(html.UnescapeString(meta.data)))
	ln := lntypes.Hash(h)

	_, pr, err := s.lndClient.AddInvoice(ctx, &invoicesrpc.AddInvoiceData{
		Memo:            "LNDURL-pay",
		Value:           lnwire.MilliSatoshi(milliSats),
		DescriptionHash: ln[:],
	})
	resp := &InvoiceResponse{
		PayRequest: pr,
	}
	if err != nil {
		http.Error(w, "invoice error", http.StatusInternalServerError)
		return
	}

	b, _ := json.Marshal(resp)
	fmt.Fprintf(w, string(b))
}

func (s *Server) lnAddress(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hellooooo")
}
