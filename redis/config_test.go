package redis

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.False(t, config.IsCluster())
	assert.False(t, config.IsSentinel())

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.Equal(t, []string{"localhost:6379"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:6379"}, options.InitAddress)
	assert.Equal(t, 0, options.SelectDB)
	assert.Equal(t, 30*time.Second, options.Dialer.KeepAlive)
	assert.False(t, options.ShuffleInit)
	assert.Nil(t, options.TLSConfig)
}

func TestTrailingSlashHostname(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Equal(t, []string{"localhost:6379"}, options.InitAddress)
	assert.Equal(t, 0, options.SelectDB)
}

func TestCustomDatabase(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Equal(t, 1, options.SelectDB)
}

func TestCustomOptions(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1?dial_timeout=30s"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Equal(t, 30*time.Second, options.Dialer.Timeout)
}

func TestTLS(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "rediss://localhost:6379/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.True(t, options.TLSConfig.InsecureSkipVerify)

	config.TLSVerify = true
	options, err = config.ToRueidisOptions()
	require.NoError(t, err)

	assert.False(t, options.TLSConfig.InsecureSkipVerify)
}

func TestTLSClientCertAvailable(t *testing.T) {
	config := NewRedisConfig()
	assert.False(t, config.TLSClientCertAvailable())

	config.TLSClientCertPath = "/path/to/cert.pem"
	assert.False(t, config.TLSClientCertAvailable())

	config.TLSClientKeyPath = "/path/to/key.pem"
	assert.True(t, config.TLSClientCertAvailable())
}

// generateTestCertFiles creates a self-signed certificate and key files for testing
func generateTestCertFiles(t *testing.T, certPath, keyPath string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	err = os.WriteFile(certPath, certPEM, 0600)
	require.NoError(t, err)

	err = os.WriteFile(keyPath, keyPEM, 0600)
	require.NoError(t, err)
}

// generateTestCACertFile creates a CA certificate file for testing
func generateTestCACertFile(t *testing.T, caPath string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	err = os.WriteFile(caPath, caPEM, 0600)
	require.NoError(t, err)
}

func TestTLSMutualTLS(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "client.crt")
	keyPath := filepath.Join(tmpDir, "client.key")

	generateTestCertFiles(t, certPath, keyPath)

	config := NewRedisConfig()
	config.URL = "rediss://localhost:6379/1"
	config.TLSClientCertPath = certPath
	config.TLSClientKeyPath = keyPath

	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.NotNil(t, options.TLSConfig)
	assert.Len(t, options.TLSConfig.Certificates, 1)
}

func TestTLSCACert(t *testing.T) {
	tmpDir := t.TempDir()
	caPath := filepath.Join(tmpDir, "ca.crt")

	generateTestCACertFile(t, caPath)

	config := NewRedisConfig()
	config.URL = "rediss://localhost:6379/1"
	config.TLSCACertPath = caPath

	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.NotNil(t, options.TLSConfig)
	assert.NotNil(t, options.TLSConfig.RootCAs)
}

func TestTLSFullMutualTLS(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "client.crt")
	keyPath := filepath.Join(tmpDir, "client.key")
	caPath := filepath.Join(tmpDir, "ca.crt")

	generateTestCertFiles(t, certPath, keyPath)
	generateTestCACertFile(t, caPath)

	config := NewRedisConfig()
	config.URL = "rediss://localhost:6379/1"
	config.TLSClientCertPath = certPath
	config.TLSClientKeyPath = keyPath
	config.TLSCACertPath = caPath
	config.TLSVerify = true

	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.NotNil(t, options.TLSConfig)
	assert.Len(t, options.TLSConfig.Certificates, 1)
	assert.NotNil(t, options.TLSConfig.RootCAs)
	assert.False(t, options.TLSConfig.InsecureSkipVerify)
}

func TestTLSMutualTLSInvalidCert(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "rediss://localhost:6379/1"
	config.TLSClientCertPath = "/nonexistent/cert.pem"
	config.TLSClientKeyPath = "/nonexistent/key.pem"

	_, err := config.ToRueidisOptions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load Redis client certificate")
}

func TestTLSCACertInvalid(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "rediss://localhost:6379/1"
	config.TLSCACertPath = "/nonexistent/ca.pem"

	_, err := config.ToRueidisOptions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read Redis CA certificate")
}

func TestTLSCACertInvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	caPath := filepath.Join(tmpDir, "ca.crt")

	err := os.WriteFile(caPath, []byte("invalid pem content"), 0600)
	require.NoError(t, err)

	config := NewRedisConfig()
	config.URL = "rediss://localhost:6379/1"
	config.TLSCACertPath = caPath

	_, err = config.ToRueidisOptions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse Redis CA certificate")
}

func TestTLSMutualTLSNoTLSURL(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1"
	config.TLSClientCertPath = "/path/to/cert.pem"
	config.TLSClientKeyPath = "/path/to/key.pem"

	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Nil(t, options.TLSConfig)
}

func TestAuth(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://user:pass@localhost:6379/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Equal(t, "user", options.Username)
	assert.Equal(t, "pass", options.Password)
}

func TestCluster(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1,redis://localhost:6389/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.True(t, config.IsCluster())
	assert.False(t, config.IsSentinel())

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.Equal(t, []string{"localhost:6379", "localhost:6389"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:6379", "localhost:6389"}, options.InitAddress)
	assert.True(t, options.ShuffleInit)
}

func TestClusterShortSyntax(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1,localhost:6389/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.True(t, config.IsCluster())
	assert.False(t, config.IsSentinel())

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.Equal(t, []string{"localhost:6379", "localhost:6389"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:6379", "localhost:6389"}, options.InitAddress)
	assert.True(t, options.ShuffleInit)
}

func TestSentinel(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://master-name"
	config.Sentinels = "user:pass@localhost:1234,localhost:1235"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.False(t, config.IsCluster())
	assert.True(t, config.IsSentinel())

	assert.Equal(t, "master-name", config.Hostname())
	assert.Equal(t, []string{"localhost:1234", "localhost:1235"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:1234", "localhost:1235"}, options.InitAddress)
	assert.Equal(t, "user", options.Username)
	assert.Equal(t, "pass", options.Password)
}

func TestSentinelImplicitFormat(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://user:pass@localhost:1234?master_set=master-name"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.False(t, config.IsCluster())
	assert.True(t, config.IsSentinel())

	assert.Equal(t, "master-name", config.Hostname())
	assert.Equal(t, []string{"localhost:1234"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:1234"}, options.InitAddress)
	assert.Equal(t, "user", options.Username)
	assert.Equal(t, "pass", options.Password)
}

func TestDefaultScheme(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "localhost"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.Equal(t, []string{"localhost:6379"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:6379"}, options.InitAddress)
}

func TestInvalidURL(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "invalid://"
	_, err := config.ToRueidisOptions()
	require.Error(t, err)
}

func TestRedisConfig__ToToml(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://example.com:6379"
	config.Sentinels = "sentinel1:26379,sentinel2:26379"
	config.SentinelDiscoveryInterval = 60
	config.KeepalivePingInterval = 45
	config.TLSVerify = true
	config.TLSCACertPath = "/path/to/ca.pem"
	config.TLSClientCertPath = "/path/to/cert.pem"
	config.TLSClientKeyPath = "/path/to/key.pem"
	config.MaxReconnectAttempts = 10
	config.DisableCache = true

	tomlStr := config.ToToml()

	assert.Contains(t, tomlStr, "url = \"redis://example.com:6379\"")
	assert.Contains(t, tomlStr, "sentinels = \"sentinel1:26379,sentinel2:26379\"")
	assert.Contains(t, tomlStr, "sentinel_discovery_interval = 60")
	assert.Contains(t, tomlStr, "keepalive_ping_interval = 45")
	assert.Contains(t, tomlStr, "tls_verify = true")
	assert.Contains(t, tomlStr, "tls_ca_cert_path = \"/path/to/ca.pem\"")
	assert.Contains(t, tomlStr, "tls_client_cert_path = \"/path/to/cert.pem\"")
	assert.Contains(t, tomlStr, "tls_client_key_path = \"/path/to/key.pem\"")
	assert.Contains(t, tomlStr, "max_reconnect_attempts = 10")
	assert.Contains(t, tomlStr, "disable_cache = true")

	// Round-trip test
	config2 := NewRedisConfig()

	_, err := toml.Decode(tomlStr, &config2)
	require.NoError(t, err)

	assert.Equal(t, config, config2) // nolint:govet
}
