package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gorilla/mux"
)

const Version = "0.0.0"

type IMDSv2Service struct {
	Profile       string
	Configuration aws.Config
	ServiceToken  string
	Credential    aws.Credentials
}

const DefaultPort = "8080"

// helper functions

func contextWithSignal(ctx context.Context) context.Context {
	newCtx, cancel := context.WithCancel(ctx)
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-signals:
			log.Printf("Received %s signal.\n", sig.String())
			cancel()
		}
	}()
	return newCtx
}

func getEnvOrDefault(key string, defaults ...string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return ""
}

func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func loadCredential(cfg aws.Config) (aws.Credentials, error) {
	credential, err := cfg.Credentials.Retrieve(context.TODO())
	if err != nil {
		return aws.Credentials{}, err
	}
	return credential, nil
}

func createTemporaryCredential(cfg aws.Config) (aws.Credentials, error) {
	stsClient := sts.NewFromConfig(cfg)
	resp, err := stsClient.GetSessionToken(context.TODO(), &sts.GetSessionTokenInput{})
	if err != nil {
		return aws.Credentials{}, err
	}
	cred := resp.Credentials

	return aws.Credentials{
		Source:          "createTemporaryCredentials",
		AccessKeyID:     *cred.AccessKeyId,
		SecretAccessKey: *cred.SecretAccessKey,
		SessionToken:    *cred.SessionToken,
		Expires:         *cred.Expiration,
		CanExpire:       true,
	}, nil
}

func loadOrCreateCredential(cfg aws.Config) (aws.Credentials, error) {
	credential, err := loadCredential(cfg)
	if err != nil {
		return aws.Credentials{}, err
	}

	// issue temporary credential
	if !credential.CanExpire {
		credential, err = createTemporaryCredential(cfg)
		if err != nil {
			return aws.Credentials{}, err
		}
	}

	return credential, nil
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		log.Printf("%s %s %d %s (from %s, ua=%q)\n",
			r.Method,
			r.URL.Path,
			lrw.statusCode,
			duration,
			r.RemoteAddr,
			r.UserAgent(),
		)
	})
}

// service entrypoint

func main() {
	versionFlag := flag.Bool("version", false, "Print the version and exit.")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [aws-profile]\n\n", os.Args[0])
		fmt.Println("If \"aws-profile\" is provided, or the AWS_PROFILE environment variable is set, the app runs with that profile.")
	}
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Version: %s\n", Version)
		return
	}

	args := flag.Args()

	log.Println("Starting MetaDock Service (IMDSv2)...")

	profile := getEnvOrDefault("AWS_PROFILE", "default")
	if len(args) > 0 {
		profile = args[0]
	}
	log.Printf("Loading configuration for AWS profile %q.\n", profile)

	configuration, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		log.Fatalf("Unable to load AWS configuration: %v\n", err)
	}
	log.Printf("Configured for %q AWS profile.\n", profile)

	serviceToken, err := generateToken(32)
	if err != nil {
		log.Fatalf("Unable to generate token: %v\n", err)
	}

	// retrieve the credential at startup, as it takes too long if retrieved within
	// the roleCredentialsHandler function, since the callee has a very short TTFB timeout
	log.Printf("Retrieving credentials...\n")
	credential, err := loadOrCreateCredential(configuration)
	if err != nil {
		log.Fatalf("Unable to load or create temporary credential: %v\n", err)
	}
	log.Printf("Retrieved credentials, expires at %q.\n", credential.Expires)

	// setup service, middleware and router
	service := &IMDSv2Service{
		Profile:       profile,
		Configuration: configuration,
		ServiceToken:  serviceToken,
		Credential:    credential,
	}

	router := mux.NewRouter()
	router.Use(loggingMiddleware)

	router.HandleFunc("/", service.rootHandler).Methods("GET")
	router.HandleFunc("/latest/api/token", service.tokenHandler).Methods("PUT")
	router.HandleFunc("/latest/meta-data/iam/security-credentials/", service.credentialsHandler).Methods("GET")
	router.HandleFunc("/latest/meta-data/iam/security-credentials/{role}", service.roleCredentialsHandler).Methods("GET")
	router.HandleFunc("/health/", service.healthHandler).Methods("GET")

	// setup HTTP server
	port := getEnvOrDefault("PORT", DefaultPort)
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// context for background processing
	ctx := contextWithSignal(context.Background())

	// run the server in a goroutine so it doesn’t block
	go func() {
		log.Printf("Listening on port %s for all network interfaces.\n", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	// wait for SIGINT/SIGTERM
	<-ctx.Done()
	log.Println("Shutting down server...")

	// context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v\n", err)
	} else {
		log.Println("Shutdown successfully.")
	}
}

// IDMS implementation

// url: / (root)
func (svc *IMDSv2Service) rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "MetaDock")
}

// url: /latest/api/token
func (svc *IMDSv2Service) tokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, svc.ServiceToken)
}

// url: /latest/meta-data/iam/security-credentials/
func (svc *IMDSv2Service) credentialsHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-aws-ec2-metadata-token")
	if token != svc.ServiceToken {
		log.Printf("Token mismatched error.\n")
		http.Error(w, "", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "metadock")
}

// url: /latest/meta-data/iam/security-credentials/{role}
func (svc *IMDSv2Service) roleCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	// NB: this handler has to be very fast!

	token := r.Header.Get("X-aws-ec2-metadata-token")
	if token != svc.ServiceToken {
		log.Printf("Token mismatched error.\n")
		http.Error(w, "", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	credential := svc.Credential
	response := map[string]string{
		"Code":            "Success",
		"LastUpdated":     time.Now().Format(time.RFC3339),
		"Type":            "AWS-HMAC",
		"AccessKeyId":     credential.AccessKeyID,
		"SecretAccessKey": credential.SecretAccessKey,
		"Token":           credential.SessionToken,
		"Expiration":      credential.Expires.Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("roleCredentialsHandler encode error: %v\n", err)
		http.Error(w, "", http.StatusInternalServerError)
	}
}

// url: /health/
func (svc *IMDSv2Service) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	resp := map[string]string{"status": "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("healthHandler encode error: %v\n", err)
		http.Error(w, "", http.StatusInternalServerError)
	}
}
