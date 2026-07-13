package hvaultapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/kubernetes"
	"github.com/stellwerk-labs/golib/hvaultapi/tokens"
	"go.uber.org/zap"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// ServiceAccountTokenPath is the default path to the service account token in Kubernetes pods
	ServiceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	// NamespacePath is the path to the namespace file in Kubernetes pods
	NamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	// DefaultTokenExpirationSeconds is the default expiration for requested tokens (1 hour)
	DefaultTokenExpirationSeconds int64 = 3600
)

type hVaultAPIClient struct {
	client             *api.Client
	vaultRole          string
	logger             *zap.Logger
	tokens             TokenSource
	k8sClientset       k8s.Interface
	namespace          string
	serviceAccountName string
	audiences          []string

	k8sInitOnce sync.Once
	k8sInitErr  error
}

type HVaultAPI interface {
	Client() *api.Client
	IsReady(ctx context.Context) (bool, error)
	Login(ctx context.Context) (*api.Secret, error)
	ManageTokenLifecycle(ctx context.Context, token *api.Secret) error
	WaitUntilReady(ctx context.Context)
	PeriodicallyRenewToken(ctx context.Context)
}

// getNamespaceFromFile reads the namespace from the mounted service account secrets.
func getNamespaceFromFile() (string, error) {
	data, err := os.ReadFile(NamespacePath)
	if err != nil {
		return "", fmt.Errorf("failed to read namespace from %s: %w", NamespacePath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// createK8sClientset creates a Kubernetes clientset using in-cluster configuration.
func createK8sClientset() (k8s.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	return clientset, nil
}

// requestServiceAccountToken requests a new token using the TokenRequest API.
// The service account must have necessary RBAC permissions to create tokens.
func requestServiceAccountToken(
	ctx context.Context,
	clientset k8s.Interface,
	namespace string,
	serviceAccountName string,
	audiences []string,
	expirationSeconds int64,
) (string, error) {
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         audiences,
			ExpirationSeconds: &expirationSeconds,
		},
	}

	response, err := clientset.CoreV1().ServiceAccounts(namespace).CreateToken(
		ctx,
		serviceAccountName,
		tokenRequest,
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to request service account token: %w", err)
	}

	return response.Status.Token, nil
}

// New creates a new HVaultAPI client.
//
// Parameters:
//   - client: The Vault API client
//   - vaultRole: The Vault role for Kubernetes authentication
//   - serviceAccountName: The Kubernetes service account name to use for TokenRequest API
//   - audiences: Optional audiences for the token (can be nil or empty)
//   - tokenSource: Optional token source for direct Vault token authentication (bypasses K8s auth if set)
//   - logger: Logger instance
//
// When tokenSource is nil, the client will use Kubernetes TokenRequest API for authentication.
// The K8s clientset and namespace are initialized lazily on first Login() call.
func New(
	client *api.Client,
	vaultRole string,
	serviceAccountName string,
	audiences []string,
	tokenSource TokenSource,
	logger *zap.Logger,
) HVaultAPI {
	return &hVaultAPIClient{
		client:             client,
		vaultRole:          vaultRole,
		serviceAccountName: serviceAccountName,
		audiences:          audiences,
		tokens:             tokenSource,
		logger:             logger,
	}
}

// NewWithDefaults creates a new HVaultAPI client with default configuration.
//
// The tokenSource parameter supports the following formats:
//   - "token:<value>" - Direct Vault token string
//   - "file:<path>" - File-based Vault token
//   - "kubernetes:<service-account-name>" - Kubernetes TokenRequest API auth
//   - "kubernetes:<service-account-name>:<audience1>,<audience2>" - With custom audiences
//   - "<path>" (default) - File-based Vault token at the specified path
func NewWithDefaults(baseURL, tokenSource, role string, httpClient *http.Client, logger *zap.Logger, cfgPatch ...func(config *api.Config)) (HVaultAPI, error) {
	cfg := api.DefaultConfig()
	cfg.Address = baseURL
	cfg.HttpClient = httpClient
	// Disable retries by default - allow these to be turned on via the config patch functions
	cfg.MaxRetries = 0
	for _, patch := range cfgPatch {
		patch(cfg)
	}
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	var vaultTokens TokenSource
	var serviceAccountName string
	var audiences []string

	if tokenSource != "" {
		parts := strings.SplitN(tokenSource, ":", 2)
		if len(parts) != 2 {
			// this is the default behavior if 'file:' or 'token:' is not the prefix
			vaultTokens = tokens.NewFileTokenSource(tokenSource)
		} else if parts[0] == "token" {
			vaultTokens = tokens.NewStringTokenSource(parts[1])
		} else if parts[0] == "file" {
			vaultTokens = tokens.NewFileTokenSource(parts[1])
		} else if parts[0] == "kubernetes" {
			// Format: "kubernetes:<service-account-name>" or "kubernetes:<sa-name>:<aud1>,<aud2>"
			subParts := strings.SplitN(parts[1], ":", 2)
			serviceAccountName = subParts[0]
			if len(subParts) == 2 && subParts[1] != "" {
				audiences = strings.Split(subParts[1], ",")
			}
		} else {
			return nil, fmt.Errorf("invalid token source: %s", tokenSource)
		}
	}

	return New(client, role, serviceAccountName, audiences, vaultTokens, logger), nil
}

func (vlt *hVaultAPIClient) Client() *api.Client {
	return vlt.client
}

func (vlt *hVaultAPIClient) Login(ctx context.Context) (*api.Secret, error) {
	vlt.logger.Debug("attempting vault login")

	// Path 1: Use TokenSource if configured (direct Vault token)
	if vlt.tokens != nil {
		token, err := vlt.tokens.GetToken()
		if err != nil {
			return nil, fmt.Errorf("can't retrieve token by token source: %w", err)
		}
		vlt.client.SetToken(token)
		return nil, nil
	}

	var jwt string

	if vlt.serviceAccountName != "" {
		// Path 2: Use Kubernetes TokenRequest API when service account name is provided
		// Thread-safe lazy initialization of Kubernetes client and namespace
		vlt.k8sInitOnce.Do(func() {
			// Skip initialization if already set (e.g., injected for testing)
			if vlt.k8sClientset != nil && vlt.namespace != "" {
				return
			}

			clientset, err := createK8sClientset()
			if err != nil {
				vlt.k8sInitErr = fmt.Errorf("failed to create Kubernetes client: %w", err)
				return
			}
			vlt.k8sClientset = clientset

			namespace, err := getNamespaceFromFile()
			if err != nil {
				vlt.k8sInitErr = fmt.Errorf("failed to detect namespace: %w", err)
				return
			}
			vlt.namespace = namespace
		})
		if vlt.k8sInitErr != nil {
			return nil, vlt.k8sInitErr
		}

		// Request a fresh token using TokenRequest API
		token, err := requestServiceAccountToken(
			ctx,
			vlt.k8sClientset,
			vlt.namespace,
			vlt.serviceAccountName,
			vlt.audiences,
			DefaultTokenExpirationSeconds,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get service account token via TokenRequest API: %w", err)
		}
		jwt = token
	} else {
		// Path 3: Fallback to reading token from file (legacy behavior)
		tokenBytes, err := os.ReadFile(ServiceAccountTokenPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read service account token from %s: %w", ServiceAccountTokenPath, err)
		}
		jwt = strings.TrimSpace(string(tokenBytes))
	}

	// Authenticate with Vault using the token
	kubeAuth, err := kubernetes.NewKubernetesAuth(vlt.vaultRole, kubernetes.WithServiceAccountToken(jwt))
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes auth method: %w", err)
	}

	authInfo, err := vlt.client.Auth().Login(ctx, kubeAuth)
	if err != nil {
		return nil, fmt.Errorf("vault login failed: %w", err)
	}

	return authInfo, nil
}

