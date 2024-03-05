package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
	"gorm.io/gorm"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CertificateKeyPair struct {
	Certificate    *x509.Certificate
	Key            any
	CertificatePem []byte
}

func ParseCertificateKeyPair(certPEMBlock, keyPEMBlock []byte) (result CertificateKeyPair, err error) {
	result.CertificatePem = certPEMBlock
	certDERBlock, _ := pem.Decode(certPEMBlock)
	if certDERBlock == nil || certDERBlock.Type != "CERTIFICATE" {
		return CertificateKeyPair{}, fmt.Errorf("cert pem block type is not CERTIFICATE")
	}
	result.Certificate, err = x509.ParseCertificate(certDERBlock.Bytes)
	if err != nil {
		return CertificateKeyPair{}, fmt.Errorf("failed to parse the certificate: %w", err)
	}

	keyDERBlock, _ := pem.Decode(keyPEMBlock)
	if keyDERBlock == nil || !strings.HasSuffix(keyDERBlock.Type, "PRIVATE KEY") {
		return CertificateKeyPair{}, fmt.Errorf("key pem block type is not PRIVATE KEY: %s", keyDERBlock.Type)
	}
	result.Key, err = x509.ParsePKCS1PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		return CertificateKeyPair{}, fmt.Errorf("failed to parse the PKCS1 key: %w", err)
	}

	return result, nil
}

func newSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
}

