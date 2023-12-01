package integration_tests

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexodus-io/nexodus/internal/nexodus"
	"golang.org/x/oauth2"

	"github.com/Nerzal/gocloak/v13"
	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

var providerType testcontainers.ProviderType
var defaultNetwork string
var hostDNSName string
var ipamDriver string

const (
	nexctl = "../dist/nexctl"
)

type ipFamily string

const (
	inetV4 ipFamily = "-4"
	inetV6 ipFamily = "-6"
)

func (f ipFamily) String() string {
	return string(f)
}

type v6Enable bool

const (
	disableV6 v6Enable = false
	enableV6  v6Enable = true
)

func init() {
	if os.Getenv("NEXODUS_TEST_PODMAN") != "" {
		fmt.Println("Using podman")
		providerType = testcontainers.ProviderPodman
		defaultNetwork = "podman"
		//defaultNetwork = "nexodus"
		ipamDriver = "host-local"
		hostDNSName = "10.88.0.1"
	} else {
		fmt.Println("Using docker")
		providerType = testcontainers.ProviderDocker
		defaultNetwork = "bridge"
		//defaultNetwork = "nexodus"
		ipamDriver = "default"
		hostDNSName = dockerKindGatewayIP()
	}
	_ = nexodus.CreateDirectory("tmp")
}

func dockerKindGatewayIP() string {
	ip := nexodus.LocalIPv4Address()
	if ip == nil {
		panic("local ip address not found")
	}
	return ip.String()
}

func findParentDirWhere(directory string, conditional func(fileName string) bool) (string, error) {
	for {
		if conditional(directory) {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("not found")
		}
		directory = parent
	}
}

func findCertsDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir, err = findParentDirWhere(dir, func(dir string) bool {
		file := filepath.Join(dir, ".certs")
		f, err := os.Stat(file)
		if err == nil && f.IsDir() {
			return true
		}
		return false
	})
	if err != nil {
		return "", fmt.Errorf("certs directory error: %w, try running 'make cacerts'", err)
	}
	return filepath.Join(dir, ".certs"), nil
}

type FnConsumer struct {
	Apply func(l testcontainers.Log)
}

func (c FnConsumer) Accept(l testcontainers.Log) {
	c.Apply(l)
}

func getContainerIfaceIP(ctx context.Context, family ipFamily, dev string, ctr testcontainers.Container) (string, error) {
	var ip string
	err := backoff.Retry(func() error {
		var err error
		ip, err = getContainerIfaceIPNoRetry(ctx, family, dev, ctr)
		return err
	}, backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx))
	return ip, err
}

func getContainerIfaceIPNoRetry(ctx context.Context, family ipFamily, dev string, ctr testcontainers.Container) (string, error) {
	code, outputRaw, err := ctr.Exec(
		ctx,
		[]string{"ip", "--brief", family.String(), "address", "show", dev},
	)
	if err != nil {
		return "", err
	}
	output, err := io.ReadAll(outputRaw)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("exit code %d. output: %s", code, string(output))
	}
	fields := strings.Fields(string(output))
	if len(fields) < 3 {
		return "", fmt.Errorf("Interface %s has no IP address", dev)
	}
	cidr := fields[2]
	if err != nil {
		return "", err
	}
	ipAddr, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}
	return ipAddr.String(), nil
}

func getTunnelIP(ctx context.Context, helper *Helper, family ipFamily, ctr testcontainers.Container) (string, error) {
	args := []string{"nexctl", "nexd", "get", "tunnelip"}
	if family == inetV6 {
		args = append(args, "--ipv6")
	}
	tunnelIP, err := helper.containerExec(ctx, ctr, args)
	return strings.TrimSpace(tunnelIP), err
}

func ping(ctx context.Context, ctr testcontainers.Container, family ipFamily, address string) error {
	err := backoff.Retry(func() error {
		code, outputRaw, err := ctr.Exec(
			ctx,
			[]string{"ping", family.String(), "-c", "2", "-w", "2", address},
		)
		if err != nil {
			return err
		}
		output, err := io.ReadAll(outputRaw)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("exit code %d. stdout: %s", code, string(output))
		}
		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx))
	return err
}

// pingWithoutRetry one shot ping for negative testing
func pingWithoutRetry(ctx context.Context, ctr testcontainers.Container, family ipFamily, address string) error {
	code, outputRaw, err := ctr.Exec(
		ctx,
		[]string{"ping", family.String(), "-c", "2", "-w", "2", address},
	)
	if err != nil {
		return err
	}
	output, err := io.ReadAll(outputRaw)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("exit code %d. stdout: %s", code, string(output))
	}

	return err
}