// ManageTokenLifecycle Starts token lifecycle management. Returns only fatal errors as errors,
// otherwise returns nil so we can attempt login again.
func (vlt *hVaultAPIClient) ManageTokenLifecycle(ctx context.Context, token *api.Secret) error {
	renew := token.Auth.Renewable // You may notice a different top-level field called Renewable. That one is used for dynamic secrets renewal, not token renewal.
	if !renew {
		vlt.logger.Sugar().Warn("Token is not configured to be renewable. Re-attempting login.")
		return nil
	}

	watcher, err := vlt.client.NewLifetimeWatcher(&api.LifetimeWatcherInput{
		Secret:    token,
		Increment: 3600, // Learn more about this optional value in https://developer.hashicorp.com/vault/docs/concepts/lease#lease-durations-and-renewal
	})
	if err != nil {
		return fmt.Errorf("unable to initialize new lifetime watcher for renewing auth token: %w", err)
	}

	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		// `DoneCh` will return if renewal fails, or if the remaining lease
		// duration is under a built-in threshold and either renewing is not
		// extending it or renewing is disabled. In any case, the caller
		// needs to attempt to log in again.
		case <-ctx.Done():
			vlt.logger.Sugar().Info("Stopping Vault token renewal")
			return nil
		case err := <-watcher.DoneCh():
			if err != nil {
				vlt.logger.Sugar().Warnw("Failed to renew token. Re-attempting login.", "err", err)
				return nil
			}
			// This occurs once the token has reached max TTL.
			vlt.logger.Sugar().Warnw("Token can no longer be renewed. Re-attempting login.")
			return nil

		// Successfully completed renewal
		case renewal := <-watcher.RenewCh():
			vlt.logger.Sugar().Debugf("Successfully renewed: %#v", renewal)
		}
	}
}

// IsReady returns true if the vault client has a token
func (vlt *hVaultAPIClient) IsReady(ctx context.Context) (bool, error) {
	if _, err := vlt.Login(ctx); err != nil {
		return false, nil
	}

	return true, nil
}

// WaitUntilReady waits until the vault client is ready
func (vlt *hVaultAPIClient) WaitUntilReady(ctx context.Context) {
	sugar := vlt.logger.Sugar()

	for {
		_, err := vlt.IsReady(ctx)
		if err != nil {
			sugar.Warnw("vault client not ready yet", "err", err)
		} else {
			return
		}

		// Wait for timer or ctx close
		select {
		case <-time.After(2 * time.Second):
			continue
		case <-ctx.Done():
			break
		}
	}
}

// PeriodicallyRenewToken renews the token periodically
func (vlt *hVaultAPIClient) PeriodicallyRenewToken(ctx context.Context) {
	sugar := vlt.logger.Sugar()

	// run routine to handle token renewal
	for {
		vaultLoginResp, err := vlt.Login(ctx)
		if err != nil {
			sugar.Fatalf("unable to authenticate to Vault", "err", err)
			break
		}

		if vaultLoginResp != nil {
			tokenErr := vlt.ManageTokenLifecycle(ctx, vaultLoginResp)
			if tokenErr != nil {
				sugar.Fatalf("unable to start managing token lifecycle", "err", tokenErr)
				break
			}
		}
		// Prevent an infinite tight loop here
		select {
		case <-time.After(time.Second):
			continue
		case <-ctx.Done():
			break
		}
	}
}