// SignCSR signs a certificate signing request
// @Summary      Signs a certificate signing request
// @Description  Signs a certificate signing request
// @Id           SignCSR
// @Tags         CA
// @Accept       json
// @Produce      json
// @Param        CertificateSigningRequest  body  models.CertificateSigningRequest  true  "Certificate signing request"
// @Success      201  {object}  models.CertificateSigningResponse
// @Failure      400  {object}  models.ValidationError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/ca/sign [post]
func (api *API) SignCSR(c *gin.Context) {
	_, span := tracer.Start(c.Request.Context(), "SignCSR")
	defer span.End()

	if !api.FlagCheck(c, "ca") {
		return
	}

	tx := api.db.WithContext(c)
	tokenClaims, apiResponseError := NxodusClaims(c, tx)
	if apiResponseError != nil {
		c.JSON(apiResponseError.Status, apiResponseError.Body)
		return
	}

	var siteId string
	if tokenClaims != nil {
		switch tokenClaims.Scope {
		case "device-token":
			siteId = tokenClaims.ID
		default:
			c.JSON(http.StatusForbidden, models.NewApiError(errors.New("a device token is required")))
			return
		}
	} else {
		c.JSON(http.StatusForbidden, models.NewApiError(errors.New("a device token is required")))
		return
	}

	//if certURI == nil {
	//	c.JSON(http.StatusForbidden, models.NewApiError(errors.New("cannot determine certificate url")))
	//	return
	//}

	var request models.CertificateSigningRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	if len(request.Request) == 0 {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("request"))
		return
	}

	csrPEM, _ := pem.Decode([]byte(request.Request))
	if csrPEM == nil {
		c.JSON(http.StatusBadRequest, models.NewFieldValidationError("request", "unexpected content"))
		return
	}
	if csrPEM.Type != "CERTIFICATE REQUEST" && csrPEM.Type != "NEW CERTIFICATE REQUEST" {
		c.JSON(http.StatusBadRequest, models.NewFieldValidationError("pem", "unexpected type, expected CERTIFICATE REQUEST, got: "+csrPEM.Type))
		return
	}
	csr, err := x509.ParseCertificateRequest(csrPEM.Bytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewFieldValidationError("request", "parse failed: "+err.Error()))
		return
	}
	err = csr.CheckSignature()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewFieldValidationError("request", "invalid signature: "+err.Error()))
		return
	}

	expiration := time.Now().AddDate(5, 0, 0)
	if request.Duration != nil {
		expiration = time.Now().Add(request.Duration.Duration)
	}

	serialNumber, err := newSerialNumber()
	if err != nil {
		api.SendInternalServerError(c, fmt.Errorf("failed to generate certificate serial number: %w", err))
		return
	}

	ku, eku, err := KeyUsagesForCertificateOrCertificateRequest(request.IsCA, request.Usages...)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewFieldValidationError("usages", err.Error()))
		return
	}

	// get the site and the ServiceNetwork CA
	site := models.Site{}
	sendServiceNetworkNotify := false
	err = api.transaction(c, func(tx *gorm.DB) error {

		if res := tx.Joins("ServiceNetwork").First(&site, "sites.id = ?", siteId); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("site"))
			}
			return err
		}

		// allocate the ServiceNetwork CA on demand
		if len(site.ServiceNetwork.CaCertificates) == 0 {
			cert, key, err := api.CreateServiceNetworkCertKeyPair(site.ServiceNetwork)
			if err != nil {
				return err
			}

			site.ServiceNetwork.CaCertificates = []string{cert}
			site.ServiceNetwork.CaKey = key
			if res := tx.Select("ca_certificates", "ca_key").
				Where("ca_key is NULL").
				Updates(site.ServiceNetwork); res.Error != nil {
				return err
			}
			sendServiceNetworkNotify = true
		}

		return nil
	})
	if err != nil {
		var apiResponseError *ApiResponseError
		if errors.As(err, &apiResponseError) {
			c.JSON(apiResponseError.Status, apiResponseError.Body)
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}
	if sendServiceNetworkNotify {
		api.signalBus.Notify(fmt.Sprintf("/service-network=%s", site.ServiceNetworkID.String()))
	}

	template := &x509.Certificate{
		SerialNumber:    serialNumber,
		Subject:         csr.Subject,
		ExtraExtensions: csr.Extensions,
		NotBefore:       time.Now(), NotAfter: expiration,
		DNSNames: csr.DNSNames,
		// this CA only enforces the spiffe URI in the CSR for now
		URIs: []*url.URL{
			{
				Scheme: "spiffe",
				Host:   api.URLParsed.Host,
				Path:   fmt.Sprintf("/o/%s/n/%s/s/%s", site.OrganizationID, site.ServiceNetworkID, site.ID),
			},
		},
		KeyUsage:    ku,
		ExtKeyUsage: eku,
	}

	if len(template.DNSNames) == 0 {
		template.DNSNames = append(template.DNSNames, csr.Subject.CommonName)
	}

	if len(csr.EmailAddresses) > 0 {
		template.ExtKeyUsage = append(template.ExtKeyUsage, x509.ExtKeyUsageEmailProtection)
	}

	serviceNetworkCaKeyPair, err := ParseCertificateKeyPair([]byte(site.ServiceNetwork.CaCertificates[0]), []byte(site.ServiceNetwork.CaKey))
	if err != nil {
		api.SendInternalServerError(c, err)
		return
	}

	cert, err := x509.CreateCertificate(rand.Reader, template, serviceNetworkCaKeyPair.Certificate, csr.PublicKey, serviceNetworkCaKeyPair.Key)
	if err != nil {
		api.SendInternalServerError(c, fmt.Errorf("failed to generate certificate: %w", err))
		return
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})

	err = VerifyCertificate(certPEM, append(serviceNetworkCaKeyPair.CertificatePem, api.caKeyPair.CertificatePem...), template.ExtKeyUsage...)
	if err != nil {
		api.SendInternalServerError(c, fmt.Errorf("failed to verify generated certificate: %w", err))
		return
	}

	c.JSON(http.StatusOK, models.CertificateSigningResponse{
		Certificate: string(certPEM),
		CA:          string(append(serviceNetworkCaKeyPair.CertificatePem, api.caKeyPair.CertificatePem...)),
		//Certificate: string(append(certPEM, serviceNetworkCaKeyPair.CertificatePem...)),
		//CA:          string(api.caKeyPair.CertificatePem),
	})

}

func (api *API) CreateServiceNetworkCertKeyPair(serviceNetwork *models.ServiceNetwork) (string, string, error) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	serviceNetworksURI, err := url.Parse(fmt.Sprintf("%s/api/service-networks/%s", api.URL, serviceNetwork.ID))
	if err != nil {
		return "", "", err
	}

	serialNumber, err := newSerialNumber()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate certificate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: serviceNetworksURI.String(),
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(5, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		IsCA:                  true,
		URIs: []*url.URL{
			{
				Scheme: "spiffe",
				Host:   api.URLParsed.Host,
				Path:   fmt.Sprintf("/o/%s/n/%s", serviceNetwork.OrganizationID, serviceNetwork.ID),
			},
		},
	}

	cert, err := x509.CreateCertificate(rand.Reader, &template, api.caKeyPair.Certificate, privateKey.Public(), api.caKeyPair.Key)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate certificate: %w", err)
	}

	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}))

	err = VerifyCertificate([]byte(certPEM), []byte(api.caKeyPair.CertificatePem), x509.ExtKeyUsageAny)
	if err != nil {
		return "", "", fmt.Errorf("failed to verify serviceNetwork certificate: %w", err)
	}

	return certPEM, keyPEM, nil
}