// LineCount for validating peer counts
func LineCount(s string) (int, error) {
	r := bufio.NewReader(strings.NewReader(s))
	count := 0
	for {
		_, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		count++
	}

	return count, nil
}

func NetworkAddr(n *net.IPNet) net.IP {
	network := net.ParseIP("0.0.0.0").To4()
	for i := 0; i < len(n.IP); i++ {
		network[i] = n.IP[i] & n.Mask[i]
	}
	return network
}

// NewTLSConfig creates a *tls.Config configured to trust the .certs/rootCA.pem
func NewTLSConfig(t *testing.T) *tls.Config {
	require := require.New(t)
	dir, err := findCertsDir()
	require.NoError(err)
	caCert, err := os.ReadFile(filepath.Join(dir, "rootCA.pem"))
	require.NoError(err)
	caCertPool, err := x509.SystemCertPool()
	require.NoError(err)
	caCertPool.AppendCertsFromPEM(caCert)

	// #nosec G402
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}
	return tlsConfig
}

func createNewUserWithName(ctx context.Context, name string, password string) (string, func(), error) {

	keycloak := gocloak.NewClient("https://auth.try.nexodus.127.0.0.1.nip.io")
	// #nosec G402
	keycloak.RestyClient().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	id, _ := uuid.NewUUID()
	userName := name + id.String()

	token, err := keycloak.LoginAdmin(ctx, "admin", "floofykittens", "master")
	if err != nil {
		return "", nil, fmt.Errorf("admin login to keycloak failed: %w", err)
	}

	userid, err := keycloak.CreateUser(ctx, token.AccessToken, "nexodus", gocloak.User{
		Username:      gocloak.StringP(userName),
		Enabled:       gocloak.BoolP(true),
		EmailVerified: gocloak.BoolP(true),
		FirstName:     gocloak.StringP("Test"),
		LastName:      gocloak.StringP(name),
		Email:         gocloak.StringP(userName + "@redhat.com"),
		Attributes:    &map[string][]string{"origin": {"nexodus-cli"}},
	})
	if err != nil {
		return "", nil, fmt.Errorf("user create failed: %w", err)
	}
	deleteUser := func() {
		// use a new context as the original is likely canceled now.
		ctx := context.Background()
		token, err := keycloak.LoginAdmin(ctx, "admin", "floofykittens", "master")
		if err == nil {
			_ = keycloak.DeleteUser(ctx, token.AccessToken, "nexodus", userid)
		}
	}

	err = keycloak.SetPassword(ctx, token.AccessToken, userid, "nexodus", password, false)
	if err != nil {
		deleteUser()
		return "", nil, fmt.Errorf("user set password failed: %w", err)
	}
	return userName, deleteUser, nil
}

func getOauth2Token(ctx context.Context, userid, password string) (*oauth2.Token, error) {
	keycloak := gocloak.NewClient("https://auth.try.nexodus.127.0.0.1.nip.io")
	// #nosec G402
	keycloak.RestyClient().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	jwt, err := keycloak.GetToken(ctx, "nexodus",
		gocloak.TokenOptions{
			ClientID:     gocloak.StringP("nexodus-cli"),
			ClientSecret: gocloak.StringP(""),
			GrantType:    gocloak.StringP("password"),
			Username:     &userid,
			Password:     &password,
			Scope:        gocloak.StringP("openid profile email read:organizations write:organizations read:users write:users read:devices write:devices"),
		})
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken:  jwt.AccessToken,
		TokenType:    jwt.TokenType,
		RefreshToken: jwt.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(jwt.ExpiresIn) * time.Second),
	}, nil
}

// filterAndTrimLines filter out keyworks and trim output
func filterAndTrimLines(lines []string, excludeWord, trimAfter string) string {
	filteredLines := make([]string, 0, len(lines))

	for _, line := range lines {
		// Check if the line contains the excludeWord. If not, process it
		if strings.Contains(line, excludeWord) {
			continue
		}
		// Check if the line contains the trimAfter word
		if idx := strings.Index(line, trimAfter); idx != -1 {
			// If it does, trim the line to remove everything after trimAfter
			line = strings.TrimSpace(line[:idx])
		}
		// Append the (potentially trimmed) line to the filteredLines slice
		filteredLines = append(filteredLines, line)
	}

	// Join the filtered lines into a single string, with lines separated by newline characters
	return strings.Join(filteredLines, "\n")
}
