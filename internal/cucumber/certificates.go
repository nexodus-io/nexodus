package cucumber

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/cucumber/godog"
	"github.com/ghodss/yaml"
)

func init() {
	StepModules = append(StepModules, func(ctx *godog.ScenarioContext, s *TestScenario) {
		ctx.Step(`^I generate a new CSR and key as \${([^"]*)}\/\${([^"]*)} using:$`, s.iGenerateANewCSRAndKeyAsUsing)
	})
	PipeFunctions["parse_x509_cert"] = parseX509Certificate
}
func (s *TestScenario) iGenerateANewCSRAndKeyAsUsing(csr, key, settings string) error {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	template := x509.CertificateRequest{}
	err = yaml.Unmarshal([]byte(settings), &template)
	if err != nil {
		return fmt.Errorf("invalid x509.CertificateRequest json: %w", err)
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, privateKey)
	if err != nil {
		return fmt.Errorf("CreateCertificateRequest failure: %w", err)
	}
	s.Variables[csr] = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes}))
	s.Variables[key] = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))

	return nil
}

func parseX509Certificate(value any, err error) (any, error) {
	if err != nil {
		return value, err
	}
	certData, err := ToString(value, "certificate", JsonEncoding)
	if err != nil {
		return value, err
	}

	certDERBlock, _ := pem.Decode([]byte(certData))
	if certDERBlock == nil || certDERBlock.Type != "CERTIFICATE" {
		return value, fmt.Errorf("pem block type is not CERTIFICATE")
	}
	certificate, err := x509.ParseCertificate(certDERBlock.Bytes)
	if err != nil {
		return value, fmt.Errorf("pem block type is not CERTIFICATE")
	}

	return certificate, err
}
