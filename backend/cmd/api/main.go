package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/daviddoherty10/Restringing.ie/backend/internal/auth"
	"github.com/daviddoherty10/Restringing.ie/backend/internal/config"
	"github.com/daviddoherty10/Restringing.ie/backend/internal/database"
	"github.com/daviddoherty10/Restringing.ie/backend/internal/marketplace"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.Println("Connected to PostgreSQL")
	phone := &auth.PhoneService{Pool: pool, RateKey: cfg.AuthRateKey, FrontendOrigin: cfg.FrontendOrigin, CookieSecure: cfg.AuthCookieSecure, AllowedCallingCodes: cfg.AuthCallingCodes, TrustedProxies: cfg.AuthTrustedProxies}
	if cfg.SMSProvider == "twilio" {
		phone.Provider = &auth.TwilioVerify{ServiceSID: cfg.TwilioServiceSID, Username: cfg.TwilioUsername, Password: cfg.TwilioPassword}
	} else {
		log.Println("SMS sign-in disabled: configure AUTH_SMS_PROVIDER to enable it")
	}

	store := &marketplace.Store{Pool: pool}
	if cfg.BillingProvider == "stripe" {
		store.Billing = &marketplace.BillingService{Provider: &marketplace.StripeBilling{SecretKey: cfg.StripeSecretKey}, PriceID: cfg.StripePriceID, WebhookSecret: cfg.StripeWebhookSecret, FrontendOrigin: cfg.FrontendOrigin, Live: strings.HasPrefix(cfg.StripeSecretKey, "sk_live_")}
	}
	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           marketplace.Router(store, cfg.FrontendOrigin, phone),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdown); err != nil {
				_ = server.Close()
			}
		case <-done:
		}
	}()

	log.Printf("API listening on %s", cfg.APIAddr)

	err = server.ListenAndServe()
	close(done)
	<-shutdownDone
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
