package hvaultapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/stellwerk-labs/golib/hvaultapi/tokens"
)

const (
	vaultURL       = "http://localhost:8200"
	vaultRole      = "k8s-vault-role"
	vaultTokenPath = "./vault/token"
)

func TestIsReady(t *testing.T) {
	vltClient, err := NewWithDefaults(vaultURL, "file:"+vaultTokenPath, vaultRole, &http.Client{}, zaptest.NewLogger(t), func(config *api.Config) {
		config.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	require.NoError(t, err)
	assert.NotZero(t, vltClient.Client().CloneConfig().MinRetryWait)
	assert.NotZero(t, vltClient.Client().CloneConfig().MaxRetryWait)
	assert.Zero(t, vltClient.Client().CloneConfig().MaxRetries)

	_, err = vltClient.Login(context.Background())
	assert.Eventually(t, func() bool {
		vaultIsReady := false
		for !vaultIsReady {
			vaultIsReady, err = vltClient.IsReady(context.Background())
			if err != nil {
				time.Sleep(2 * time.Second)
			}
		}
		return true
	}, time.Second*10, time.Second, "vault not ready after 10 secs")
}

func TestLogin(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	logger, err := hlogger.New("INFO", false, "console")
	assert.NoError(err)

	client, err := api.NewClient(&api.Config{
		Address:    vaultURL,
		HttpClient: &http.Client{},
	})
	assert.NoError(err)

	vltClient := New(client, vaultRole, "", nil, tokens.NewFileTokenSource(vaultTokenPath), logger)
	_, err = vltClient.Login(ctx)
	assert.NoError(err)
}

func TestLogin_WithTokenRequestAPI(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	expectedNamespace := "test-namespace"
	expectedServiceAccount := "test-sa"
	expectedAudiences := []string{"vault", "custom-audience"}
	expectedJWT := "fake-jwt-token-from-token-request"
	expectedVaultToken := "s.mock-vault-token"

	var capturedNamespace, capturedServiceAccount string
	var capturedAudiences []string
	var capturedExpiration int64
	var capturedJWT, capturedRole string

	// Mock Vault server
	vaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/kubernetes/login" && r.Method == http.MethodPut {
			var reqBody map[string]string
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			capturedJWT = reqBody["jwt"]
			capturedRole = reqBody["role"]

			resp := map[string]interface{}{
				"auth": map[string]interface{}{
					"client_token": expectedVaultToken,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer vaultServer.Close()

	// Fake Kubernetes client
	fakeClientset := fake.NewClientset()
	fakeClientset.PrependReactor("create", "serviceaccounts/token", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateActionImpl)
		tokenRequest := createAction.GetObject().(*authenticationv1.TokenRequest)

		capturedNamespace = action.GetNamespace()
		capturedServiceAccount = createAction.Name
		capturedAudiences = tokenRequest.Spec.Audiences
		if tokenRequest.Spec.ExpirationSeconds != nil {
			capturedExpiration = *tokenRequest.Spec.ExpirationSeconds
		}

		return true, &authenticationv1.TokenRequest{
			Status: authenticationv1.TokenRequestStatus{
				Token:               expectedJWT,
				ExpirationTimestamp: metav1.Now(),
			},
		}, nil
	})

	client, err := api.NewClient(&api.Config{
		Address:    vaultServer.URL,
		HttpClient: &http.Client{},
	})
	require.NoError(t, err)

	vltClient := &hVaultAPIClient{
		client:             client,
		vaultRole:          vaultRole,
		serviceAccountName: expectedServiceAccount,
		audiences:          expectedAudiences,
		logger:             logger,
		k8sClientset:       fakeClientset,
		namespace:          expectedNamespace,
	}

	authInfo, err := vltClient.Login(ctx)

	require.NoError(t, err)
	require.NotNil(t, authInfo)

	// Verify CreateToken was called with expected parameters
	assert.Equal(t, expectedNamespace, capturedNamespace)
	assert.Equal(t, expectedServiceAccount, capturedServiceAccount)
	assert.Equal(t, expectedAudiences, capturedAudiences)
	assert.Equal(t, DefaultTokenExpirationSeconds, capturedExpiration)

	// Verify JWT from TokenRequest API was passed to Vault
	assert.Equal(t, expectedJWT, capturedJWT)
	assert.Equal(t, vaultRole, capturedRole)

	// Verify Vault token was set on the client
	assert.Equal(t, expectedVaultToken, vltClient.client.Token())
}