func KeyUsagesForCertificateOrCertificateRequest(isCA bool, usages ...models.KeyUsage) (ku x509.KeyUsage, eku []x509.ExtKeyUsage, err error) {
	var unk []models.KeyUsage
	if isCA {
		ku |= x509.KeyUsageCertSign
	}
	if len(usages) == 0 {
		usages = []models.KeyUsage{models.UsageDigitalSignature, models.UsageKeyEncipherment}
	}
	for _, u := range usages {
		if kuse, ok := keyUsages[u]; ok {
			ku |= kuse
		} else if ekuse, ok := extKeyUsages[u]; ok {
			eku = append(eku, ekuse)
		} else {
			unk = append(unk, u)
		}
	}
	if len(unk) > 0 {
		err = fmt.Errorf("unknown key usages: %v", unk)
	}
	return
}

var keyUsages = map[models.KeyUsage]x509.KeyUsage{
	models.UsageSigning:           x509.KeyUsageDigitalSignature,
	models.UsageDigitalSignature:  x509.KeyUsageDigitalSignature,
	models.UsageContentCommitment: x509.KeyUsageContentCommitment,
	models.UsageKeyEncipherment:   x509.KeyUsageKeyEncipherment,
	models.UsageKeyAgreement:      x509.KeyUsageKeyAgreement,
	models.UsageDataEncipherment:  x509.KeyUsageDataEncipherment,
	models.UsageCertSign:          x509.KeyUsageCertSign,
	models.UsageCRLSign:           x509.KeyUsageCRLSign,
	models.UsageEncipherOnly:      x509.KeyUsageEncipherOnly,
	models.UsageDecipherOnly:      x509.KeyUsageDecipherOnly,
}
var extKeyUsages = map[models.KeyUsage]x509.ExtKeyUsage{
	models.UsageAny:             x509.ExtKeyUsageAny,
	models.UsageServerAuth:      x509.ExtKeyUsageServerAuth,
	models.UsageClientAuth:      x509.ExtKeyUsageClientAuth,
	models.UsageCodeSigning:     x509.ExtKeyUsageCodeSigning,
	models.UsageEmailProtection: x509.ExtKeyUsageEmailProtection,
	models.UsageSMIME:           x509.ExtKeyUsageEmailProtection,
	models.UsageIPsecEndSystem:  x509.ExtKeyUsageIPSECEndSystem,
	models.UsageIPsecTunnel:     x509.ExtKeyUsageIPSECTunnel,
	models.UsageIPsecUser:       x509.ExtKeyUsageIPSECUser,
	models.UsageTimestamping:    x509.ExtKeyUsageTimeStamping,
	models.UsageOCSPSigning:     x509.ExtKeyUsageOCSPSigning,
	models.UsageMicrosoftSGC:    x509.ExtKeyUsageMicrosoftServerGatedCrypto,
	models.UsageNetscapeSGC:     x509.ExtKeyUsageNetscapeServerGatedCrypto,
}

func VerifyCertificate(certPEMBlock []byte, caCertPEMBlock []byte, keyUsages ...x509.ExtKeyUsage) error {

	// Decode the PEM block to get the DER-encoded certificate
	pemBlock, _ := pem.Decode(certPEMBlock)
	if pemBlock == nil {
		return fmt.Errorf("error decoding PEM block")
	}

	// Parse the DER-encoded certificate
	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing certificate: %w", err)
	}

	// Create a pool of trusted CA certificates
	roots, intermediates, err := NewCertPools(caCertPEMBlock)
	if err != nil {
		return fmt.Errorf("error parsing ca certificates: %w", err)
	}

	// Verify that the certificate was signed by the CA
	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     keyUsages,
	}

	if _, err := cert.Verify(opts); err != nil {
		return err
	}
	return nil
}

// NewCertPools creates x509 cert pools from the given PEM bytes.
func NewCertPools(pemBytes []byte) (*x509.CertPool, *x509.CertPool, error) {
	certs := []*x509.Certificate{}
	for {
		var block *pem.Block
		block, pemBytes = pem.Decode(pemBytes)
		if block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, err
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("no certificates found")
	}
	roots := x509.NewCertPool()
	intermediates := x509.NewCertPool()
	for i, cert := range certs {
		if i == len(certs)-1 {
			roots.AddCert(cert)
		} else {
			intermediates.AddCert(cert)
		}
	}
	return roots, intermediates, nil
}
